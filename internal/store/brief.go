package store

import "time"

// Brief groups open work into the buckets a morning summary cares about.
type Brief struct {
	Date         string  `json:"date"`
	NeedsYou     []*Task `json:"needs_you"` // latest report asks the user something unanswered
	InReview     []*Task `json:"in_review"`
	InProgress   []*Task `json:"in_progress"`
	Overdue      []*Task `json:"overdue"`
	DueToday     []*Task `json:"due_today"`
	FollowUps    []*Task `json:"follow_ups"` // blocked tasks whose follow-up date has arrived
	Stale        []*Task `json:"stale"`      // in-progress, untouched for StaleAfter
	Ready        []*Task `json:"ready"`
	Inbox        []*Task `json:"inbox"`
	DoneRecently []*Task `json:"done_recently"` // finished in the last 24h
}

const StaleAfter = 72 * time.Hour

func (s *Store) Brief() (*Brief, error) {
	all, err := s.ListTasks(Filter{IncludeDone: true})
	if err != nil {
		return nil, err
	}
	t0 := time.Now()
	today := t0.Format("2006-01-02")
	b := &Brief{Date: today}
	for _, t := range all {
		if t.Status == StatusDone {
			if done, err := time.Parse(time.RFC3339, t.DoneAt); err == nil && t0.Sub(done) < 24*time.Hour {
				b.DoneRecently = append(b.DoneRecently, t)
			}
			continue
		}
		if t.NeedsYou {
			b.NeedsYou = append(b.NeedsYou, t)
		}
		switch t.Status {
		case StatusInReview:
			b.InReview = append(b.InReview, t)
		case StatusInProgress:
			b.InProgress = append(b.InProgress, t)
			if upd, err := time.Parse(time.RFC3339, t.UpdatedAt); err == nil && t0.Sub(upd) > StaleAfter {
				b.Stale = append(b.Stale, t)
			}
		case StatusBlocked:
			if t.FollowUp != "" && t.FollowUp <= today {
				b.FollowUps = append(b.FollowUps, t)
			}
		case StatusReady:
			b.Ready = append(b.Ready, t)
		case StatusInbox:
			b.Inbox = append(b.Inbox, t)
		}
		if t.Due != "" && t.Due < today {
			b.Overdue = append(b.Overdue, t)
		} else if t.Due == today {
			b.DueToday = append(b.DueToday, t)
		}
	}
	return b, nil
}
