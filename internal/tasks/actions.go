package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"barbtils/internal/database"

	"github.com/google/uuid"
)

// NewTaskInput describes a task to create. Only Name is required; every other
// field falls back to the column default in the schema, so creating a task is
// just `tasks new "leetcode practice"`.
type NewTaskInput struct {
	Name     string
	Type     database.TasksTypes
	Priority database.TasksPriorities
	Category []database.TasksCategories
	Tags     []string
}

func (in NewTaskInput) params() (database.CreateTaskParams, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return database.CreateTaskParams{}, ErrNameEmpty
	}
	p := database.CreateTaskParams{
		Name:     name,
		Type:     in.Type,
		Priority: in.Priority,
		Category: in.Category,
		Tags:     in.Tags,
	}
	if p.Type == "" {
		p.Type = database.TasksTypesOneTime
	}
	if p.Priority == "" {
		p.Priority = database.TasksPrioritiesUnknown
	}
	if len(p.Category) == 0 {
		p.Category = []database.TasksCategories{database.TasksCategoriesUnknown}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	return p, nil
}

// ActionResult is what a state-changing operation reports back.
type ActionResult struct {
	Task    database.Task
	Session *database.TasksSession // the session opened or closed, when there was one
	Total   time.Duration          // summed task time after the action
}

// CreateTask creates a task with no session. It does not start a timer: a task
// must exist before it can be started, so a mistyped name is an error rather
// than a new junk task.
func (s *Store) CreateTask(ctx context.Context, in NewTaskInput) (database.Task, error) {
	p, err := in.params()
	if err != nil {
		return database.Task{}, err
	}
	t, err := s.q.CreateTask(ctx, p)
	if constraintViolation(err, "tasks_name_lower_key") {
		return database.Task{}, fmt.Errorf("%w: %q", ErrNameTaken, p.Name)
	}
	if err != nil {
		return database.Task{}, fmt.Errorf("creating task: %w", err)
	}
	return t, nil
}

// CreateTaskAndStart creates a task and opens its first session atomically.
func (s *Store) CreateTaskAndStart(ctx context.Context, in NewTaskInput) (ActionResult, error) {
	p, err := in.params()
	if err != nil {
		return ActionResult{}, err
	}
	var res ActionResult
	err = s.withTx(ctx, func(q *database.Queries) error {
		t, err := q.CreateTask(ctx, p)
		if constraintViolation(err, "tasks_name_lower_key") {
			return fmt.Errorf("%w: %q", ErrNameTaken, p.Name)
		}
		if err != nil {
			return fmt.Errorf("creating task: %w", err)
		}
		sess, err := q.OpenSession(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("opening session: %w", err)
		}
		t, err = q.SetTaskStatus(ctx, database.SetTaskStatusParams{
			ID: t.ID, Status: database.TasksStatusesInProgress,
		})
		if err != nil {
			return err
		}
		res = ActionResult{Task: t, Session: &sess}
		return nil
	})
	return res, err
}

// Start opens a new session and marks the task In Progress. It is also how a
// paused task resumes: resuming is simply opening the next session.
func (s *Store) Start(ctx context.Context, taskID uuid.UUID) (ActionResult, error) {
	var res ActionResult
	err := s.withTx(ctx, func(q *database.Queries) error {
		if _, err := q.GetOpenSessionForTask(ctx, taskID); err == nil {
			return ErrAlreadyRunning
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		sess, err := q.OpenSession(ctx, taskID)
		if constraintViolation(err, "tasks_sessions_one_open_per_task") {
			return ErrAlreadyRunning // lost race against another process
		}
		if err != nil {
			return fmt.Errorf("opening session: %w", err)
		}
		t, err := q.SetTaskStatus(ctx, database.SetTaskStatusParams{
			ID: taskID, Status: database.TasksStatusesInProgress,
		})
		if err != nil {
			return err
		}
		total, err := totalFor(ctx, q, taskID)
		if err != nil {
			return err
		}
		res = ActionResult{Task: t, Session: &sess, Total: total}
		return nil
	})
	return res, err
}

// Pause closes the open session and marks the task Paused. The task is NOT
// completed and stays resumable; ending the session is what makes the elapsed
// time of each stretch of work recordable, so the overall total is the sum of
// them rather than a wall-clock span that would count idle gaps.
func (s *Store) Pause(ctx context.Context, taskID uuid.UUID) (ActionResult, error) {
	var res ActionResult
	err := s.withTx(ctx, func(q *database.Queries) error {
		sess, err := q.CloseOpenSessionForTask(ctx, taskID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotRunning
		} else if err != nil {
			return fmt.Errorf("closing session: %w", err)
		}
		t, err := q.SetTaskStatus(ctx, database.SetTaskStatusParams{
			ID: taskID, Status: database.TasksStatusesPaused,
		})
		if err != nil {
			return err
		}
		total, err := totalFor(ctx, q, taskID)
		if err != nil {
			return err
		}
		res = ActionResult{Task: t, Session: &sess, Total: total}
		return nil
	})
	return res, err
}

// Complete closes any open session and marks the task Completed. Refused for a
// recurring task, which cycles indefinitely and is retired with Archive.
func (s *Store) Complete(ctx context.Context, t database.Task) (ActionResult, error) {
	if t.Type == database.TasksTypesRecurring {
		return ActionResult{}, fmt.Errorf("%w: #%d %q is recurring", ErrRecurringCannotComplete, t.Seq, t.Name)
	}
	return s.closeAndSet(ctx, t.ID, database.TasksStatusesCompleted)
}

// Archive closes any open session and marks the task Archived. This is how a
// recurring task is retired.
func (s *Store) Archive(ctx context.Context, taskID uuid.UUID) (ActionResult, error) {
	return s.closeAndSet(ctx, taskID, database.TasksStatusesArchived)
}

// closeAndSet ends the open session if there is one — tolerating its absence,
// since completing an already-paused task is valid — and writes the status.
func (s *Store) closeAndSet(ctx context.Context, taskID uuid.UUID, status database.TasksStatuses) (ActionResult, error) {
	var res ActionResult
	err := s.withTx(ctx, func(q *database.Queries) error {
		var closed *database.TasksSession
		sess, err := q.CloseOpenSessionForTask(ctx, taskID)
		switch {
		case err == nil:
			closed = &sess
		case errors.Is(err, sql.ErrNoRows):
			// already paused or never started; nothing to close
		default:
			return fmt.Errorf("closing session: %w", err)
		}
		t, err := q.SetTaskStatus(ctx, database.SetTaskStatusParams{ID: taskID, Status: status})
		if err != nil {
			return err
		}
		total, err := totalFor(ctx, q, taskID)
		if err != nil {
			return err
		}
		res = ActionResult{Task: t, Session: closed, Total: total}
		return nil
	})
	return res, err
}

// Rename changes a task's name, keeping its number and its whole session history.
func (s *Store) Rename(ctx context.Context, taskID uuid.UUID, name string) (database.Task, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return database.Task{}, ErrNameEmpty
	}
	t, err := s.q.RenameTask(ctx, database.RenameTaskParams{ID: taskID, Name: name})
	if constraintViolation(err, "tasks_name_lower_key") {
		return database.Task{}, fmt.Errorf("%w: %q", ErrNameTaken, name)
	}
	return t, err
}

func (s *Store) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	n, err := s.q.DeleteTask(ctx, taskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// CloseSession ends one specific session by its own number, whatever its task's
// status is.
func (s *Store) CloseSession(ctx context.Context, seq int64) (database.TasksSession, error) {
	sess, err := s.q.CloseSessionBySeq(ctx, seq)
	if errors.Is(err, sql.ErrNoRows) {
		return database.TasksSession{}, fmt.Errorf("%w: session s%d is not open", ErrSessionNotFound, seq)
	}
	return sess, err
}

func (s *Store) DeleteSession(ctx context.Context, seq int64) error {
	n, err := s.q.DeleteSessionBySeq(ctx, seq)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: s%d", ErrSessionNotFound, seq)
	}
	return nil
}

// ---- reads ----

func (s *Store) ListTasks(ctx context.Context, f Filter) ([]TaskRow, error) {
	rows, err := s.q.ListTasks(ctx, f.params())
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	out := make([]TaskRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, taskRowFrom(r))
	}
	return out, nil
}

// GetTaskRow returns one task with its timing, by short number.
func (s *Store) GetTaskRow(ctx context.Context, seq int64) (TaskRow, error) {
	rows, err := s.ListTasks(ctx, Filter{Seq: &seq})
	if err != nil {
		return TaskRow{}, err
	}
	if len(rows) == 0 {
		return TaskRow{}, fmt.Errorf("%w: no task #%d", ErrTaskNotFound, seq)
	}
	return rows[0], nil
}

// Sessions lists a task's sessions. Archived ones are hidden unless asked for,
// since a long-running recurring task accumulates many of them.
func (s *Store) Sessions(ctx context.Context, taskID uuid.UUID, includeArchived bool) ([]SessionRow, error) {
	arg := database.ListSessionsForTaskParams{TaskID: taskID}
	if includeArchived {
		arg.IncludeArchived = nullBool(true)
	}
	rows, err := s.q.ListSessionsForTask(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	out := make([]SessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, sessionRowFrom(r))
	}
	return out, nil
}

// ArchiveSessions banks every finished session of a task, resetting its running
// total to zero while keeping the history. An open session is left running, so a
// timer spanning the rollover keeps counting into the new period.
func (s *Store) ArchiveSessions(ctx context.Context, taskID uuid.UUID) ([]SessionRow, error) {
	rows, err := s.q.ArchiveTaskSessions(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("archiving sessions: %w", err)
	}
	out := make([]SessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, SessionOf(r))
	}
	return out, nil
}

// UnarchiveSessions undoes a rollover, returning banked time to the total.
func (s *Store) UnarchiveSessions(ctx context.Context, taskID uuid.UUID) ([]SessionRow, error) {
	rows, err := s.q.UnarchiveTaskSessions(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("unarchiving sessions: %w", err)
	}
	out := make([]SessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, SessionOf(r))
	}
	return out, nil
}

// ArchiveSession banks one finished session.
func (s *Store) ArchiveSession(ctx context.Context, seq int64) (SessionRow, error) {
	r, err := s.q.ArchiveSessionBySeq(ctx, seq)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRow{}, fmt.Errorf("%w: s%d is either still running or already archived", ErrSessionNotFound, seq)
	}
	if err != nil {
		return SessionRow{}, err
	}
	return SessionOf(r), nil
}

// UnarchiveSession returns one banked session to the total.
func (s *Store) UnarchiveSession(ctx context.Context, seq int64) (SessionRow, error) {
	r, err := s.q.UnarchiveSessionBySeq(ctx, seq)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRow{}, fmt.Errorf("%w: s%d is not archived", ErrSessionNotFound, seq)
	}
	if err != nil {
		return SessionRow{}, err
	}
	return SessionOf(r), nil
}

// TotalDuration is the authoritative server-side total for a task's current
// period; archived time is excluded.
func (s *Store) TotalDuration(ctx context.Context, taskID uuid.UUID) (time.Duration, error) {
	return totalFor(ctx, s.q, taskID)
}

func totalFor(ctx context.Context, q *database.Queries, taskID uuid.UUID) (time.Duration, error) {
	d, err := q.TaskDuration(ctx, taskID)
	if err != nil {
		return 0, err
	}
	return secs(d.ClosedSeconds + d.OpenSeconds), nil
}

func (s *Store) TruncateAll(ctx context.Context) error {
	return s.q.TruncateAllTaskData(ctx)
}

func (s *Store) Doctor(ctx context.Context) ([]database.FindInconsistentTasksRow, error) {
	return s.q.FindInconsistentTasks(ctx)
}

// MetaPatch changes a task's configuration. A nil field is left alone, so the
// same patch type serves a CLI flag that was not passed and a TUI field that
// was not touched.
type MetaPatch struct {
	Type     *database.TasksTypes
	Priority *database.TasksPriorities
	Category []database.TasksCategories // nil = unchanged; empty = reset to {Unknown}
	Tags     []string                   // nil = unchanged; empty = clear
}

func (p MetaPatch) IsEmpty() bool {
	return p.Type == nil && p.Priority == nil && p.Category == nil && p.Tags == nil
}

// UpdateMeta applies a patch to a task's type, priority, categories and tags.
func (s *Store) UpdateMeta(ctx context.Context, taskID uuid.UUID, p MetaPatch) (database.Task, error) {
	if p.IsEmpty() {
		return database.Task{}, errors.New("nothing to change")
	}
	arg := database.UpdateTaskMetaParams{ID: taskID}
	if p.Type != nil {
		arg.Type = database.NullTasksTypes{TasksTypes: *p.Type, Valid: true}
	}
	if p.Priority != nil {
		arg.Priority = database.NullTasksPriorities{TasksPriorities: *p.Priority, Valid: true}
	}
	if p.Category != nil {
		// The column is NOT NULL with a {Unknown} default; an explicit clear
		// means "no category", which the schema spells as {Unknown}.
		if len(p.Category) == 0 {
			arg.Category = []database.TasksCategories{database.TasksCategoriesUnknown}
		} else {
			arg.Category = p.Category
		}
	}
	if p.Tags != nil {
		arg.Tags = p.Tags
		if len(arg.Tags) == 0 {
			arg.Tags = []string{}
		}
	}

	t, err := s.q.UpdateTaskMeta(ctx, arg)
	if err != nil {
		return database.Task{}, fmt.Errorf("updating task: %w", err)
	}
	return t, nil
}
