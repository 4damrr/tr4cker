package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

func at(y int, m time.Month, d, hh int) time.Time {
	return time.Date(y, m, d, hh, 0, 0, 0, time.Local)
}

// fixture builds a store with two accounts and three categories.
func fixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 1000000)
	mustAccount(t, s, "Gopay", 200000)
	mustCategory(t, s, "Food", BucketNeeds)
	mustCategory(t, s, "Transport", BucketNeeds)
	mustCategory(t, s, "Fun", BucketWants)
	mustCategory(t, s, "Emergency", BucketSavings)
	return s
}

func balance(t *testing.T, s *Store, key string) money.Amount {
	t.Helper()
	a, err := s.GetAccount(key)
	if err != nil {
		t.Fatalf("get account %q: %v", key, err)
	}
	return a.Balance
}

func expense(t *testing.T, amount string, from, category string, when time.Time) NewTransaction {
	return NewTransaction{
		Amount: money.New(parseFloat(t, amount)),
		Type:   TypeExpense, Category: category, At: when,
		AccountFrom: from, AccountTo: db.External,
	}
}

// parseFloat is a small helper so table tests can spell amounts as strings.
func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	parsed, err := money.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return float64(parsed)
}

func TestCreateTransactionAdjustsBalancesByType(t *testing.T) {
	tests := []struct {
		name      string
		in        NewTransaction
		wantBRI   string
		wantGopay string
	}{
		{
			name: "expense leaves the source",
			in: NewTransaction{Amount: money.New(25000), Type: TypeExpense, Category: "Food",
				At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: db.External},
			wantBRI: "975,000", wantGopay: "200,000",
		},
		{
			name: "income lands on the destination",
			in: NewTransaction{Amount: money.New(500000), Type: TypeIncome, Category: "Food",
				At: at(2026, 10, 28, 12), AccountFrom: db.External, AccountTo: "BRI"},
			wantBRI: "1,500,000", wantGopay: "200,000",
		},
		{
			name: "transfer moves between accounts",
			in: NewTransaction{Amount: money.New(100000), Type: TypeTransfer, Category: "Food",
				At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
			wantBRI: "900,000", wantGopay: "300,000",
		},
		{
			name: "investment transfer moves between accounts",
			in: NewTransaction{Amount: money.New(250000), Type: TypeTransfer, Category: "Emergency",
				At: at(2026, 10, 28, 12), IsInvestment: true, AccountFrom: "BRI", AccountTo: "Gopay"},
			wantBRI: "750,000", wantGopay: "450,000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			if _, err := s.CreateTransaction(tc.in); err != nil {
				t.Fatalf("create: %v", err)
			}
			if got := balance(t, s, "BRI").String(); got != tc.wantBRI {
				t.Errorf("BRI = %s, want %s", got, tc.wantBRI)
			}
			if got := balance(t, s, "Gopay").String(); got != tc.wantGopay {
				t.Errorf("Gopay = %s, want %s", got, tc.wantGopay)
			}
		})
	}
}

func TestBalanceStaysAtTwoDecimals(t *testing.T) {
	t.Run("100000 minus 25000.50", func(t *testing.T) {
		s := newTestStore(t)
		mustAccount(t, s, "BRI", 100000)
		mustCategory(t, s, "Food", BucketNeeds)

		in := expense(t, "25000.50", "BRI", "Food", at(2026, 10, 28, 12))
		if _, err := s.CreateTransaction(in); err != nil {
			t.Fatalf("create: %v", err)
		}
		if got := balance(t, s, "BRI").String(); got != "74,999.50" {
			t.Errorf("BRI = %s, want 74,999.50", got)
		}
	})

	t.Run("0 minus 0.756 rounds away from zero", func(t *testing.T) {
		s := newTestStore(t)
		mustAccount(t, s, "BRI", 0)
		mustCategory(t, s, "Food", BucketNeeds)

		in := expense(t, "0.756", "BRI", "Food", at(2026, 10, 28, 12))
		if _, err := s.CreateTransaction(in); err != nil {
			t.Fatalf("create: %v", err)
		}
		if got := balance(t, s, "BRI").String(); got != "-0.76" {
			t.Errorf("BRI = %s, want -0.76", got)
		}
	})
}

func TestCreateEditDeleteRoundTripsBalances(t *testing.T) {
	s := fixture(t)

	startBRI := balance(t, s, "BRI")
	startGopay := balance(t, s, "Gopay")

	created, err := s.CreateTransaction(expense(t, "25000", "BRI", "Food", at(2026, 10, 28, 12)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := balance(t, s, "BRI").String(); got != "975,000" {
		t.Fatalf("after create BRI = %s, want 975,000", got)
	}

	// Edit the amount: the balance must reflect only the new amount.
	if _, err := s.UpdateTransaction(created.ID, expense(t, "27000", "BRI", "Food", at(2026, 10, 28, 12))); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := balance(t, s, "BRI").String(); got != "973,000" {
		t.Errorf("after edit BRI = %s, want 973,000", got)
	}

	// Re-point the transaction at another account: both balances must settle.
	if _, err := s.UpdateTransaction(created.ID, NewTransaction{
		Amount: money.New(27000), Type: TypeExpense, Category: "Food",
		At: at(2026, 10, 28, 12), AccountFrom: "Gopay", AccountTo: db.External,
	}); err != nil {
		t.Fatalf("update accounts: %v", err)
	}
	if got := balance(t, s, "BRI").String(); got != startBRI.String() {
		t.Errorf("after re-point BRI = %s, want %s", got, startBRI)
	}
	if got := balance(t, s, "Gopay").String(); got != "173,000" {
		t.Errorf("after re-point Gopay = %s, want 173,000", got)
	}

	// Change the type from expense to transfer: money should move, not vanish.
	if _, err := s.UpdateTransaction(created.ID, NewTransaction{
		Amount: money.New(27000), Type: TypeTransfer, Category: "Food",
		At: at(2026, 10, 28, 12), AccountFrom: "Gopay", AccountTo: "BRI",
	}); err != nil {
		t.Fatalf("update type: %v", err)
	}
	if got := balance(t, s, "BRI").String(); got != "1,027,000" {
		t.Errorf("after type change BRI = %s, want 1,027,000", got)
	}

	// Soft-delete must reverse the whole effect.
	if err := s.DeleteTransaction(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := balance(t, s, "BRI").String(); got != startBRI.String() {
		t.Errorf("after delete BRI = %s, want %s", got, startBRI)
	}
	if got := balance(t, s, "Gopay").String(); got != startGopay.String() {
		t.Errorf("after delete Gopay = %s, want %s", got, startGopay)
	}
}

func TestDeleteTransactionSoftDeletesTheRow(t *testing.T) {
	s := fixture(t)

	created, err := s.CreateTransaction(expense(t, "25000", "BRI", "Food", at(2026, 10, 28, 12)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.DeleteTransaction(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := s.GetTransaction(created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTransaction after delete = %v, want ErrNotFound", err)
	}

	var deletedAt any
	if err := s.db.QueryRow(`SELECT deleted_at FROM transactions WHERE id = ?`, created.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if deletedAt == nil {
		t.Error("transaction row was removed instead of soft-deleted")
	}

	listed, err := s.ListTransactions(TransactionFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("active list = %d rows, want 0", len(listed))
	}
}

func TestListTransactionsExcludesSoftDeleted(t *testing.T) {
	s := fixture(t)

	keep, err := s.CreateTransaction(expense(t, "10000", "BRI", "Food", at(2026, 10, 20, 9)))
	if err != nil {
		t.Fatalf("create keep: %v", err)
	}
	drop, err := s.CreateTransaction(expense(t, "20000", "BRI", "Food", at(2026, 10, 21, 9)))
	if err != nil {
		t.Fatalf("create drop: %v", err)
	}
	if err := s.DeleteTransaction(drop.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	listed, err := s.ListTransactions(TransactionFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != keep.ID {
		t.Errorf("listed %d rows, want only %d", len(listed), keep.ID)
	}
}

func TestListTransactionsFilters(t *testing.T) {
	s := fixture(t)

	mustCreate := func(in NewTransaction) Transaction {
		t.Helper()
		got, err := s.CreateTransaction(in)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return got
	}
	oct28 := at(2026, 10, 28, 12)
	nov2 := at(2026, 11, 2, 12)
	oct20 := at(2026, 10, 20, 12)

	expenseOct := mustCreate(expense(t, "10000", "BRI", "Food", oct28))
	_ = mustCreate(NewTransaction{Amount: money.New(500000), Type: TypeIncome, Category: "Food",
		At: nov2, AccountFrom: db.External, AccountTo: "BRI"})
	_ = mustCreate(NewTransaction{Amount: money.New(30000), Type: TypeTransfer, Category: "Emergency",
		At: oct20, AccountFrom: "Gopay", AccountTo: "BRI"})
	_ = mustCreate(expense(t, "40000", "Gopay", "Fun", oct28))

	tests := []struct {
		name   string
		filter TransactionFilter
		want   int
	}{
		{"no filter returns everything active", TransactionFilter{}, 4},
		{"filter by type expense", TransactionFilter{Types: []TransactionType{TypeExpense}}, 2},
		{"filter by two types", TransactionFilter{Types: []TransactionType{TypeExpense, TypeTransfer}}, 3},
		{"filter by category", TransactionFilter{Categories: []string{"Food"}}, 2},
		{"filter by account matches either side", TransactionFilter{Accounts: []string{"Gopay"}}, 2},
		{"filter by date range", TransactionFilter{From: &oct28, To: &nov2}, 2},
		{"filters combine", TransactionFilter{
			Types: []TransactionType{TypeExpense}, Categories: []string{"Food"},
		}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ListTransactions(tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("got %d rows, want %d", len(got), tc.want)
			}
		})
	}

	_ = expenseOct
}

func TestValidateTransactionRejectsUnknownAccountKey(t *testing.T) {
	s := fixture(t)

	_, err := s.CreateTransaction(expense(t, "25000", "Nonexistent", "Food", at(2026, 10, 28, 12)))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("error = %v, want it to name the offending account", err)
	}

	// Nothing was written and no balance moved.
	if got := balance(t, s, "BRI").String(); got != "1,000,000" {
		t.Errorf("BRI = %s, want 1,000,000", got)
	}
	listed, err := s.ListTransactions(TransactionFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("stored %d transactions, want 0", len(listed))
	}
}

func TestValidateTransactionRules(t *testing.T) {
	base := func() NewTransaction { return expense(t, "25000", "BRI", "Food", at(2026, 10, 28, 12)) }

	tests := []struct {
		name   string
		mutate func(*NewTransaction)
	}{
		{"unknown type", func(in *NewTransaction) { in.Type = "bogus" }},
		{"zero amount", func(in *NewTransaction) { in.Amount = 0 }},
		{"negative amount", func(in *NewTransaction) { in.Amount = money.New(-100) }},
		{"empty category", func(in *NewTransaction) { in.Category = "" }},
		{"zero time", func(in *NewTransaction) { in.At = time.Time{} }},
		{"expense with a non-external destination", func(in *NewTransaction) { in.AccountTo = "Gopay" }},
		{"expense from external", func(in *NewTransaction) { in.AccountFrom = db.External }},
		{"income with a non-external source", func(in *NewTransaction) {
			in.Type, in.AccountFrom, in.AccountTo = TypeIncome, "Gopay", "BRI"
		}},
		{"transfer to itself", func(in *NewTransaction) {
			in.Type, in.AccountFrom, in.AccountTo = TypeTransfer, "BRI", "BRI"
		}},
		{"transfer with an unknown account", func(in *NewTransaction) {
			in.Type, in.AccountFrom, in.AccountTo = TypeTransfer, "BRI", "Nonexistent"
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			in := base()
			tc.mutate(&in)
			_, err := s.CreateTransaction(in)
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestBalanceChangeRollsBackWhenALaterStepFails(t *testing.T) {
	s := fixture(t)

	startBRI := balance(t, s, "BRI")
	startGopay := balance(t, s, "Gopay")

	// Reproduce a transfer: the first balance moves, then the second adjustment
	// fails because the account is not active. The whole thing must roll back.
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := applyBalance(tx, TypeTransfer, "BRI", "Gopay", money.New(50000)); err != nil {
		t.Fatalf("first adjustment should succeed: %v", err)
	}
	if got := balanceInTx(t, tx, "BRI"); got != "950,000" {
		t.Errorf("mid-transaction BRI = %s, want 950,000", got)
	}

	err = applyBalance(tx, TypeTransfer, "BRI", "Nonexistent", money.New(50000))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("second adjustment error = %v, want ErrNotFound", err)
	}
	_ = tx.Rollback()

	if got := balance(t, s, "BRI"); got != startBRI {
		t.Errorf("BRI after rollback = %s, want %s", got, startBRI)
	}
	if got := balance(t, s, "Gopay"); got != startGopay {
		t.Errorf("Gopay after rollback = %s, want %s", got, startGopay)
	}
}

func TestDeactivatedAccountFailsTheBalanceAdjustment(t *testing.T) {
	s := fixture(t)

	if err := s.DeactivateAccount("Gopay"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// Validation rejects it up front, so nothing is written and no balance moves.
	_, err := s.CreateTransaction(NewTransaction{
		Amount: money.New(50000), Type: TypeTransfer, Category: "Food",
		At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay",
	})
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
	if got := balance(t, s, "BRI").String(); got != "1,000,000" {
		t.Errorf("BRI = %s, want 1,000,000", got)
	}
}

func TestCategoryForeignKeyRejectsAnUnknownCategory(t *testing.T) {
	s := fixture(t)

	// The category foreign key is the schema's backstop behind validateTransaction.
	if _, err := s.db.Exec(
		`INSERT INTO transactions (amount, type, category, trx_time, is_investment, account_from, account_to)
		 VALUES (?, ?, ?, ?, 0, ?, ?)`,
		25000.0, string(TypeExpense), "MissingCategory", FormatTime(at(2026, 10, 28, 12)), "BRI", db.External,
	); err == nil {
		t.Error("insert with an unknown category succeeded, want foreign key violation")
	}
}

// balanceInTx reads a balance from inside an open transaction.
func balanceInTx(t *testing.T, tx *sql.Tx, key string) string {
	t.Helper()
	var b float64
	if err := tx.QueryRow(`SELECT balance FROM account WHERE key = ?`, key).Scan(&b); err != nil {
		t.Fatalf("read balance in tx: %v", err)
	}
	return coerceAmount(b).String()
}

func TestUpdateTransactionMissingReturnsNotFound(t *testing.T) {
	s := fixture(t)

	if _, err := s.UpdateTransaction(999, expense(t, "1", "BRI", "Food", at(2026, 10, 28, 12))); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if err := s.DeleteTransaction(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestRecentTransactionsOrdersNewestFirst(t *testing.T) {
	s := fixture(t)

	ids := []int64{}
	for _, when := range []time.Time{at(2026, 10, 20, 12), at(2026, 10, 28, 12), at(2026, 10, 15, 12)} {
		got, err := s.CreateTransaction(expense(t, "10000", "BRI", "Food", when))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		ids = append(ids, got.ID)
	}

	recent, err := s.RecentTransactions(2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("got %d rows, want 2", len(recent))
	}
	if recent[0].ID != ids[1] || recent[1].ID != ids[0] {
		t.Errorf("recent order = [%d %d], want [%d %d]", recent[0].ID, recent[1].ID, ids[1], ids[0])
	}
}

func TestListTransactionsInPeriod(t *testing.T) {
	s := fixture(t)

	p := period.New(2026, 11) // 25 Oct - 24 Nov 2026
	if _, err := s.CreateTransaction(expense(t, "10000", "BRI", "Food", at(2026, 10, 24, 12))); err != nil {
		t.Fatalf("create before: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "20000", "BRI", "Food", at(2026, 10, 25, 12))); err != nil {
		t.Fatalf("create inside: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "40000", "BRI", "Food", at(2026, 11, 2, 12))); err != nil {
		t.Fatalf("create inside later: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "80000", "BRI", "Food", at(2026, 11, 24, 12))); err != nil {
		t.Fatalf("create last day: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "160000", "BRI", "Food", at(2026, 11, 25, 12))); err != nil {
		t.Fatalf("create after: %v", err)
	}

	got, err := s.ListTransactionsInPeriod(p)
	if err != nil {
		t.Fatalf("list in period: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d transactions in %s, want 3", len(got), p.Label())
	}
}
