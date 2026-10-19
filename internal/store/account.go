package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/4damrr/tr4cker/internal/money"
)

// Account is a place where the user holds money.
type Account struct {
	Key     string
	Balance money.Amount
	Notes   string
}

// accountColumns is the projection every account read uses.
const accountColumns = `key, balance, notes, deleted_at`

// scanAccount reads one row. deleted is reported so callers can tell an active
// account from a deactivated one without a second query.
func scanAccount(row interface{ Scan(...any) error }) (Account, bool, error) {
	var (
		a       Account
		deleted sql.NullString
		balance float64
	)
	if err := row.Scan(&a.Key, &balance, &a.Notes, &deleted); err != nil {
		return Account{}, false, err
	}
	a.Balance = coerceAmount(balance)
	return a, deleted.Valid, nil
}

// ListAccounts returns the active accounts, ordered by key. Deactivated
// accounts are excluded so they never appear as a choice on a new record.
func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(
		`SELECT ` + accountColumns + ` FROM account WHERE deleted_at IS NULL ORDER BY key`,
	)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		a, _, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// TotalBalance returns the sum of the active account balances. Each stored
// balance already holds at most two decimals, so the displayed total equals the
// sum of the listed balances.
func (s *Store) TotalBalance() (money.Amount, error) {
	var total float64
	err := s.db.QueryRow(
		`SELECT ROUND(COALESCE(SUM(balance), 0), 2) FROM account WHERE deleted_at IS NULL`,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total balance: %w", err)
	}
	return coerceAmount(total), nil
}

// GetAccount returns an active account by key.
func (s *Store) GetAccount(key string) (Account, error) {
	row := s.db.QueryRow(
		`SELECT `+accountColumns+` FROM account WHERE key = ? AND deleted_at IS NULL`, key,
	)
	a, _, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account %q: %w", key, err)
	}
	return a, nil
}

// CreateAccount adds a new account. The balance is rounded to two decimals.
func (s *Store) CreateAccount(key string, balance money.Amount, notes string) (Account, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Account{}, fmt.Errorf("%w: account key is required", ErrInvalid)
	}

	var existing string
	err := s.db.QueryRow(`SELECT key FROM account WHERE key = ?`, key).Scan(&existing)
	switch {
	case err == nil:
		return Account{}, fmt.Errorf("%w: account %q", ErrDuplicate, key)
	case !errors.Is(err, sql.ErrNoRows):
		return Account{}, fmt.Errorf("check account %q: %w", key, err)
	}

	_, err = s.db.Exec(
		`INSERT INTO account (key, balance, notes) VALUES (?, ?, ?)`,
		key, balance.Round().Float(), strings.TrimSpace(notes),
	)
	if err != nil {
		return Account{}, fmt.Errorf("insert account %q: %w", key, err)
	}
	return Account{Key: key, Balance: balance.Round(), Notes: strings.TrimSpace(notes)}, nil
}

// UpdateAccount changes the balance and notes of an active account. The key is
// the identifier and is immutable; see design.md.
func (s *Store) UpdateAccount(key string, balance money.Amount, notes string) (Account, error) {
	res, err := s.db.Exec(
		`UPDATE account SET balance = ?, notes = ? WHERE key = ? AND deleted_at IS NULL`,
		balance.Round().Float(), strings.TrimSpace(notes), key,
	)
	if err != nil {
		return Account{}, fmt.Errorf("update account %q: %w", key, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return Account{}, fmt.Errorf("%w: account %q", ErrNotFound, key)
	}
	return Account{Key: key, Balance: balance.Round(), Notes: strings.TrimSpace(notes)}, nil
}

// DeactivateAccount soft-deletes an account, preserving its balance and the
// history recorded against it.
func (s *Store) DeactivateAccount(key string) error {
	res, err := s.db.Exec(
		`UPDATE account SET deleted_at = ? WHERE key = ? AND deleted_at IS NULL`,
		nowString(), key,
	)
	if err != nil {
		return fmt.Errorf("deactivate account %q: %w", key, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: account %q", ErrNotFound, key)
	}
	return nil
}

// AccountExists reports whether key is an active account.
func (s *Store) AccountExists(key string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM account WHERE key = ? AND deleted_at IS NULL`, key,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("account exists %q: %w", key, err)
	}
	return n > 0, nil
}

// adjustBalance adds delta to an account's stored balance and rounds the result
// to two decimals. It is unexported: only transaction writes may move money, and
// they must do so inside the caller's transaction.
//
// An account that no longer exists or has been deactivated affects no rows, which
// is reported as an error rather than silently ignored. That turns a stale
// account reference into a rollback instead of money that quietly disappears.
func adjustBalance(tx *sql.Tx, key string, delta money.Amount) error {
	res, err := tx.Exec(
		`UPDATE account SET balance = ROUND(balance + ?, 2) WHERE key = ? AND deleted_at IS NULL`,
		delta.Float(), key,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: account %q is not active", ErrNotFound, key)
	}
	return nil
}
