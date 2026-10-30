package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Suggested repo profile sections. Any section name is accepted.
var RepoSections = []string{"dod", "tests", "setup", "db", "conventions", "gotchas"}

// Suggested personal profile sections. Any section name is accepted.
var ProfileSections = []string{"style", "review", "commits", "avoid"}

type Section struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Repo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`
	UpdatedAt string    `json:"updated_at"`
	Sections  []Section `json:"sections,omitempty"`
}

// UpsertRepo creates the repo if missing and updates its path when non-empty.
func (s *Store) UpsertRepo(name, path string) error {
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		path = abs
	}
	_, err := s.db.Exec(`INSERT INTO repos (name, path, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			path = CASE WHEN excluded.path != '' THEN excluded.path ELSE repos.path END,
			updated_at = excluded.updated_at`, name, path, now())
	return err
}

// SetRepoSection stores content for a section; empty content deletes it.
func (s *Store) SetRepoSection(repo, section, content string) error {
	if err := s.UpsertRepo(repo, ""); err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		_, err := s.db.Exec(`DELETE FROM repo_sections WHERE repo = ? AND section = ?`, repo, section)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO repo_sections (repo, section, content) VALUES (?, ?, ?)
		ON CONFLICT(repo, section) DO UPDATE SET content = excluded.content`, repo, section, content)
	return err
}

func (s *Store) GetRepo(name string) (*Repo, error) {
	var r Repo
	err := s.db.QueryRow(`SELECT name, path, updated_at FROM repos WHERE name = ?`, name).
		Scan(&r.Name, &r.Path, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("repo %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	r.Sections, err = s.sections(`SELECT section, content FROM repo_sections WHERE repo = ?`, name)
	return &r, err
}

// FindRepoByPath returns the repo whose path contains dir (deepest match wins).
func (s *Store) FindRepoByPath(dir string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	repos, err := s.ListRepos()
	if err != nil {
		return nil, err
	}
	var best *Repo
	for _, r := range repos {
		if r.Path == "" {
			continue
		}
		if abs == r.Path || strings.HasPrefix(abs, r.Path+string(filepath.Separator)) {
			if best == nil || len(r.Path) > len(best.Path) {
				best = r
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no repo profile covers %s: %w", abs, ErrNotFound)
	}
	return s.GetRepo(best.Name)
}

func (s *Store) ListRepos() ([]*Repo, error) {
	rows, err := s.db.Query(`SELECT name, path, updated_at FROM repos ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Repo
	for rows.Next() {
		var r Repo
		if err := rows.Scan(&r.Name, &r.Path, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

// RenameRepo renames a repo profile. Its sections follow (ON UPDATE CASCADE)
// and tasks that named the old repo are moved to the new name.
func (s *Store) RenameRepo(from, to string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("repo name can't be empty")
	}
	if to == from {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taken int
	if err := tx.QueryRow(`SELECT count(*) FROM repos WHERE name = ?`, to).Scan(&taken); err != nil {
		return err
	}
	if taken > 0 {
		return fmt.Errorf("there's already a repo called %q", to)
	}
	res, err := tx.Exec(`UPDATE repos SET name = ?, updated_at = ? WHERE name = ?`, to, now(), from)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("repo %q: %w", from, ErrNotFound)
	}
	if _, err := tx.Exec(`UPDATE tasks SET repo = ? WHERE repo = ?`, to, from); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteRepo(name string) error {
	res, err := s.db.Exec(`DELETE FROM repos WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("repo %q: %w", name, ErrNotFound)
	}
	return nil
}

func (s *Store) SetProfileSection(section, content string) error {
	if strings.TrimSpace(content) == "" {
		_, err := s.db.Exec(`DELETE FROM profile WHERE section = ?`, section)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO profile (section, content) VALUES (?, ?)
		ON CONFLICT(section) DO UPDATE SET content = excluded.content`, section, content)
	return err
}

func (s *Store) Profile() ([]Section, error) {
	return s.sections(`SELECT section, content FROM profile`)
}

// sections returns section rows ordered by name.
func (s *Store) sections(query string, args ...any) ([]Section, error) {
	rows, err := s.db.Query(query+` ORDER BY section`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Section
	for rows.Next() {
		var sec Section
		if err := rows.Scan(&sec.Name, &sec.Content); err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}
