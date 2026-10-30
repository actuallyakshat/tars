package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Render without ANSI so string assertions don't depend on the terminal.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	// Tests may run inside Claude Code; never link or read the real session.
	os.Setenv("CLAUDE_CODE_SESSION_ID", "")
	os.Setenv("CLAUDE_CONFIG_DIR", os.TempDir()+"/tars-test-no-claude")
	os.Exit(m.Run())
}
