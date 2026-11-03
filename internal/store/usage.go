package store

import "sort"

// SessionLink puts a session on a task from FirstAt on. A session can move
// between tasks, so usage is charged to the link that was current at the time.
type SessionLink struct {
	TaskID  int64
	Title   string
	Status  string
	FirstAt string
}

// SessionLinks returns every session's task links, earliest first.
func (s *Store) SessionLinks() (map[string][]SessionLink, error) {
	rows, err := s.db.Query(`SELECT ts.session_id, ts.task_id, t.title, t.status, ts.first_at
		FROM task_sessions ts JOIN tasks t ON t.id = ts.task_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]SessionLink{}
	for rows.Next() {
		var id string
		var l SessionLink
		if err := rows.Scan(&id, &l.TaskID, &l.Title, &l.Status, &l.FirstAt); err != nil {
			return nil, err
		}
		out[id] = append(out[id], l)
	}
	for _, ls := range out {
		sort.Slice(ls, func(i, j int) bool { return ls[i].FirstAt < ls[j].FirstAt })
	}
	return out, rows.Err()
}

// CaptureRepos maps captured sessions to the repo they ran in.
func (s *Store) CaptureRepos() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT session_id, repo FROM captures WHERE repo != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, repo string
		if err := rows.Scan(&id, &repo); err != nil {
			return nil, err
		}
		out[id] = repo
	}
	return out, rows.Err()
}
