package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/actuallyakshat/tars/internal/claude"
	"github.com/actuallyakshat/tars/internal/store"
)

// The Unfiled view lists Claude sessions that changed code but matched no
// task. It sits after the task views (key 5) and has its own keys: f files the
// selected session into a task by number, n makes it a new task, d dismisses.

func (m *inbox) selectedCapture() *store.Capture {
	if m.capCur < 0 || m.capCur >= len(m.caps) {
		return nil
	}
	return m.caps[m.capCur]
}

func (m *inbox) showUnfiled() {
	m.unfiled = true
	m.detail = false
	m.capCur = 0
}

// updateUnfiled handles keys in the Unfiled view. It returns tea.Quit for q.
func (m *inbox) updateUnfiled(msg tea.KeyMsg) tea.Cmd {
	m.flash = ""
	c := m.selectedCapture()
	switch msg.String() {
	case "q":
		return tea.Quit
	case "up", "k":
		m.capCur = max(m.capCur-1, 0)
	case "down", "j":
		m.capCur = max(min(m.capCur+1, len(m.caps)-1), 0)
	case "tab":
		m.unfiled = false
		m.setView(0)
	case "shift+tab":
		m.unfiled = false
		m.setView(len(views) - 1)
	case "1", "2", "3", "4":
		m.unfiled = false
		m.setView(int(msg.String()[0] - '1'))
	case "f":
		if c != nil {
			cmd := m.startInput(modeFileInto, "", "task number, like 21")
			m.input.Width = 22 // room for an error after it
			return cmd
		}
	case "n":
		if c != nil {
			return m.startInput(modeFileNew, c.Title(), "title for the new task")
		}
	case "d":
		if c == nil {
			break
		}
		if err := m.st.SetCaptureState(c.SessionID, store.CaptureDismissed); err != nil {
			m.setErr(err)
			break
		}
		m.reloadOrFlash()
		m.say("Dismissed %q", Truncate(c.Title(), 40))
	case "o":
		if c != nil {
			if err := m.copy(claude.ResumeCommand(c.SessionID)); err != nil {
				m.setErr(err)
			} else {
				m.say("Copied: %s", claude.ResumeCommand(c.SessionID))
			}
		}
	case "r":
		m.reloadOrFlash()
	case "?":
		m.help = true
	}
	return nil
}

func (m *inbox) reloadOrFlash() {
	if err := m.reload(); err != nil {
		m.setErr(err)
	}
}

// fileCapture files the selected session into task id, or a new task titled
// title when id is 0. It reports whether the input can close.
func (m *inbox) fileCapture(val string, newTask bool) bool {
	c := m.selectedCapture()
	if c == nil || val == "" {
		return true
	}
	var t *store.Task
	if newTask {
		t = &store.Task{Title: val, Repo: c.Repo, Body: c.Prompt, Priority: 2}
		if err := m.st.CreateTask(t); err != nil {
			m.setErr(err)
			return false
		}
	} else {
		n, err := strconv.ParseInt(strings.TrimPrefix(val, "#"), 10, 64)
		if err != nil || n <= 0 {
			m.setErr(fmt.Errorf("type a task number, like 21"))
			return false
		}
		if t, err = m.st.GetTask(n); err != nil {
			m.setErr(fmt.Errorf("no task #%d", n))
			return false
		}
	}
	s := claude.Inspect(claude.ConfigDir(), c.SessionID)
	if err := m.st.FileCapture(c, t, s.Name, s.LaunchDir); err != nil {
		m.setErr(err)
		return false
	}
	if err := m.st.AddLog(t.ID, "user", "note", "Filed session "+c.SessionID[:min(8, len(c.SessionID))]+": "+c.Title()); err != nil {
		m.setErr(err)
		return false
	}
	m.reloadOrFlash()
	m.capCur = min(m.capCur, max(len(m.caps)-1, 0))
	if newTask {
		m.say("Added #%d %s", t.ID, Truncate(t.Title, 40))
	} else {
		m.say("Filed into #%d %s", t.ID, Truncate(t.Title, 40))
	}
	return true
}

func (m *inbox) unfiledView(width, height int) string {
	if len(m.caps) == 0 {
		return "\n  " + Muted.Render("Nothing to file. Sessions that change code without matching a task land here.")
	}
	width = min(width, maxRowWidth)
	now := m.now()
	// Two lines per session: title, then where and what it changed.
	per := 3
	top := 0
	if m.capCur*per >= height {
		top = m.capCur - height/per + 1
	}
	var lines []string
	for i := top; i < len(m.caps) && len(lines)+per <= height+1; i++ {
		c := m.caps[i]
		sel := i == m.capCur
		mark := " "
		tst := textSt
		if sel {
			mark = Accent.Render("▌")
			tst = Bold
		}
		when := shortAgo(c.EndedAt, now)
		if when == "" {
			when = shortAgo(c.StartedAt, now)
		}
		titleW := max(width-4-runewidth.StringWidth(when)-2, 8)
		title := Truncate(c.Title(), titleW)
		pad := strings.Repeat(" ", max(titleW-runewidth.StringWidth(title), 0))
		lines = append(lines, mark+"  "+tst.Render(title)+pad+"  "+Muted.Render(when))

		meta := []string{}
		if c.Repo != "" {
			meta = append(meta, c.Repo)
		}
		if c.Branch != "" {
			meta = append(meta, c.Branch)
		}
		meta = append(meta, fmt.Sprintf("%d %s", c.Edits, plural(c.Edits, "edit", "edits")))
		if c.Commits > 0 {
			meta = append(meta, fmt.Sprintf("%d %s", c.Commits, plural(c.Commits, "commit", "commits")))
		}
		if len(c.Files) > 0 {
			meta = append(meta, strings.Join(c.Files, ", "))
		}
		lines = append(lines, "   "+Muted.Render(Truncate(strings.Join(meta, " · "), width-4)), "")
	}
	return strings.Join(lines, "\n")
}
