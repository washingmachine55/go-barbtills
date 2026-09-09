package cmd

import (
	"time"

	"barbtils/internal/database"
	"barbtils/internal/tasks"
)

// The tasks payloads. See json.go for the stdout/stderr split every --json
// command follows.
//
// Every duration is reported twice: <name>_seconds for arithmetic, <name> as the
// same string the styled output shows, for humans reading the payload.

// ---- payload types ----

// taskJSON is a task with its timing, as `ls` and `show` report it.
type taskJSON struct {
	ID       string   `json:"id"`
	Seq      int64    `json:"seq"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Type     string   `json:"type"`
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	Category []string `json:"category"`

	Sessions         int64 `json:"sessions"`
	ArchivedSessions int64 `json:"archived_sessions"`

	TotalSeconds    int64  `json:"total_seconds"`
	Total           string `json:"total"`
	ArchivedSeconds int64  `json:"archived_seconds"`
	LifetimeSeconds int64  `json:"lifetime_seconds"`

	Running        bool       `json:"running"`
	OpenSessionSeq *int64     `json:"open_session_seq"`
	OpenStartedAt  *time.Time `json:"open_started_at"`
	FirstStartedAt *time.Time `json:"first_started_at"`
	LastEndedAt    *time.Time `json:"last_ended_at"`
}

// taskRefJSON identifies the task a mutation acted on. Deliberately without
// timing fields: a mutation reports the resulting total once, at the top level,
// rather than a second copy of it that could disagree.
type taskRefJSON struct {
	ID       string   `json:"id"`
	Seq      int64    `json:"seq"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Type     string   `json:"type"`
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	Category []string `json:"category"`
}

type sessionJSON struct {
	ID     string `json:"id"`
	Seq    int64  `json:"seq"`
	TaskID string `json:"task_id"`

	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time"`
	ArchivedAt *time.Time `json:"archived_at"`

	Open     bool `json:"open"`
	Archived bool `json:"archived"`

	DurationSeconds int64  `json:"duration_seconds"`
	Duration        string `json:"duration"`
}

// sessionTotalsJSON is the footer of a session listing.
type sessionTotalsJSON struct {
	Sessions        int    `json:"sessions"`
	TotalSeconds    int64  `json:"total_seconds"`
	Total           string `json:"total"`
	ArchivedCount   int    `json:"archived_sessions"`
	ArchivedSeconds int64  `json:"archived_seconds"`
	LifetimeSeconds int64  `json:"lifetime_seconds"`
}

// actionJSON is the shape every mutating command emits, so a script can branch
// on .action without knowing which subcommand produced the document.
type actionJSON struct {
	Action  string       `json:"action"`
	Task    *taskRefJSON `json:"task,omitempty"`
	Session *sessionJSON `json:"session,omitempty"`

	TotalSeconds *int64 `json:"total_seconds,omitempty"`
	Total        string `json:"total,omitempty"`
	PreviousName string `json:"previous_name,omitempty"`
	Running      *bool  `json:"running,omitempty"`
	Message      string `json:"message,omitempty"`

	// Bulk results, for rollover and its --undo.
	Sessions        []sessionJSON `json:"sessions,omitempty"`
	SessionCount    *int          `json:"session_count,omitempty"`
	BankedSeconds   *int64        `json:"banked_seconds,omitempty"`
	LifetimeSeconds *int64        `json:"lifetime_seconds,omitempty"`
}

// ---- conversions ----

func taskJSONOf(r tasks.TaskRow, now time.Time) taskJSON {
	elapsed := r.Elapsed(now)
	return taskJSON{
		ID:       r.ID.String(),
		Seq:      r.Seq,
		Name:     r.Name,
		Status:   string(r.Status),
		Type:     string(r.Type),
		Priority: string(r.Priority),
		Tags:     strSlice(r.Tags),
		Category: catStrings(r.Category),

		Sessions:         r.SessionCount,
		ArchivedSessions: r.ArchivedSessionCount,

		TotalSeconds:    int64(elapsed.Seconds()),
		Total:           fmtShort(elapsed),
		ArchivedSeconds: int64(r.ArchivedDuration.Seconds()),
		LifetimeSeconds: int64(r.Lifetime(now).Seconds()),

		Running:        r.IsRunning(),
		OpenSessionSeq: r.OpenSessionSeq,
		OpenStartedAt:  localTime(r.OpenStartedAt),
		FirstStartedAt: localTime(r.FirstStartedAt),
		LastEndedAt:    localTime(r.LastEndedAt),
	}
}

func taskJSONList(rows []tasks.TaskRow, now time.Time) []taskJSON {
	out := make([]taskJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, taskJSONOf(r, now))
	}
	return out
}

func taskRefJSONOf(t database.Task) taskRefJSON {
	return taskRefJSON{
		ID:       t.ID.String(),
		Seq:      t.Seq,
		Name:     t.Name,
		Status:   string(t.Status),
		Type:     string(t.Type),
		Priority: string(t.Priority),
		Tags:     strSlice(t.Tags),
		Category: catStrings(t.Category),
	}
}

func sessionJSONOf(s tasks.SessionRow) sessionJSON {
	return sessionJSON{
		ID:     s.ID.String(),
		Seq:    s.Seq,
		TaskID: s.TaskID.String(),

		StartTime:  s.StartTime.Local(),
		EndTime:    localTime(s.EndTime),
		ArchivedAt: localTime(s.ArchivedAt),

		Open:     s.IsOpen,
		Archived: s.IsArchived(),

		DurationSeconds: int64(s.Duration.Seconds()),
		Duration:        fmtShort(s.Duration),
	}
}

func sessionJSONList(rows []tasks.SessionRow) []sessionJSON {
	out := make([]sessionJSON, 0, len(rows))
	for _, s := range rows {
		out = append(out, sessionJSONOf(s))
	}
	return out
}

// sessionJSONPtr adapts the optional session an ActionResult carries.
func sessionJSONPtr(s *database.TasksSession) *sessionJSON {
	if s == nil {
		return nil
	}
	j := sessionJSONOf(tasks.SessionOf(*s))
	return &j
}

func sessionTotalsOf(rows []tasks.SessionRow) sessionTotalsJSON {
	var live, archived time.Duration
	var liveN, archivedN int
	for _, s := range rows {
		if s.IsArchived() {
			archived += s.Duration
			archivedN++
			continue
		}
		live += s.Duration
		liveN++
	}
	return sessionTotalsJSON{
		Sessions:        liveN,
		TotalSeconds:    int64(live.Seconds()),
		Total:           fmtShort(live),
		ArchivedCount:   archivedN,
		ArchivedSeconds: int64(archived.Seconds()),
		LifetimeSeconds: int64((live + archived).Seconds()),
	}
}

// actionResultJSON is the common case: an action on a task, reporting the new total.
func actionResultJSON(action string, res tasks.ActionResult) actionJSON {
	ref := taskRefJSONOf(res.Task)
	total := int64(res.Total.Seconds())
	return actionJSON{
		Action:       action,
		Task:         &ref,
		Session:      sessionJSONPtr(res.Session),
		TotalSeconds: &total,
		// fmtShort, not fmtDuration: every duration string in the JSON reads the
		// same way, whichever command produced it.
		Total: fmtShort(res.Total),
	}
}

// sessionsDurationJSON sums a batch of sessions, for rollover reporting.
func sessionsDurationJSON(action string, t database.Task, rows []tasks.SessionRow) actionJSON {
	var d time.Duration
	for _, r := range rows {
		d += r.Duration
	}
	ref := taskRefJSONOf(t)
	n, secs := len(rows), int64(d.Seconds())
	return actionJSON{
		Action:        action,
		Task:          &ref,
		Sessions:      sessionJSONList(rows),
		SessionCount:  &n,
		BankedSeconds: &secs,
	}
}

// ---- small helpers ----

// localTime renders a timestamp in the local zone, so a JSON offset matches the
// clock time the styled output prints.
func localTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	l := t.Local()
	return &l
}

// strSlice keeps an absent list as [] rather than null: a consumer can then
// iterate unconditionally.
func strSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func catStrings(cats []database.TasksCategories) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, string(c))
	}
	return out
}

func int64Val(v int64) *int64 { return &v }
func boolVal(v bool) *bool    { return &v }

// doctorJSON is one row of `tasks doctor`: a task whose status disagrees with
// its sessions.
type doctorJSON struct {
	Seq          int64  `json:"seq"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	OpenSessions int64  `json:"open_sessions"`
}

func doctorJSONList(rows []database.FindInconsistentTasksRow) []doctorJSON {
	out := make([]doctorJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, doctorJSON{
			Seq:          r.Seq,
			Name:         r.Name,
			Status:       string(r.Status),
			OpenSessions: r.OpenSessions,
		})
	}
	return out
}
