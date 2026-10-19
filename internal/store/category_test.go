package store

import (
	"errors"
	"testing"
)

func TestCreateCategoryInEveryBucket(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		key    string
		bucket Bucket
	}{
		{"Food", BucketNeeds},
		{"Entertainment", BucketWants},
		{"Emergency", BucketSavings},
	}

	for _, tc := range tests {
		t.Run(string(tc.bucket), func(t *testing.T) {
			got, err := s.CreateCategory(tc.key, tc.bucket)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got.Key != tc.key || got.Bucket != tc.bucket {
				t.Errorf("created %+v, want key %q bucket %q", got, tc.key, tc.bucket)
			}
		})
	}

	categories, err := s.ListCategories()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(categories) != 3 {
		t.Errorf("listed %d categories, want 3", len(categories))
	}
}

func TestCreateCategoryRejectsUnknownBucket(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateCategory("Nonsense", Bucket("bogus"))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}

	// The schema CHECK is a second line of defence.
	if _, err := s.db.Exec(`INSERT INTO category (key, buckets) VALUES ('Raw', 'bogus')`); err == nil {
		t.Error("raw insert with an unknown bucket succeeded, want CHECK violation")
	}
}

func TestCreateCategoryRejectsDuplicateAndEmptyKey(t *testing.T) {
	s := newTestStore(t)
	mustCategory(t, s, "Food", BucketNeeds)

	if _, err := s.CreateCategory("Food", BucketWants); !errors.Is(err, ErrDuplicate) {
		t.Errorf("error = %v, want ErrDuplicate", err)
	}
	if _, err := s.CreateCategory("  ", BucketNeeds); !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
}

func TestDeactivateCategoryHidesItFromChoicesButKeepsTheRow(t *testing.T) {
	s := newTestStore(t)
	mustCategory(t, s, "Food", BucketNeeds)
	mustCategory(t, s, "Fun", BucketWants)

	if err := s.DeactivateCategory("Fun"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	categories, err := s.ListCategories()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(categories) != 1 || categories[0].Key != "Food" {
		t.Errorf("categories = %+v, want only Food", categories)
	}

	var deletedAt any
	if err := s.db.QueryRow(`SELECT deleted_at FROM category WHERE key = 'Fun'`).Scan(&deletedAt); err != nil {
		t.Fatalf("read deactivated row: %v", err)
	}
	if deletedAt == nil {
		t.Error("deactivated category row was removed instead of soft-deleted")
	}

	if _, err := s.GetCategory("Fun"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetCategory(Fun) error = %v, want ErrNotFound", err)
	}
}

func TestUpdateCategoryChangesBucket(t *testing.T) {
	s := newTestStore(t)
	mustCategory(t, s, "Entertainment", BucketWants)

	if _, err := s.UpdateCategory("Entertainment", BucketSavings); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetCategory("Entertainment")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Bucket != BucketSavings {
		t.Errorf("bucket = %q, want savings", got.Bucket)
	}

	if _, err := s.UpdateCategory("Entertainment", Bucket("bogus")); !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
}

func TestListCategoriesOrdersByBucketThenKey(t *testing.T) {
	s := newTestStore(t)
	mustCategory(t, s, "Shopping", BucketWants)
	mustCategory(t, s, "Bills", BucketNeeds)
	mustCategory(t, s, "Emergency", BucketSavings)
	mustCategory(t, s, "Food", BucketNeeds)

	categories, err := s.ListCategories()
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []string{"Bills", "Food", "Shopping", "Emergency"}
	if len(categories) != len(want) {
		t.Fatalf("got %d categories, want %d", len(categories), len(want))
	}
	for i, key := range want {
		if categories[i].Key != key {
			t.Errorf("category %d = %q, want %q", i, categories[i].Key, key)
		}
	}
}

func TestDeactivateAndUpdateMissingCategoryReturnNotFound(t *testing.T) {
	s := newTestStore(t)

	if err := s.DeactivateCategory("Nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateCategory("Nope", BucketNeeds); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
