package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

// TransactionType is the movement recorded on a transaction.
type TransactionType string

// The three transaction types defined by openspec/specs/database-design.md.
const (
	TypeIncome   TransactionType = "income"
	TypeExpense  TransactionType = "expense"
	TypeTransfer TransactionType = "transfer"
)

// ValidType reports whether t is one of the three defined types.
func ValidType(t TransactionType) bool {
	switch t {
	case TypeIncome, TypeExpense, TypeTransfer:
		return true
	}
	return false
}

// Transaction is a recorded movement of money.
type Transaction struct {
	ID           int64
	Amount       money.Amount
	Type         TransactionType
	Category     string
	At           time.Time
	IsInvestment bool
	AccountFrom  string
	AccountTo    string
}

// IsExternal reports whether the transaction touches money outside the user's
// tracked accounts.
func (t Transaction) IsExternal() bool {
	return t.AccountFrom == db.External || t.AccountTo == db.External
}

// TransactionFilter narrows a transaction listing. A nil or zero value matches
// everything active.
type TransactionFilter struct {
	Types      []TransactionType
	Categories []string
	Accounts   []string
	From, To   *time.Time
}

// NewTransaction is the input for creating or replacing a transaction.
type NewTransaction struct {
	Amount       money.Amount
	Type         TransactionType
	Category     string
	At           time.Time
	IsInvestment bool
	AccountFrom  string
	AccountTo    string
}

const transactionColumns = `id, amount, type, category, trx_time, is_investment, account_from, account_to`

func scanTransaction(row interface{ Scan(...any) error }) (Transaction, error) {
	var (
		t          Transaction
		amount     float64
		typ        string
		at         string
		investment int
	)
	if err := row.Scan(&t.ID, &amount, &typ, &t.Category, &at, &investment, &t.AccountFrom, &t.AccountTo); err != nil {
		return Transaction{}, err
	}
	parsed, err := ParseTime(at)
	if err != nil {
		return Transaction{}, fmt.Errorf("parse trx_time %q: %w", at, err)
	}
	t.Amount = coerceAmount(amount)
	t.Type = TransactionType(typ)
	t.At = parsed
	t.IsInvestment = investment != 0
	return t, nil
}

// ListTransactions returns the active transactions matching f, newest first.
// Soft-deleted rows are never returned.
func (s *Store) ListTransactions(f TransactionFilter) ([]Transaction, error) {
	where := []string{"deleted_at IS NULL"}
	var args []any

	if types := typesOf(f.Types); len(types) > 0 {
		list, a := inClause(types)
		where = append(where, "type IN "+list)
		args = append(args, a...)
	}
	if len(f.Categories) > 0 {
		list, a := inClause(f.Categories)
		where = append(where, "category IN "+list)
		args = append(args, a...)
	}
	if len(f.Accounts) > 0 {
		list, a := inClause(f.Accounts)
		where = append(where, "(account_from IN "+list+" OR account_to IN "+list+")")
		args = append(args, a...)
		args = append(args, a...)
	}
	if f.From != nil {
		where = append(where, "trx_time >= ?")
		args = append(args, FormatTime(*f.From))
	}
	if f.To != nil {
		where = append(where, "trx_time < ?")
		args = append(args, FormatTime(*f.To))
	}

	q := `SELECT ` + transactionColumns + ` FROM transactions WHERE ` +
		strings.Join(where, " AND ") + ` ORDER BY trx_time DESC, id DESC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	out := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTransaction returns an active transaction by id.
func (s *Store) GetTransaction(id int64) (Transaction, error) {
	row := s.db.QueryRow(
		`SELECT `+transactionColumns+` FROM transactions WHERE id = ? AND deleted_at IS NULL`, id,
	)
	t, err := scanTransaction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction %d: %w", id, err)
	}
	return t, nil
}

func typesOf(types []TransactionType) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, string(t))
	}
	return out
}

// CreateTransaction validates a new transaction and stores it together with its
// effect on every account balance, in a single SQLite transaction.
func (s *Store) CreateTransaction(in NewTransaction) (Transaction, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Transaction{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := validateTransaction(s, tx, in); err != nil {
		return Transaction{}, err
	}

	amount := in.Amount.Round()
	res, err := tx.Exec(
		`INSERT INTO transactions
			(amount, type, category, trx_time, is_investment, account_from, account_to)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		amount.Float(), string(in.Type), in.Category,
		FormatTime(in.At), boolToInt(in.IsInvestment),
		in.AccountFrom, in.AccountTo,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("insert transaction: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Transaction{}, fmt.Errorf("transaction id: %w", err)
	}

	if err := applyBalance(tx, in.Type, in.AccountFrom, in.AccountTo, amount); err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transaction{}, fmt.Errorf("commit: %w", err)
	}

	return Transaction{
		ID: id, Amount: amount, Type: in.Type, Category: in.Category,
		At: in.At, IsInvestment: in.IsInvestment,
		AccountFrom: in.AccountFrom, AccountTo: in.AccountTo,
	}, nil
}

// UpdateTransaction replaces a transaction: the old effect on balances is
// reversed, then the new one is applied, in a single SQLite transaction.
func (s *Store) UpdateTransaction(id int64, in NewTransaction) (Transaction, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Transaction{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getTransactionTx(tx, id)
	if err != nil {
		return Transaction{}, err
	}
	if err := validateTransaction(s, tx, in); err != nil {
		return Transaction{}, err
	}

	amount := in.Amount.Round()
	_, err = tx.Exec(
		`UPDATE transactions
		 SET amount = ?, type = ?, category = ?, trx_time = ?, is_investment = ?,
		     account_from = ?, account_to = ?
		 WHERE id = ? AND deleted_at IS NULL`,
		amount.Float(), string(in.Type), in.Category,
		FormatTime(in.At), boolToInt(in.IsInvestment),
		in.AccountFrom, in.AccountTo, id,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("update transaction %d: %w", id, err)
	}

	// Reverse the old effect first so the balance never reflects both rows.
	if err := applyBalance(tx, existing.Type, existing.AccountFrom, existing.AccountTo, existing.Amount.Neg()); err != nil {
		return Transaction{}, err
	}
	if err := applyBalance(tx, in.Type, in.AccountFrom, in.AccountTo, amount); err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transaction{}, fmt.Errorf("commit: %w", err)
	}

	return Transaction{
		ID: id, Amount: amount, Type: in.Type, Category: in.Category,
		At: in.At, IsInvestment: in.IsInvestment,
		AccountFrom: in.AccountFrom, AccountTo: in.AccountTo,
	}, nil
}

// DeleteTransaction soft-deletes a transaction and reverses its effect on every
// account balance, in a single SQLite transaction. The row itself is kept:
// financial history is never physically deleted.
func (s *Store) DeleteTransaction(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getTransactionTx(tx, id)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE transactions SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		nowString(), id,
	); err != nil {
		return fmt.Errorf("soft-delete transaction %d: %w", id, err)
	}

	if err := applyBalance(tx, existing.Type, existing.AccountFrom, existing.AccountTo, existing.Amount.Neg()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func getTransactionTx(tx *sql.Tx, id int64) (Transaction, error) {
	row := tx.QueryRow(
		`SELECT `+transactionColumns+` FROM transactions WHERE id = ? AND deleted_at IS NULL`, id,
	)
	t, err := scanTransaction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, fmt.Errorf("%w: transaction %d", ErrNotFound, id)
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction %d: %w", id, err)
	}
	return t, nil
}

// applyBalance moves delta onto the accounts a transaction touches. The sign is
// the single place the balance rules live: an expense leaves account_from, an
// income lands in account_to, and a transfer does both. Passing a negated amount
// reverses the move, which is how update and delete undo their predecessor.
//
// Every write rounds the resulting balance to two decimals.
func applyBalance(tx *sql.Tx, typ TransactionType, from, to string, amount money.Amount) error {
	switch typ {
	case TypeExpense:
		if err := adjustBalance(tx, from, amount.Neg()); err != nil {
			return fmt.Errorf("adjust %s: %w", from, err)
		}
	case TypeIncome:
		if err := adjustBalance(tx, to, amount); err != nil {
			return fmt.Errorf("adjust %s: %w", to, err)
		}
	case TypeTransfer:
		if err := adjustBalance(tx, from, amount.Neg()); err != nil {
			return fmt.Errorf("adjust %s: %w", from, err)
		}
		if err := adjustBalance(tx, to, amount); err != nil {
			return fmt.Errorf("adjust %s: %w", to, err)
		}
	default:
		return fmt.Errorf("%w: transaction type %q", ErrInvalid, typ)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// validateTransaction rejects a transaction that is not internally consistent
// before any write happens. Account references are checked here because
// account_from and account_to carry no foreign key; see design.md.
func validateTransaction(s *Store, tx *sql.Tx, in NewTransaction) error {
	if !ValidType(in.Type) {
		return fmt.Errorf("%w: transaction type %q", ErrInvalid, in.Type)
	}
	if in.Amount.Round() <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalid)
	}
	if strings.TrimSpace(in.Category) == "" {
		return fmt.Errorf("%w: category is required", ErrInvalid)
	}
	if in.At.IsZero() {
		return fmt.Errorf("%w: occurrence time is required", ErrInvalid)
	}

	activeCategory, err := categoryActive(tx, in.Category)
	if err != nil {
		return err
	}
	if !activeCategory {
		return fmt.Errorf("%w: category %q is not active", ErrInvalid, in.Category)
	}

	fromOK, err := accountActive(tx, in.AccountFrom)
	if err != nil {
		return err
	}
	toOK, err := accountActive(tx, in.AccountTo)
	if err != nil {
		return err
	}

	switch in.Type {
	case TypeExpense:
		if !fromOK {
			return fmt.Errorf("%w: expense source account %q is not active", ErrInvalid, in.AccountFrom)
		}
		if in.AccountTo != db.External {
			return fmt.Errorf("%w: expense destination must be %q", ErrInvalid, db.External)
		}
	case TypeIncome:
		if !toOK {
			return fmt.Errorf("%w: income destination account %q is not active", ErrInvalid, in.AccountTo)
		}
		if in.AccountFrom != db.External {
			return fmt.Errorf("%w: income source must be %q", ErrInvalid, db.External)
		}
	case TypeTransfer:
		if !fromOK || !toOK {
			return fmt.Errorf("%w: transfer needs two active accounts", ErrInvalid)
		}
		if in.AccountFrom == in.AccountTo {
			return fmt.Errorf("%w: transfer accounts must differ", ErrInvalid)
		}
	}
	return nil
}

func accountActive(tx *sql.Tx, key string) (bool, error) {
	if key == db.External {
		return false, nil
	}
	var n int
	err := tx.QueryRow(
		`SELECT COUNT(*) FROM account WHERE key = ? AND deleted_at IS NULL`, key,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check account %q: %w", key, err)
	}
	return n > 0, nil
}

func categoryActive(tx *sql.Tx, key string) (bool, error) {
	var n int
	err := tx.QueryRow(
		`SELECT COUNT(*) FROM category WHERE key = ? AND deleted_at IS NULL`, key,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check category %q: %w", key, err)
	}
	return n > 0, nil
}

// ListTransactionsInPeriod returns the active transactions dated inside p.
func (s *Store) ListTransactionsInPeriod(p period.Period) ([]Transaction, error) {
	from, to := p.Start(), p.End()
	return s.ListTransactions(TransactionFilter{From: &from, To: &to})
}

// RecentTransactions returns the newest active transactions, capped at limit.
func (s *Store) RecentTransactions(limit int) ([]Transaction, error) {
	rows, err := s.db.Query(
		`SELECT `+transactionColumns+` FROM transactions
		 WHERE deleted_at IS NULL ORDER BY trx_time DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent transactions: %w", err)
	}
	defer rows.Close()

	out := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
