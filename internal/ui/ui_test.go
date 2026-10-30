package ui

import (
	"strings"
	"testing"
	"time"

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

func TestListGroupsInWorkflowOrder(t *testing.T) {
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
		day(-2): "2d late",
		day(0):  "today",
		day(1):  "tomorrow",
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
