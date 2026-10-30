package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/akshat-dubey/tars/internal/store"
)

func TestRowFitsWidth(t *testing.T) {
	task := &store.Task{
		ID: 7, Title: strings.Repeat("very long title ", 20), Status: store.StatusReady,
		Jira: "BILL-881", Repo: "billing-svc", Tags: []string{"backend"}, Due: "2026-11-03",
	}
	for _, w := range []int{40, 80, 120, 200} {
		row := Row(task, w, 3)
		if got := lipgloss.Width(row); got > w {
			t.Errorf("width %d: row is %d wide", w, got)
		}
		if !strings.Contains(row, "…") {
			t.Errorf("width %d: long title not truncated", w)
		}
	}
	// Narrow terminals drop metadata before squeezing the title.
	if row := Row(task, 40, 3); strings.Contains(row, "BILL-881") {
		t.Error("metadata kept on a narrow row")
	}
	if row := Row(task, 160, 3); !strings.Contains(row, "BILL-881") || !strings.Contains(row, "#backend") {
		t.Error("metadata missing on a wide row")
	}
}

func TestListGroupsInBoardOrder(t *testing.T) {
	tasks := []*store.Task{
		{ID: 1, Title: "a", Status: store.StatusInReview},
		{ID: 2, Title: "b", Status: store.StatusInbox},
	}
	out := List(tasks, 100)
	if strings.Index(out, "INBOX") > strings.Index(out, "IN-REVIEW") {
		t.Fatalf("groups out of order:\n%s", out)
	}
	if !strings.Contains(List(nil, 100), "tars add") {
		t.Error("empty list should hint at tars add")
	}
}

func TestDue(t *testing.T) {
	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }
	cases := map[string]string{
		day(-2): "overdue 2d",
		day(0):  "due today",
		day(1):  "due tomorrow",
	}
	for in, want := range cases {
		if got := Due(in); !strings.Contains(got, want) {
			t.Errorf("Due(%s) = %q, want %q", in, got, want)
		}
	}
	if Due("") != "" {
		t.Error("empty due should render nothing")
	}
}

func TestTaskCardShowsEverything(t *testing.T) {
	task := &store.Task{
		ID: 3, Title: "Fix export", Status: store.StatusBlocked, Priority: 0, Jira: "BILL-1",
		Body: "the body", Acceptance: "the criteria", WaitingOn: "Rahul", Branch: "tars/3",
		CreatedAt: time.Now().UTC().Format(time.RFC3339), UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Log: []store.Entry{{Author: "agent", Kind: "sweep", Text: "tests green", At: time.Now().UTC().Format(time.RFC3339)}},
	}
	out := Task(task, 90)
	for _, want := range []string{"#3", "BLOCKED", "Fix export", "urgent", "BILL-1", "the body", "the criteria", "Rahul", "tars/3", "tests green"} {
		if !strings.Contains(out, want) {
			t.Errorf("card missing %q", want)
		}
	}
}

func TestBriefEmpty(t *testing.T) {
	if out := Brief(&store.Brief{Date: "2026-10-30"}, 100); !strings.Contains(out, "All clear") {
		t.Fatalf("empty brief: %s", out)
	}
}

// --- board ---

func key(s string) tea.KeyMsg {
	switch s {
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func newBoard(t *testing.T, w, h int, tasks ...*store.Task) (*board, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for _, task := range tasks {
		if err := st.CreateTask(task); err != nil {
			t.Fatal(err)
		}
	}
	b := &board{st: st, row: map[string]int{}, offset: map[string]int{}}
	if err := b.reload(); err != nil {
		t.Fatal(err)
	}
	b.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return b, st
}

func press(b *board, keys ...string) {
	for _, k := range keys {
		b.Update(key(k))
	}
}

func TestBoardRendersAtManySizes(t *testing.T) {
	var tasks []*store.Task
	for i := 0; i < 30; i++ {
		tasks = append(tasks, &store.Task{Title: strings.Repeat("task ", i%7+1), Status: store.Statuses[i%5], Jira: "X-1", Due: "2026-11-01"})
	}
	for _, size := range [][2]int{{20, 8}, {60, 20}, {120, 40}, {240, 60}} {
		b, _ := newBoard(t, size[0], size[1], tasks...)
		press(b, "down", "down", "down", "down", "down", "down", "down", "down", "right", "down")
		view := b.View()
		if lines := strings.Count(view, "\n") + 1; lines > size[1] {
			t.Errorf("%dx%d: view is %d lines tall", size[0], size[1], lines)
		}
	}
}

func TestBoardEmptyDoesNotPanic(t *testing.T) {
	b, _ := newBoard(t, 100, 30)
	press(b, "down", "right", "enter", "d", "]", "[", "+", "-", "a", "a", "r")
	if !strings.Contains(b.View(), "TARS") {
		t.Fatal("header missing")
	}
}

func TestBoardKeysChangeTasks(t *testing.T) {
	b, st := newBoard(t, 200, 40, // wide enough to show all six columns
		&store.Task{Title: "first", Priority: 2},
		&store.Task{Title: "second", Priority: 2},
	)
	// Inbox column is focused; move the selected task forward one status.
	press(b, "]")
	got, _ := st.GetTask(1)
	if got.Status != store.StatusReady {
		t.Fatalf("] should move inbox → ready, got %s", got.Status)
	}
	if len(got.Log) != 1 || got.Log[0].Author != "user" {
		t.Fatalf("move not logged: %+v", got.Log)
	}

	press(b, "+") // still on inbox column: task 2
	if got, _ := st.GetTask(2); got.Priority != 1 {
		t.Fatalf("+ should raise priority, got %d", got.Priority)
	}

	press(b, "d")
	if got, _ := st.GetTask(2); got.Status != store.StatusDone {
		t.Fatalf("d should mark done, got %s", got.Status)
	}
	if strings.Contains(b.View(), "second") {
		t.Error("done task visible while done column is hidden")
	}
	press(b, "a")
	if !strings.Contains(b.View(), "second") {
		t.Error("a should reveal done tasks")
	}
}

func TestBoardDetailView(t *testing.T) {
	b, _ := newBoard(t, 120, 40, &store.Task{Title: "detail me", Body: "hidden body text"})
	if strings.Contains(b.View(), "hidden body text") {
		t.Fatal("body shown on board")
	}
	press(b, "enter")
	if !b.detail || !strings.Contains(b.View(), "hidden body text") {
		t.Fatal("enter should open detail with body")
	}
	press(b, "esc")
	if b.detail {
		t.Fatal("esc should close detail")
	}
}
