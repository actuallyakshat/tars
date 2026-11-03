// Package claude reads what tars needs to know about the Claude Code session
// it is running in: the session ID, and the session's name and launch folder.
package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// Work is what a session changed, read from its transcript.
type Work struct {
	Edits   int      // file edits and writes
	Commits int      // git commits run through Bash
	Files   []string // files edited, in first-touched order
	// Shell reports tools that can change files without saying which: Bash
	// (sed, heredocs, scripts) and subagents, whose own calls live in
	// separate transcripts.
	Shell bool
}

// editTools are the tools that change files.
var editTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// ReadWork scans a transcript for edits and commits. A missing or unreadable
// transcript counts as no work.
func ReadWork(transcript string) Work {
	var w Work
	f, err := os.Open(transcript)
	if err != nil {
		return w
	}
	defer f.Close()
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, []byte(`"tool_use"`)) {
			continue
		}
		var e struct {
			Message struct {
				Content []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Input struct {
						FilePath     string `json:"file_path"`
						NotebookPath string `json:"notebook_path"`
						Command      string `json:"command"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		for _, c := range e.Message.Content {
			if c.Type != "tool_use" {
				continue
			}
			switch {
			case editTools[c.Name]:
				w.Edits++
				p := c.Input.FilePath
				if p == "" {
					p = c.Input.NotebookPath
				}
				if p != "" && !seen[p] {
					seen[p] = true
					w.Files = append(w.Files, p)
				}
			case c.Name == "Bash":
				w.Shell = true
				if strings.Contains(c.Input.Command, "git commit") {
					w.Commits++
				}
			case c.Name == "Agent" || c.Name == "Task":
				w.Shell = true
			}
		}
	}
	return w
}
