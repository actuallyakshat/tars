package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDate accepts YYYY-MM-DD, today, tomorrow, a weekday (next occurrence,
// never today), +Nd / +Nw, or "" / "none" to clear. Returns YYYY-MM-DD or "".
func parseDate(s string, now time.Time) (string, error) {
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

// parsePriority accepts 0-3 or urgent/high/normal/low (also p0-p3).
func parsePriority(s string) (int, error) {
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
