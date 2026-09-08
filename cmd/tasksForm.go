package cmd

import (
	"fmt"
	"strings"

	"barbtils/internal/database"
	"barbtils/internal/tasks"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/google/uuid"
)

// maxTagLen mirrors tags varchar(55)[] in the schema, so an over-long tag is
// reported here rather than as a database error.
const maxTagLen = 55

type formField int

const (
	fieldName formField = iota
	fieldType
	fieldPriority
	fieldCategory
	fieldTags
	fieldCount
)

// taskForm edits every configurable property of a task. It backs both creating
// a task and changing an existing one, so the two can never drift apart.
type taskForm struct {
	editing bool
	taskID  uuid.UUID
	taskSeq int64

	name textinput.Model
	tags textinput.Model

	typeIdx   int
	prioIdx   int
	cats      map[database.TasksCategories]bool
	catCursor int

	focus formField
	err   string
}

func newForm() *taskForm {
	name := textinput.New()
	name.Prompt = ""
	name.Placeholder = "task name"
	name.CharLimit = 512
	name.SetWidth(46)

	tags := textinput.New()
	tags.Prompt = ""
	tags.Placeholder = "comma, separated, tags"
	tags.CharLimit = 512
	tags.SetWidth(46)

	return &taskForm{
		name: name,
		tags: tags,
		cats: map[database.TasksCategories]bool{},
	}
}

// createForm starts a blank form at the schema's defaults.
func createForm() *taskForm {
	f := newForm()
	f.typeIdx = indexOfType(database.TasksTypesOneTime)
	f.prioIdx = indexOfPriority(database.TasksPrioritiesUnknown)
	return f
}

// editForm pre-fills from an existing task.
func editForm(r tasks.TaskRow) *taskForm {
	f := newForm()
	f.editing = true
	f.taskID, f.taskSeq = r.ID, r.Seq
	f.name.SetValue(r.Name)
	f.tags.SetValue(strings.Join(r.Tags, ", "))
	f.typeIdx = indexOfType(r.Type)
	f.prioIdx = indexOfPriority(r.Priority)
	for _, c := range r.Category {
		if c != database.TasksCategoriesUnknown {
			f.cats[c] = true
		}
	}
	return f
}

func indexOfType(v database.TasksTypes) int {
	for i, t := range tasks.AllTypes() {
		if t == v {
			return i
		}
	}
	return 0
}

func indexOfPriority(v database.TasksPriorities) int {
	for i, p := range tasks.AllPriorities() {
		if p == v {
			return i
		}
	}
	return 0
}

func (f *taskForm) selectedType() database.TasksTypes {
	return tasks.AllTypes()[f.typeIdx]
}

func (f *taskForm) selectedPriority() database.TasksPriorities {
	return tasks.AllPriorities()[f.prioIdx]
}

// selectedCategories returns the ticked categories in schema order. Nothing
// ticked means {Unknown}, which is the column default.
func (f *taskForm) selectedCategories() []database.TasksCategories {
	out := []database.TasksCategories{}
	for _, c := range tasks.AllCategories() {
		if c == database.TasksCategoriesUnknown {
			continue
		}
		if f.cats[c] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return []database.TasksCategories{database.TasksCategoriesUnknown}
	}
	return out
}

// parsedTags splits the tag field, trimming, dropping blanks and de-duplicating
// case-insensitively.
func (f *taskForm) parsedTags() ([]string, error) {
	raw := strings.Split(f.tags.Value(), ",")
	seen := map[string]bool{}
	out := []string{}
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len(t) > maxTagLen {
			return nil, fmt.Errorf("tag %q is longer than %d characters", t, maxTagLen)
		}
		if k := strings.ToLower(t); !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *taskForm) focusCurrent() tea.Cmd {
	f.name.Blur()
	f.tags.Blur()
	switch f.focus {
	case fieldName:
		return f.name.Focus()
	case fieldTags:
		return f.tags.Focus()
	}
	return nil
}

func (f *taskForm) move(delta int) tea.Cmd {
	f.focus = formField((int(f.focus) + delta + int(fieldCount)) % int(fieldCount))
	return f.focusCurrent()
}

// Update handles one message. It returns whether the form consumed it, so the
// caller knows not to treat the key as a list shortcut.
func (f *taskForm) Update(msg tea.Msg) (tea.Cmd, bool) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		switch f.focus {
		case fieldName:
			f.name, cmd = f.name.Update(msg)
		case fieldTags:
			f.tags, cmd = f.tags.Update(msg)
		}
		return cmd, false
	}

	switch kp.String() {
	case "tab", "down":
		return f.move(1), true
	case "shift+tab", "up":
		return f.move(-1), true
	}

	// Enum fields are cycled; text fields keep every other key.
	switch f.focus {
	case fieldType:
		switch kp.String() {
		case "left", "h":
			f.typeIdx = (f.typeIdx - 1 + len(tasks.AllTypes())) % len(tasks.AllTypes())
			return nil, true
		case "right", "l", " ", "space":
			f.typeIdx = (f.typeIdx + 1) % len(tasks.AllTypes())
			return nil, true
		}
	case fieldPriority:
		switch kp.String() {
		case "left", "h":
			f.prioIdx = (f.prioIdx - 1 + len(tasks.AllPriorities())) % len(tasks.AllPriorities())
			return nil, true
		case "right", "l":
			f.prioIdx = (f.prioIdx + 1) % len(tasks.AllPriorities())
			return nil, true
		}
	case fieldCategory:
		pick := f.pickableCategories()
		switch kp.String() {
		case "left", "h":
			f.catCursor = (f.catCursor - 1 + len(pick)) % len(pick)
			return nil, true
		case "right", "l":
			f.catCursor = (f.catCursor + 1) % len(pick)
			return nil, true
		// bubbletea reports the space key as "space", not " ".
		case " ", "space", "x":
			c := pick[f.catCursor]
			f.cats[c] = !f.cats[c]
			return nil, true
		}
	}

	var cmd tea.Cmd
	switch f.focus {
	case fieldName:
		f.name, cmd = f.name.Update(msg)
		return cmd, true
	case fieldTags:
		f.tags, cmd = f.tags.Update(msg)
		return cmd, true
	}
	return nil, false
}

// pickableCategories excludes Unknown, which is the absence of a category
// rather than one you choose.
func (f *taskForm) pickableCategories() []database.TasksCategories {
	out := make([]database.TasksCategories, 0, len(tasks.AllCategories()))
	for _, c := range tasks.AllCategories() {
		if c != database.TasksCategoriesUnknown {
			out = append(out, c)
		}
	}
	return out
}

func (f *taskForm) newTaskInput() (tasks.NewTaskInput, error) {
	tags, err := f.parsedTags()
	if err != nil {
		return tasks.NewTaskInput{}, err
	}
	name := strings.TrimSpace(f.name.Value())
	if name == "" {
		return tasks.NewTaskInput{}, tasks.ErrNameEmpty
	}
	t, p := f.selectedType(), f.selectedPriority()
	return tasks.NewTaskInput{
		Name: name, Type: t, Priority: p,
		Category: f.selectedCategories(), Tags: tags,
	}, nil
}

func (f *taskForm) metaPatch() (tasks.MetaPatch, error) {
	tags, err := f.parsedTags()
	if err != nil {
		return tasks.MetaPatch{}, err
	}
	t, p := f.selectedType(), f.selectedPriority()
	return tasks.MetaPatch{
		Type: &t, Priority: &p,
		Category: f.selectedCategories(), Tags: tags,
	}, nil
}

// ---- rendering ----

func (f *taskForm) View() string {
	label := func(field formField, text string) string {
		if f.focus == field {
			return tasksAccent.Render("▸ " + text)
		}
		return tasksMuted.Render("  " + text)
	}

	row := func(field formField, name, value string) string {
		return label(field, fmt.Sprintf("%-9s", name)) + " " + value
	}

	title := tasksAccent.Render("New task")
	if f.editing {
		title = tasksAccent.Render(fmt.Sprintf("Edit #%d", f.taskSeq))
	}

	lines := []string{
		title,
		"",
		row(fieldName, "Name", f.name.View()),
		row(fieldType, "Type", f.renderChoice(f.focus == fieldType, string(f.selectedType()))),
		row(fieldPriority, "Priority", f.renderChoice(f.focus == fieldPriority, string(f.selectedPriority()))),
		row(fieldCategory, "Category", f.renderCategories()),
		row(fieldTags, "Tags", f.tags.View()),
	}
	if f.err != "" {
		lines = append(lines, "", tasksErr.Render(f.err))
	}

	var hint string
	switch f.focus {
	case fieldCategory:
		hint = "←/→ pick • space toggle • tab next • enter save • esc cancel"
	case fieldType, fieldPriority:
		hint = "←/→ change • tab next • enter save • esc cancel"
	default:
		hint = "tab next field • enter save • esc cancel"
	}
	lines = append(lines, "", tasksMuted.Render(hint))

	return tasksBorder.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (f *taskForm) renderChoice(focused bool, value string) string {
	if focused {
		return tasksMuted.Render("‹ ") + tasksHeader.Render(value) + tasksMuted.Render(" ›")
	}
	return "  " + value + "  "
}

func (f *taskForm) renderCategories() string {
	pick := f.pickableCategories()
	parts := make([]string, 0, len(pick))
	for i, c := range pick {
		box := "☐"
		if f.cats[c] {
			box = "☑"
		}
		cell := box + " " + string(c)
		switch {
		case f.focus == fieldCategory && i == f.catCursor:
			cell = tasksHeader.Render("[" + cell + "]")
		case f.cats[c]:
			cell = tasksOK.Render(" " + cell + " ")
		default:
			cell = tasksMuted.Render(" " + cell + " ")
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, " ")
}
