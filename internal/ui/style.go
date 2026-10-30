// Package ui renders tars output: static views for commands and the interactive board.
package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/akshat-dubey/tars/internal/store"
)

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#9D8CFF"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6C6C6C"}
	colText   = lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#E4E4E4"}
	colJira   = lipgloss.AdaptiveColor{Light: "#0B66E4", Dark: "#5AA2FF"}

	statusColor = map[string]lipgloss.AdaptiveColor{
		store.StatusInbox:      {Light: "#7A7A7A", Dark: "#9A9A9A"},
		store.StatusReady:      {Light: "#00869E", Dark: "#3FD0E6"},
		store.StatusInProgress: {Light: "#B57900", Dark: "#FFC447"},
		store.StatusInReview:   {Light: "#A0399F", Dark: "#E77FE6"},
		store.StatusBlocked:    {Light: "#C0392B", Dark: "#FF6B5E"},
		store.StatusDone:       {Light: "#2E8B47", Dark: "#5BD47A"},
	}
	statusIcon = map[string]string{
		store.StatusInbox:      "○",
		store.StatusReady:      "◇",
		store.StatusInProgress: "◐",
		store.StatusInReview:   "◉",
		store.StatusBlocked:    "■",
		store.StatusDone:       "✔",
	}

	Muted  = lipgloss.NewStyle().Foreground(colMuted)
	Bold   = lipgloss.NewStyle().Bold(true).Foreground(colText)
	Accent = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	jiraSt = lipgloss.NewStyle().Foreground(colJira)
	errSt  = lipgloss.NewStyle().Foreground(statusColor[store.StatusBlocked]).Bold(true)
	okSt   = lipgloss.NewStyle().Foreground(statusColor[store.StatusDone]).Bold(true)
)

func StatusStyle(s string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(statusColor[s])
}

func StatusBadge(s string) string {
	return StatusStyle(s).Bold(true).Render(statusIcon[s] + " " + strings.ToUpper(s))
}

// focusedBadge is StatusBadge underlined. It styles the plain label in one
// pass: wrapping an already-styled string in Underline mangles its ANSI codes.
func focusedBadge(s string) string {
	return StatusStyle(s).Bold(true).Underline(true).Render(statusIcon[s] + " " + strings.ToUpper(s))
}

func Priority(p int) string {
	switch p {
	case 0:
		return lipgloss.NewStyle().Foreground(statusColor[store.StatusBlocked]).Bold(true).Render("▲▲")
	case 1:
		return lipgloss.NewStyle().Foreground(statusColor[store.StatusInProgress]).Render("▲ ")
	case 3:
		return Muted.Render("▽ ")
	default:
		return Muted.Render("· ")
	}
}

func PriorityName(p int) string {
	return [...]string{"urgent", "high", "normal", "low"}[clampPriority(p)]
}

func clampPriority(p int) int {
	if p < 0 {
		return 0
	}
	if p > 3 {
		return 3
	}
	return p
}

func Jira(key string) string {
	if key == "" {
		return ""
	}
	return jiraSt.Render(key)
}

// Due renders a due date relative to today, highlighting overdue and today.
func Due(d string) string {
	if d == "" {
		return ""
	}
	day, err := time.ParseInLocation("2006-01-02", d, time.Local)
	if err != nil {
		return Muted.Render(d)
	}
	y, m, dd := time.Now().Date()
	today := time.Date(y, m, dd, 0, 0, 0, 0, time.Local)
	days := int(math.Round(day.Sub(today).Hours() / 24)) // round: DST days are 23h/25h
	switch {
	case days < 0:
		return errSt.Render(fmt.Sprintf("overdue %dd", -days))
	case days == 0:
		return StatusStyle(store.StatusInProgress).Bold(true).Render("due today")
	case days == 1:
		return StatusStyle(store.StatusInProgress).Render("due tomorrow")
	case days < 7:
		return Muted.Render("due " + day.Format("Mon"))
	default:
		return Muted.Render("due " + day.Format("Jan 2"))
	}
}

func Truncate(s string, w int) string {
	if w <= 1 {
		return ""
	}
	return runewidth.Truncate(s, w, "…")
}

func Success(msg string) string { return okSt.Render("✔ ") + msg }
func Error(msg string) string   { return errSt.Render("✘ ") + msg }

// Ago renders an RFC3339 timestamp as a short relative time.
func Ago(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Local().Format("Jan 2, 2006")
	}
}
