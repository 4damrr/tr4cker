package store

import (
	"errors"
	"testing"

	"github.com/4damrr/tr4cker/internal/money"
)

func TestCreateAccountRoundsBalance(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name    string
		balance string
		want    string
	}{
		{"whole balance", "2500000", "2,500,000"},
		{"fractional balance is kept", "25000.60", "25,000.60"},
		{"finer input is rounded", "25000.605", "25,000.61"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := money.Parse(tc.balance)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.balance, err)
			}
			key := "acct-" + tc.name
			if _, err := s.CreateAccount(key, parsed, ""); err != nil {
				t.Fatalf("create: %v", err)
			}
			got, err := s.GetAccount(key)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.Balance.String() != tc.want {
				t.Errorf("balance = %s, want %s", got.Balance, tc.want)
			}
		})
	}
}

func TestCreateAccountRejectsDuplicateKey(t *testing.T) {
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 1000)

	_, err := s.CreateAccount("BRI", 0, "")
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("error = %v, want ErrDuplicate", err)
	}
}

func TestCreateAccountRequiresKey(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateAccount("   ", 0, "")
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
}

func TestListAccountsExcludesDeactivatedButKeepsHistory(t *testing.T) {
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 2500000)
	mustAccount(t, s, "Gopay", 500000)
	mustAccount(t, s, "Cash", 150000)

	if err := s.DeactivateAccount("Cash"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	accounts, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	keys := map[string]money.Amount{}
	for _, a := range accounts {
		keys[a.Key] = a.Balance
	}
	if len(accounts) != 2 {
		t.Errorf("active accounts = %d, want 2 (Cash must be excluded)", len(accounts))
	}
	if _, ok := keys["Cash"]; ok {
		t.Error("deactivated account still listed as active")
	}
	if keys["BRI"] != money.New(2500000) {
		t.Errorf("BRI balance = %s, want 2500000", keys["BRI"])
	}
	if keys["Gopay"] != money.New(500000) {
		t.Errorf("Gopay balance = %s, want 500000", keys["Gopay"])
	}

	// The row itself survives so history recorded against it stays readable.
	var deletedAt any
	err = s.db.QueryRow(`SELECT deleted_at FROM account WHERE key = 'Cash'`).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("read deactivated row: %v", err)
	}
	if deletedAt == nil {
		t.Error("deactivated account row was removed instead of soft-deleted")
	}

	if ok, err := s.AccountExists("Cash"); err != nil || ok {
		t.Errorf("AccountExists(Cash) = %v, %v; want false, nil", ok, err)
	}
}

func TestUpdateAccountChangesBalanceAndNotes(t *testing.T) {
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 2500000)

	if _, err := s.UpdateAccount("BRI", money.New(2600000), "primary checking"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := s.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Balance != money.New(2600000) {
		t.Errorf("balance = %s, want 2600000", got.Balance)
	}
	if got.Notes != "primary checking" {
		t.Errorf("notes = %q, want %q", got.Notes, "primary checking")
	}
}

func TestUpdateAccountMissingReturnsNotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.UpdateAccount("Nope", money.New(1), ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if err := s.DeactivateAccount("Nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestTotalBalanceExcludesDeactivatedAccounts(t *testing.T) {
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 2500000)
	mustAccount(t, s, "Gopay", 500000)
	mustAccount(t, s, "Cash", 150000)

	before, err := s.TotalBalance()
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	if before.String() != "3,150,000" {
		t.Errorf("total before deactivation = %s, want 3,150,000", before)
	}

	if err := s.DeactivateAccount("Cash"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	after, err := s.TotalBalance()
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	if after.String() != "3,000,000" {
		t.Errorf("total after deactivation = %s, want 3,000,000 (Cash excluded)", after)
	}
}

func TestTotalBalanceOnEmptyDatabaseIsZero(t *testing.T) {
	s := newTestStore(t)

	got, err := s.TotalBalance()
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	if got.String() != "0" {
		t.Errorf("total = %s, want 0", got)
	}
}
