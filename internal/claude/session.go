// Package claude reads what tars needs to know about the Claude Code session
// it is running in: the session ID, and the session's name and launch folder.
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// SessionEnv is set by Claude Code in every shell command it runs.
const SessionEnv = "CLAUDE_CODE_SESSION_ID"

// Session describes a Claude Code session.
type Session struct {
	ID        string
	Name      string // the /rename title if set, otherwise Claude's own title
	LaunchDir string // the folder the session was started in
}

// CurrentID returns the ID of the Claude Code session running this process,
// or "" outside Claude Code.
func CurrentID() string { return os.Getenv(SessionEnv) }

// ConfigDir is Claude Code's config directory: $CLAUDE_CONFIG_DIR or ~/.claude.
func ConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// Inspect reads the session's transcript, if it can be found under configDir,
// for its name and launch folder. A missing transcript is not an error: the
// session ID alone is still enough to resume it.
func Inspect(configDir, id string) Session {
	s := Session{ID: id}
	if id == "" || configDir == "" {
		return s
	}
	matches, _ := filepath.Glob(filepath.Join(configDir, "projects", "*", id+".jsonl"))
	if len(matches) == 0 {
		return s
	}
	f, err := os.Open(matches[0])
	if err != nil {
		return s
	}
	defer f.Close()

	var aiTitle, customTitle string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // transcript lines can be large
	for sc.Scan() {
		var e struct {
			Type        string `json:"type"`
			Cwd         string `json:"cwd"`
			AITitle     string `json:"aiTitle"`
			CustomTitle string `json:"customTitle"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if s.LaunchDir == "" && e.Cwd != "" {
			s.LaunchDir = e.Cwd
		}
		switch e.Type {
		case "ai-title":
			if e.AITitle != "" {
				aiTitle = e.AITitle
			}
		case "custom-title":
			if e.CustomTitle != "" {
				customTitle = e.CustomTitle
			}
		}
	}
	s.Name = customTitle
	if s.Name == "" {
		s.Name = aiTitle
	}
	return s
}

// ResumeCommand is what the user runs to reopen a session. It works from any
// directory.
func ResumeCommand(id string) string { return "claude --resume " + id }
