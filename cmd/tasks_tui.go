package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiPhase uint8

const (
	tuiPhaseMenu tuiPhase = iota
	tuiPhaseInput
	tuiPhaseOutput
)

type menuTag int

const (
	menuNew menuTag = iota
	menuListRunning
	menuListCompleted
	menuStop
	menuShow
	menuTruncate
	menuQuit
)

type menuEntry struct {
	title, desc string
	tag         menuTag
}

func (e menuEntry) Title() string       { return e.title }
func (e menuEntry) Description() string { return e.desc }
func (e menuEntry) FilterValue() string { return e.title }

type tasksInteractiveModel struct {
	db       *sql.DB
	phase    tuiPhase
	list     list.Model
	input    textinput.Model
	inputFor menuTag
	inputCtx string
	output   string
	errLine  string
	width    int
	height   int
}

func newTasksInteractiveModel(db *sql.DB) *tasksInteractiveModel {
	menuDefs := []struct {
		title, desc string
		tag         menuTag
	}{
		{"Create new task", "Start a timer (name optional)", menuNew},
		{"List running timers", "Tasks still open", menuListRunning},
		{"List completed timers", "Tasks with an end time", menuListCompleted},
		{"Stop a running timer", "Enter task ID", menuStop},
		{"Show task details", "Enter task ID", menuShow},
		{"Truncate all tasks", "Deletes every row — needs YES", menuTruncate},
		{"Quit", "Leave interactive mode", menuQuit},
	}
	items := make([]list.Item, len(menuDefs))
	for i, d := range menuDefs {
		items[i] = menuEntry{
			title: fmt.Sprintf("%d. %s", i+1, d.title),
			desc:  d.desc,
			tag:   d.tag,
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	st := delegate.Styles
	st.SelectedTitle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("62")).Foreground(lipgloss.Color("86")).Bold(true).Padding(0, 0, 0, 1)
	st.SelectedDesc = st.SelectedTitle.Copy().Foreground(lipgloss.Color("245")).Bold(false)
	st.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 0, 0, 2)
	st.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 0, 0, 2)
	delegate.Styles = st

	l := list.New(items, delegate, 40, 20)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)

	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 40

	return &tasksInteractiveModel{
		db:    db,
		phase: tuiPhaseMenu,
		list:  l,
		input: ti,
	}
}

func (m *tasksInteractiveModel) Init() tea.Cmd {
	return nil
}

func (m *tasksInteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listH := msg.Height - 14
		if listH < 6 {
			listH = 6
		}
		listW := msg.Width - 4
		if listW < 20 {
			listW = 20
		}
		m.list.SetSize(listW, listH)
		m.input.Width = min(listW-4, 72)
		return m, nil
	}

	switch m.phase {
	case tuiPhaseOutput:
		if km, ok := msg.(tea.KeyMsg); ok {
			if m.errLine != "" {
				switch km.String() {
				case "ctrl+c":
					return m, tea.Quit
				case "enter":
					m.returnFromError()
				}
				return m, nil
			}
			m.phase = tuiPhaseMenu
			m.output = ""
			m.errLine = ""
		}
		return m, nil

	case tuiPhaseInput:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "ctrl+c", "esc":
				m.phase = tuiPhaseMenu
				m.input.Blur()
				return m, nil
			case "enter":
				return m.submitInput()
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case tuiPhaseMenu:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "enter":
				return m.menuEnter()
			}
			if key := km.String(); len(key) == 1 {
				c := key[0]
				n := len(m.list.Items())
				if n > 0 && n <= 9 && c >= '1' && c <= byte('0'+n) {
					idx := int(c - '1')
					if m.list.Index() == idx {
						return m.menuEnter()
					}
					m.list.Select(idx)
					return m, nil
				}
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *tasksInteractiveModel) menuEnter() (tea.Model, tea.Cmd) {
	raw := m.list.SelectedItem()
	if raw == nil {
		return m, nil
	}
	e, ok := raw.(menuEntry)
	if !ok {
		return m, nil
	}

	switch e.tag {
	case menuQuit:
		return m, tea.Quit

	case menuNew:
		m.phase = tuiPhaseInput
		m.inputFor = menuNew
		m.inputCtx = ""
		m.input.SetValue("")
		m.input.Placeholder = "optional name"
		m.input.Focus()
		return m, textinput.Blink

	case menuListRunning:
		m.inputFor = menuListRunning
		return m.runAndShow(func(w *bytes.Buffer) error {
			return getAllRunningTimers(m.db, false, w)
		})

	case menuListCompleted:
		m.inputFor = menuListCompleted
		return m.runAndShow(func(w *bytes.Buffer) error {
			return getAllRunningTimers(m.db, true, w)
		})

	case menuStop:
		m.phase = tuiPhaseInput
		m.inputFor = menuStop
		m.inputCtx = m.taskSelectionPrompt(false)
		m.input.SetValue("")
		m.input.Placeholder = "task ID"
		m.input.Focus()
		return m, textinput.Blink

	case menuShow:
		m.phase = tuiPhaseInput
		m.inputFor = menuShow
		m.inputCtx = m.taskSelectionPrompt(true)
		m.input.SetValue("")
		m.input.Placeholder = "task ID"
		m.input.Focus()
		return m, textinput.Blink

	case menuTruncate:
		m.phase = tuiPhaseInput
		m.inputFor = menuTruncate
		m.inputCtx = ""
		m.input.SetValue("")
		m.input.Placeholder = "type YES/Y/1 to confirm"
		m.input.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m *tasksInteractiveModel) runAndShow(fn func(*bytes.Buffer) error) (tea.Model, tea.Cmd) {
	var buf bytes.Buffer
	err := fn(&buf)
	m.phase = tuiPhaseOutput
	m.output = buf.String()
	if err != nil {
		m.errLine = err.Error()
	} else {
		m.errLine = ""
	}
	return m, nil
}

func (m *tasksInteractiveModel) submitInput() (tea.Model, tea.Cmd) {
	var buf bytes.Buffer
	var err error

	switch m.inputFor {
	case menuNew:
		err = saveNewTask(m.db, strings.TrimSpace(m.input.Value()), &buf)

	case menuStop:
		id, convErr := strconv.Atoi(strings.TrimSpace(m.input.Value()))
		if convErr != nil {
			m.finishInputWithError("invalid task ID")
			return m, nil
		}
		err = stopTimer(m.db, id, &buf)

	case menuShow:
		id, convErr := strconv.Atoi(strings.TrimSpace(m.input.Value()))
		if convErr != nil {
			m.finishInputWithError("invalid task ID")
			return m, nil
		}
		err = showSpecificTimer(m.db, id, &buf)

	case menuTruncate:
		// var answer any = strings.ToLower(strings.TrimSpace(m.input.Value()))
		// if answer == "yes" {
		allowedString := []string{"yes","y","1"}
		var answer string = m.input.Value()
		if slices.Contains(allowedString, strings.ToLower(answer)) {
			err = truncateAllTasks(m.db, &buf)
		} else {
			fmt.Fprintln(&buf, tasksMuted.Render("Cancelled."))
		}

	default:
		m.input.Blur()
		m.phase = tuiPhaseMenu
		return m, nil
	}

	m.input.Blur()
	m.phase = tuiPhaseOutput
	m.output = buf.String()
	if err != nil {
		m.errLine = err.Error()
	} else {
		m.errLine = ""
	}
	return m, nil
}

func (m *tasksInteractiveModel) finishInputWithError(msg string) {
	m.input.Blur()
	m.phase = tuiPhaseOutput
	m.output = ""
	m.errLine = msg
}

func (m *tasksInteractiveModel) returnFromError() {
	m.output = ""
	m.errLine = ""

	switch m.inputFor {
	case menuNew:
		m.phase = tuiPhaseInput
		m.inputCtx = ""
		m.input.Placeholder = "optional name"
		m.input.Focus()
	case menuStop:
		m.phase = tuiPhaseInput
		m.inputCtx = m.taskSelectionPrompt(false)
		m.input.Placeholder = "task ID"
		m.input.Focus()
	case menuShow:
		m.phase = tuiPhaseInput
		m.inputCtx = m.taskSelectionPrompt(true)
		m.input.Placeholder = "task ID"
		m.input.Focus()
	case menuTruncate:
		m.phase = tuiPhaseInput
		m.inputCtx = ""
		m.input.Placeholder = "type YES/Y/1 to confirm"
		m.input.Focus()
	default:
		m.phase = tuiPhaseMenu
	}
}

func (m *tasksInteractiveModel) taskSelectionPrompt(includeCompleted bool) string {
	var (
		query string
		title string
	)
	if includeCompleted {
		query = `SELECT id, task_name, end_time FROM tasks ORDER BY end_time IS NOT NULL, id`
		title = "Available tasks (running + completed)"
	} else {
		query = `SELECT id, task_name, end_time FROM tasks WHERE end_time IS NULL ORDER BY id`
		title = "Running tasks you can stop"
	}

	rows, err := m.db.Query(query)
	if err != nil {
		return tasksErr.Render("Failed loading tasks: " + err.Error())
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var (
			id      int
			name    sql.NullString
			endTime sql.NullTime
		)
		if err := rows.Scan(&id, &name, &endTime); err != nil {
			return tasksErr.Render("Failed reading tasks: " + err.Error())
		}

		label := strings.TrimSpace(name.String)
		if label == "" {
			label = "(unnamed)"
		}

		status := tasksMuted.Render("running")
		if endTime.Valid {
			status = tasksMuted.Render("completed")
		}
		lines = append(lines, fmt.Sprintf("  #%d  %s  [%s]", id, label, status))
	}
	if err := rows.Err(); err != nil {
		return tasksErr.Render("Failed iterating tasks: " + err.Error())
	}

	var b strings.Builder
	b.WriteString(tasksAccent.Render(title))
	b.WriteByte('\n')
	if len(lines) == 0 {
		b.WriteString(tasksMuted.Render("  (none)"))
		return b.String()
	}
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (m *tasksInteractiveModel) View() string {
	header := tasksBorder.Render(lipgloss.JoinVertical(lipgloss.Left,
		tasksAccent.Render("Tasks"),
		// tasksMuted.Render("Now: "+time.Now().Format("Mon 02 Jan 2006  15:04:05")),
		tasksMuted.Render("Now: "+time.Now().Format("Mon 02 Jan 2006, 03:04:05 PM")),
	))

	switch m.phase {
	case tuiPhaseMenu:
		help := tasksMuted.Render("↑/↓ move • 1–7 highlight • same key again or enter to open • q quit")
		return lipgloss.JoinVertical(lipgloss.Left, header, "", m.list.View(), "", help)

	case tuiPhaseInput:
		var prompt string
		switch m.inputFor {
		case menuNew:
			prompt = tasksMuted.Render("Task name")
		case menuStop:
			prompt = tasksMuted.Render("Task ID to stop")
		case menuShow:
			prompt = tasksMuted.Render("Task ID")
		case menuTruncate:
			prompt = tasksWarn.Render("Truncate all rows — type YES to confirm")
		default:
			prompt = ""
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			prompt,
			m.inputCtx,
			"",
			m.input.View(),
			"",
			tasksMuted.Render("<esc> cancel • <enter> submit"),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, "", body)

	case tuiPhaseOutput:
		var b strings.Builder
		if strings.TrimSpace(m.output) != "" {
			b.WriteString(m.output)
		}
		if m.errLine != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(tasksErr.Render(m.errLine))
		}
		if b.Len() == 0 {
			b.WriteString(tasksMuted.Render("(no output)"))
		}
		b.WriteString("\n\n")
		if m.errLine != "" {
			b.WriteString(tasksMuted.Render("error shown • <enter> return to action • <ctrl+c> quit"))
		} else {
			b.WriteString(tasksMuted.Render("any key to continue"))
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, "", b.String())
	}

	return ""
}

func taskInteractiveLoop(db *sql.DB) error {
	p := tea.NewProgram(newTasksInteractiveModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
