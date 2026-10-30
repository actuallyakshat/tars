package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusInbox      = "inbox"
	StatusReady      = "ready"
	StatusInProgress = "in-progress"
	StatusInReview   = "in-review"
	StatusBlocked    = "blocked"
	StatusDone       = "done"
)

// Statuses is the board order.
var Statuses = []string{StatusInbox, StatusReady, StatusInProgress, StatusInReview, StatusBlocked, StatusDone}

func ValidStatus(s string) bool {
	for _, v := range Statuses {
		if v == s {
			return true
		}
	}
	return false
}

var ErrNotFound = errors.New("not found")

type Task struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Status     string   `json:"status"`
	Priority   int      `json:"priority"` // 0 = urgent … 3 = low
	Jira       string   `json:"jira,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Due        string   `json:"due,omitempty"`
	Acceptance string   `json:"acceptance,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Worktree   string   `json:"worktree,omitempty"`
	WaitingOn  string   `json:"waiting_on,omitempty"`
	FollowUp   string   `json:"follow_up,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	DoneAt     string   `json:"done_at,omitempty"`
	Log        []Entry  `json:"log,omitempty"`
}

type Entry struct {
	ID     int64  `json:"id"`
	At     string `json:"at"`
	Author string `json:"author"` // "user" or "agent"
	Kind   string `json:"kind"`   // note, park, sweep, test, status
	Text   string `json:"text"`
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

const taskCols = `id, title, body, status, priority, jira, repo, tags, due, acceptance,
	branch, worktree, waiting_on, follow_up, created_at, updated_at, done_at`

type scanner interface{ Scan(...any) error }

func scanTask(r scanner) (*Task, error) {
	var t Task
	var tags string
	err := r.Scan(&t.ID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.Jira, &t.Repo, &tags,
		&t.Due, &t.Acceptance, &t.Branch, &t.Worktree, &t.WaitingOn, &t.FollowUp,
		&t.CreatedAt, &t.UpdatedAt, &t.DoneAt)
	if err != nil {
		return nil, err
	}
	t.Tags = splitTags(tags)
	return &t, nil
}

func splitTags(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) CreateTask(t *Task) error {
	if t.Status == "" {
		t.Status = StatusInbox
	}
	if !ValidStatus(t.Status) {
		return fmt.Errorf("invalid status %q", t.Status)
	}
	t.CreatedAt, t.UpdatedAt = now(), now()
	res, err := s.db.Exec(`INSERT INTO tasks (title, body, status, priority, jira, repo, tags, due,
		acceptance, branch, worktree, waiting_on, follow_up, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Body, t.Status, t.Priority, t.Jira, t.Repo, strings.Join(t.Tags, ","), t.Due,
		t.Acceptance, t.Branch, t.Worktree, t.WaitingOn, t.FollowUp, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return err
	}
	t.ID, err = res.LastInsertId()
	return err
}

func (s *Store) GetTask(id int64) (*Task, error) {
	t, err := scanTask(s.db.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	t.Log, err = s.Log(id)
	return t, err
}

// SaveTask writes every field of t back. Moving into or out of done maintains done_at.
func (s *Store) SaveTask(t *Task) error {
	if !ValidStatus(t.Status) {
		return fmt.Errorf("invalid status %q", t.Status)
	}
	if t.Status == StatusDone && t.DoneAt == "" {
		t.DoneAt = now()
	} else if t.Status != StatusDone {
		t.DoneAt = ""
	}
	t.UpdatedAt = now()
	_, err := s.db.Exec(`UPDATE tasks SET title=?, body=?, status=?, priority=?, jira=?, repo=?, tags=?,
		due=?, acceptance=?, branch=?, worktree=?, waiting_on=?, follow_up=?, updated_at=?, done_at=?
		WHERE id=?`,
		t.Title, t.Body, t.Status, t.Priority, t.Jira, t.Repo, strings.Join(t.Tags, ","), t.Due,
		t.Acceptance, t.Branch, t.Worktree, t.WaitingOn, t.FollowUp, t.UpdatedAt, t.DoneAt, t.ID)
	return err
}

func (s *Store) DeleteTask(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	return nil
}

type Filter struct {
	Statuses    []string
	IncludeDone bool
	HasJira     *bool
	Repo        string
	Tag         string
	Search      string
}

func (s *Store) ListTasks(f Filter) ([]*Task, error) {
	var where []string
	var args []any
	if len(f.Statuses) > 0 {
		where = append(where, "status IN (?"+strings.Repeat(",?", len(f.Statuses)-1)+")")
		for _, st := range f.Statuses {
			args = append(args, st)
		}
	} else if !f.IncludeDone {
		where = append(where, "status != 'done'")
	}
	if f.HasJira != nil {
		if *f.HasJira {
			where = append(where, "jira != ''")
		} else {
			where = append(where, "jira = ''")
		}
	}
	if f.Repo != "" {
		where = append(where, "repo = ?")
		args = append(args, f.Repo)
	}
	if f.Tag != "" {
		where = append(where, "(',' || tags || ',') LIKE ?")
		args = append(args, "%,"+f.Tag+",%")
	}
	if f.Search != "" {
		where = append(where, "(title LIKE ? OR body LIKE ? OR jira LIKE ?)")
		q := "%" + f.Search + "%"
		args = append(args, q, q, q)
	}
	query := `SELECT ` + taskCols + ` FROM tasks`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += ` ORDER BY priority, CASE WHEN due = '' THEN 1 ELSE 0 END, due, id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) AddLog(taskID int64, author, kind, text string) error {
	if _, err := s.db.Exec(`UPDATE tasks SET updated_at = ? WHERE id = ?`, now(), taskID); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO task_log (task_id, at, author, kind, text) VALUES (?, ?, ?, ?, ?)`,
		taskID, now(), author, kind, text)
	return err
}

func (s *Store) Log(taskID int64) ([]Entry, error) {
	rows, err := s.db.Query(`SELECT id, at, author, kind, text FROM task_log WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.At, &e.Author, &e.Kind, &e.Text); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
