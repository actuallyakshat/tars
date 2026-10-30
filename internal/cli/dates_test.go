package cli

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
		got, err := parseDate(in, now)
		if err != nil || got != want {
			t.Errorf("parseDate(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"someday", "mo", "+xd", "+3y", "2026-13-01", "sunshine"} {
		if got, err := parseDate(bad, now); err == nil {
			t.Errorf("parseDate(%q) = %q; want error", bad, got)
		}
	}
}

func TestParsePriority(t *testing.T) {
	for in, want := range map[string]int{"0": 0, "P0": 0, "urgent": 0, "high": 1, "": 2, "normal": 2, "p3": 3, "Low": 3} {
		if got, err := parsePriority(in); err != nil || got != want {
			t.Errorf("parsePriority(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parsePriority("4"); err == nil {
		t.Error("priority 4 accepted")
	}
}

func TestNormalizeStatus(t *testing.T) {
	for in, want := range map[string]string{"ready": "ready", "WIP": "in-progress", "review": "in-review", "waiting": "blocked", "todo": "inbox"} {
		if got, err := normalizeStatus(in); err != nil || got != want {
			t.Errorf("normalizeStatus(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := normalizeStatus("someday"); err == nil {
		t.Error("unknown status accepted")
	}
}

func TestParseID(t *testing.T) {
	if id, err := parseID("#12"); err != nil || id != 12 {
		t.Fatalf("got %d, %v", id, err)
	}
	for _, bad := range []string{"", "#", "x", "0", "-1", "1.5"} {
		if _, err := parseID(bad); err == nil {
			t.Errorf("parseID(%q) accepted", bad)
		}
	}
}
