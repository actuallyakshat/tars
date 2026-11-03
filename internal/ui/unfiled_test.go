package ui

import (
	"strings"
	"testing"

	"github.com/actuallyakshat/tars/internal/store"
)

func addUnfiled(t *testing.T, st *store.Store, id, name, prompt string) {
	t.Helper()
	if _, err := st.StartCapture(&store.Capture{SessionID: id, Repo: "billing", Branch: "faster-export", Prompt: prompt}); err != nil {
		t.Fatal(err)
	}
	if err := st.EndCapture(id, 3, 1, []string{"export.go"}); err != nil {
		t.Fatal(err)
	}
	if name != "" {
		st.RecordSession(store.Session{ID: id, Name: name})
	}
}

func TestUnfiledView(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	m, st := newTestInbox(t, 120, 30, &store.Task{Title: "Speed up exports"})
	addUnfiled(t, st, "aaaa1111", "Stream CSV rows", "exports time out")
	addUnfiled(t, st, "bbbb2222", "", "Poke at the logger\nmore")
	addUnfiled(t, st, "cccc3333", "", "Try a thing")
	m.reloadOrFlash()

	if v := m.View(); !strings.Contains(v, "Unfiled 3 ●") {
		t.Fatalf("tab missing:\n%s", v)
	}
	// Tab from the last task view reaches Unfiled; 5 jumps there.
	press(m, "4", "tab")
	v := m.View()
	if !m.unfiled || !strings.Contains(v, "Stream CSV rows") || !strings.Contains(v, "billing · faster-export · 3 edits · 1 commit · export.go") {
		t.Fatalf("unfiled view:\n%s", v)
	}
	if !strings.Contains(v, "Poke at the logger") || strings.Contains(v, "more") {
		t.Fatalf("title should be the prompt's first line:\n%s", v)
	}

	// f files the selected session into a task by number.
	press(m, "f")
	typeText(m, "99")
	press(m, "enter")
	if !strings.Contains(m.View(), "no task #99") || m.mode != modeFileInto {
		t.Fatalf("bad number should keep the prompt open:\n%s", m.View())
	}
	m.input.SetValue("#1")
	press(m, "enter")
	if task := get(t, st, 1); len(task.Sessions) != 1 || task.Branch != "faster-export" || task.Repo != "billing" {
		t.Fatalf("filed: %+v", task)
	}
	if len(m.caps) != 2 {
		t.Fatalf("caps left: %d", len(m.caps))
	}

	// n makes a new task, titled after the session unless changed.
	press(m, "n")
	if m.input.Value() != "Poke at the logger" {
		t.Fatalf("suggested title %q", m.input.Value())
	}
	press(m, "enter")
	if task := get(t, st, 2); task.Title != "Poke at the logger" || len(task.Sessions) != 1 {
		t.Fatalf("new task: %+v", task)
	}

	// d dismisses; the view empties.
	press(m, "d")
	if len(m.caps) != 0 || !strings.Contains(m.View(), "Nothing to file") {
		t.Fatalf("after dismiss:\n%s", m.View())
	}
	// x does nothing here: it's not a task view.
	press(m, "x")
	if get(t, st, 1).Status == store.StatusDone {
		t.Fatal("x acted on a hidden task")
	}
	press(m, "1")
	if m.unfiled || m.view != viewAll {
		t.Fatal("1 should leave Unfiled")
	}
	press(m, "5")
	if !m.unfiled {
		t.Fatal("5 should open Unfiled")
	}
}
