package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "tars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustCreate(t *testing.T, s *Store, task *Task) *Task {
	t.Helper()
	if err := s.CreateTask(task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "tars.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustCreate(t, s, &Task{Title: "survives reopen"})
	s.Close()

	s, err = Open(path) // migrations must not re-run
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tasks, err := s.ListTasks(Filter{})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("got %d tasks, err %v", len(tasks), err)
	}
}

func TestDefaultPathHonoursTarsHome(t *testing.T) {
	t.Setenv("TARS_HOME", "/tmp/tars-home")
	p, err := DefaultPath()
	if err != nil || p != "/tmp/tars-home/tars.db" {
		t.Fatalf("got %q, %v", p, err)
	}
	t.Setenv("TARS_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	if p, _ := DefaultPath(); p != "/tmp/xdg/tars/tars.db" {
		t.Fatalf("got %q", p)
	}
}

func TestCreateDefaultsAndRoundTrip(t *testing.T) {
	s := openTemp(t)
	in := mustCreate(t, s, &Task{
		Title: "Fix export", Jira: "BILL-1", Repo: "billing", Tags: []string{"a", "b"},
		Due: "2026-11-03", Acceptance: "exports 1M rows", Priority: 1,
	})
	if in.ID == 0 || in.Status != StatusInbox || in.CreatedAt == "" {
		t.Fatalf("defaults not applied: %+v", in)
	}
	got, err := s.GetTask(in.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Fix export" || got.Jira != "BILL-1" || got.Repo != "billing" ||
		len(got.Tags) != 2 || got.Tags[1] != "b" || got.Due != "2026-11-03" ||
		got.Acceptance != "exports 1M rows" || got.Priority != 1 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	s := openTemp(t)
	if err := s.CreateTask(&Task{Title: "x", Status: "someday"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetMissingIsNotFound(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetTask(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.DeleteTask(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSaveMaintainsDoneAt(t *testing.T) {
	s := openTemp(t)
	task := mustCreate(t, s, &Task{Title: "x"})
	task.Status = StatusDone
	if err := s.SaveTask(task); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetTask(task.ID)
	if got.DoneAt == "" {
		t.Fatal("done_at not set")
	}
	got.Status = StatusReady
	if err := s.SaveTask(got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTask(task.ID)
	if got.DoneAt != "" {
		t.Fatal("done_at not cleared on reopen")
	}
	got.Status = "bogus"
	if err := s.SaveTask(got); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestListFilters(t *testing.T) {
	s := openTemp(t)
	a := mustCreate(t, s, &Task{Title: "alpha export", Jira: "BILL-1", Repo: "billing", Tags: []string{"backend"}, Status: StatusReady})
	b := mustCreate(t, s, &Task{Title: "beta", Repo: "web", Tags: []string{"frontend", "backend-ish"}})
	c := mustCreate(t, s, &Task{Title: "gamma", Jira: "WEB-9", Status: StatusDone})

	ids := func(f Filter) []int64 {
		t.Helper()
		ts, err := s.ListTasks(f)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, x := range ts {
			out = append(out, x.ID)
		}
		return out
	}
	yes, no := true, false
	cases := []struct {
		name string
		f    Filter
		want []int64
	}{
		{"open by default", Filter{}, []int64{a.ID, b.ID}},
		{"include done", Filter{IncludeDone: true}, []int64{a.ID, b.ID, c.ID}},
		{"explicit done status", Filter{Statuses: []string{StatusDone}}, []int64{c.ID}},
		{"has jira (open)", Filter{HasJira: &yes}, []int64{a.ID}},
		{"has jira incl done", Filter{HasJira: &yes, IncludeDone: true}, []int64{a.ID, c.ID}},
		{"no jira", Filter{HasJira: &no}, []int64{b.ID}},
		{"repo", Filter{Repo: "web"}, []int64{b.ID}},
		{"tag exact match only", Filter{Tag: "backend"}, []int64{a.ID}},
		{"search title", Filter{Search: "export"}, []int64{a.ID}},
		{"search jira", Filter{Search: "bill-1"}, []int64{a.ID}},
		{"multi status", Filter{Statuses: []string{StatusReady, StatusInbox}}, []int64{a.ID, b.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(tc.f)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestListOrdersByPriorityThenDue(t *testing.T) {
	s := openTemp(t)
	low := mustCreate(t, s, &Task{Title: "low", Priority: 3})
	noDue := mustCreate(t, s, &Task{Title: "normal no due", Priority: 2})
	late := mustCreate(t, s, &Task{Title: "normal later", Priority: 2, Due: "2026-12-01"})
	soon := mustCreate(t, s, &Task{Title: "normal soon", Priority: 2, Due: "2026-11-01"})
	urgent := mustCreate(t, s, &Task{Title: "urgent", Priority: 0})

	ts, _ := s.ListTasks(Filter{})
	want := []int64{urgent.ID, soon.ID, late.ID, noDue.ID, low.ID}
	for i, x := range ts {
		if x.ID != want[i] {
			t.Fatalf("position %d: got #%d want #%d", i, x.ID, want[i])
		}
	}
}

func TestLogAndCascadeDelete(t *testing.T) {
	s := openTemp(t)
	task := mustCreate(t, s, &Task{Title: "x"})
	if err := s.AddLog(task.ID, "agent", "sweep", "tests green"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLog(task.ID, "user", "note", "looks good"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetTask(task.ID)
	if len(got.Log) != 2 || got.Log[0].Author != "agent" || got.Log[1].Text != "looks good" {
		t.Fatalf("log: %+v", got.Log)
	}
	if err := s.AddLog(999, "user", "note", "orphan"); err == nil {
		t.Fatal("expected foreign key error for missing task")
	}
	if err := s.DeleteTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ := s.Log(task.ID); len(entries) != 0 {
		t.Fatal("log entries survived task delete")
	}
}

func TestRepoProfiles(t *testing.T) {
	s := openTemp(t)
	root := t.TempDir()
	nested := filepath.Join(root, "services")
	if err := os.MkdirAll(filepath.Join(nested, "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := s.UpsertRepo("mono", root); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertRepo("services", nested); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRepoSection("mono", "tests", "make test"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRepoSection("mono", "dod", "PR + green CI"); err != nil {
		t.Fatal(err)
	}
	// Upsert without a path must keep the existing path.
	if err := s.UpsertRepo("mono", ""); err != nil {
		t.Fatal(err)
	}

	r, err := s.GetRepo("mono")
	if err != nil {
		t.Fatal(err)
	}
	if r.Path != root || len(r.Sections) != 2 || r.Sections[0].Name != "dod" {
		t.Fatalf("repo: %+v", r)
	}

	// Deepest matching path wins.
	found, err := s.FindRepoByPath(filepath.Join(nested, "api"))
	if err != nil || found.Name != "services" {
		t.Fatalf("got %+v, %v", found, err)
	}
	found, err = s.FindRepoByPath(root)
	if err != nil || found.Name != "mono" {
		t.Fatalf("got %+v, %v", found, err)
	}
	// A sibling sharing a name prefix must not match.
	if _, err := s.FindRepoByPath(root + "-other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("prefix sibling matched: %v", err)
	}

	// Empty content removes a section.
	if err := s.SetRepoSection("mono", "tests", "  "); err != nil {
		t.Fatal(err)
	}
	r, _ = s.GetRepo("mono")
	if len(r.Sections) != 1 {
		t.Fatalf("section not removed: %+v", r.Sections)
	}

	if err := s.DeleteRepo("mono"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRepo("mono"); !errors.Is(err, ErrNotFound) {
		t.Fatal("repo survived delete")
	}
}

func TestProfile(t *testing.T) {
	s := openTemp(t)
	if err := s.SetProfileSection("style", "small PRs"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProfileSection("style", "tiny PRs"); err != nil {
		t.Fatal(err)
	}
	secs, _ := s.Profile()
	if len(secs) != 1 || secs[0].Content != "tiny PRs" {
		t.Fatalf("profile: %+v", secs)
	}
	s.SetProfileSection("style", "")
	if secs, _ := s.Profile(); len(secs) != 0 {
		t.Fatal("section not removed")
	}
}

func TestBriefBuckets(t *testing.T) {
	s := openTemp(t)
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	review := mustCreate(t, s, &Task{Title: "review", Status: StatusInReview})
	overdue := mustCreate(t, s, &Task{Title: "overdue", Status: StatusReady, Due: yesterday})
	dueToday := mustCreate(t, s, &Task{Title: "today", Due: today})
	follow := mustCreate(t, s, &Task{Title: "follow", Status: StatusBlocked, FollowUp: today})
	notYet := mustCreate(t, s, &Task{Title: "later follow", Status: StatusBlocked, FollowUp: tomorrow})
	stale := mustCreate(t, s, &Task{Title: "stale", Status: StatusInProgress})
	done := mustCreate(t, s, &Task{Title: "done"})
	done.Status = StatusDone
	s.SaveTask(done)
	// Backdate the stale task past the threshold.
	old := time.Now().Add(-StaleAfter - time.Hour).UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE tasks SET updated_at = ? WHERE id = ?`, old, stale.ID); err != nil {
		t.Fatal(err)
	}

	b, err := s.Brief()
	if err != nil {
		t.Fatal(err)
	}
	has := func(name string, list []*Task, id int64) {
		t.Helper()
		for _, x := range list {
			if x.ID == id {
				return
			}
		}
		t.Errorf("%s missing #%d", name, id)
	}
	has("in_review", b.InReview, review.ID)
	has("overdue", b.Overdue, overdue.ID)
	has("ready", b.Ready, overdue.ID)
	has("due_today", b.DueToday, dueToday.ID)
	has("inbox", b.Inbox, dueToday.ID)
	has("follow_ups", b.FollowUps, follow.ID)
	has("stale", b.Stale, stale.ID)
	has("done_recently", b.DoneRecently, done.ID)
	for _, x := range b.FollowUps {
		if x.ID == notYet.ID {
			t.Error("future follow-up listed as due")
		}
	}
}

// Parallel sweep agents write concurrently; WAL + busy_timeout must absorb it.
func TestConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tars.db")
	s0, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	task := mustCreate(t, s0, &Task{Title: "shared"})
	s0.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(path) // separate connection, like separate processes
			if err != nil {
				errs <- err
				return
			}
			defer s.Close()
			for i := 0; i < 10; i++ {
				if err := s.AddLog(task.ID, "agent", "note", "progress"); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	s, _ := Open(path)
	defer s.Close()
	if entries, _ := s.Log(task.ID); len(entries) != 40 {
		t.Fatalf("got %d log entries, want 40", len(entries))
	}
}
