// Package store persists tasks, repo profiles and the personal profile in SQLite.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// DefaultPath honours $TARS_HOME, then $XDG_DATA_HOME, then ~/.local/share.
func DefaultPath() (string, error) {
	if dir := os.Getenv("TARS_HOME"); dir != "" {
		return filepath.Join(dir, "tars.db"), nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "tars", "tars.db"), nil
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// WAL + busy_timeout let parallel sweep agents write without "database is locked".
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migration is one schema step. Every step must be safe to run again: tars
// 0.1.3 and older wrote their own (lower) version over newer databases, and
// a newer tars then reruns steps that were already applied. Plain DDL uses
// IF NOT EXISTS; steps SQLite can't guard that way say when they're done.
type migration struct {
	sql     string
	applied func(tx *sql.Tx) (bool, error) // optional
}

var migrations = []migration{
	{sql: `CREATE TABLE IF NOT EXISTS tasks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title       TEXT NOT NULL,
		body        TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'inbox',
		priority    INTEGER NOT NULL DEFAULT 2,
		jira        TEXT NOT NULL DEFAULT '',
		repo        TEXT NOT NULL DEFAULT '',
		tags        TEXT NOT NULL DEFAULT '',
		due         TEXT NOT NULL DEFAULT '',
		acceptance  TEXT NOT NULL DEFAULT '',
		branch      TEXT NOT NULL DEFAULT '',
		worktree    TEXT NOT NULL DEFAULT '',
		waiting_on  TEXT NOT NULL DEFAULT '',
		follow_up   TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		done_at     TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS task_log (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id  INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		at       TEXT NOT NULL,
		author   TEXT NOT NULL,
		kind     TEXT NOT NULL,
		text     TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS task_log_task ON task_log(task_id);
	CREATE TABLE IF NOT EXISTS repos (
		name        TEXT PRIMARY KEY,
		path        TEXT NOT NULL DEFAULT '',
		updated_at  TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS repo_sections (
		repo     TEXT NOT NULL REFERENCES repos(name) ON DELETE CASCADE ON UPDATE CASCADE,
		section  TEXT NOT NULL,
		content  TEXT NOT NULL,
		PRIMARY KEY (repo, section)
	);
	CREATE TABLE IF NOT EXISTS profile (
		section  TEXT PRIMARY KEY,
		content  TEXT NOT NULL
	);`},
	// seen_at drives the "updated by the agent since you last looked" marker.
	// Existing tasks count as seen so the marker starts clean.
	{
		sql: `ALTER TABLE tasks ADD COLUMN seen_at TEXT NOT NULL DEFAULT '';
	UPDATE tasks SET seen_at = updated_at;`,
		applied: func(tx *sql.Tx) (bool, error) { return hasColumn(tx, "tasks", "seen_at") },
	},
	// Reports are the short, structured outcomes agents write instead of long
	// narratives; sessions link tasks to the Claude sessions that worked on them.

	{sql: `CREATE TABLE IF NOT EXISTS reports (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id      INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		at           TEXT NOT NULL,
		session_id   TEXT NOT NULL DEFAULT '',
		done         TEXT NOT NULL,
		verified     TEXT NOT NULL DEFAULT '',
		needs_you    TEXT NOT NULL DEFAULT '',
		next         TEXT NOT NULL DEFAULT '',
		answer       TEXT NOT NULL DEFAULT '',
		answered_at  TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS reports_task ON reports(task_id);
	CREATE TABLE IF NOT EXISTS sessions (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL DEFAULT '',
		launch_dir  TEXT NOT NULL DEFAULT '',
		first_seen  TEXT NOT NULL,
		last_seen   TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS task_sessions (
		task_id     INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		first_at    TEXT NOT NULL,
		last_at     TEXT NOT NULL,
		PRIMARY KEY (task_id, session_id)
	);`},
	// The pull request holding the work, as a URL, set when an agent opens one.
	{
		sql:     `ALTER TABLE tasks ADD COLUMN pr TEXT NOT NULL DEFAULT '';`,
		applied: func(tx *sql.Tx) (bool, error) { return hasColumn(tx, "tasks", "pr") },
	},
}

// migrate runs pending migrations. Transactions are IMMEDIATE (see the DSN), so
// two processes opening a fresh database at once serialize instead of racing.
func hasColumn(tx *sql.Tx, table, column string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	return n > 0, err
}

func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_version (v INTEGER NOT NULL)`); err != nil {
		return err
	}
	var v int
	err = tx.QueryRow(`SELECT v FROM schema_version`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.Exec(`INSERT INTO schema_version (v) VALUES (0)`); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// A newer tars wrote this database. Writing our older version over it would
	// make the newer tars rerun migrations it already applied.
	if v > len(migrations) {
		return fmt.Errorf("this database was written by a newer tars (schema %d, this tars knows %d); upgrade tars", v, len(migrations))
	}
	if v == len(migrations) {
		return tx.Commit()
	}
	for i := v; i < len(migrations); i++ {
		m := migrations[i]
		if m.applied != nil {
			done, err := m.applied(tx)
			if err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
			if done {
				continue
			}
		}
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	if _, err := tx.Exec(`UPDATE schema_version SET v = ?`, len(migrations)); err != nil {
		return err
	}
	return tx.Commit()
}
