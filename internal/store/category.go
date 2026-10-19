package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Bucket is the high-level classification shared by transactions, planning and
// to-buy items.
type Bucket string

// The three buckets defined by openspec/specs/database-design.md.
const (
	BucketNeeds   Bucket = "needs"
	BucketWants   Bucket = "wants"
	BucketSavings Bucket = "savings"
)

// ValidBucket reports whether b is one of the three defined buckets.
func ValidBucket(b Bucket) bool {
	switch b {
	case BucketNeeds, BucketWants, BucketSavings:
		return true
	}
	return false
}

// Category classifies transactions, planning entries and to-buy items.
type Category struct {
	Key    string
	Bucket Bucket
}

const categoryColumns = `key, buckets, deleted_at`

func scanCategory(row interface{ Scan(...any) error }) (Category, bool, error) {
	var (
		c       Category
		bucket  string
		deleted sql.NullString
	)
	if err := row.Scan(&c.Key, &bucket, &deleted); err != nil {
		return Category{}, false, err
	}
	c.Bucket = Bucket(bucket)
	return c, deleted.Valid, nil
}

// ListCategories returns the active categories grouped by bucket order
// (needs, wants, savings) and then by key. Deactivated categories are excluded
// so they are never offered on a new record.
func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(
		`SELECT ` + categoryColumns + `
		 FROM category
		 WHERE deleted_at IS NULL
		 ORDER BY CASE buckets WHEN 'needs' THEN 0 WHEN 'wants' THEN 1 ELSE 2 END, key`,
	)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		c, _, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// GetCategory returns an active category by key.
func (s *Store) GetCategory(key string) (Category, error) {
	row := s.db.QueryRow(
		`SELECT `+categoryColumns+` FROM category WHERE key = ? AND deleted_at IS NULL`, key,
	)
	c, _, err := scanCategory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, fmt.Errorf("get category %q: %w", key, err)
	}
	return c, nil
}

// CreateCategory adds a new category. The bucket is checked here as well as by
// the schema's CHECK constraint so the caller gets ErrInvalid rather than a
// driver error.
func (s *Store) CreateCategory(key string, bucket Bucket) (Category, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Category{}, fmt.Errorf("%w: category key is required", ErrInvalid)
	}
	if !ValidBucket(bucket) {
		return Category{}, fmt.Errorf("%w: bucket %q", ErrInvalid, bucket)
	}

	var existing string
	err := s.db.QueryRow(`SELECT key FROM category WHERE key = ?`, key).Scan(&existing)
	switch {
	case err == nil:
		return Category{}, fmt.Errorf("%w: category %q", ErrDuplicate, key)
	case !errors.Is(err, sql.ErrNoRows):
		return Category{}, fmt.Errorf("check category %q: %w", key, err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO category (key, buckets) VALUES (?, ?)`, key, string(bucket),
	); err != nil {
		return Category{}, fmt.Errorf("insert category %q: %w", key, err)
	}
	return Category{Key: key, Bucket: bucket}, nil
}

// UpdateCategory changes the bucket of an active category.
func (s *Store) UpdateCategory(key string, bucket Bucket) (Category, error) {
	if !ValidBucket(bucket) {
		return Category{}, fmt.Errorf("%w: bucket %q", ErrInvalid, bucket)
	}
	res, err := s.db.Exec(
		`UPDATE category SET buckets = ? WHERE key = ? AND deleted_at IS NULL`,
		string(bucket), key,
	)
	if err != nil {
		return Category{}, fmt.Errorf("update category %q: %w", key, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return Category{}, fmt.Errorf("%w: category %q", ErrNotFound, key)
	}
	return Category{Key: key, Bucket: bucket}, nil
}

// DeactivateCategory soft-deletes a category so it disappears from new-record
// choices without removing the row, leaving historical transactions readable.
func (s *Store) DeactivateCategory(key string) error {
	res, err := s.db.Exec(
		`UPDATE category SET deleted_at = ? WHERE key = ? AND deleted_at IS NULL`,
		nowString(), key,
	)
	if err != nil {
		return fmt.Errorf("deactivate category %q: %w", key, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: category %q", ErrNotFound, key)
	}
	return nil
}

// CategoryExists reports whether key is an active category.
func (s *Store) CategoryExists(key string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM category WHERE key = ? AND deleted_at IS NULL`, key,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("category exists %q: %w", key, err)
	}
	return n > 0, nil
}
