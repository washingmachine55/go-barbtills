package tasks

import (
	"time"

	"barbtils/internal/database"

	"github.com/google/uuid"
)

// TaskRow is a task plus its session-derived timing, as the CLI and TUI want it.
type TaskRow struct {
	ID       uuid.UUID
	Seq      int64
	Name     string
	Status   database.TasksStatuses
	Type     database.TasksTypes
	Priority database.TasksPriorities
	IsPaused bool
	Tags     []string
	Category []database.TasksCategories

	// SessionCount counts live (unarchived) sessions; ArchivedSessionCount the
	// banked ones.
	SessionCount         int64
	ArchivedSessionCount int64

	// ClosedDuration is the summed duration of finished, unarchived sessions.
	// The live portion of an open session is added by Elapsed, so a caller can
	// tick a clock without re-querying.
	ClosedDuration time.Duration

	// ArchivedDuration is time banked by earlier rollovers. It is excluded from
	// Elapsed and included in Lifetime.
	ArchivedDuration time.Duration

	OpenStartedAt  *time.Time // nil unless a session is currently open
	OpenSessionSeq *int64
	FirstStartedAt *time.Time
	LastEndedAt    *time.Time
}

func (r TaskRow) IsRunning() bool { return r.OpenStartedAt != nil }

// Lifetime is every session ever recorded, archived ones included.
func (r TaskRow) Lifetime(now time.Time) time.Duration {
	return r.Elapsed(now) + r.ArchivedDuration
}

// HasArchive reports whether this task has been rolled over before.
func (r TaskRow) HasArchive() bool  { return r.ArchivedSessionCount > 0 }
func (r TaskRow) IsRecurring() bool { return r.Type == database.TasksTypesRecurring }

// Elapsed is the one implementation of task duration math: the sum of every
// finished session in the current period plus the live portion of the open one.
// Never a MAX(end)-MIN(start) span, which would wrongly include paused gaps,
// and never archived time, which belongs to a period already banked.
func (r TaskRow) Elapsed(now time.Time) time.Duration {
	d := r.ClosedDuration
	if r.OpenStartedAt != nil {
		if live := now.Sub(*r.OpenStartedAt); live > 0 {
			d += live
		}
	}
	return d
}

// SessionRow is one work session.
type SessionRow struct {
	ID         uuid.UUID
	Seq        int64
	TaskID     uuid.UUID
	StartTime  time.Time
	EndTime    *time.Time
	ArchivedAt *time.Time
	IsOpen     bool
	Duration   time.Duration
}

func (s SessionRow) IsArchived() bool { return s.ArchivedAt != nil }

// Filter selects which tasks ListTasks returns. A nil field means "no filter".
type Filter struct {
	Seq      *int64
	Status   *database.TasksStatuses
	NameLike *string
	OnlyOpen bool
}

func (f Filter) params() database.ListTasksParams {
	p := database.ListTasksParams{}
	if f.Seq != nil {
		p.Seq = nullInt64(*f.Seq)
	}
	if f.Status != nil {
		p.Status = database.NullTasksStatuses{TasksStatuses: *f.Status, Valid: true}
	}
	if f.NameLike != nil {
		p.NameLike = nullString(*f.NameLike)
	}
	if f.OnlyOpen {
		p.OnlyOpen = nullBool(true)
	}
	return p
}

func taskRowFrom(r database.ListTasksRow) TaskRow {
	return TaskRow{
		ID:                   r.ID,
		Seq:                  r.Seq,
		Name:                 r.Name,
		Status:               r.Status,
		Type:                 r.Type,
		Priority:             r.Priority,
		IsPaused:             r.IsPaused,
		Tags:                 r.Tags,
		Category:             r.Category,
		SessionCount:         r.SessionCount,
		ArchivedSessionCount: r.ArchivedSessionCount,
		ClosedDuration:       secs(r.ClosedSeconds),
		ArchivedDuration:     secs(r.ArchivedSeconds),
		OpenStartedAt:        timePtr(r.OpenStartedAt),
		OpenSessionSeq:       int64Ptr(r.OpenSessionSeq),
		FirstStartedAt:       timePtr(r.FirstStartedAt),
		LastEndedAt:          timePtr(r.LastEndedAt),
	}
}

func sessionRowFrom(r database.ListSessionsForTaskRow) SessionRow {
	return SessionRow{
		ID:         r.ID,
		Seq:        r.Seq,
		TaskID:     r.TaskID,
		StartTime:  r.StartTime,
		EndTime:    timePtr(r.EndTime),
		ArchivedAt: timePtr(r.ArchivedAt),
		IsOpen:     !r.EndTime.Valid,
		Duration:   secs(r.DurationSeconds),
	}
}

// SessionOf adapts a bare session row (from a mutation's RETURNING) for display.
func SessionOf(s database.TasksSession) SessionRow {
	row := SessionRow{
		ID:         s.ID,
		Seq:        s.Seq,
		TaskID:     s.TaskID,
		StartTime:  s.StartTime,
		EndTime:    timePtr(s.EndTime),
		ArchivedAt: timePtr(s.ArchivedAt),
		IsOpen:     !s.EndTime.Valid,
	}
	if s.EndTime.Valid {
		row.Duration = s.EndTime.Time.Sub(s.StartTime)
	} else {
		row.Duration = time.Since(s.StartTime)
	}
	return row
}
