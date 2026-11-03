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
	Via     string `json:"via,omitempty"` // why it's on the task; see the Via constants
	// Title says what this session did for the task, in a few words, as
	// Claude named it. It's per task: one session can do two jobs.
	Title string `json:"title,omitempty"`
}

// ThreadTitleMax caps a thread title: a few words, not a summary.
const ThreadTitleMax = 80

// SetThreadTitle names what a session did for a task.
func (s *Store) SetThreadTitle(taskID int64, sessionID, title string) error {
	title = strings.Join(strings.Fields(title), " ")
	if n := len([]rune(title)); n > ThreadTitleMax {
		return fmt.Errorf("--thread is %d characters; name it in a few words (under %d)", n, ThreadTitleMax)
	}
	res, err := s.db.Exec(`UPDATE task_sessions SET title = ? WHERE task_id = ? AND session_id = ?`, title, taskID, sessionID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %s isn't on #%d: %w", sessionID, taskID, ErrNotFound)
	}
	return nil
}

// Why a session is on a task.
const (
	ViaBranch   = "branch"   // a tars/<id> branch
	ViaWorktree = "worktree" // a worktree recorded on the task
	ViaPR       = "pr"       // the task's pull request was in the first prompt
	ViaJira     = "jira"     // the task's Jira key was in the branch or prompt
	ViaClaude   = "claude"   // Claude judged the request to be this work (tars link)
	ViaUser     = "user"     // the user filed it
	ViaAgent    = "agent"    // an agent in the session wrote to the task
)

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
// for whichever agent picks the task up next. Answering again replaces the
// answer, so a hasty one can be corrected; the log keeps both.
func (s *Store) Answer(taskID int64, answer, by string) error {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fmt.Errorf("answer is empty")
	}
	r, err := s.LatestReport(taskID)
	if err != nil {
		return err
	}
	if r == nil || r.NeedsYou == "" {
		return fmt.Errorf("task %d: %w", taskID, ErrNothingToAnswer)
	}
	if r.AnsweredAt != "" && r.Answer == answer {
		return nil
	}
	if _, err := s.db.Exec(`UPDATE reports SET answer = ?, answered_at = ? WHERE id = ?`, answer, now(), r.ID); err != nil {
		return err
	}
	text := "Q: " + r.NeedsYou + "\nA: " + answer
	if r.AnsweredAt != "" {
		text = "Q: " + r.NeedsYou + "\nA (changed from \"" + r.Answer + "\"): " + answer
	}
	return s.AddLog(taskID, by, "answer", text)
}

// LinkSession records that session sess touched task taskID, creating or
// refreshing the session itself. Empty name or launch dir keep stored values.
// via says why; the first reason recorded is kept.
func (s *Store) LinkSession(taskID int64, sess Session, via string) error {
	if sess.ID == "" {
		return nil
	}
	if err := s.RecordSession(sess); err != nil {
		return err
	}
	if err := s.linkCapture(taskID, sess.ID); err != nil {
		return err
	}
	t := now()
	_, err := s.db.Exec(`INSERT INTO task_sessions (task_id, session_id, first_at, last_at, via) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(task_id, session_id) DO UPDATE SET last_at = excluded.last_at,
			via = CASE WHEN task_sessions.via = '' THEN excluded.via ELSE task_sessions.via END`, taskID, sess.ID, t, t, via)
	return err
}

// SessionTasks resolves a session ID or unique prefix and lists the tasks
// it's on, most recent first.
func (s *Store) SessionTasks(prefix string) (string, []int64, error) {
	rows, err := s.db.Query(`SELECT DISTINCT session_id FROM task_sessions WHERE session_id LIKE ? ESCAPE '\' LIMIT 2`,
		strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)+"%")
	if err != nil {
		return "", nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return "", nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	switch len(ids) {
	case 0:
		return "", nil, fmt.Errorf("session %s isn't on any task: %w", prefix, ErrNotFound)
	case 2:
		return "", nil, fmt.Errorf("%q matches more than one session; use more of the ID", prefix)
	}
	trows, err := s.db.Query(`SELECT task_id FROM task_sessions WHERE session_id = ? ORDER BY last_at DESC`, ids[0])
	if err != nil {
		return "", nil, err
	}
	defer trows.Close()
	var tasks []int64
	for trows.Next() {
		var t int64
		if err := trows.Scan(&t); err != nil {
			return "", nil, err
		}
		tasks = append(tasks, t)
	}
	return ids[0], tasks, trows.Err()
}

// UnlinkSession takes a session off a task, for when it was linked by
// mistake. A session that changed code goes back to unfiled so it can be
// filed where it belongs; its reports stay on the task as history.
func (s *Store) UnlinkSession(taskID int64, sessionID string) error {
	res, err := s.db.Exec(`DELETE FROM task_sessions WHERE task_id = ? AND session_id = ?`, taskID, sessionID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %s isn't on #%d: %w", sessionID, taskID, ErrNotFound)
	}
	// Still on another task: leave its capture there.
	var others int
	if err := s.db.QueryRow(`SELECT count(*) FROM task_sessions WHERE session_id = ?`, sessionID).Scan(&others); err != nil {
		return err
	}
	if others > 0 {
		_, err = s.db.Exec(`UPDATE captures SET task_id = (SELECT task_id FROM task_sessions WHERE session_id = ? ORDER BY last_at DESC LIMIT 1)
			WHERE session_id = ? AND task_id = ?`, sessionID, sessionID, taskID)
		return err
	}
	_, err = s.db.Exec(`INSERT INTO captures (session_id, state, started_at) VALUES (?, 'unfiled', ?)
		ON CONFLICT(session_id) DO UPDATE SET state = 'unfiled', task_id = NULL`, sessionID, now())
	return err
}

// RecordSession creates or refreshes a session without tying it to a task.
// Empty name or launch dir keep stored values.
func (s *Store) RecordSession(sess Session) error {
	t := now()
	_, err := s.db.Exec(`INSERT INTO sessions (id, name, launch_dir, first_seen, last_seen) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE sessions.name END,
			launch_dir = CASE WHEN excluded.launch_dir != '' THEN excluded.launch_dir ELSE sessions.launch_dir END,
			last_seen = excluded.last_seen`, sess.ID, sess.Name, sess.LaunchDir, t, t)
	return err
}

// TaskSessions lists the sessions that touched a task, most recent first.
func (s *Store) TaskSessions(taskID int64) ([]TaskSession, error) {
	rows, err := s.db.Query(`SELECT s.id, s.name, s.launch_dir, s.first_seen, s.last_seen, ts.first_at, ts.last_at, ts.via, ts.title
		FROM task_sessions ts JOIN sessions s ON s.id = ts.session_id
		WHERE ts.task_id = ? ORDER BY ts.last_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskSession
	for rows.Next() {
		var ts TaskSession
		if err := rows.Scan(&ts.ID, &ts.Name, &ts.LaunchDir, &ts.FirstSeen, &ts.LastSeen, &ts.FirstAt, &ts.LastAt, &ts.Via, &ts.Title); err != nil {
			return nil, err
		}
		out = append(out, ts)
	}
	return out, rows.Err()
}
