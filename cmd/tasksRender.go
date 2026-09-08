package cmd

import (
	"fmt"
	"strings"
	"time"

	"barbtils/internal/database"
	"barbtils/internal/tasks"

	"charm.land/lipgloss/v2"
)

// renderColumns lays out a table. It measures with lipgloss.Width rather than
// len, because every cell may carry ANSI styling that text/tabwriter would
// count as visible characters and mis-align.
func renderColumns(headers []string, rows [][]string, rightAlign []bool) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && lipgloss.Width(c) > widths[i] {
				widths[i] = lipgloss.Width(c)
			}
		}
	}

	pad := func(cell string, w int, right bool) string {
		gap := w - lipgloss.Width(cell)
		if gap < 0 {
			gap = 0
		}
		if right {
			return strings.Repeat(" ", gap) + cell
		}
		return cell + strings.Repeat(" ", gap)
	}
	isRight := func(i int) bool { return i < len(rightAlign) && rightAlign[i] }

	line := func(cells []string) string {
		out := make([]string, 0, len(cells))
		for i, c := range cells {
			w := 0
			if i < len(widths) {
				w = widths[i]
			}
			if i == len(cells)-1 && !isRight(i) {
				out = append(out, c) // no trailing padding
				continue
			}
			out = append(out, pad(c, w, isRight(i)))
		}
		return strings.TrimRight(strings.Join(out, "  "), " ")
	}

	var b strings.Builder
	styledHeaders := make([]string, len(headers))
	for i, h := range headers {
		styledHeaders[i] = tasksHeader.Render(h)
	}
	b.WriteString(line(styledHeaders))
	for _, r := range rows {
		b.WriteString("\n")
		b.WriteString(line(r))
	}
	return b.String()
}

// fmtDuration renders a duration in words, for detail output.
func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	days, hours := total/86400, (total%86400)/3600
	minutes, seconds := (total%3600)/60, total%60

	var parts []string
	if days > 0 {
		parts = append(parts, plural(days, "day"))
	}
	if hours > 0 {
		parts = append(parts, plural(hours, "hour"))
	}
	if minutes > 0 {
		parts = append(parts, plural(minutes, "minute"))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, plural(seconds, "second"))
	}
	return strings.Join(parts, ", ")
}

// plural renders "1 session" but "2 sessions".
func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// fmtShort renders a duration compactly, for table cells.
func fmtShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	days, hours := total/86400, (total%86400)/3600
	minutes, seconds := (total%3600)/60, total%60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %02dm %02ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func statusStyled(s database.TasksStatuses) string {
	switch s {
	case database.TasksStatusesInProgress:
		return tasksAccent.Render(string(s))
	case database.TasksStatusesPaused:
		return tasksWarn.Render(string(s))
	case database.TasksStatusesCompleted:
		return tasksOK.Render(string(s))
	case database.TasksStatusesArchived:
		return tasksDim.Render(string(s))
	default:
		return tasksMuted.Render(string(s))
	}
}

// sessionCountCell shows the live session count, with a dim "+N" when earlier
// periods have been banked.
func sessionCountCell(r tasks.TaskRow) string {
	if !r.HasArchive() {
		return fmt.Sprintf("%d", r.SessionCount)
	}
	return fmt.Sprintf("%d", r.SessionCount) + tasksDim.Render(fmt.Sprintf("+%d", r.ArchivedSessionCount))
}

// priorityStyled dims Unknown, so a deliberately set priority stands out.
func priorityStyled(p database.TasksPriorities) string {
	switch p {
	case database.TasksPrioritiesVeryHigh, database.TasksPrioritiesHigh:
		return tasksWarn.Render(string(p))
	case database.TasksPrioritiesUnknown:
		return tasksDim.Render(string(p))
	default:
		return tasksMuted.Render(string(p))
	}
}

const clockFmt = "03:04:05 PM"

// renderTaskTable lists tasks with their summed elapsed time.
func renderTaskTable(rows []tasks.TaskRow, now time.Time) string {
	if len(rows) == 0 {
		return tasksMuted.Render("  (no tasks)")
	}
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		since := tasksMuted.Render("—")
		if r.OpenStartedAt != nil {
			since = r.OpenStartedAt.Local().Format(clockFmt)
		}
		cells = append(cells, []string{
			fmt.Sprintf("%d", r.Seq),
			r.Name,
			tasksMuted.Render(string(r.Type)),
			priorityStyled(r.Priority),
			statusStyled(r.Status),
			sessionCountCell(r),
			tasksAccent.Render(fmtShort(r.Elapsed(now))),
			since,
		})
	}
	return renderColumns(
		[]string{"#", "TASK", "TYPE", "PRIORITY", "STATUS", "SESSIONS", "TOTAL", "SINCE"},
		cells,
		[]bool{true, false, false, false, false, true, false, false},
	)
}

// renderSessionTable lists a task's sessions, newest first, with a summed footer.
func renderSessionTable(rows []tasks.SessionRow) string {
	if len(rows) == 0 {
		return tasksMuted.Render("  (no sessions yet)")
	}
	cells := make([][]string, 0, len(rows))
	var live, archived time.Duration
	var liveN, archivedN int
	for _, s := range rows {
		end := tasksWarn.Render("running")
		if s.EndTime != nil {
			end = s.EndTime.Local().Format(clockFmt)
		}
		state := ""
		if s.IsArchived() {
			archived += s.Duration
			archivedN++
			state = tasksDim.Render("archived")
		} else {
			live += s.Duration
			liveN++
		}
		num := fmt.Sprintf("s%d", s.Seq)
		start := s.StartTime.Local().Format("Mon 02 Jan " + clockFmt)
		dur := fmtShort(s.Duration)
		if s.IsArchived() {
			// Banked rows are dimmed so the current period reads at a glance.
			num, start, dur = tasksDim.Render(num), tasksDim.Render(start), tasksDim.Render(dur)
		}
		cells = append(cells, []string{num, start, end, dur, state})
	}
	table := renderColumns(
		[]string{"#", "START", "END", "DURATION", ""},
		cells,
		[]bool{true, false, false, true, false},
	)

	footer := tasksMuted.Render("total of "+plural(liveN, "session")+": ") +
		tasksAccent.Render(fmtDuration(live))
	if archivedN > 0 {
		footer += "\n" + tasksDim.Render("archived: "+plural(archivedN, "session")+", "+fmtDuration(archived)) +
			tasksMuted.Render("  ·  lifetime ") + tasksAccent.Render(fmtDuration(live+archived))
	}
	return table + "\n" + footer
}

// sessionsEmptyNote explains an empty session list. After a rollover the list
// is empty but the history is not, so "no sessions yet" would be a lie.
func sessionsEmptyNote(r tasks.TaskRow, showingArchived bool) string {
	switch {
	case r.HasArchive() && !showingArchived:
		return tasksMuted.Render(fmt.Sprintf("  no sessions in this period — %s archived",
			plural(int(r.ArchivedSessionCount), "session")))
	case r.SessionCount == 0 && r.ArchivedSessionCount == 0:
		return tasksMuted.Render("  (no sessions yet)")
	default:
		return tasksMuted.Render("  (no sessions)")
	}
}

// renderTaskDetail is the `tasks show` body.
func renderTaskDetail(r tasks.TaskRow, sessions []tasks.SessionRow, now time.Time) string {
	lines := []string{
		tasksAccent.Render(fmt.Sprintf("#%d %s", r.Seq, r.Name)),
		tasksMuted.Render("Status:    ") + statusStyled(r.Status),
		tasksMuted.Render("Type:      ") + string(r.Type),
		tasksMuted.Render("Priority:  ") + string(r.Priority),
		tasksMuted.Render("Total:     ") + tasksAccent.Render(fmtDuration(r.Elapsed(now))),
		tasksMuted.Render("Sessions:  ") + fmt.Sprintf("%d", r.SessionCount),
	}
	if r.HasArchive() {
		lines = append(lines,
			tasksMuted.Render("Archived:  ")+tasksDim.Render(fmtDuration(r.ArchivedDuration))+
				tasksMuted.Render(fmt.Sprintf(" in %s", plural(int(r.ArchivedSessionCount), "session"))),
			tasksMuted.Render("Lifetime:  ")+tasksAccent.Render(fmtDuration(r.Lifetime(now))),
		)
	}
	if r.FirstStartedAt != nil {
		lines = append(lines, tasksMuted.Render("First run: ")+
			r.FirstStartedAt.Local().Format("Mon 02 Jan 2006, "+clockFmt))
	}
	if r.OpenStartedAt != nil {
		lines = append(lines, tasksMuted.Render("Running:   ")+
			tasksWarn.Render("since "+r.OpenStartedAt.Local().Format(clockFmt)))
	}
	if len(r.Tags) > 0 {
		lines = append(lines, tasksMuted.Render("Tags:      ")+strings.Join(r.Tags, ", "))
	}
	if len(r.Category) > 0 {
		cats := make([]string, 0, len(r.Category))
		for _, c := range r.Category {
			cats = append(cats, string(c))
		}
		lines = append(lines, tasksMuted.Render("Category:  ")+strings.Join(cats, ", "))
	}
	body := renderSessionTable(sessions)
	if len(sessions) == 0 {
		body = sessionsEmptyNote(r, false)
	}
	return strings.Join(lines, "\n") + "\n\n" + body
}
