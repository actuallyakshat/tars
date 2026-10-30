package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ReportFieldMax caps every report field. Reports exist so the user reads a few
// lines instead of a transcript; the cap keeps agents from drifting back into
// narrative.
const ReportFieldMax = 240

// Report is an agent's short, structured outcome for a task.
type Report struct {
	ID         int64  `json:"id"`
	TaskID     int64  `json:"task_id"`
	At         string `json:"at"`
	SessionID  string `json:"session_id,omitempty"`
	Done       string `json:"done"`
	Verified   string `json:"verified,omitempty"`
	NeedsYou   string `json:"needs_you,omitempty"`
	Next       string `json:"next,omitempty"`
	Answer     string `json:"answer,omitempty"`
	AnsweredAt string `json:"answered_at,omitempty"`
}

// Session is a Claude Code session that worked on tasks.
type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	LaunchDir string `json:"launch_dir,omitempty"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// TaskSession is a session as seen from one task.
type TaskSession struct {
	Session
	FirstAt string `json:"first_at"` // first time this session touched the task
	LastAt  string `json:"last_at"`
}

// Validate trims fields and enforces the length cap. Done is required.
func (r *Report) Validate() error {
	fields := []struct {
		name string
		v    *string
	}{{"done", &r.Done}, {"verified", &r.Verified}, {"needs-you", &r.NeedsYou}, {"next", &r.Next}}
	for _, f := range fields {
		*f.v = strings.TrimSpace(*f.v)
		if n := len([]rune(*f.v)); n > ReportFieldMax {
			return fmt.Errorf("--%s is %d characters; keep it under %d. Say what matters, the details belong in the session", f.name, n, ReportFieldMax)
		}
	}
	if r.Done == "" {
		return fmt.Errorf("--done is required: what changed, in a line or two")
	}
	return nil
}

// AddReport stores r and logs it as agent activity, so the task shows as
// unread in the TUI until the user opens it.
func (s *Store) AddReport(r *Report) error {
	if err := r.Validate(); err != nil {
		return err
	}
	r.At = now()
	res, err := s.db.Exec(`INSERT INTO reports (task_id, at, session_id, done, verified, needs_you, next)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, r.TaskID, r.At, r.SessionID, r.Done, r.Verified, r.NeedsYou, r.Next)
	if err != nil {
		return err
	}
	if r.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	summary := "Reported: " + r.Done
	if r.NeedsYou != "" {
		summary += "\nNeeds you: " + r.NeedsYou
	}
	return s.AddLog(r.TaskID, "agent", "report", summary)
}

func (s *Store) LatestReport(taskID int64) (*Report, error) {
	var r Report
	err := s.db.QueryRow(`SELECT id, task_id, at, session_id, done, verified, needs_you, next, answer, answered_at
		FROM reports WHERE task_id = ? ORDER BY id DESC LIMIT 1`, taskID).
		Scan(&r.ID, &r.TaskID, &r.At, &r.SessionID, &r.Done, &r.Verified, &r.NeedsYou, &r.Next, &r.Answer, &r.AnsweredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) Reports(taskID int64) ([]Report, error) {
	rows, err := s.db.Query(`SELECT id, task_id, at, session_id, done, verified, needs_you, next, answer, answered_at
		FROM reports WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.TaskID, &r.At, &r.SessionID, &r.Done, &r.Verified, &r.NeedsYou, &r.Next, &r.Answer, &r.AnsweredAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var ErrNothingToAnswer = errors.New("no open question on this task")

// Answer records the user's answer to the latest report's question and logs it
// for whichever agent picks the task up next.
func (s *Store) Answer(taskID int64, answer, by string) error {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fmt.Errorf("answer is empty")
	}
	r, err := s.LatestReport(taskID)
	if err != nil {
		return err
	}
	if r == nil || r.NeedsYou == "" || r.AnsweredAt != "" {
		return fmt.Errorf("task %d: %w", taskID, ErrNothingToAnswer)
	}
	if _, err := s.db.Exec(`UPDATE reports SET answer = ?, answered_at = ? WHERE id = ?`, answer, now(), r.ID); err != nil {
		return err
	}
	return s.AddLog(taskID, by, "answer", "Q: "+r.NeedsYou+"\nA: "+answer)
}

// LinkSession records that session sess touched task taskID, creating or
// refreshing the session itself. Empty name or launch dir keep stored values.
func (s *Store) LinkSession(taskID int64, sess Session) error {
	if sess.ID == "" {
		return nil
	}
	t := now()
	if _, err := s.db.Exec(`INSERT INTO sessions (id, name, launch_dir, first_seen, last_seen) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE sessions.name END,
			launch_dir = CASE WHEN excluded.launch_dir != '' THEN excluded.launch_dir ELSE sessions.launch_dir END,
			last_seen = excluded.last_seen`, sess.ID, sess.Name, sess.LaunchDir, t, t); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO task_sessions (task_id, session_id, first_at, last_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(task_id, session_id) DO UPDATE SET last_at = excluded.last_at`, taskID, sess.ID, t, t)
	return err
}

// TaskSessions lists the sessions that touched a task, most recent first.
func (s *Store) TaskSessions(taskID int64) ([]TaskSession, error) {
	rows, err := s.db.Query(`SELECT s.id, s.name, s.launch_dir, s.first_seen, s.last_seen, ts.first_at, ts.last_at
		FROM task_sessions ts JOIN sessions s ON s.id = ts.session_id
		WHERE ts.task_id = ? ORDER BY ts.last_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskSession
	for rows.Next() {
		var ts TaskSession
		if err := rows.Scan(&ts.ID, &ts.Name, &ts.LaunchDir, &ts.FirstSeen, &ts.LastSeen, &ts.FirstAt, &ts.LastAt); err != nil {
			return nil, err
		}
		out = append(out, ts)
	}
	return out, rows.Err()
}
