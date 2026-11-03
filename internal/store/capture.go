package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Capture states. A capture starts open on a session's first prompt in a git
// repo, and ends linked (to a task), unfiled (work done, no task: the user
// sorts it), dropped (no work) or dismissed (the user said it isn't a task).
// Skipped captures mark sessions tars won't follow, so later prompts in them
// cost one lookup.
const (
	CaptureOpen      = "open"
	CaptureLinked    = "linked"
	CaptureUnfiled   = "unfiled"
	CaptureDropped   = "dropped"
	CaptureDismissed = "dismissed"
	CaptureSkipped   = "skipped"
)

// Capture is what tars knows about one Claude session in a git repo.
type Capture struct {
	SessionID  string   `json:"session_id"`
	State      string   `json:"state"`
	TaskID     int64    `json:"task_id,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Root       string   `json:"root,omitempty"`     // the main checkout
	Worktree   string   `json:"worktree,omitempty"` // where the session ran
	Branch     string   `json:"branch,omitempty"`
	Head       string   `json:"-"`                // commit at the first prompt
	Tree       string   `json:"-"`                // hash of git status at the first prompt
	Prompt     string   `json:"prompt,omitempty"` // the first prompt
	Transcript string   `json:"-"`
	Edits      int      `json:"edits"`
	Commits    int      `json:"commits"`
	Files      []string `json:"files,omitempty"`
	StartedAt  string   `json:"started_at"`
	EndedAt    string   `json:"ended_at,omitempty"`
	Name       string   `json:"name,omitempty"` // the session's title, when tars has seen it
}

const captureCols = `c.session_id, c.state, COALESCE(c.task_id, 0), c.repo, c.root, c.worktree, c.branch,
	c.head, c.tree, c.prompt, c.transcript, c.edits, c.commits, c.files, c.started_at, c.ended_at,
	COALESCE((SELECT s.name FROM sessions s WHERE s.id = c.session_id), '')`

func scanCapture(r scanner) (*Capture, error) {
	var c Capture
	var files string
	err := r.Scan(&c.SessionID, &c.State, &c.TaskID, &c.Repo, &c.Root, &c.Worktree, &c.Branch,
		&c.Head, &c.Tree, &c.Prompt, &c.Transcript, &c.Edits, &c.Commits, &files, &c.StartedAt, &c.EndedAt, &c.Name)
	if err != nil {
		return nil, err
	}
	c.Files = splitTags(files)
	return &c, nil
}

// Title is what to call a captured session: its title, else the first line
// of its first prompt.
func (c *Capture) Title() string {
	if c.Name != "" {
		return c.Name
	}
	line, _, _ := strings.Cut(strings.TrimSpace(c.Prompt), "\n")
	if r := []rune(strings.TrimSpace(line)); len(r) > 80 {
		line = string(r[:80]) + "…"
	}
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return "Work in " + c.Repo
}

// FileCapture ties a captured session to task t, and records the session's
// branch, worktree and repo on the task where it has none, so the feature's
// other threads match it without asking. name and launchDir describe the
// session when known.
func (s *Store) FileCapture(c *Capture, t *Task, name, launchDir, via string) error {
	if launchDir == "" {
		launchDir = c.Worktree
	}
	if err := s.LinkSession(t.ID, Session{ID: c.SessionID, Name: name, LaunchDir: launchDir}, via); err != nil {
		return err
	}
	changed := false
	if t.Branch == "" && !TrunkBranch(c.Branch) {
		t.Branch, changed = c.Branch, true
	}
	if t.Worktree == "" && c.Worktree != "" && c.Worktree != c.Root {
		t.Worktree, changed = c.Worktree, true
	}
	if t.Repo == "" && c.Repo != "" {
		t.Repo, changed = c.Repo, true
	}
	if changed {
		return s.SaveTask(t)
	}
	return nil
}

// TrunkBranch reports branches shared by unrelated work, which never identify
// a task.
func TrunkBranch(branch string) bool {
	switch branch {
	case "", "HEAD", "main", "master", "develop", "dev", "trunk":
		return true
	}
	return false
}

// StartCapture records a session's first prompt. It reports false when the
// session was already captured, so callers only do their work once.
func (s *Store) StartCapture(c *Capture) (bool, error) {
	if c.State == "" {
		c.State = CaptureOpen
	}
	c.StartedAt = now()
	var task any
	if c.TaskID != 0 {
		task = c.TaskID
	}
	res, err := s.db.Exec(`INSERT INTO captures (session_id, state, task_id, repo, root, worktree, branch, head, tree, prompt, transcript, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(session_id) DO NOTHING`,
		c.SessionID, c.State, task, c.Repo, c.Root, c.Worktree, c.Branch, c.Head, c.Tree, c.Prompt, c.Transcript, c.StartedAt)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) GetCapture(id string) (*Capture, error) {
	c, err := scanCapture(s.db.QueryRow(`SELECT `+captureCols+` FROM captures c WHERE c.session_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("session %s: %w", id, ErrNotFound)
	}
	return c, err
}

// FindCapture resolves a session ID or a unique prefix of one, as shown in
// `tars unfiled`.
func (s *Store) FindCapture(prefix string) (*Capture, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, fmt.Errorf("session id is empty")
	}
	rows, err := s.db.Query(`SELECT `+captureCols+` FROM captures c WHERE c.session_id LIKE ? ESCAPE '\' LIMIT 2`,
		strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Capture
	for rows.Next() {
		c, err := scanCapture(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(out) {
	case 0:
		return nil, fmt.Errorf("session %s: %w", prefix, ErrNotFound)
	case 1:
		return out[0], nil
	}
	return nil, fmt.Errorf("%q matches more than one session; use more of the ID", prefix)
}

// Captures lists captures in the given states, newest first.
func (s *Store) Captures(states ...string) ([]*Capture, error) {
	if len(states) == 0 {
		return nil, nil
	}
	args := make([]any, len(states))
	for i, st := range states {
		args[i] = st
	}
	rows, err := s.db.Query(`SELECT `+captureCols+` FROM captures c WHERE c.state IN (?`+
		strings.Repeat(",?", len(states)-1)+`) ORDER BY c.started_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Capture
	for rows.Next() {
		c, err := scanCapture(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EndCapture records the work a session did. A session that's still open
// becomes unfiled when it changed something and dropped when it didn't; a
// dropped session that was resumed and did work becomes unfiled.
func (s *Store) EndCapture(id string, edits, commits int, files []string) error {
	state := CaptureDropped
	if edits > 0 || commits > 0 {
		state = CaptureUnfiled
	}
	_, err := s.db.Exec(`UPDATE captures SET edits = ?, commits = ?, files = ?, ended_at = ?,
		state = CASE WHEN state IN ('open', 'dropped', 'unfiled') THEN ? ELSE state END
		WHERE session_id = ?`, edits, commits, strings.Join(files, ","), now(), state, id)
	return err
}

// SetCaptureState moves a capture to state, e.g. dismissing it.
func (s *Store) SetCaptureState(id, state string) error {
	res, err := s.db.Exec(`UPDATE captures SET state = ? WHERE session_id = ?`, state, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %s: %w", id, ErrNotFound)
	}
	return nil
}

// linkCapture marks the session's capture as belonging to a task. Called from
// LinkSession, so anything that ties a session to a task files its capture.
func (s *Store) linkCapture(taskID int64, sessionID string) error {
	_, err := s.db.Exec(`UPDATE captures SET state = 'linked', task_id = ?
		WHERE session_id = ? AND state IN ('open', 'unfiled', 'dropped', 'dismissed')`, taskID, sessionID)
	return err
}

// Setting reads a stored setting, or def when it isn't set.
func (s *Store) Setting(key, def string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// CaptureEnabled is the global capture switch; it's on unless turned off.
func (s *Store) CaptureEnabled() (bool, error) {
	v, err := s.Setting("capture", "on")
	return v != "off", err
}

// SetRepoCapture turns capture on or off for one repo profile.
func (s *Store) SetRepoCapture(name string, on bool) error {
	res, err := s.db.Exec(`UPDATE repos SET capture = ?, updated_at = ? WHERE name = ?`, on, now(), name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("repo %q: %w", name, ErrNotFound)
	}
	return nil
}

// MergeTask folds task from into task into: its sessions, reports, log and
// captures move over, empty fields on into are filled from from, tags are
// combined, and from is deleted. Both tasks must exist and differ.
func (s *Store) MergeTask(from, into int64) error {
	if from == into {
		return fmt.Errorf("can't merge #%d into itself", from)
	}
	src, err := s.GetTask(from)
	if err != nil {
		return err
	}
	dst, err := s.GetTask(into)
	if err != nil {
		return err
	}
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&dst.Body, src.Body)
	fill(&dst.Jira, src.Jira)
	fill(&dst.Repo, src.Repo)
	fill(&dst.Due, src.Due)
	fill(&dst.Acceptance, src.Acceptance)
	fill(&dst.Branch, src.Branch)
	fill(&dst.Worktree, src.Worktree)
	prs := dst.PRs
	for _, p := range src.PRs {
		if !contains(prs, p) {
			prs = append(prs, p)
		}
	}
	tags := dst.Tags
	for _, t := range src.Tags {
		if !contains(tags, t) {
			tags = append(tags, t)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := now()
	steps := []struct {
		q    string
		args []any
	}{
		{`UPDATE tasks SET body=?, jira=?, repo=?, tags=?, due=?, acceptance=?, branch=?, worktree=?, pr=?, updated_at=? WHERE id=?`,
			[]any{dst.Body, dst.Jira, dst.Repo, strings.Join(tags, ","), dst.Due, dst.Acceptance, dst.Branch, dst.Worktree, strings.Join(prs, ","), t, into}},
		// A session that touched both tasks keeps its row on into.
		{`INSERT OR IGNORE INTO task_sessions (task_id, session_id, first_at, last_at, via, title)
			SELECT ?, session_id, first_at, last_at, via, title FROM task_sessions WHERE task_id = ?`, []any{into, from}},
		{`DELETE FROM task_sessions WHERE task_id = ?`, []any{from}},
		{`UPDATE reports SET task_id = ? WHERE task_id = ?`, []any{into, from}},
		{`UPDATE task_log SET task_id = ? WHERE task_id = ?`, []any{into, from}},
		{`UPDATE captures SET task_id = ? WHERE task_id = ?`, []any{into, from}},
		{`INSERT INTO task_log (task_id, at, author, kind, text) VALUES (?, ?, 'user', 'note', ?)`,
			[]any{into, t, fmt.Sprintf("Merged #%d into this task: %s", from, src.Title)}},
		{`DELETE FROM tasks WHERE id = ?`, []any{from}},
	}
	for _, st := range steps {
		if _, err := tx.Exec(st.q, st.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
