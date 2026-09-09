package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"barbtils/internal/database"
	"barbtils/internal/tasks"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/google/uuid"
)

type tuiView uint8

const (
	viewList tuiView = iota
	viewSessions
	viewInput
	viewForm
)

type inputPurpose uint8

const (
	inputConfirmDelete inputPurpose = iota
)

// resyncEvery bounds how stale the list can get when another barbtils process
// changes something. Elapsed time itself needs no query — it is arithmetic.
const resyncEvery = 10 * time.Second

type tasksTuiModel struct {
	ctx   context.Context
	store *tasks.Store

	view       tuiView
	rows       []tasks.TaskRow
	cursor     int
	filter     tasks.Filter
	filterName string

	selTask  *tasks.TaskRow
	sessions []tasks.SessionRow
	// showArchived toggles banked sessions into the drill-down view.
	showArchived bool

	input    textinput.Model
	inputFor inputPurpose

	form *taskForm

	// now is the only clock View reads, so every row on a frame agrees.
	now         time.Time
	lastRefresh time.Time

	status string
	err    error

	width, height int
}

type tickMsg time.Time
type tasksLoadedMsg struct{ rows []tasks.TaskRow }
type sessionsLoadedMsg struct {
	taskID uuid.UUID
	rows   []tasks.SessionRow
}
type actionDoneMsg struct{ note string }
type errMsg struct{ err error }

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadTasksCmd(ctx context.Context, s *tasks.Store, f tasks.Filter) tea.Cmd {
	return func() tea.Msg {
		rows, err := s.ListTasks(ctx, f)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{rows}
	}
}

func loadSessionsCmd(ctx context.Context, s *tasks.Store, id uuid.UUID, includeArchived bool) tea.Cmd {
	return func() tea.Msg {
		rows, err := s.Sessions(ctx, id, includeArchived)
		if err != nil {
			return errMsg{err}
		}
		return sessionsLoadedMsg{taskID: id, rows: rows}
	}
}

// actionCmd runs a mutation off the Update goroutine.
func actionCmd(fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		note, err := fn()
		if err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{note}
	}
}

func newTasksTuiModel(ctx context.Context, s *tasks.Store) *tasksTuiModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "task name"
	ti.CharLimit = 512
	ti.SetWidth(48)

	return &tasksTuiModel{
		ctx:        ctx,
		store:      s,
		view:       viewList,
		input:      ti,
		now:        time.Now(),
		filterName: "all",
	}
}

func (m *tasksTuiModel) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestWindowSize,
		loadTasksCmd(m.ctx, m.store, m.filter),
		tickCmd(),
	)
}

func (m *tasksTuiModel) selected() *tasks.TaskRow {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

func (m *tasksTuiModel) reload() tea.Cmd {
	cmds := []tea.Cmd{loadTasksCmd(m.ctx, m.store, m.filter)}
	if m.view == viewSessions && m.selTask != nil {
		cmds = append(cmds, loadSessionsCmd(m.ctx, m.store, m.selTask.ID, m.showArchived))
	}
	return tea.Batch(cmds...)
}

func (m *tasksTuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		// tea.Tick fires once; it must be re-armed or the clock stops.
		cmds := []tea.Cmd{tickCmd()}
		if m.now.Sub(m.lastRefresh) >= resyncEvery {
			m.lastRefresh = m.now
			cmds = append(cmds, loadTasksCmd(m.ctx, m.store, m.filter))
		}
		return m, tea.Batch(cmds...)

	case tasksLoadedMsg:
		m.rows = msg.rows
		if m.cursor >= len(m.rows) {
			m.cursor = max(0, len(m.rows)-1)
		}
		if m.selTask != nil {
			for i := range m.rows {
				if m.rows[i].ID == m.selTask.ID {
					r := m.rows[i]
					m.selTask = &r
					break
				}
			}
		}
		return m, nil

	case sessionsLoadedMsg:
		m.sessions = msg.rows
		return m, nil

	case actionDoneMsg:
		m.status, m.err = msg.note, nil
		if m.view == viewForm {
			m.form = nil
			m.view = viewList
		}
		return m, m.reload()

	case errMsg:
		m.err, m.status = msg.err, ""
		if m.view == viewForm && m.form != nil {
			// Keep the form open so the value can be corrected, e.g. a name
			// that is already taken.
			m.form.err = msg.err.Error()
			m.err = nil
		}
		return m, nil
	}

	if m.view == viewForm {
		return m.updateForm(msg)
	}
	if m.view == viewInput {
		return m.updateInput(msg)
	}

	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.view == viewSessions {
		return m.updateSessions(km)
	}
	return m.updateList(km)
}

func (m *tasksTuiModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "esc":
			m.form = nil
			m.view = viewList
			m.err = nil
			return m, nil
		case "enter":
			return m.submitForm()
		}
	}
	cmd, _ := m.form.Update(msg)
	return m, cmd
}

// submitForm creates or updates a task from the form. In edit mode a changed
// name is applied too, so one screen configures everything about a task.
func (m *tasksTuiModel) submitForm() (tea.Model, tea.Cmd) {
	f := m.form

	if !f.editing {
		in, err := f.newTaskInput()
		if err != nil {
			f.err = err.Error()
			return m, nil
		}
		return m, actionCmd(func() (string, error) {
			t, err := m.store.CreateTask(m.ctx, in)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("created #%d %q", t.Seq, t.Name), nil
		})
	}

	patch, err := f.metaPatch()
	if err != nil {
		f.err = err.Error()
		return m, nil
	}
	newName := strings.TrimSpace(f.name.Value())
	if newName == "" {
		f.err = tasks.ErrNameEmpty.Error()
		return m, nil
	}
	id, seq := f.taskID, f.taskSeq
	return m, actionCmd(func() (string, error) {
		if _, err := m.store.UpdateMeta(m.ctx, id, patch); err != nil {
			return "", err
		}
		t, err := m.store.Queries().GetTaskByID(m.ctx, id)
		if err != nil {
			return "", err
		}
		if t.Name != newName {
			if _, err := m.store.Rename(m.ctx, id, newName); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("updated #%d %q", seq, newName), nil
	})
}

func (m *tasksTuiModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "esc":
			m.input.Reset()
			m.view = viewList
			m.err = nil
			return m, nil
		case "enter":
			return m.submitInput()
		}
	}
	// Everything else belongs to the component: word delete, home/end, paste.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *tasksTuiModel) submitInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())

	switch m.inputFor {
	case inputConfirmDelete:
		row := m.selTask
		if row == nil || !strings.EqualFold(value, "yes") {
			m.input.Reset()
			m.view = viewList
			m.status = "cancelled"
			return m, nil
		}
		id, seq, name := row.ID, row.Seq, row.Name
		m.input.Reset()
		m.view = viewList
		m.selTask = nil
		return m, actionCmd(func() (string, error) {
			if err := m.store.DeleteTask(m.ctx, id); err != nil {
				return "", err
			}
			return fmt.Sprintf("deleted #%d %q", seq, name), nil
		})
	}
	return m, nil
}

func (m *tasksTuiModel) updateSessions(km tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc", "backspace", "q":
		m.view = viewList
		m.selTask = nil
		m.sessions = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "R":
		return m, m.reload()
	case "A":
		m.showArchived = !m.showArchived
		if m.selTask != nil {
			m.sessions = nil
			return m, loadSessionsCmd(m.ctx, m.store, m.selTask.ID, m.showArchived)
		}
		return m, nil
	}
	return m, nil
}

func (m *tasksTuiModel) updateList(km tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if len(m.rows) > 0 {
			m.cursor = (m.cursor - 1 + len(m.rows)) % len(m.rows)
		}
		return m, nil
	case "down", "j":
		if len(m.rows) > 0 {
			m.cursor = (m.cursor + 1) % len(m.rows)
		}
		return m, nil
	case "g":
		m.cursor = 0
		return m, nil
	case "G":
		m.cursor = max(0, len(m.rows)-1)
		return m, nil

	case "enter":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		r := *row
		m.selTask = &r
		m.view = viewSessions
		m.sessions = nil
		return m, loadSessionsCmd(m.ctx, m.store, r.ID, m.showArchived)

	case "n":
		m.form = createForm()
		m.view = viewForm
		m.err = nil
		return m, m.form.focusCurrent()

	case "e":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		m.form = editForm(*row)
		m.view = viewForm
		m.err = nil
		return m, m.form.focusCurrent()

	case "s":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		if row.IsRunning() {
			m.status = fmt.Sprintf("#%d is already running", row.Seq)
			return m, nil
		}
		id, seq, name := row.ID, row.Seq, row.Name
		return m, actionCmd(func() (string, error) {
			res, err := m.store.Start(m.ctx, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("started s%d on #%d %q", res.Session.Seq, seq, name), nil
		})

	case "p":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		if !row.IsRunning() {
			m.status = fmt.Sprintf("#%d is not running", row.Seq)
			return m, nil
		}
		id, seq := row.ID, row.Seq
		return m, actionCmd(func() (string, error) {
			res, err := m.store.Pause(m.ctx, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("paused #%d, total %s", seq, fmtShort(res.Total)), nil
		})

	case "x":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		if row.IsRecurring() {
			m.status = fmt.Sprintf("#%d is recurring — press a to archive instead", row.Seq)
			return m, nil
		}
		task := database.Task{ID: row.ID, Seq: row.Seq, Name: row.Name, Type: row.Type}
		return m, actionCmd(func() (string, error) {
			res, err := m.store.Complete(m.ctx, task)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("completed #%d, total %s", task.Seq, fmtShort(res.Total)), nil
		})

	case "a":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		id, seq := row.ID, row.Seq
		return m, actionCmd(func() (string, error) {
			res, err := m.store.Archive(m.ctx, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("archived #%d, total %s", seq, fmtShort(res.Total)), nil
		})

	case "A":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		if row.SessionCount == 0 {
			m.status = fmt.Sprintf("#%d has no finished sessions to archive", row.Seq)
			return m, nil
		}
		id, seq := row.ID, row.Seq
		return m, actionCmd(func() (string, error) {
			banked, err := m.store.ArchiveSessions(m.ctx, id)
			if err != nil {
				return "", err
			}
			if len(banked) == 0 {
				return "", fmt.Errorf("#%d has no finished sessions to archive", seq)
			}
			var d time.Duration
			for _, b := range banked {
				d += b.Duration
			}
			return fmt.Sprintf("archived %d session(s) on #%d, banking %s", len(banked), seq, fmtShort(d)), nil
		})

	case "d":
		row := m.selected()
		if row == nil {
			return m, nil
		}
		r := *row
		m.selTask = &r
		m.inputFor = inputConfirmDelete
		m.input.Reset()
		m.view = viewInput
		return m, m.input.Focus()

	case "f":
		m.cycleFilter()
		return m, loadTasksCmd(m.ctx, m.store, m.filter)

	case "R":
		return m, m.reload()
	}
	return m, nil
}

func (m *tasksTuiModel) cycleFilter() {
	inProgress := database.TasksStatusesInProgress
	paused := database.TasksStatusesPaused
	completed := database.TasksStatusesCompleted

	switch m.filterName {
	case "all":
		m.filter, m.filterName = tasks.Filter{OnlyOpen: true}, "running"
	case "running":
		m.filter, m.filterName = tasks.Filter{Status: &paused}, "paused"
	case "paused":
		m.filter, m.filterName = tasks.Filter{Status: &completed}, "completed"
	case "completed":
		m.filter, m.filterName = tasks.Filter{Status: &inProgress}, "in progress"
	default:
		m.filter, m.filterName = tasks.Filter{}, "all"
	}
	m.cursor = 0
}

// ---- view ----

func (m *tasksTuiModel) View() tea.View {
	var body string
	switch m.view {
	case viewSessions:
		body = m.renderSessionsView()
	case viewForm:
		body = m.form.View()
	case viewInput:
		body = m.renderInputView()
	default:
		body = m.renderListView()
	}

	stack := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(), "", body, "", m.renderFooter())

	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	v := tea.NewView(lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, stack))
	v.AltScreen = true
	v.WindowTitle = "barbtils · tasks"
	return v
}

func (m *tasksTuiModel) renderHeader() string {
	running := 0
	for _, r := range m.rows {
		if r.IsRunning() {
			running++
		}
	}
	return tasksBorder.Render(lipgloss.JoinVertical(lipgloss.Left,
		tasksAccent.Render("Tasks")+tasksMuted.Render(fmt.Sprintf("   filter: %s   running: %d", m.filterName, running)),
		tasksMuted.Render(m.now.Format("Mon 02 Jan 2006, "+clockFmt)),
	))
}

func (m *tasksTuiModel) renderListView() string {
	if len(m.rows) == 0 {
		return tasksMuted.Render("  no tasks — press n to create one")
	}
	lines := strings.Split(renderTaskTable(m.rows, m.now), "\n")
	out := make([]string, 0, len(lines))
	out = append(out, "  "+lines[0]) // header row
	for i, ln := range lines[1:] {
		if i == m.cursor {
			out = append(out, tasksAccent.Render("> ")+ln)
		} else {
			out = append(out, "  "+ln)
		}
	}
	return strings.Join(out, "\n")
}

func (m *tasksTuiModel) renderSessionsView() string {
	if m.selTask == nil {
		return ""
	}
	head := tasksAccent.Render(fmt.Sprintf("#%d %s", m.selTask.Seq, m.selTask.Name)) + "  " +
		statusStyled(m.selTask.Status) + "  " +
		tasksMuted.Render("total ") + tasksAccent.Render(fmtShort(m.selTask.Elapsed(m.now)))
	if m.selTask.HasArchive() {
		head += tasksMuted.Render("  ·  lifetime ") + tasksAccent.Render(fmtShort(m.selTask.Lifetime(m.now))) +
			tasksMuted.Render(fmt.Sprintf(" (%d archived)", m.selTask.ArchivedSessionCount))
	}
	if m.showArchived {
		head += tasksMuted.Render("  ·  showing archived")
	}
	if m.sessions == nil {
		return head + "\n\n" + tasksMuted.Render("  loading…")
	}
	if len(m.sessions) == 0 {
		return head + "\n\n" + sessionsEmptyNote(*m.selTask, m.showArchived)
	}
	return head + "\n\n" + renderSessionTable(m.sessions)
}

func (m *tasksTuiModel) renderInputView() string {
	name := ""
	if m.selTask != nil {
		name = fmt.Sprintf(" #%d %q and its %d session(s)", m.selTask.Seq, m.selTask.Name, m.selTask.SessionCount)
	}
	prompt := tasksErr.Render("Delete"+name) + "\n" + tasksWarn.Render("type yes to confirm")
	return prompt + "\n\n" + tasksBorder.Render(m.input.View())
}

func (m *tasksTuiModel) renderFooter() string {
	var line string
	switch {
	case m.err != nil:
		line = tasksErr.Render(m.err.Error())
	case m.status != "":
		line = tasksOK.Render(m.status)
	}

	var help string
	switch m.view {
	case viewSessions:
		help = "esc back • A show/hide archived • R refresh • q quit"
	case viewForm:
		help = ""
	case viewInput:
		help = "enter submit • esc cancel"
	default:
		help = "↑/↓ move • s start • p pause • x complete • a archive • enter sessions • n new • e edit • A archive sessions • d delete • f filter • R refresh • q quit"
	}
	if line == "" {
		return tasksMuted.Render(help)
	}
	return line + "\n" + tasksMuted.Render(help)
}

func taskInteractiveLoop(ctx context.Context, s *tasks.Store) error {
	p := tea.NewProgram(newTasksTuiModel(ctx, s), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
