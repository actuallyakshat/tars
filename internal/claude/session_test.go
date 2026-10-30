package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "projects", "-Users-me-work")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := `{"type":"user","cwd":"/Users/me/work","message":{}}
not json at all
{"type":"ai-title","aiTitle":"First guess"}
{"type":"assistant","cwd":"/Users/me/work/repo"}
{"type":"ai-title","aiTitle":"Better guess"}
`
	os.WriteFile(filepath.Join(proj, "abc-123.jsonl"), []byte(transcript), 0o644)

	s := Inspect(dir, "abc-123")
	if s.LaunchDir != "/Users/me/work" || s.Name != "Better guess" {
		t.Fatalf("got %+v", s)
	}

	// A /rename title beats Claude's own title, wherever it appears.
	os.WriteFile(filepath.Join(proj, "abc-123.jsonl"), []byte(transcript+`{"type":"custom-title","customTitle":"webhook retry"}
{"type":"ai-title","aiTitle":"Later guess"}
`), 0o644)
	if s := Inspect(dir, "abc-123"); s.Name != "webhook retry" {
		t.Fatalf("custom title should win, got %q", s.Name)
	}

	// Unknown sessions still return the ID so they can be resumed.
	if s := Inspect(dir, "missing"); s.ID != "missing" || s.Name != "" {
		t.Fatalf("got %+v", s)
	}
}

func TestCurrentIDAndConfigDir(t *testing.T) {
	t.Setenv(SessionEnv, "sess-1")
	if CurrentID() != "sess-1" {
		t.Fatal("CurrentID should read the env var")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cc")
	if ConfigDir() != "/tmp/cc" {
		t.Fatal("ConfigDir should honour CLAUDE_CONFIG_DIR")
	}
	if ResumeCommand("x") != "claude --resume x" {
		t.Fatal("resume command")
	}
}
