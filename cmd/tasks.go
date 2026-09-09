package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"barbtils/internal/database"
	"barbtils/internal/tasks"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagInteractive bool

	// Reference interpretation overrides, for the rare task literally named
	// with digits.
	flagRefName bool
	flagRefSeq  bool

	// list filters
	flagListStatus  string
	flagListRunning bool
	flagListGrep    string

	// new
	flagNewStart    bool
	flagNewType     string
	flagNewPriority string
	flagNewCategory []string
	flagNewTags     []string

	// edit
	flagEditType     string
	flagEditPriority string
	flagEditCategory []string
	flagEditTags     []string

	// archive / rollover
	flagShowArchived bool
	flagUndoRollover bool

	// confirmations
	flagYes bool

	// session edit
	flagSessStart    string
	flagSessEnd      string
	flagSessClearEnd bool
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Task timer tracker",
	Long: `Track time against named tasks. A task is referred to by its name — the
primary handle — or by the short number shown in the listing.

A task accumulates sessions. Pausing ends the current session but leaves the
task resumable; starting it again opens a new one. The total for a task is the
sum of its sessions, so time spent paused is never counted.

Recurring tasks cycle between running and paused indefinitely and are retired
with "archive"; only one-time tasks can be completed.

Examples:
  barbtils tasks new "leetcode practice" --type recurring
  barbtils tasks start "leetcode practice"
  barbtils tasks pause "leetcode practice"
  barbtils tasks start "leetcode practice"      # a new session, days later
  barbtils tasks sessions "leetcode practice"
  barbtils tasks ls
  barbtils tasks stop 3                    # by number; completes a one-time task
  barbtils tasks archive "leetcode practice"
  barbtils tasks -i                        # full-screen TUI`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagInteractive {
			if jsonEnabled {
				return errJSONUnsupported("the interactive TUI")
			}
			return runWithStore(func(ctx context.Context, s *tasks.Store, _ []string) error {
				return taskInteractiveLoop(ctx, s)
			})(cmd, args)
		}
		return tasksListCmd.RunE(cmd, args)
	},
}

// runWithStore opens the database, runs fn, and always closes the handle.
// Deliberately a wrapper rather than a PersistentPreRunE on tasksCmd: cobra
// runs only the closest PersistentPreRun in the chain, and the root's is what
// calls initConfig() to populate DB_URL.
func runWithStore(fn func(ctx context.Context, s *tasks.Store, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		st, err := tasks.Open(viper.GetString("DB_URL"))
		if err != nil {
			return err
		}
		defer st.Close()
		return fn(cmd.Context(), st, args)
	}
}

// runWithStoreCmd is runWithStore for commands that must inspect their own
// flags, e.g. to tell "--tag not passed" from "--tag passed empty".
func runWithStoreCmd(fn func(cmd *cobra.Command, ctx context.Context, s *tasks.Store, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		st, err := tasks.Open(viper.GetString("DB_URL"))
		if err != nil {
			return err
		}
		defer st.Close()
		return fn(cmd, cmd.Context(), st, args)
	}
}

func refMode() tasks.RefMode {
	switch {
	case flagRefName:
		return tasks.RefName
	case flagRefSeq:
		return tasks.RefSeq
	default:
		return tasks.RefAuto
	}
}

// addRefFlags gives every command taking a <task> the disambiguation escape hatch.
func addRefFlags(c *cobra.Command) {
	c.Flags().BoolVar(&flagRefName, "name", false, "Treat the argument as a task name, even if it is all digits")
	c.Flags().BoolVar(&flagRefSeq, "seq", false, "Treat the argument as a task number")
	c.MarkFlagsMutuallyExclusive("name", "seq")
}

var tasksListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List tasks with their total tracked time",
	Args:    cobra.NoArgs,
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, _ []string) error {
		f := tasks.Filter{OnlyOpen: flagListRunning}
		if flagListStatus != "" {
			st, err := parseStatus(flagListStatus)
			if err != nil {
				return err
			}
			f.Status = &st
		}
		if flagListGrep != "" {
			f.NameLike = &flagListGrep
		}
		rows, err := s.ListTasks(ctx, f)
		if err != nil {
			return err
		}
		now := time.Now()
		if jsonEnabled {
			return emitJSON(map[string]any{
				"tasks": taskJSONList(rows, now),
				"count": len(rows),
			})
		}
		fmt.Println(renderTaskTable(rows, now))
		return nil
	}),
}

var tasksNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a task (does not start it unless --start is given)",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		in := tasks.NewTaskInput{Name: args[0], Tags: flagNewTags}
		if flagNewType != "" {
			t, err := parseType(flagNewType)
			if err != nil {
				return err
			}
			in.Type = t
		}
		if flagNewPriority != "" {
			p, err := parsePriority(flagNewPriority)
			if err != nil {
				return err
			}
			in.Priority = p
		}
		for _, c := range flagNewCategory {
			cat, err := parseCategory(c)
			if err != nil {
				return err
			}
			in.Category = append(in.Category, cat)
		}

		if flagNewStart {
			res, err := s.CreateTaskAndStart(ctx, in)
			if err != nil {
				return err
			}
			if jsonEnabled {
				return emitJSON(actionResultJSON("created_and_started", res))
			}
			fmt.Println(tasksOK.Render(fmt.Sprintf("Created #%d %q and started session s%d",
				res.Task.Seq, res.Task.Name, res.Session.Seq)))
			return nil
		}
		t, err := s.CreateTask(ctx, in)
		if err != nil {
			return err
		}
		if jsonEnabled {
			ref := taskRefJSONOf(t)
			return emitJSON(actionJSON{Action: "created", Task: &ref})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Created #%d %q", t.Seq, t.Name)) + " " +
			tasksMuted.Render("start it with: barbtils tasks start "+quote(t.Name)))
		return nil
	}),
}

var tasksStartCmd = &cobra.Command{
	Use:   "start <task>",
	Short: "Start a new session (also how a paused task resumes)",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		res, err := s.Start(ctx, t.ID)
		if err != nil {
			if errors.Is(err, tasks.ErrAlreadyRunning) {
				return fmt.Errorf("#%d %q is already running", t.Seq, t.Name)
			}
			return err
		}
		if jsonEnabled {
			return emitJSON(actionResultJSON("started", res))
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Started session s%d on #%d %q",
			res.Session.Seq, res.Task.Seq, res.Task.Name)) + " " +
			tasksMuted.Render("total so far "+fmtShort(res.Total)))
		return nil
	}),
}

var tasksPauseCmd = &cobra.Command{
	Use:   "pause <task>",
	Short: "End the current session; the task stays resumable",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		res, err := s.Pause(ctx, t.ID)
		if err != nil {
			if errors.Is(err, tasks.ErrNotRunning) {
				return fmt.Errorf("#%d %q is not running", t.Seq, t.Name)
			}
			return err
		}
		if jsonEnabled {
			return emitJSON(actionResultJSON("paused", res))
		}
		sess := tasks.SessionOf(*res.Session)
		fmt.Println(tasksWarn.Render(fmt.Sprintf("Paused #%d %q", res.Task.Seq, res.Task.Name)) +
			tasksMuted.Render(fmt.Sprintf(" — session s%d ran %s, total ", sess.Seq, fmtShort(sess.Duration))) +
			tasksAccent.Render(fmtShort(res.Total)))
		return nil
	}),
}

var tasksStopCmd = &cobra.Command{
	Use:     "stop <task>",
	Aliases: []string{"done", "complete"},
	Short:   "Close the session and complete the task (not for recurring tasks)",
	Args:    cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		res, err := s.Complete(ctx, t)
		if err != nil {
			if errors.Is(err, tasks.ErrRecurringCannotComplete) {
				return fmt.Errorf("#%d %q is recurring, so it cannot be completed — retire it with: barbtils tasks archive %s",
					t.Seq, t.Name, quote(t.Name))
			}
			return err
		}
		if jsonEnabled {
			return emitJSON(actionResultJSON("completed", res))
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Completed #%d %q", res.Task.Seq, res.Task.Name)) + " " +
			tasksMuted.Render("total ") + tasksAccent.Render(fmtDuration(res.Total)))
		return nil
	}),
}

var tasksArchiveCmd = &cobra.Command{
	Use:   "archive <task>",
	Short: "Close any open session and archive the task",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		res, err := s.Archive(ctx, t.ID)
		if err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(actionResultJSON("archived", res))
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Archived #%d %q", res.Task.Seq, res.Task.Name)) + " " +
			tasksMuted.Render("total ") + tasksAccent.Render(fmtDuration(res.Total)))
		return nil
	}),
}

var tasksShowCmd = &cobra.Command{
	Use:   "show <task>",
	Short: "Show a task in detail with its session history",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		row, err := s.GetTaskRow(ctx, t.Seq)
		if err != nil {
			return err
		}
		sessions, err := s.Sessions(ctx, t.ID, flagShowArchived)
		if err != nil {
			return err
		}
		now := time.Now()
		if jsonEnabled {
			return emitJSON(map[string]any{
				"task":     taskJSONOf(row, now),
				"sessions": sessionJSONList(sessions),
				"totals":   sessionTotalsOf(sessions),
			})
		}
		fmt.Println(renderTaskDetail(row, sessions, time.Now()))
		return nil
	}),
}

var tasksSessionsCmd = &cobra.Command{
	Use:   "sessions <task>",
	Short: "List every session recorded against a task",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		sessions, err := s.Sessions(ctx, t.ID, flagShowArchived)
		if err != nil {
			return err
		}
		row, err := s.GetTaskRow(ctx, t.Seq)
		if err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(map[string]any{
				"task":     taskJSONOf(row, time.Now()),
				"sessions": sessionJSONList(sessions),
				"totals":   sessionTotalsOf(sessions),
			})
		}
		fmt.Println(tasksAccent.Render(fmt.Sprintf("#%d %s", t.Seq, t.Name)))
		if len(sessions) == 0 {
			fmt.Println(sessionsEmptyNote(row, flagShowArchived))
			return nil
		}
		fmt.Println(renderSessionTable(sessions))
		return nil
	}),
}

var tasksRenameCmd = &cobra.Command{
	Use:   "rename <task> <new-name>",
	Short: "Rename a task, keeping its number and session history",
	Args:  cobra.ExactArgs(2),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		renamed, err := s.Rename(ctx, t.ID, args[1])
		if err != nil {
			return err
		}
		if jsonEnabled {
			ref := taskRefJSONOf(renamed)
			return emitJSON(actionJSON{Action: "renamed", Task: &ref, PreviousName: t.Name})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Renamed #%d %q -> %q", renamed.Seq, t.Name, renamed.Name)))
		return nil
	}),
}

var tasksEditCmd = &cobra.Command{
	Use:   "edit <task>",
	Short: "Change a task's type, priority, categories or tags",
	Long: `Change a task's configuration. Only the flags you pass are altered.

  --category and --tag replace the whole list; pass an empty value to clear,
  e.g. --tag "" removes every tag.

Examples:
  barbtils tasks edit "leetcode practice" --priority high
  barbtils tasks edit "leetcode practice" --tag api --tag urgent
  barbtils tasks edit 3 --type recurring --category client-project`,
	Args: cobra.ExactArgs(1),
	RunE: runWithStoreCmd(func(cmd *cobra.Command, ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}

		var patch tasks.MetaPatch
		if flagEditType != "" {
			v, err := parseType(flagEditType)
			if err != nil {
				return err
			}
			patch.Type = &v
		}
		if flagEditPriority != "" {
			v, err := parsePriority(flagEditPriority)
			if err != nil {
				return err
			}
			patch.Priority = &v
		}
		if cmd.Flags().Changed("category") {
			cats := []database.TasksCategories{}
			for _, c := range flagEditCategory {
				if strings.TrimSpace(c) == "" {
					continue
				}
				v, err := parseCategory(c)
				if err != nil {
					return err
				}
				cats = append(cats, v)
			}
			patch.Category = cats
		}
		if cmd.Flags().Changed("tag") {
			tags := []string{}
			for _, tag := range flagEditTags {
				if tag = strings.TrimSpace(tag); tag != "" {
					tags = append(tags, tag)
				}
			}
			patch.Tags = tags
		}

		if patch.IsEmpty() {
			return errors.New("nothing to change — pass --type, --priority, --category or --tag")
		}
		updated, err := s.UpdateMeta(ctx, t.ID, patch)
		if err != nil {
			return err
		}
		row, err := s.GetTaskRow(ctx, updated.Seq)
		if err != nil {
			return err
		}
		sessions, err := s.Sessions(ctx, updated.ID, false)
		if err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(map[string]any{
				"action":   "updated",
				"task":     taskJSONOf(row, time.Now()),
				"sessions": sessionJSONList(sessions),
				"totals":   sessionTotalsOf(sessions),
			})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Updated #%d %q", updated.Seq, updated.Name)))
		fmt.Println(renderTaskDetail(row, sessions, time.Now()))
		return nil
	}),
}

var tasksRmCmd = &cobra.Command{
	Use:   "rm <task>",
	Short: "Delete a task and all of its recorded sessions",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}
		row, err := s.GetTaskRow(ctx, t.Seq)
		if err != nil {
			return err
		}
		if !flagYes {
			return fmt.Errorf("this deletes #%d %q and its %d recorded session(s) — re-run with --yes to confirm, or use: barbtils tasks archive %s",
				row.Seq, row.Name, row.SessionCount, quote(row.Name))
		}
		if err := s.DeleteTask(ctx, t.ID); err != nil {
			return err
		}
		if jsonEnabled {
			ref := taskRefJSONOf(t)
			// Archived sessions are deleted along with the live ones, so the
			// count reports both rather than only the current period.
			deleted := int(row.SessionCount + row.ArchivedSessionCount)
			return emitJSON(actionJSON{Action: "deleted", Task: &ref, SessionCount: &deleted})
		}
		fmt.Println(tasksWarn.Render(fmt.Sprintf("Deleted #%d %q and %d session(s)", row.Seq, row.Name, row.SessionCount)))
		return nil
	}),
}

var tasksRolloverCmd = &cobra.Command{
	Use:     "rollover <task>",
	Aliases: []string{"archive-sessions"},
	Short:   "Bank the task's finished sessions, resetting its running total",
	Long: `Archive every finished session of a task so its total starts again from zero,
without losing any history.

Made for a recurring task you return to daily: roll it over at the end of the
day and the next day's total counts only that day's sessions. Archived time is
still in the record -- "tasks show" reports it as the lifetime total, and
"tasks sessions <task> --archived" lists it.

A session still running is deliberately left alone, so a timer spanning the
rollover keeps counting into the new period.

  --undo brings every archived session of the task back into the total.`,
	Args: cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		t, err := s.ResolveTask(ctx, args[0], refMode())
		if err != nil {
			return err
		}

		if flagUndoRollover {
			restored, err := s.UnarchiveSessions(ctx, t.ID)
			if err != nil {
				return err
			}
			if len(restored) == 0 {
				return fmt.Errorf("#%d %q has no archived sessions", t.Seq, t.Name)
			}
			if jsonEnabled {
				return emitJSON(sessionsDurationJSON("sessions_restored", t, restored))
			}
			var d time.Duration
			for _, r := range restored {
				d += r.Duration
			}
			fmt.Println(tasksOK.Render(fmt.Sprintf("Restored %s to #%d %q", plural(len(restored), "session"), t.Seq, t.Name)) +
				tasksMuted.Render(" adding back ") + tasksAccent.Render(fmtDuration(d)))
			return nil
		}

		banked, err := s.ArchiveSessions(ctx, t.ID)
		if err != nil {
			return err
		}
		if len(banked) == 0 {
			return fmt.Errorf("#%d %q has no finished sessions to archive", t.Seq, t.Name)
		}
		row, err := s.GetTaskRow(ctx, t.Seq)
		if err != nil {
			return err
		}
		if jsonEnabled {
			now := time.Now()
			out := sessionsDurationJSON("sessions_archived", t, banked)
			out.TotalSeconds = int64Val(int64(row.Elapsed(now).Seconds()))
			out.Total = fmtShort(row.Elapsed(now))
			out.LifetimeSeconds = int64Val(int64(row.Lifetime(now).Seconds()))
			out.Running = boolVal(row.IsRunning())
			return emitJSON(out)
		}
		var d time.Duration
		for _, r := range banked {
			d += r.Duration
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Archived %s on #%d %q", plural(len(banked), "session"), t.Seq, t.Name)) +
			tasksMuted.Render(" banking ") + tasksAccent.Render(fmtDuration(d)))
		if row.IsRunning() {
			fmt.Println(tasksMuted.Render("A session is still running and keeps counting into the new period."))
		}
		fmt.Println(tasksMuted.Render("Total now ") + tasksAccent.Render(fmtDuration(row.Elapsed(time.Now()))) +
			tasksMuted.Render("  ·  lifetime ") + tasksAccent.Render(fmtDuration(row.Lifetime(time.Now()))))
		return nil
	}),
}

var tasksSessionArchiveCmd = &cobra.Command{
	Use:   "archive <session>",
	Short: "Bank one finished session",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		seq, err := tasks.ParseSeq(args[0])
		if err != nil {
			return err
		}
		row, err := s.ArchiveSession(ctx, seq)
		if err != nil {
			return err
		}
		if jsonEnabled {
			sess := sessionJSONOf(row)
			return emitJSON(actionJSON{Action: "session_archived", Session: &sess})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Archived session s%d", row.Seq)) +
			tasksMuted.Render(" banking ") + tasksAccent.Render(fmtShort(row.Duration)))
		return nil
	}),
}

var tasksSessionUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <session>",
	Short: "Return one banked session to the total",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		seq, err := tasks.ParseSeq(args[0])
		if err != nil {
			return err
		}
		row, err := s.UnarchiveSession(ctx, seq)
		if err != nil {
			return err
		}
		if jsonEnabled {
			sess := sessionJSONOf(row)
			return emitJSON(actionJSON{Action: "session_restored", Session: &sess})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Restored session s%d", row.Seq)) +
			tasksMuted.Render(" adding back ") + tasksAccent.Render(fmtShort(row.Duration)))
		return nil
	}),
}

var tasksSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Operate on an individual session by its number",
}

var tasksSessionCloseCmd = &cobra.Command{
	Use:   "close <session>",
	Short: "End one specific open session",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		seq, err := tasks.ParseSeq(args[0])
		if err != nil {
			return err
		}
		sess, err := s.CloseSession(ctx, seq)
		if err != nil {
			return err
		}
		row := tasks.SessionOf(sess)
		if jsonEnabled {
			j := sessionJSONOf(row)
			return emitJSON(actionJSON{Action: "session_closed", Session: &j})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Closed session s%d", row.Seq)) + " " +
			tasksMuted.Render("ran ") + tasksAccent.Render(fmtShort(row.Duration)))
		return nil
	}),
}

var tasksSessionRmCmd = &cobra.Command{
	Use:   "rm <session>",
	Short: "Delete one session",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		seq, err := tasks.ParseSeq(args[0])
		if err != nil {
			return err
		}
		if !flagYes {
			return fmt.Errorf("this permanently removes session s%d from the record — re-run with --yes to confirm", seq)
		}
		if err := s.DeleteSession(ctx, seq); err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(map[string]any{
				"action":  "session_deleted",
				"session": map[string]any{"seq": seq},
			})
		}
		fmt.Println(tasksWarn.Render(fmt.Sprintf("Deleted session s%d", seq)))
		return nil
	}),
}

var tasksSessionEditCmd = &cobra.Command{
	Use:   "edit <session>",
	Short: "Correct a session's start or end time",
	Args:  cobra.ExactArgs(1),
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, args []string) error {
		seq, err := tasks.ParseSeq(args[0])
		if err != nil {
			return err
		}
		p := database.SetSessionTimesParams{Seq: seq, ClearEndTime: flagSessClearEnd}
		if flagSessStart != "" {
			t, err := parseWhen(flagSessStart)
			if err != nil {
				return err
			}
			p.StartTime = nullTime(t)
		}
		if flagSessEnd != "" {
			if flagSessClearEnd {
				return errors.New("use either --end or --clear-end, not both")
			}
			t, err := parseWhen(flagSessEnd)
			if err != nil {
				return err
			}
			p.EndTime = nullTime(t)
		}
		sess, err := s.Queries().SetSessionTimes(ctx, p)
		if err != nil {
			return err
		}
		row := tasks.SessionOf(sess)
		if jsonEnabled {
			j := sessionJSONOf(row)
			return emitJSON(actionJSON{Action: "session_updated", Session: &j})
		}
		fmt.Println(tasksOK.Render(fmt.Sprintf("Updated session s%d", row.Seq)) + " " +
			tasksMuted.Render("now ") + tasksAccent.Render(fmtShort(row.Duration)))
		return nil
	}),
}

var tasksDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report tasks whose status disagrees with their sessions",
	Args:  cobra.NoArgs,
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, _ []string) error {
		bad, err := s.Doctor(ctx)
		if err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(map[string]any{
				"consistent":   len(bad) == 0,
				"inconsistent": doctorJSONList(bad),
			})
		}
		if len(bad) == 0 {
			fmt.Println(tasksOK.Render("All tasks consistent with their sessions."))
			return nil
		}
		fmt.Println(tasksWarn.Render("Inconsistent tasks:"))
		for _, r := range bad {
			fmt.Printf("  #%d %s — status %s but %d open session(s)\n", r.Seq, r.Name, r.Status, r.OpenSessions)
		}
		return nil
	}),
}

var tasksTruncateCmd = &cobra.Command{
	Use:   "truncate",
	Short: "Delete every task and session",
	Args:  cobra.NoArgs,
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, _ []string) error {
		if !flagYes {
			return errors.New("this deletes every task and every recorded session — re-run with --yes to confirm")
		}
		if err := s.TruncateAll(ctx); err != nil {
			return err
		}
		if jsonEnabled {
			return emitJSON(actionJSON{Action: "truncated", Message: "all task data removed"})
		}
		fmt.Println(tasksWarn.Render("All task data removed."))
		return nil
	}),
}

var tasksTuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Full-screen interactive task view",
	Args:  cobra.NoArgs,
	RunE: runWithStore(func(ctx context.Context, s *tasks.Store, _ []string) error {
		if jsonEnabled {
			return errJSONUnsupported("the interactive TUI")
		}
		return taskInteractiveLoop(ctx, s)
	}),
}

// ---- enum + time parsing ----

func parseStatus(v string) (database.TasksStatuses, error) {
	all := []database.TasksStatuses{
		database.TasksStatusesPending, database.TasksStatusesInProgress,
		database.TasksStatusesCompleted, database.TasksStatusesPaused,
		database.TasksStatusesArchived,
	}
	for _, s := range all {
		if strings.EqualFold(string(s), v) || strings.EqualFold(collapse(string(s)), collapse(v)) {
			return s, nil
		}
	}
	return "", fmt.Errorf("unknown status %q: expected one of pending, in-progress, completed, paused, archived", v)
}

func parseType(v string) (database.TasksTypes, error) {
	for _, t := range []database.TasksTypes{database.TasksTypesRecurring, database.TasksTypesOneTime} {
		if strings.EqualFold(string(t), v) || strings.EqualFold(collapse(string(t)), collapse(v)) {
			return t, nil
		}
	}
	return "", fmt.Errorf("unknown type %q: expected recurring or one-time", v)
}

func parsePriority(v string) (database.TasksPriorities, error) {
	all := []database.TasksPriorities{
		database.TasksPrioritiesUnknown, database.TasksPrioritiesVeryLow,
		database.TasksPrioritiesLow, database.TasksPrioritiesMedium,
		database.TasksPrioritiesHigh, database.TasksPrioritiesVeryHigh,
	}
	for _, p := range all {
		if strings.EqualFold(string(p), v) || strings.EqualFold(collapse(string(p)), collapse(v)) {
			return p, nil
		}
	}
	return "", fmt.Errorf("unknown priority %q: expected one of unknown, very-low, low, medium, high, very-high", v)
}

func parseCategory(v string) (database.TasksCategories, error) {
	all := []database.TasksCategories{
		database.TasksCategoriesUnknown, database.TasksCategoriesClientProject,
		database.TasksCategoriesPersonalProject, database.TasksCategoriesTroubleshooting,
		database.TasksCategoriesRoutine,
	}
	for _, c := range all {
		if strings.EqualFold(string(c), v) || strings.EqualFold(collapse(string(c)), collapse(v)) {
			return c, nil
		}
	}
	return "", fmt.Errorf("unknown category %q: expected one of unknown, client-project, personal-project, troubleshooting, routine", v)
}

// collapse makes "In Progress", "in-progress" and "inprogress" comparable.
func collapse(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(s))
}

// parseWhen accepts RFC3339, "2006-01-02 15:04", "2006-01-02 15:04:05" and "15:04"
// (today), in local time.
func parseWhen(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, nil
		}
	}
	if t, err := time.ParseInLocation("15:04", v, time.Local); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q: try 15:04, \"2006-01-02 15:04\" or RFC3339", v)
}

func nullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

func quote(s string) string {
	if strings.ContainsAny(s, " \t\"'") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func init() {
	tasksCmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "Start the interactive TUI")

	tasksListCmd.Flags().StringVar(&flagListStatus, "status", "", "Only tasks with this status")
	tasksListCmd.Flags().BoolVarP(&flagListRunning, "running", "r", false, "Only tasks with an open session")
	tasksListCmd.Flags().StringVar(&flagListGrep, "grep", "", "Only tasks whose name contains this text")

	tasksNewCmd.Flags().BoolVar(&flagNewStart, "start", false, "Open a session immediately")
	tasksNewCmd.Flags().StringVar(&flagNewType, "type", "", "recurring or one-time (default one-time)")
	tasksNewCmd.Flags().StringVar(&flagNewPriority, "priority", "", "unknown, very-low, low, medium, high, very-high")
	tasksNewCmd.Flags().StringSliceVar(&flagNewCategory, "category", nil, "Category (repeatable)")
	tasksNewCmd.Flags().StringSliceVar(&flagNewTags, "tag", nil, "Tag (repeatable)")

	for _, c := range []*cobra.Command{
		tasksStartCmd, tasksPauseCmd, tasksStopCmd, tasksArchiveCmd,
		tasksShowCmd, tasksSessionsCmd, tasksRenameCmd, tasksRmCmd, tasksEditCmd, tasksRolloverCmd,
	} {
		addRefFlags(c)
	}

	tasksEditCmd.Flags().StringVar(&flagEditType, "type", "", "recurring or one-time")
	tasksEditCmd.Flags().StringVar(&flagEditPriority, "priority", "", "unknown, very-low, low, medium, high, very-high")
	tasksEditCmd.Flags().StringSliceVar(&flagEditCategory, "category", nil, "Replace categories (repeatable; empty clears)")
	tasksEditCmd.Flags().StringSliceVar(&flagEditTags, "tag", nil, "Replace tags (repeatable; empty clears)")

	tasksRmCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm destruction of recorded time")
	tasksTruncateCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm destruction of all recorded time")
	tasksSessionRmCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm removal of the session")

	tasksSessionEditCmd.Flags().StringVar(&flagSessStart, "start", "", "New start time")
	tasksSessionEditCmd.Flags().StringVar(&flagSessEnd, "end", "", "New end time")
	tasksSessionEditCmd.Flags().BoolVar(&flagSessClearEnd, "clear-end", false, "Reopen the session by clearing its end time")

	tasksSessionsCmd.Flags().BoolVar(&flagShowArchived, "archived", false, "Include archived sessions")
	tasksShowCmd.Flags().BoolVar(&flagShowArchived, "archived", false, "Include archived sessions")
	tasksRolloverCmd.Flags().BoolVar(&flagUndoRollover, "undo", false, "Bring archived sessions back into the total")

	tasksSessionCmd.AddCommand(tasksSessionCloseCmd, tasksSessionRmCmd, tasksSessionEditCmd,
		tasksSessionArchiveCmd, tasksSessionUnarchiveCmd)
	tasksCmd.AddCommand(
		tasksListCmd, tasksNewCmd, tasksStartCmd, tasksPauseCmd, tasksStopCmd,
		tasksArchiveCmd, tasksShowCmd, tasksSessionsCmd, tasksRenameCmd, tasksRmCmd, tasksEditCmd,
		tasksSessionCmd, tasksRolloverCmd, tasksDoctorCmd, tasksTruncateCmd, tasksTuiCmd,
	)
	RootCmd.AddCommand(tasksCmd)
}
