package store

import (
	"path/filepath"
	"testing"

	"github.com/4damrr/tr4cker/internal/db"
)

// newTestStore returns a Store over a fresh temporary database. The starter
// rows seeded by db.Open are removed so each test starts from a known set of
// accounts and categories.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "tr4cker.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	s := New(conn)
	clearSeeded(t, s)
	return s
}

func clearSeeded(t *testing.T, s *Store) {
	t.Helper()
	for _, stmt := range []string{
		`DELETE FROM account`,
		`DELETE FROM category`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("clear %q: %v", stmt, err)
		}
	}
}

// mustAccount creates an account or fails the test.
func mustAccount(t *testing.T, s *Store, key string, balance float64) Account {
	t.Helper()
	a, err := s.CreateAccount(key, coerceAmount(balance), "")
	if err != nil {
		t.Fatalf("create account %q: %v", key, err)
	}
	return a
}

// mustCategory creates a category or fails the test.
func mustCategory(t *testing.T, s *Store, key string, bucket Bucket) Category {
	t.Helper()
	c, err := s.CreateCategory(key, bucket)
	if err != nil {
		t.Fatalf("create category %q: %v", key, err)
	}
	return c
}

func TestTimeFormatMatchesDB(t *testing.T) {
	if timeFormat != db.TimeFormat {
		t.Errorf("store timeFormat = %q, db.TimeFormat = %q; they must match", timeFormat, db.TimeFormat)
	}
}
