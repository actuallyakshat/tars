package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func setVersion(t *testing.T, path string, v int) {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`UPDATE schema_version SET v = ?`, v); err != nil {
		t.Fatal(err)
	}
}

func version(t *testing.T, s *Store) int {
	t.Helper()
	var v int
	if err := s.db.QueryRow(`SELECT v FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// tars 0.1.3 and older wrote their lower version over newer databases.
// Whatever version a database is knocked back to, reopening it reruns the
// steps cleanly and keeps the data.
func TestHealsADatabaseWhoseVersionWasKnockedBack(t *testing.T) {
	for back := 0; back < len(migrations); back++ {
		path := filepath.Join(t.TempDir(), "tars.db")
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		task := mustCreate(t, s, &Task{Title: "keep me"})
		if err := s.AddReport(&Report{TaskID: task.ID, Done: "did it", NeedsYou: "which?"}); err != nil {
			t.Fatal(err)
		}
		s.Close()
		setVersion(t, path, back)

		s, err = Open(path)
		if err != nil {
			t.Fatalf("version knocked back to %d: reopen: %v", back, err)
		}
		if v := version(t, s); v != len(migrations) {
			t.Fatalf("version knocked back to %d: now %d, want %d", back, v, len(migrations))
		}
		got, err := s.GetTask(task.ID)
		if err != nil || got.Title != "keep me" {
			t.Fatalf("version knocked back to %d: task = %+v, %v", back, got, err)
		}
		r, err := s.LatestReport(task.ID)
		if err != nil || r == nil || r.Done != "did it" {
			t.Fatalf("version knocked back to %d: report = %+v, %v", back, r, err)
		}
		s.Close()
	}
}

func TestRefusesADatabaseFromANewerTars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tars.db")
	setVersion(t, path, len(migrations)+1)
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "newer tars") {
		t.Fatalf("Open = %v, want a newer-tars error", err)
	}
	// The version is left alone for the newer tars.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`SELECT v FROM schema_version`).Scan(&v); err != nil || v != len(migrations)+1 {
		t.Fatalf("version = %d, %v; want %d", v, err, len(migrations)+1)
	}
}
