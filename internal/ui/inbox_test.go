package ui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/actuallyakshat/tars/internal/store"
)

// fixedNow is a Friday; due dates in tests are relative to it.
var fixedNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local)

func day(offset int) string { return fixedNow.AddDate(0, 0, offset).Format("2006-01-02") }

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func newTestInbox(t *testing.T, w, h int, tasks ...*store.Task) (*inbox, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for _, task := range tasks {
		if task.Priority == 0 && task.Title[0] != '!' {
			task.Priority = 2 // store default is 0; tests mean "normal" unless they say otherwise
		}
		task.Title = strings.TrimPrefix(task.Title, "!")
		if err := st.CreateTask(task); err != nil {
			t.Fatal(err)
		}
	}
	m, err := newInbox(st)
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return fixedNow }
	m.rebuild(0)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m, st
}

func press(m *inbox, keys ...string) {
	for _, k := range keys {
		m.Update(key(k))
	}
}

// typeText sends text as one paste-like key event, as a terminal would.
func typeText(m *inbox, text string) { m.Update(key(text)) }

func get(t *testing.T, st *store.Store, id int64) *store.Task {
	t.Helper()
	task, err := st.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestEmptyInbox(t *testing.T) {
	m, _ := newTestInbox(t, 100, 30)
	press(m, "down", "enter", "x", "s", "1", "p", "1", "n", "u", "l", "g", "tab")
	if !strings.Contains(m.View(), "TARS") {
		t.Fatal("header missing")
	}
	m.setView(viewOpen)
	if !strings.Contains(m.View(), "No open tasks. Press c to add one.") {
		t.Fatalf("empty state missing:\n%s", m.View())
	}
}

func TestGroupsByPriorityByDefault(t *testing.T) {
	m, _ := newTestInbox(t, 120, 30,
		&store.Task{Title: "low one", Priority: 3},
		&store.Task{Title: "!urgent one", Priority: 0},
		&store.Task{Title: "normal one"},
	)
	v := m.View()
	u, n, l := strings.Index(v, "URGENT · 1"), strings.Index(v, "NORMAL · 1"), strings.Index(v, "LOW · 1")
	if u < 0 || n < 0 || l < 0 || !(u < n && n < l) {
		t.Fatalf("priority groups wrong:\n%s", v)
	}
	if strings.Contains(v, "HIGH") {
		t.Error("empty group rendered")
	}
	if m.selected().Title != "urgent one" {
		t.Errorf("cursor should start on the first task, got %q", m.selected().Title)
	}
}

func TestGroupingCycles(t *testing.T) {
	m, _ := newTestInbox(t, 120, 30,
		&store.Task{Title: "late", Due: day(-2), Repo: "billing", Status: store.StatusReady},
		&store.Task{Title: "today", Due: day(0)},
		&store.Task{Title: "someday", Status: store.StatusInReview},
	)
	press(m, "g")
	v := m.View()
	if !(strings.Index(v, "OVERDUE") < strings.Index(v, "TODAY") && strings.Index(v, "TODAY") < strings.Index(v, "NO DATE")) {
		t.Fatalf("due grouping:\n%s", v)
	}
	press(m, "g")
	v = m.View()
	if !(strings.Index(v, "IN REVIEW") < strings.Index(v, "READY") && strings.Index(v, "READY") < strings.Index(v, "INBOX")) {
		t.Fatalf("status grouping:\n%s", v)
	}
	press(m, "g")
	if v = m.View(); !strings.Contains(v, "BILLING · 1") || !strings.Contains(v, "NO REPO · 2") {
		t.Fatalf("repo grouping:\n%s", v)
	}
	press(m, "g")
	for _, r := range m.rows {
		if r.task == nil {
			t.Fatalf("grouping none still has header %q", r.header)
		}
	}
	press(m, "g")
	if m.group != byPriority {
		t.Fatal("grouping should cycle back to priority")
	}
}

func TestViewsAndCounts(t *testing.T) {
	m, _ := newTestInbox(t, 140, 30,
		&store.Task{Title: "in inbox"},
		&store.Task{Title: "ready one", Status: store.StatusReady},
		&store.Task{Title: "finished", Status: store.StatusDone},
	)
	top := m.topBar()
	for _, want := range []string{"Open 2", "Inbox 1", "Ready 1", "Review 0", "Blocked 0", "Done 1"} {
		if !strings.Contains(top, want) {
			t.Errorf("top bar missing %q: %s", want, top)
		}
	}
	if strings.Contains(m.View(), "finished") {
		t.Error("done task shown in Open")
	}
	press(m, "tab")
	if v := m.View(); !strings.Contains(v, "in inbox") || strings.Contains(v, "ready one") {
		t.Fatalf("tab should switch to Inbox:\n%s", v)
	}
	press(m, "7")
	if !strings.Contains(m.View(), "finished") {
		t.Fatal("7 should show Done")
	}
}

func TestEnterOpensDetailAdaptively(t *testing.T) {
	tasks := func() []*store.Task {
		return []*store.Task{
			{Title: "first task", Body: "first body text"},
			{Title: "second task"},
		}
	}

	wide, _ := newTestInbox(t, 160, 30, tasks()...)
	press(wide, "enter")
	if !wide.split() {
		t.Fatal("wide terminal should split")
	}
	v := wide.View()
	if !strings.Contains(v, "second task") || !strings.Contains(v, "first body text") {
		t.Fatalf("split should show list and detail:\n%s", v)
	}
	press(wide, "down") // detail follows the cursor
	if wide.detailTask == nil || wide.detailTask.Title != "second task" {
		t.Fatal("detail should follow the cursor in split view")
	}

	narrow, _ := newTestInbox(t, 100, 30, tasks()...)
	press(narrow, "enter")
	if narrow.split() {
		t.Fatal("narrow terminal should not split")
	}
	v = narrow.View()
	if !strings.Contains(v, "first body text") || strings.Contains(v, "second task") {
		t.Fatalf("narrow detail should be full-screen:\n%s", v)
	}
	press(narrow, "j") // j/k step through tasks in full-screen detail
	if narrow.detailTask.Title != "second task" {
		t.Fatal("j should open the next task")
	}
	press(narrow, "esc")
	if narrow.detail {
		t.Fatal("esc should close the detail")
	}
}

func TestActivityCollapsedByDefault(t *testing.T) {
	m, st := newTestInbox(t, 100, 40, &store.Task{Title: "with log"})
	st.AddLog(1, "user", "note", "oldest entry")
	st.AddLog(1, "agent", "sweep", "newest entry")
	m.reload()
	press(m, "enter")
	v := m.View()
	if !strings.Contains(v, "▸ ACTIVITY") || !strings.Contains(v, "2 entries") || !strings.Contains(v, "last: agent · sweep") {
		t.Fatalf("collapsed summary missing:\n%s", v)
	}
	if strings.Contains(v, "oldest entry") {
		t.Fatal("activity should be collapsed by default")
	}
	press(m, "l")
	v = m.View()
	if !strings.Contains(v, "▾ ACTIVITY") || strings.Index(v, "newest entry") > strings.Index(v, "oldest entry") {
		t.Fatalf("expanded activity should list newest first:\n%s", v)
	}
}

func TestUnreadMarker(t *testing.T) {
	m, st := newTestInbox(t, 100, 30, &store.Task{Title: "agent touched"}, &store.Task{Title: "user touched"})
	st.AddLog(1, "agent", "sweep", "done, tests green")
	st.AddLog(2, "user", "note", "my own note")
	m.reload()

	if !get(t, st, 1).Unread || get(t, st, 2).Unread {
		t.Fatal("only agent activity should mark a task unread")
	}
	if !strings.Contains(m.topBar(), "Open 2 ●") {
		t.Fatalf("top bar should flag unread: %s", m.topBar())
	}
	press(m, "enter") // opening marks it seen
	if get(t, st, 1).Unread {
		t.Fatal("opening should mark the task seen")
	}
	if strings.Contains(m.topBar(), "●") {
		t.Fatalf("unread marker should clear: %s", m.topBar())
	}
}

func TestDoneAndUndo(t *testing.T) {
	m, st := newTestInbox(t, 100, 30,
		&store.Task{Title: "finish me", Status: store.StatusBlocked, WaitingOn: "Rahul"},
		&store.Task{Title: "next"},
	)
	press(m, "x")
	if task := get(t, st, 1); task.Status != store.StatusDone || task.WaitingOn != "" {
		t.Fatalf("x should mark done and clear blocker: %+v", task)
	}
	if m.selected().Title != "next" {
		t.Fatalf("cursor should land on the next task, got %q", m.selected().Title)
	}
	if !strings.Contains(m.footer(), "u to undo") {
		t.Fatalf("footer should offer undo: %s", m.footer())
	}
	press(m, "u")
	task := get(t, st, 1)
	if task.Status != store.StatusBlocked || task.WaitingOn != "Rahul" || task.DoneAt != "" {
		t.Fatalf("undo should restore everything: %+v", task)
	}
	if last := task.Log[len(task.Log)-1]; last.Kind != "undo" {
		t.Fatalf("undo not logged: %+v", last)
	}
	press(m, "u")
	if !strings.Contains(m.footer(), "Nothing to undo") {
		t.Fatal("second undo should be a no-op")
	}
}

func TestStatusAndPriorityPickers(t *testing.T) {
	m, st := newTestInbox(t, 140, 30, &store.Task{Title: "pick"})
	press(m, "s")
	if !strings.Contains(m.footer(), "Move #1 to") {
		t.Fatalf("status picker not shown: %s", m.footer())
	}
	press(m, "4")
	if get(t, st, 1).Status != store.StatusInReview {
		t.Fatal("s 4 should move to in-review")
	}
	press(m, "p", "1")
	if get(t, st, 1).Priority != 0 {
		t.Fatal("p 1 should set urgent")
	}
	press(m, "p", "esc")
	if m.mode != modeList || get(t, st, 1).Priority != 0 {
		t.Fatal("esc should cancel the picker")
	}
}

func TestQuickAdd(t *testing.T) {
	m, st := newTestInbox(t, 120, 30)
	press(m, "3") // from Inbox view
	press(m, "c")
	typeText(m, "!high")
	press(m, "enter")
	if m.mode != modeAdd || !m.flashErr {
		t.Fatal("invalid input should keep the prompt open with an error")
	}
	press(m, "esc", "c")
	typeText(m, "Ship webhooks BILL-9 !high @web #api due:tomorrow")
	press(m, "enter")
	if m.mode != modeList {
		t.Fatal("prompt should close after adding")
	}
	task := get(t, st, 1)
	if task.Title != "Ship webhooks" || task.Jira != "BILL-9" || task.Priority != 1 ||
		task.Repo != "web" || len(task.Tags) != 1 || task.Due == "" {
		t.Fatalf("quick add parsed wrong: %+v", task)
	}
	if m.selected() == nil || m.selected().ID != 1 {
		t.Fatal("cursor should land on the new task")
	}
}

func TestNoteAndSearch(t *testing.T) {
	m, st := newTestInbox(t, 120, 30, &store.Task{Title: "fix export"}, &store.Task{Title: "write docs", Jira: "DOC-1"})
	press(m, "n")
	typeText(m, "root cause found")
	press(m, "enter")
	if log := get(t, st, 1).Log; len(log) != 1 || log[0].Text != "root cause found" || log[0].Author != "user" {
		t.Fatalf("note: %+v", log)
	}

	press(m, "/")
	typeText(m, "doc-1")
	if v := m.View(); strings.Contains(v, "fix export") || !strings.Contains(v, "write docs") {
		t.Fatalf("search should filter live, Jira included:\n%s", v)
	}
	press(m, "enter")
	if m.query != "doc-1" || m.mode != modeList {
		t.Fatal("enter should keep the filter")
	}
	press(m, "esc")
	if m.query != "" || !strings.Contains(m.View(), "fix export") {
		t.Fatal("esc should clear the filter")
	}
}

func TestEveryLineFitsTheTerminal(t *testing.T) {
	var tasks []*store.Task
	for i := 0; i < 40; i++ {
		tasks = append(tasks, &store.Task{
			Title: strings.Repeat("long title words ", i%6+1), Jira: "VERYLONGPROJ-12345",
			Repo: "a-really-long-repo-name", Due: day(i%9 - 3), Status: store.Statuses[i%6],
			Body: strings.Repeat("body ", 80),
		})
	}
	for _, size := range [][2]int{{50, 12}, {80, 24}, {119, 30}, {120, 30}, {200, 50}} {
		m, _ := newTestInbox(t, size[0], size[1], tasks...)
		for _, keys := range [][]string{{}, {"down", "down", "down"}, {"enter"}, {"enter", "l"}, {"?"}, {"g"}, {"7"}} {
			press(m, keys...)
			v := m.View()
			lines := strings.Split(v, "\n")
			if len(lines) > size[1] {
				t.Errorf("%dx%d after %v: %d lines", size[0], size[1], keys, len(lines))
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > size[0] {
					t.Errorf("%dx%d after %v: line %d is %d wide", size[0], size[1], keys, i, w)
					break
				}
			}
			press(m, "esc", "esc")
		}
	}
}

// Styling an already-styled string splits its ANSI codes into visible garbage
// like "[1;38;2;0;134;158m". Render in full color and check nothing escape-like
// survives once real sequences are stripped.
func TestNoLeakedEscapeCodes(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	m, st := newTestInbox(t, 160, 40,
		&store.Task{Title: "!urgent", Priority: 0, Jira: "X-1", Repo: "r", Due: day(-1), Status: store.StatusReady, Tags: []string{"t"}},
		&store.Task{Title: "high", Priority: 1, Due: day(0), Status: store.StatusBlocked, WaitingOn: "Rahul"},
		&store.Task{Title: "done", Status: store.StatusDone},
	)
	st.AddLog(1, "agent", "sweep", "unread marker")
	m.reload()
	leak := regexp.MustCompile(`\[[0-9;]+m`)
	check := func(when string) {
		t.Helper()
		if s := leak.FindString(ansi.Strip(m.View())); s != "" {
			t.Fatalf("%s: leaked escape code %q", when, s)
		}
	}
	for g := 0; g < int(groupingCount); g++ {
		for v := range views {
			m.setView(v)
			check("list")
			press(m, "down")
			check("selected row")
		}
		press(m, "g")
	}
	m.setView(0)
	press(m, "enter")
	check("split detail")
	press(m, "l")
	check("expanded activity")
	press(m, "s")
	check("status picker")
	press(m, "esc", "?")
	check("help")
}

// The list refreshes every few seconds so work an agent does while the TUI is
// open shows up without losing the user's place.
func TestAutoRefreshKeepsSelection(t *testing.T) {
	m, st := newTestInbox(t, 120, 30, &store.Task{Title: "first"}, &store.Task{Title: "second"})
	press(m, "down")
	if m.selected().ID != 2 {
		t.Fatal("setup: cursor should be on #2")
	}
	agentTask := &store.Task{Title: "!added by agent", Priority: 0}
	agentTask.Title = "added by agent"
	if err := st.CreateTask(agentTask); err != nil {
		t.Fatal(err)
	}
	st.AddLog(2, "agent", "sweep", "worked on it")

	m.Update(tickMsg{})
	if !strings.Contains(m.View(), "added by agent") {
		t.Fatal("tick should pick up tasks created by an agent")
	}
	if m.selected().ID != 2 {
		t.Fatalf("cursor should stay on #2, got #%d", m.selected().ID)
	}

	// While a prompt is open, refreshing must not disturb it.
	press(m, "n")
	typeText(m, "half-typed")
	m.Update(tickMsg{})
	if m.mode != modeNote || m.input.Value() != "half-typed" {
		t.Fatal("tick should not interrupt an open prompt")
	}
}

func TestArrowsDoNotSwitchViews(t *testing.T) {
	m, _ := newTestInbox(t, 120, 30, &store.Task{Title: "x"})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	press(m, "L", "H")
	if m.view != viewOpen {
		t.Fatalf("arrows/L/H should not change view, now on %s", views[m.view].name)
	}
	press(m, "enter")
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.detail {
		t.Fatal("← should close the detail")
	}
}

func TestNeedsYouViewAnswerAndResume(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	asked := &store.Task{Title: "Add retry to webhook sender", Priority: 2}
	quiet := &store.Task{Title: "Write docs", Priority: 2}
	st.CreateTask(asked)
	st.CreateTask(quiet)
	st.AddReport(&store.Report{TaskID: asked.ID, Done: "Backoff, 5 tries", NeedsYou: "Alert #oncall or #billing-alerts?"})
	st.LinkSession(asked.ID, store.Session{ID: "sess-42", Name: "webhook retry"})

	m, err := newInbox(st)
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return fixedNow }
	var copied string
	m.copy = func(s string) error { copied = s; return nil }
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if m.view != viewNeedsYou {
		t.Fatalf("should open on Needs you when an answer is waiting, got %s", views[m.view].name)
	}
	if !strings.Contains(m.topBar(), "Needs you 1") {
		t.Fatalf("top bar: %s", m.topBar())
	}
	v := m.View()
	if !strings.Contains(v, "? ") || !strings.Contains(v, "Add retry") || strings.Contains(v, "Write docs") {
		t.Fatalf("needs-you list should show only the asking task, marked ?:\n%s", v)
	}

	press(m, "enter")
	v = m.View()
	for _, want := range []string{"REPORT", "Backoff, 5 tries", "Alert #oncall", "press a to answer", "SESSIONS", "webhook retry", "claude --resume sess-42"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q:\n%s", want, v)
		}
	}

	press(m, "o")
	if copied != "claude --resume sess-42" || !strings.Contains(m.footer(), "Copied") {
		t.Fatalf("o should copy the resume command, got %q / %s", copied, m.footer())
	}

	press(m, "a")
	if m.mode != modeAnswer || !strings.Contains(m.footer(), "Answer #1") {
		t.Fatalf("a should open the answer prompt: %s", m.footer())
	}
	typeText(m, "billing-alerts")
	press(m, "enter")
	got, _ := st.GetTask(asked.ID)
	if got.NeedsYou || got.Report.Answer != "billing-alerts" {
		t.Fatalf("answer not saved: %+v", got.Report)
	}
	if !strings.Contains(m.topBar(), "Needs you 0") {
		t.Fatalf("needs-you count should drop: %s", m.topBar())
	}

	// Tasks without a question or a session explain why nothing happened.
	m.setView(viewOpen)
	for m.selected() != nil && m.selected().ID != quiet.ID {
		press(m, "down")
	}
	press(m, "a")
	if m.mode != modeList || !strings.Contains(m.footer(), "no open question") {
		t.Fatalf("a on a task without a question: %s", m.footer())
	}
	press(m, "o")
	if !strings.Contains(m.footer(), "No Claude session") {
		t.Fatalf("o on a task without a session: %s", m.footer())
	}
}

func TestStartsOnOpenWhenNothingNeedsYou(t *testing.T) {
	m, _ := newTestInbox(t, 100, 30, &store.Task{Title: "x"})
	if m.view != viewOpen {
		t.Fatalf("should start on Open, got %s", views[m.view].name)
	}
	m.setView(viewNeedsYou)
	if !strings.Contains(m.View(), "Nothing needs you") {
		t.Fatalf("empty needs-you state:\n%s", m.View())
	}
}
