package store

import (
	"errors"
	"testing"

	"github.com/4damrr/tr4cker/internal/money"
)

func TestCreateToBuyStoresEveryField(t *testing.T) {
	s := fixture(t)

	item, err := s.CreateToBuy(NewToBuy{
		Category: "Food", Name: "Keyboard", Priority: PriorityHigh,
		Qty: 1, Price: money.New(750000), URL: "", Notes: "",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.ID == 0 {
		t.Error("item was not assigned an id")
	}
	if item.Name != "Keyboard" {
		t.Errorf("name = %q, want Keyboard", item.Name)
	}
	if item.Priority != PriorityHigh {
		t.Errorf("priority = %d, want 1", int(item.Priority))
	}
	if item.Status != StatusToBuy {
		t.Errorf("status = %q, want to_buy", item.Status)
	}

	got, err := s.GetToBuy(item.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Price != money.New(750000) {
		t.Errorf("price = %s, want 750,000", got.Price)
	}
}

func TestExpectedTotalIsQuantityTimesPrice(t *testing.T) {
	tests := []struct {
		name  string
		qty   int
		price string
		want  string
	}{
		{"whole units", 3, "125000", "375,000"},
		{"single unit", 1, "750000", "750,000"},
		{"fractional price", 3, "125000.55", "375,001.65"},
		{"single fractional unit", 1, "74999.99", "74,999.99"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := ToBuyItem{Qty: tc.qty, Price: money.New(parseFloat(t, tc.price))}
			if got := item.ExpectedTotal().String(); got != tc.want {
				t.Errorf("ExpectedTotal() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCreateToBuyRoundsFinerPriceInput(t *testing.T) {
	s := fixture(t)

	parsed, err := money.Parse("74999.995")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	item, err := s.CreateToBuy(NewToBuy{
		Category: "Food", Name: "Fine", Priority: PriorityMedium, Qty: 1, Price: parsed,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := item.Price.String(); got != "75,000" {
		t.Errorf("price = %s, want 75,000", got)
	}
}

func TestListToBuyOrdersByPriorityThenName(t *testing.T) {
	s := fixture(t)

	mustCreate := func(name string, priority Priority) {
		t.Helper()
		if _, err := s.CreateToBuy(NewToBuy{
			Category: "Food", Name: name, Priority: priority, Qty: 1, Price: money.New(1000),
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mustCreate("Medium item", PriorityMedium)
	mustCreate("High item", PriorityHigh)
	mustCreate("Low item", PriorityLow)
	mustCreate("Another high item", PriorityHigh)

	items, err := s.ListToBuy(nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []string{"Another high item", "High item", "Medium item", "Low item"}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, name := range want {
		if items[i].Name != name {
			t.Errorf("item %d = %q, want %q", i, items[i].Name, name)
		}
	}
}

func TestListToBuyFiltersByStatus(t *testing.T) {
	s := fixture(t)

	pending, err := s.CreateToBuy(NewToBuy{Category: "Food", Name: "Pending", Priority: PriorityHigh, Qty: 1, Price: money.New(1000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	bought, err := s.CreateToBuy(NewToBuy{Category: "Food", Name: "Bought", Priority: PriorityHigh, Qty: 1, Price: money.New(2000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	canceled, err := s.CreateToBuy(NewToBuy{Category: "Food", Name: "Canceled", Priority: PriorityHigh, Qty: 1, Price: money.New(3000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := s.SetToBuyStatus(bought.ID, StatusBought); err != nil {
		t.Fatalf("mark bought: %v", err)
	}
	if _, err := s.SetToBuyStatus(canceled.ID, StatusCanceled); err != nil {
		t.Fatalf("mark canceled: %v", err)
	}

	all, err := s.ListToBuy(nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("unfiltered list = %d items, want 3", len(all))
	}

	wantToBuy := StatusToBuy
	pendingOnly, err := s.ListToBuy(&wantToBuy)
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(pendingOnly) != 1 || pendingOnly[0].ID != pending.ID {
		t.Errorf("filtered list = %+v, want only the pending item", pendingOnly)
	}
}

func TestSetToBuyStatusCoversAllThreeStatuses(t *testing.T) {
	s := fixture(t)

	item, err := s.CreateToBuy(NewToBuy{Category: "Food", Name: "Monitor", Priority: PriorityMedium, Qty: 2, Price: money.New(500000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	steps := []struct {
		to   BuyingStatus
		name string
	}{
		{StatusBought, "mark bought"},
		{StatusCanceled, "cancel"},
		{StatusToBuy, "reopen"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got, err := s.SetToBuyStatus(item.ID, step.to)
			if err != nil {
				t.Fatalf("set status: %v", err)
			}
			if got.Status != step.to {
				t.Errorf("status = %q, want %q", got.Status, step.to)
			}
		})
	}

	if _, err := s.SetToBuyStatus(item.ID, BuyingStatus("bogus")); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid status error = %v, want ErrInvalid", err)
	}
}

func TestCreateToBuyValidatesInput(t *testing.T) {
	tests := []struct {
		name   string
		in     NewToBuy
		wantIs error
	}{
		{"missing name", NewToBuy{Category: "Food", Priority: PriorityHigh, Qty: 1}, ErrInvalid},
		{"missing category", NewToBuy{Name: "Thing", Priority: PriorityHigh, Qty: 1}, ErrInvalid},
		{"priority zero", NewToBuy{Name: "Thing", Category: "Food", Qty: 1}, ErrInvalid},
		{"priority four", NewToBuy{Name: "Thing", Category: "Food", Priority: 4, Qty: 1}, ErrInvalid},
		{"negative quantity", NewToBuy{Name: "Thing", Category: "Food", Priority: PriorityHigh, Qty: -1}, ErrInvalid},
		{"negative price", NewToBuy{Name: "Thing", Category: "Food", Priority: PriorityHigh, Qty: 1, Price: money.New(-5)}, ErrInvalid},
		{"bad status", NewToBuy{Name: "Thing", Category: "Food", Priority: PriorityHigh, Qty: 1, Status: BuyingStatus("bogus")}, ErrInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			if _, err := s.CreateToBuy(tc.in); !errors.Is(err, tc.wantIs) {
				t.Errorf("error = %v, want %v", err, tc.wantIs)
			}
			items, err := s.ListToBuy(nil)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(items) != 0 {
				t.Errorf("stored %d items after a rejected create, want 0", len(items))
			}
		})
	}
}

func TestUpdateToBuyChangesFieldsAndRecalculatesTotal(t *testing.T) {
	s := fixture(t)

	item, err := s.CreateToBuy(NewToBuy{
		Category: "Food", Name: "Keyboard", Priority: PriorityHigh,
		Qty: 1, Price: money.New(750000), URL: "https://example.test/k",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.UpdateToBuy(item.ID, NewToBuy{
		Category: "Fun", Name: "Keyboard v2", Priority: PriorityLow,
		Qty: 3, Price: money.New(700000), URL: "https://example.test/k2", Notes: "wait for sale",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Keyboard v2" || updated.Category != "Fun" {
		t.Errorf("updated = %+v, want the new name and category", updated)
	}
	if updated.Priority != PriorityLow {
		t.Errorf("priority = %d, want 3", int(updated.Priority))
	}
	if got := updated.ExpectedTotal().String(); got != "2,100,000" {
		t.Errorf("ExpectedTotal() = %s, want 2,100,000", got)
	}
	if updated.Notes != "wait for sale" {
		t.Errorf("notes = %q, want %q", updated.Notes, "wait for sale")
	}
}

func TestDeleteToBuyRemovesTheItem(t *testing.T) {
	s := fixture(t)

	item, err := s.CreateToBuy(NewToBuy{Category: "Food", Name: "Thing", Priority: PriorityHigh, Qty: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.DeleteToBuy(item.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	items, err := s.ListToBuy(nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("list = %d items, want 0", len(items))
	}
	if _, err := s.GetToBuy(item.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetToBuy error = %v, want ErrNotFound", err)
	}
}

func TestMissingToBuyOperationsReturnNotFound(t *testing.T) {
	s := fixture(t)

	if err := s.DeleteToBuy(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteToBuy error = %v, want ErrNotFound", err)
	}
	if _, err := s.SetToBuyStatus(999, StatusBought); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetToBuyStatus error = %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateToBuy(999, NewToBuy{Name: "x", Category: "Food", Priority: PriorityHigh}); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateToBuy error = %v, want ErrNotFound", err)
	}
}
