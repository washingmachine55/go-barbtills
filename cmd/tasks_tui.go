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
	output   string
	errLine  string
	width    int
	height   int
}

func newTasksInteractiveModel(db *sql.DB) *tasksInteractiveModel {
	items := []list.Item{
		menuEntry{title: "Create new task", desc: "Start a timer (name optional)", tag: menuNew},
		menuEntry{title: "List running timers", desc: "Tasks still open", tag: menuListRunning},
		menuEntry{title: "List completed timers", desc: "Tasks with an end time", tag: menuListCompleted},
		menuEntry{title: "Stop a running timer", desc: "Enter task ID", tag: menuStop},
		menuEntry{title: "Show task details", desc: "Enter task ID", tag: menuShow},
		menuEntry{title: "Truncate all tasks", desc: "Deletes every row — needs YES", tag: menuTruncate},
		menuEntry{title: "Quit", desc: "Leave interactive mode", tag: menuQuit},
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
		if _, ok := msg.(tea.KeyMsg); ok {
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
		m.input.SetValue("")
		m.input.Placeholder = "optional name"
		m.input.Focus()
		return m, textinput.Blink

	case menuListRunning:
		return m.runAndShow(func(w *bytes.Buffer) error {
			return getAllRunningTimers(m.db, false, w)
		})

	case menuListCompleted:
		return m.runAndShow(func(w *bytes.Buffer) error {
			return getAllRunningTimers(m.db, true, w)
		})

	case menuStop:
		m.phase = tuiPhaseInput
		m.inputFor = menuStop
		m.input.SetValue("")
		m.input.Placeholder = "task ID"
		m.input.Focus()
		return m, textinput.Blink

	case menuShow:
		m.phase = tuiPhaseInput
		m.inputFor = menuShow
		m.input.SetValue("")
		m.input.Placeholder = "task ID"
		m.input.Focus()
		return m, textinput.Blink

	case menuTruncate:
		m.phase = tuiPhaseInput
		m.inputFor = menuTruncate
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

func (m *tasksInteractiveModel) View() string {
	header := tasksBorder.Render(lipgloss.JoinVertical(lipgloss.Left,
		tasksAccent.Render("Tasks"),
		// tasksMuted.Render("Now: "+time.Now().Format("Mon 02 Jan 2006  15:04:05")),
		tasksMuted.Render("Now: "+time.Now().Format("Mon 02 Jan 2006, 03:04:05 PM")),
	))

	switch m.phase {
	case tuiPhaseMenu:
		help := tasksMuted.Render("↑/↓ move • enter select • q quit")
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
			m.input.View(),
			"",
			tasksMuted.Render("esc cancel • enter submit"),
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
		b.WriteString(tasksMuted.Render("any key to continue"))
		return lipgloss.JoinVertical(lipgloss.Left, header, "", b.String())
	}

	return ""
}

func taskInteractiveLoop(db *sql.DB) error {
	p := tea.NewProgram(newTasksInteractiveModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
