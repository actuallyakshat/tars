package cli

import "testing"

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
