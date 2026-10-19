package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/4damrr/tr4cker/internal/money"
)

// Priority is an item's purchase priority, where 1 is the highest.
type Priority int

// The three priorities defined by openspec/specs/database-design.md.
const (
	PriorityHigh   Priority = 1
	PriorityMedium Priority = 2
	PriorityLow    Priority = 3
)

// ValidPriority reports whether p is one of the three defined priorities.
func ValidPriority(p Priority) bool {
	return p >= PriorityHigh && p <= PriorityLow
}

// BuyingStatus is where an intended purchase currently stands.
type BuyingStatus string

// The three statuses defined by openspec/specs/database-design.md.
const (
	StatusToBuy    BuyingStatus = "to_buy"
	StatusBought   BuyingStatus = "bought"
	StatusCanceled BuyingStatus = "canceled"
)

// ValidStatus reports whether s is one of the three defined statuses.
func ValidStatus(s BuyingStatus) bool {
	switch s {
	case StatusToBuy, StatusBought, StatusCanceled:
		return true
	}
	return false
}

// ToBuyItem is something the user intends to purchase.
type ToBuyItem struct {
	ID       int64
	Category string
	Name     string
	Priority Priority
	Qty      int
	Price    money.Amount
	URL      string
	Notes    string
	Status   BuyingStatus
}

// ExpectedTotal is the item's expected cost as quantity times unit price.
func (i ToBuyItem) ExpectedTotal() money.Amount {
	return money.Amount(float64(i.Qty) * float64(i.Price)).Round()
}

// NewToBuy is the input for creating or replacing a to-buy item.
type NewToBuy struct {
	Category string
	Name     string
	Priority Priority
	Qty      int
	Price    money.Amount
	URL      string
	Notes    string
	Status   BuyingStatus
}

const toBuyColumns = `id, category, name, priority, qty, price, url, notes, status`

func scanToBuy(row interface{ Scan(...any) error }) (ToBuyItem, error) {
	var (
		i      ToBuyItem
		price  float64
		url    sql.NullString
		notes  sql.NullString
		status string
	)
	if err := row.Scan(&i.ID, &i.Category, &i.Name, &i.Priority, &i.Qty, &price, &url, &notes, &status); err != nil {
		return ToBuyItem{}, err
	}
	i.Price = coerceAmount(price)
	i.URL = url.String
	i.Notes = notes.String
	i.Status = BuyingStatus(status)
	return i, nil
}

// ListToBuy returns items ordered by priority then name. A nil status matches
// every status.
func (s *Store) ListToBuy(status *BuyingStatus) ([]ToBuyItem, error) {
	query := `SELECT ` + toBuyColumns + ` FROM to_buy`
	var args []any
	if status != nil {
		query += ` WHERE status = ?`
		args = append(args, string(*status))
	}
	query += ` ORDER BY priority, name, id`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list to_buy: %w", err)
	}
	defer rows.Close()

	items := []ToBuyItem{}
	for rows.Next() {
		i, err := scanToBuy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan to_buy item: %w", err)
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetToBuy returns an item by id.
func (s *Store) GetToBuy(id int64) (ToBuyItem, error) {
	row := s.db.QueryRow(`SELECT `+toBuyColumns+` FROM to_buy WHERE id = ?`, id)
	i, err := scanToBuy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ToBuyItem{}, ErrNotFound
	}
	if err != nil {
		return ToBuyItem{}, fmt.Errorf("get to_buy item %d: %w", id, err)
	}
	return i, nil
}

// CreateToBuy adds an item. Changing an item's status never creates a
// transaction and never moves a balance; see specs/to-buy-list.
func (s *Store) CreateToBuy(in NewToBuy) (ToBuyItem, error) {
	clean, err := validateToBuy(in)
	if err != nil {
		return ToBuyItem{}, err
	}

	res, err := s.db.Exec(
		`INSERT INTO to_buy (category, name, priority, qty, price, url, notes, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		clean.Category, clean.Name, int(clean.Priority), clean.Qty,
		clean.Price.Float(), clean.URL, clean.Notes, string(clean.Status),
	)
	if err != nil {
		return ToBuyItem{}, fmt.Errorf("insert to_buy item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ToBuyItem{}, fmt.Errorf("to_buy id: %w", err)
	}
	return ToBuyItem{
		ID: id, Category: clean.Category, Name: clean.Name, Priority: clean.Priority,
		Qty: clean.Qty, Price: clean.Price, URL: clean.URL, Notes: clean.Notes,
		Status: clean.Status,
	}, nil
}

// UpdateToBuy replaces an item's editable fields.
func (s *Store) UpdateToBuy(id int64, in NewToBuy) (ToBuyItem, error) {
	clean, err := validateToBuy(in)
	if err != nil {
		return ToBuyItem{}, err
	}

	res, err := s.db.Exec(
		`UPDATE to_buy
		 SET category = ?, name = ?, priority = ?, qty = ?, price = ?, url = ?, notes = ?, status = ?
		 WHERE id = ?`,
		clean.Category, clean.Name, int(clean.Priority), clean.Qty,
		clean.Price.Float(), clean.URL, clean.Notes, string(clean.Status), id,
	)
	if err != nil {
		return ToBuyItem{}, fmt.Errorf("update to_buy item %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ToBuyItem{}, fmt.Errorf("%w: to_buy item %d", ErrNotFound, id)
	}
	return ToBuyItem{
		ID: id, Category: clean.Category, Name: clean.Name, Priority: clean.Priority,
		Qty: clean.Qty, Price: clean.Price, URL: clean.URL, Notes: clean.Notes,
		Status: clean.Status,
	}, nil
}

// SetToBuyStatus moves an item between still to buy, bought and canceled. It
// writes only the status column, so no transaction or balance is involved.
func (s *Store) SetToBuyStatus(id int64, status BuyingStatus) (ToBuyItem, error) {
	if !ValidStatus(status) {
		return ToBuyItem{}, fmt.Errorf("%w: status %q", ErrInvalid, status)
	}
	res, err := s.db.Exec(`UPDATE to_buy SET status = ? WHERE id = ?`, string(status), id)
	if err != nil {
		return ToBuyItem{}, fmt.Errorf("set to_buy status %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ToBuyItem{}, fmt.Errorf("%w: to_buy item %d", ErrNotFound, id)
	}
	return s.GetToBuy(id)
}

// DeleteToBuy removes an item from the list.
func (s *Store) DeleteToBuy(id int64) error {
	res, err := s.db.Exec(`DELETE FROM to_buy WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete to_buy item %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: to_buy item %d", ErrNotFound, id)
	}
	return nil
}

func validateToBuy(in NewToBuy) (NewToBuy, error) {
	out := NewToBuy{
		Category: strings.TrimSpace(in.Category),
		Name:     strings.TrimSpace(in.Name),
		Priority: in.Priority,
		Qty:      in.Qty,
		Price:    in.Price.Round(),
		URL:      strings.TrimSpace(in.URL),
		Notes:    strings.TrimSpace(in.Notes),
		Status:   in.Status,
	}
	if out.Status == "" {
		out.Status = StatusToBuy
	}
	if out.Qty == 0 {
		out.Qty = 1
	}

	if out.Name == "" {
		return NewToBuy{}, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if out.Category == "" {
		return NewToBuy{}, fmt.Errorf("%w: category is required", ErrInvalid)
	}
	if !ValidPriority(out.Priority) {
		return NewToBuy{}, fmt.Errorf("%w: priority %d must be 1, 2 or 3", ErrInvalid, int(out.Priority))
	}
	if out.Qty < 1 {
		return NewToBuy{}, fmt.Errorf("%w: quantity must be at least 1", ErrInvalid)
	}
	if out.Price < 0 {
		return NewToBuy{}, fmt.Errorf("%w: price cannot be negative", ErrInvalid)
	}
	if !ValidStatus(out.Status) {
		return NewToBuy{}, fmt.Errorf("%w: status %q", ErrInvalid, out.Status)
	}
	return out, nil
}
