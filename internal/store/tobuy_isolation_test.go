package store

import (
	"testing"
	"time"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

// TestMarkingBoughtCreatesNoTransaction is the guard behind
// specs/to-buy-list - "Marking an item bought does not record a transaction".
func TestMarkingBoughtCreatesNoTransaction(t *testing.T) {
	s := fixture(t)
	p := period.Now()

	item, err := s.CreateToBuy(NewToBuy{
		Category: "Food", Name: "Keyboard", Priority: PriorityHigh,
		Qty: 1, Price: money.New(750000),
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	startBRI := balance(t, s, "BRI")
	startGopay := balance(t, s, "Gopay")

	before, err := s.ListTransactions(TransactionFilter{})
	if err != nil {
		t.Fatalf("list before: %v", err)
	}
	spendingBefore, err := s.ActualForCategory("Food", BucketNeeds, p)
	if err != nil {
		t.Fatalf("actual before: %v", err)
	}

	for _, status := range []BuyingStatus{StatusBought, StatusCanceled, StatusToBuy} {
		if _, err := s.SetToBuyStatus(item.ID, status); err != nil {
			t.Fatalf("set status %s: %v", status, err)
		}
	}

	after, err := s.ListTransactions(TransactionFilter{})
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("transactions = %d after status changes, want %d", len(after), len(before))
	}

	if got := balance(t, s, "BRI"); got != startBRI {
		t.Errorf("BRI = %s after status changes, want %s", got, startBRI)
	}
	if got := balance(t, s, "Gopay"); got != startGopay {
		t.Errorf("Gopay = %s after status changes, want %s", got, startGopay)
	}

	spendingAfter, err := s.ActualForCategory("Food", BucketNeeds, p)
	if err != nil {
		t.Fatalf("actual after: %v", err)
	}
	if spendingAfter != spendingBefore {
		t.Errorf("spending = %s after status changes, want %s", spendingAfter, spendingBefore)
	}

	// The planned total for the category must be untouched too.
	rows, err := s.PlanRows(p)
	if err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	for _, r := range rows {
		if r.Category.Key == "Food" && r.Planned {
			t.Errorf("Food gained a plan of %s from a status change", r.Amount)
		}
	}
}

// TestMarkingBoughtDoesNotAffectAnExistingPlan covers the case where the item's
// category already has both a plan and spending in the current period.
func TestMarkingBoughtDoesNotAffectAnExistingPlan(t *testing.T) {
	s := fixture(t)
	p := period.Now()

	if _, err := s.SetPlan(p, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "900000", "BRI", "Food", p.Start().Add(24*time.Hour))); err != nil {
		t.Fatalf("expense: %v", err)
	}

	before, err := s.PlanRows(p)
	if err != nil {
		t.Fatalf("plan rows before: %v", err)
	}

	item, err := s.CreateToBuy(NewToBuy{
		Category: "Food", Name: "Keyboard", Priority: PriorityHigh, Qty: 1, Price: money.New(750000),
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := s.SetToBuyStatus(item.ID, StatusBought); err != nil {
		t.Fatalf("mark bought: %v", err)
	}

	after, err := s.PlanRows(p)
	if err != nil {
		t.Fatalf("plan rows after: %v", err)
	}

	beforeFood := findRow(t, before, "Food")
	afterFood := findRow(t, after, "Food")

	if afterFood.Actual != beforeFood.Actual {
		t.Errorf("Food actual = %s, want %s", afterFood.Actual, beforeFood.Actual)
	}
	if afterFood.Remaining != beforeFood.Remaining {
		t.Errorf("Food remaining = %s, want %s", afterFood.Remaining, beforeFood.Remaining)
	}
	if afterFood.Amount != beforeFood.Amount {
		t.Errorf("Food planned = %s, want %s", afterFood.Amount, beforeFood.Amount)
	}
	if beforeFood.Actual.String() != "900,000" {
		t.Errorf("fixture spending = %s, want 900,000", beforeFood.Actual)
	}
}

func findRow(t *testing.T, rows []PlanRow, key string) PlanRow {
	t.Helper()
	for _, r := range rows {
		if r.Category.Key == key {
			return r
		}
	}
	t.Fatalf("category %q not found in plan rows", key)
	return PlanRow{}
}
