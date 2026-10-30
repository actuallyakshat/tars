package parse

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 4, 0, 0, time.Local) // a Friday afternoon
	cases := map[string]string{
		"":             "",
		"none":         "",
		"today":        "2026-10-30",
		"Tomorrow":     "2026-10-31",
		"tmr":          "2026-10-31",
		"+0d":          "2026-10-30",
		"+3d":          "2026-11-02",
		"+2w":          "2026-11-13",
		"mon":          "2026-11-02",
		"monday":       "2026-11-02",
		"thurs":        "2026-11-05",
		"fri":          "2026-11-06", // same weekday means next week, never today
		"2026-12-25":   "2026-12-25",
		" 2027-01-05 ": "2027-01-05",
	}
	for in, want := range cases {
		got, err := Date(in, now)
		if err != nil || got != want {
			t.Errorf("Date(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"someday", "mo", "+xd", "+3y", "2026-13-01", "sunshine"} {
		if got, err := Date(bad, now); err == nil {
			t.Errorf("Date(%q) = %q; want error", bad, got)
		}
	}
}

func TestParsePriority(t *testing.T) {
	for in, want := range map[string]int{"0": 0, "P0": 0, "urgent": 0, "high": 1, "": 2, "normal": 2, "p3": 3, "Low": 3} {
		if got, err := Priority(in); err != nil || got != want {
			t.Errorf("Priority(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := Priority("4"); err == nil {
		t.Error("priority 4 accepted")
	}
}

func TestQuickAdd(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 4, 0, 0, time.Local)
	q, err := QuickAdd("Fix CSV export BILL-881 !high @billing #perf #backend due:fri for #12", now)
	if err != nil {
		t.Fatal(err)
	}
	if q.Title != "Fix CSV export for #12" || q.Jira != "BILL-881" || q.Priority != 1 ||
		q.Repo != "billing" || q.Due != "2026-11-06" || len(q.Tags) != 2 || q.Tags[1] != "backend" {
		t.Fatalf("got %+v", q)
	}

	q, err = QuickAdd("plain title ^tomorrow", now)
	if err != nil || q.Title != "plain title" || q.Priority != 2 || q.Due != "2026-10-31" {
		t.Fatalf("got %+v, %v", q, err)
	}

	// Only the first Jira-looking token is the key; later ones stay in the title.
	q, _ = QuickAdd("Port AB-1 changes to CD-2", now)
	if q.Jira != "AB-1" || q.Title != "Port changes to CD-2" {
		t.Fatalf("got %+v", q)
	}

	// Lowercase hyphenated words are not Jira keys.
	q, _ = QuickAdd("Handle utf-8 and covid-19 data", now)
	if q.Jira != "" || q.Title != "Handle utf-8 and covid-19 data" {
		t.Fatalf("got %+v", q)
	}

	for _, bad := range []string{"", "  ", "!high @repo", "task !whenever", "task due:someday"} {
		if _, err := QuickAdd(bad, now); err == nil {
			t.Errorf("QuickAdd(%q) accepted", bad)
		}
	}
}
