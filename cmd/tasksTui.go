package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	db          *sql.DB
	phase       tuiPhase
	menuEntries []menuEntry
	menuIndex   int
	inputFor    menuTag
	inputCtx    string
	inputValue  string
	output      string
	errLine     string
	width       int
	height      int
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
	items := make([]menuEntry, len(menuDefs))
	for i, d := range menuDefs {
		items[i] = menuEntry{
			title: fmt.Sprintf("%d. %s", i+1, d.title),
			desc:  d.desc,
			tag:   d.tag,
		}
	}

	return &tasksInteractiveModel{
		db:          db,
		phase:       tuiPhaseMenu,
		menuEntries: items,
	}
}

func (m *tasksInteractiveModel) Init() tea.Cmd {
	return func() tea.Msg {
		return tea.RequestWindowSize()
	}
}

func (m *tasksInteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
		kp, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return m, nil
		}
		switch kp.String() {
		case "ctrl+c", "esc":
			m.phase = tuiPhaseMenu
			return m, nil
		case "enter":
			return m.submitInput()
		case "backspace", "ctrl+h":
			m.inputValue = trimLastRune(m.inputValue)
			return m, nil
		}

		// Use Key.Text so space and other printables work; String() reports "space", not " ".
		k := kp.Key()
		if k.Text != "" {
			if len(m.inputValue)+len(k.Text) <= 512 {
				m.inputValue += k.Text
			}
			return m, nil
		}
		return m, nil

	case tuiPhaseMenu:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "enter":
				return m.menuEnter()
			case "up", "k":
				if len(m.menuEntries) > 0 {
					m.menuIndex = (m.menuIndex - 1 + len(m.menuEntries)) % len(m.menuEntries)
				}
				return m, nil
			case "down", "j":
				if len(m.menuEntries) > 0 {
					m.menuIndex = (m.menuIndex + 1) % len(m.menuEntries)
				}
				return m, nil
			}
			if key := km.String(); len(key) == 1 {
				c := key[0]
				n := len(m.menuEntries)
				if n > 0 && n <= 9 && c >= '1' && c <= byte('0'+n) {
					idx := int(c - '1')
					if m.menuIndex == idx {
						return m.menuEnter()
					}
					m.menuIndex = idx
					return m, nil
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *tasksInteractiveModel) viewHeader() string {
	return tasksBorder.Render(lipgloss.JoinVertical(lipgloss.Left,
		tasksAccent.Render("Tasks"),
		tasksMuted.Render("Now: "+time.Now().Format("Mon 02 Jan 2006, 03:04:05 PM")),
	))
}

func (m *tasksInteractiveModel) menuHelpLine() string {
	return tasksMuted.Render("↑/↓ move • 1–7 highlight • same key again or enter to open • q quit")
}

// layoutMenuListHeight is the max number of menu entries visible at once (each entry is two rows).
func (m *tasksInteractiveModel) layoutMenuListHeight(termH int) int {
	headerH := lipgloss.Height(m.viewHeader())
	helpH := lipgloss.Height(m.menuHelpLine())
	gap := 2 // JoinVertical "" between header, list, and help
	avail := termH - headerH - helpH - gap
	// Each menu entry is two lipgloss rows (title + description).
	maxEntries := avail / 2
	if maxEntries < 1 {
		return 1
	}
	return maxEntries
}

func (m *tasksInteractiveModel) placeInTerminal(content string, vPos lipgloss.Position) string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Left, vPos, content)
}

func (m *tasksInteractiveModel) menuEnter() (tea.Model, tea.Cmd) {
	if len(m.menuEntries) == 0 || m.menuIndex < 0 || m.menuIndex >= len(m.menuEntries) {
		return m, nil
	}
	e := m.menuEntries[m.menuIndex]

	switch e.tag {
	case menuQuit:
		return m, tea.Quit

	case menuNew:
		m.phase = tuiPhaseInput
		m.inputFor = menuNew
		m.inputCtx = ""
		m.inputValue = ""
		return m, nil

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
		m.inputValue = ""
		return m, nil

	case menuShow:
		m.phase = tuiPhaseInput
		m.inputFor = menuShow
		m.inputCtx = m.taskSelectionPrompt(true)
		m.inputValue = ""
		return m, nil

	case menuTruncate:
		m.phase = tuiPhaseInput
		m.inputFor = menuTruncate
		m.inputCtx = ""
		m.inputValue = ""
		return m, nil
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
		err = saveNewTask(m.db, strings.TrimSpace(m.inputValue), &buf)

	case menuStop:
		id, convErr := strconv.Atoi(strings.TrimSpace(m.inputValue))
		if convErr != nil {
			m.finishInputWithError("invalid task ID")
			return m, nil
		}
		err = stopTimer(m.db, id, &buf)

	case menuShow:
		id, convErr := strconv.Atoi(strings.TrimSpace(m.inputValue))
		if convErr != nil {
			m.finishInputWithError("invalid task ID")
			return m, nil
		}
		err = showSpecificTimer(m.db, id, &buf)

	case menuTruncate:
		// var answer any = strings.ToLower(strings.TrimSpace(m.input.Value()))
		// if answer == "yes" {
		allowedString := []string{"yes","y","1"}
		var answer string = m.inputValue
		if slices.Contains(allowedString, strings.ToLower(answer)) {
			err = truncateAllTasks(m.db, &buf)
		} else {
			fmt.Fprintln(&buf, tasksMuted.Render("Cancelled."))
		}

	default:
		m.phase = tuiPhaseMenu
		return m, nil
	}

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
		m.inputValue = ""
	case menuStop:
		m.phase = tuiPhaseInput
		m.inputCtx = m.taskSelectionPrompt(false)
		m.inputValue = ""
	case menuShow:
		m.phase = tuiPhaseInput
		m.inputCtx = m.taskSelectionPrompt(true)
		m.inputValue = ""
	case menuTruncate:
		m.phase = tuiPhaseInput
		m.inputCtx = ""
		m.inputValue = ""
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

func (m *tasksInteractiveModel) View() tea.View {
	header := m.viewHeader()
	wrap := func(content string) tea.View {
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	switch m.phase {
	case tuiPhaseMenu:
		stack := lipgloss.JoinVertical(lipgloss.Left, header, "", m.renderMenu(), "", m.menuHelpLine())
		return wrap(m.placeInTerminal(stack, lipgloss.Top))

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
			m.renderInput(),
			"",
			tasksMuted.Render("<esc> cancel • <enter> submit"),
		)
		stack := lipgloss.JoinVertical(lipgloss.Left, header, "", body)
		return wrap(m.placeInTerminal(stack, lipgloss.Top))

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
		stack := lipgloss.JoinVertical(lipgloss.Left, header, "", b.String())
		return wrap(m.placeInTerminal(stack, lipgloss.Top))
	}

	return wrap("")
}

func (m *tasksInteractiveModel) renderMenu() string {
	if len(m.menuEntries) == 0 {
		return tasksMuted.Render("(no actions)")
	}

	selectedTitle := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("62")).Foreground(lipgloss.Color("86")).Bold(true).Padding(0, 0, 0, 1)
	selectedDesc := selectedTitle.Copy().Foreground(lipgloss.Color("245")).Bold(false)
	normalTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 0, 0, 2)
	normalDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 0, 0, 2)

	maxRows := m.layoutMenuListHeight(m.height)
	if maxRows > len(m.menuEntries) {
		maxRows = len(m.menuEntries)
	}
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if m.menuIndex >= maxRows {
		start = m.menuIndex - maxRows + 1
	}
	end := min(start+maxRows, len(m.menuEntries))

	var rows []string
	for i := start; i < end; i++ {
		e := m.menuEntries[i]
		if i == m.menuIndex {
			rows = append(rows, selectedTitle.Render(e.title))
			rows = append(rows, selectedDesc.Render(e.desc))
		} else {
			rows = append(rows, normalTitle.Render(e.title))
			rows = append(rows, normalDesc.Render(e.desc))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m *tasksInteractiveModel) inputPlaceholder() string {
	switch m.inputFor {
	case menuNew:
		return "optional name"
	case menuStop, menuShow:
		return "task ID"
	case menuTruncate:
		return "type YES/Y/1 to confirm"
	default:
		return ""
	}
}

func (m *tasksInteractiveModel) renderInput() string {
	value := m.inputValue
	if value == "" {
		value = tasksMuted.Render(m.inputPlaceholder())
	}
	cursor := tasksAccent.Render("█")
	return tasksBorder.Render(value + cursor)
}

func trimLastRune(s string) string {
	if s == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

func taskInteractiveLoop(db *sql.DB) error {
	p := tea.NewProgram(newTasksInteractiveModel(db))
	_, err := p.Run()
	return err
}
