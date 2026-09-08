package cmd

import (
	"strings"
	"testing"
	"time"

	"barbtils/internal/database"
	"barbtils/internal/tasks"

	tea "charm.land/bubbletea/v2"

	"github.com/google/uuid"
)

func testModel(rows []tasks.TaskRow, now time.Time) *tasksTuiModel {
	m := newTasksTuiModel(nil, nil)
	m.rows = rows
	m.now = now
	m.lastRefresh = now // keep the periodic resync (which needs a store) out of the way
	m.width, m.height = 100, 30
	return m
}

func sampleRows(now time.Time) []tasks.TaskRow {
	started := now.Add(-10 * time.Second)
	return []tasks.TaskRow{
		{
			Seq: 1, Name: "leetcode practice", Type: database.TasksTypesRecurring,
			Status: database.TasksStatusesInProgress, SessionCount: 3,
			ClosedDuration: time.Minute, OpenStartedAt: &started,
		},
		{
			Seq: 2, Name: "write docs", Type: database.TasksTypesOneTime,
			Status: database.TasksStatusesPaused, SessionCount: 1,
			ClosedDuration: 30 * time.Second,
		},
	}
}

// A tick must re-arm itself. tea.Tick fires once, so a missing re-arm leaves
// the elapsed column frozen after a single second.
func TestTickReArmsItself(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	_, cmd := m.Update(tickMsg(now.Add(time.Second)))
	if cmd == nil {
		t.Fatal("tick produced no command, so the clock would stop")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatal("tick did not schedule another tick")
	}
	if !m.now.Equal(now.Add(time.Second)) {
		t.Fatalf("tick did not advance the model clock: %v", m.now)
	}
}

// Ticking must move a running task's total and leave a paused one alone,
// without any database access.
func TestTickAdvancesOnlyRunningTasks(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	runningBefore := m.rows[0].Elapsed(m.now)
	pausedBefore := m.rows[1].Elapsed(m.now)

	m.Update(tickMsg(now.Add(5 * time.Second)))

	if got := m.rows[0].Elapsed(m.now); got != runningBefore+5*time.Second {
		t.Fatalf("running total did not advance by 5s: %v -> %v", runningBefore, got)
	}
	if got := m.rows[1].Elapsed(m.now); got != pausedBefore {
		t.Fatalf("paused total moved: %v -> %v", pausedBefore, got)
	}
}

func TestListNavigationAndDrillDown(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.cursor != 1 {
		t.Fatalf("j should move down, cursor = %d", m.cursor)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.cursor != 0 {
		t.Fatalf("j should wrap to the top, cursor = %d", m.cursor)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.view != viewSessions || m.selTask == nil || m.selTask.Seq != 1 {
		t.Fatalf("enter should drill into the selected task's sessions (view=%v)", m.view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.view != viewList {
		t.Fatal("esc should return to the list")
	}
}

// Guards must refuse impossible actions locally instead of firing a doomed
// query: completing a recurring task, pausing a stopped one, starting a
// running one.
func TestActionGuards(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	m.cursor = 0 // recurring + running
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}); cmd != nil {
		t.Fatal("completing a recurring task should be refused locally")
	}
	if !strings.Contains(m.status, "recurring") {
		t.Fatalf("expected a recurring explanation, got %q", m.status)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"}); cmd != nil {
		t.Fatal("starting an already-running task should be refused locally")
	}

	m.cursor = 1 // one-time + paused
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"}); cmd != nil {
		t.Fatal("pausing a task with no open session should be refused locally")
	}
	if !strings.Contains(m.status, "not running") {
		t.Fatalf("expected a not-running explanation, got %q", m.status)
	}
}

func TestNewOpensConfigurableForm(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.view != viewForm || m.form == nil {
		t.Fatal("n should open the task form")
	}
	// Typing goes through the textinput component, not a hand-rolled buffer.
	for _, r := range "kixmon v2" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := m.form.name.Value(); got != "kixmon v2" {
		t.Fatalf("name = %q; spaces and text must reach the component", got)
	}
	if m.form.name.Prompt != "" {
		t.Fatalf("Prompt is the display prefix, not the value; got %q", m.form.name.Prompt)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.view != viewList || m.form != nil {
		t.Fatal("esc should cancel the form")
	}
}

func TestEditFormPreFillsFromTask(t *testing.T) {
	row := tasks.TaskRow{
		ID: uuid.New(), Seq: 7, Name: "leetcode practice",
		Type:     database.TasksTypesRecurring,
		Priority: database.TasksPrioritiesHigh,
		Tags:     []string{"api", "urgent"},
		Category: []database.TasksCategories{database.TasksCategoriesClientProject},
	}
	f := editForm(row)

	if !f.editing || f.taskSeq != 7 {
		t.Fatal("edit form must know which task it edits")
	}
	if f.name.Value() != "leetcode practice" {
		t.Fatalf("name not pre-filled: %q", f.name.Value())
	}
	if f.selectedType() != database.TasksTypesRecurring {
		t.Fatalf("type not pre-filled: %v", f.selectedType())
	}
	if f.selectedPriority() != database.TasksPrioritiesHigh {
		t.Fatalf("priority not pre-filled: %v", f.selectedPriority())
	}
	if f.tags.Value() != "api, urgent" {
		t.Fatalf("tags not pre-filled: %q", f.tags.Value())
	}
	cats := f.selectedCategories()
	if len(cats) != 1 || cats[0] != database.TasksCategoriesClientProject {
		t.Fatalf("category not pre-filled: %v", cats)
	}

	// A round trip through the form must not alter anything untouched.
	patch, err := f.metaPatch()
	if err != nil {
		t.Fatal(err)
	}
	if *patch.Type != database.TasksTypesRecurring || *patch.Priority != database.TasksPrioritiesHigh {
		t.Fatal("patch changed values the user never touched")
	}
	if strings.Join(patch.Tags, ",") != "api,urgent" {
		t.Fatalf("patch tags = %v", patch.Tags)
	}
}

func TestFormCyclesEnumsAndTogglesCategories(t *testing.T) {
	f := createForm()

	// Defaults come from the schema.
	if f.selectedType() != database.TasksTypesOneTime {
		t.Fatalf("default type = %v, want One Time", f.selectedType())
	}
	if f.selectedPriority() != database.TasksPrioritiesUnknown {
		t.Fatalf("default priority = %v, want Unknown", f.selectedPriority())
	}

	f.focus = fieldType
	f.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if f.selectedType() != database.TasksTypesRecurring {
		t.Fatalf("right should cycle type, got %v", f.selectedType())
	}
	f.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if f.selectedType() != database.TasksTypesOneTime {
		t.Fatal("type should wrap around")
	}

	// Every priority in the schema must be reachable.
	f.focus = fieldPriority
	seen := map[database.TasksPriorities]bool{}
	for range tasks.AllPriorities() {
		seen[f.selectedPriority()] = true
		f.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	}
	if len(seen) != len(tasks.AllPriorities()) {
		t.Fatalf("only %d of %d priorities reachable", len(seen), len(tasks.AllPriorities()))
	}

	// Categories are multi-select; nothing ticked means {Unknown}.
	f.focus = fieldCategory
	if got := f.selectedCategories(); len(got) != 1 || got[0] != database.TasksCategoriesUnknown {
		t.Fatalf("untouched categories should be {Unknown}, got %v", got)
	}
	f.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	f.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	f.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if got := f.selectedCategories(); len(got) != 2 {
		t.Fatalf("two categories should be selected, got %v", got)
	}
	// Toggling off again removes it.
	f.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if got := f.selectedCategories(); len(got) != 1 {
		t.Fatalf("toggle should deselect, got %v", got)
	}
}

func TestFormTagParsing(t *testing.T) {
	f := createForm()
	f.tags.SetValue("  api ,, URGENT, api ,billing  ")
	got, err := f.parsedTags()
	if err != nil {
		t.Fatal(err)
	}
	// Blanks dropped, duplicates removed case-insensitively, order kept.
	if strings.Join(got, "|") != "api|URGENT|billing" {
		t.Fatalf("parsed tags = %v", got)
	}

	f.tags.SetValue(strings.Repeat("x", maxTagLen+1))
	if _, err := f.parsedTags(); err == nil {
		t.Fatalf("a tag longer than %d chars must be rejected before it reaches the database", maxTagLen)
	}
}

func TestFormRequiresName(t *testing.T) {
	f := createForm()
	f.tags.SetValue("api")
	if _, err := f.newTaskInput(); err == nil {
		t.Fatal("a task with no name must be refused")
	}
	f.name.SetValue("  ok  ")
	in, err := f.newTaskInput()
	if err != nil {
		t.Fatal(err)
	}
	if in.Name != "ok" {
		t.Fatalf("name should be trimmed, got %q", in.Name)
	}
}

func TestFormNavigationVisitsEveryField(t *testing.T) {
	f := createForm()
	seen := map[formField]bool{f.focus: true}
	for i := 0; i < int(fieldCount); i++ {
		f.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		seen[f.focus] = true
	}
	if len(seen) != int(fieldCount) {
		t.Fatalf("tab reached %d of %d fields", len(seen), fieldCount)
	}
	if f.focus != fieldName {
		t.Fatal("tab should wrap back to the first field")
	}
}

func TestViewRendersTasksAndSurvivesEmptyList(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)

	out := m.View().Content
	for _, want := range []string{"leetcode practice", "write docs", "TOTAL", "Recurring"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered view missing %q", want)
		}
	}

	m.rows = nil
	if out := m.View().Content; !strings.Contains(out, "no tasks") {
		t.Fatal("empty list should invite creating a task")
	}
}

func TestArchiveSessionsGuardedWhenNothingToBank(t *testing.T) {
	now := time.Now()
	rows := sampleRows(now)
	rows[1].SessionCount = 0 // nothing finished to archive
	m := testModel(rows, now)
	m.cursor = 1

	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'A', Text: "A"}); cmd != nil {
		t.Fatal("archiving with no finished sessions should be refused locally")
	}
	if !strings.Contains(m.status, "no finished sessions") {
		t.Fatalf("expected an explanation, got %q", m.status)
	}

	// With sessions present the action is dispatched.
	m.cursor = 0
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'A', Text: "A"}); cmd == nil {
		t.Fatal("archiving a task with finished sessions should dispatch")
	}
}

func TestSessionsViewTogglesArchived(t *testing.T) {
	now := time.Now()
	m := testModel(sampleRows(now), now)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // drill in
	if m.view != viewSessions {
		t.Fatal("enter should open the sessions view")
	}
	if m.showArchived {
		t.Fatal("archived sessions should be hidden by default")
	}
	m.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	if !m.showArchived {
		t.Fatal("A should reveal archived sessions")
	}
	m.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	if m.showArchived {
		t.Fatal("A should toggle back")
	}
}

func TestSessionsViewShowsLifetimeOnlyWhenArchived(t *testing.T) {
	now := time.Now()
	rows := sampleRows(now)
	m := testModel(rows, now)
	m.selTask = &m.rows[0]
	m.view = viewSessions
	m.sessions = []tasks.SessionRow{}

	if strings.Contains(m.View().Content, "lifetime") {
		t.Fatal("a task with no archive should not mention lifetime")
	}

	m.selTask.ArchivedSessionCount = 2
	m.selTask.ArchivedDuration = 5 * time.Minute
	if !strings.Contains(m.View().Content, "lifetime") {
		t.Fatal("a rolled-over task should show its lifetime total")
	}
}
