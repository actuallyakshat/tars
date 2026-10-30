// Package parse reads the loose, human inputs tars accepts: dates, priorities,
// and one-line quick-add strings.
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Date accepts YYYY-MM-DD, today, tomorrow, a weekday (next occurrence,
// never today), +Nd / +Nw, or "" / "none" to clear. Returns YYYY-MM-DD or "".
func Date(s string, now time.Time) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch s {
	case "", "none", "clear":
		return "", nil
	case "today":
		return day.Format(time.DateOnly), nil
	case "tomorrow", "tmr":
		return day.AddDate(0, 0, 1).Format(time.DateOnly), nil
	}
	if strings.HasPrefix(s, "+") && len(s) > 2 {
		n, err := strconv.Atoi(s[1 : len(s)-1])
		if err == nil && n >= 0 {
			switch s[len(s)-1] {
			case 'd':
				return day.AddDate(0, 0, n).Format(time.DateOnly), nil
			case 'w':
				return day.AddDate(0, 0, 7*n).Format(time.DateOnly), nil
			}
		}
	}
	if len(s) >= 3 {
		for wd := time.Sunday; wd <= time.Saturday; wd++ {
			if strings.HasPrefix(strings.ToLower(wd.String()), s) {
				delta := (int(wd) - int(day.Weekday()) + 7) % 7
				if delta == 0 {
					delta = 7
				}
				return day.AddDate(0, 0, delta).Format(time.DateOnly), nil
			}
		}
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.Format(time.DateOnly), nil
	}
	return "", fmt.Errorf("can't read date %q (try 2026-11-03, today, tomorrow, fri, +3d)", s)
}

// Priority accepts 0-3 or urgent/high/normal/low (also p0-p3).
func Priority(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "p0", "urgent":
		return 0, nil
	case "1", "p1", "high":
		return 1, nil
	case "2", "p2", "normal", "":
		return 2, nil
	case "3", "p3", "low":
		return 3, nil
	}
	return 0, fmt.Errorf("invalid priority %q (use urgent, high, normal, low or 0-3)", s)
}

// jiraKey only matches uppercase keys, so words like utf-8 or covid-19 stay
// in the title.
var jiraKey = regexp.MustCompile(`^[A-Z][A-Z0-9]+-[0-9]+$`)

// Quick is a task parsed from a one-line quick-add string.
type Quick struct {
	Title    string
	Jira     string
	Repo     string
	Tags     []string
	Due      string
	Priority int
}

// QuickAdd parses inline tokens out of a one-line task:
//
//	Fix CSV export BILL-881 !high @billing #perf due:fri
//
// !urgent/!high/!normal/!low (or !0-3) set priority, @name the repo, #tag a tag,
// due:<date> or ^<date> the due date, and a bare ABC-123 the Jira key.
// Everything else is the title.
func QuickAdd(s string, now time.Time) (Quick, error) {
	q := Quick{Priority: 2}
	var words []string
	for _, w := range strings.Fields(s) {
		var err error
		switch {
		case len(w) > 1 && w[0] == '!':
			q.Priority, err = Priority(w[1:])
		case len(w) > 1 && w[0] == '@':
			q.Repo = w[1:]
		case len(w) > 1 && w[0] == '#' && !isDigits(w[1:]):
			q.Tags = append(q.Tags, w[1:])
		case len(w) > 4 && strings.HasPrefix(strings.ToLower(w), "due:"):
			q.Due, err = Date(w[4:], now)
		case len(w) > 1 && w[0] == '^':
			q.Due, err = Date(w[1:], now)
		case q.Jira == "" && jiraKey.MatchString(w):
			q.Jira = w
		default:
			words = append(words, w)
		}
		if err != nil {
			return Quick{}, err
		}
	}
	q.Title = strings.Join(words, " ")
	if q.Title == "" {
		return Quick{}, fmt.Errorf("title can't be empty")
	}
	return q, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
