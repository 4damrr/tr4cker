package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

// Plan is a planned amount for one category in one financial period.
type Plan struct {
	Category string
	Amount   money.Amount
	Notes    string
}

// PlanRow pairs a category's plan for a period with what actually happened, and
// carries the derived remaining amount and progress. A category with no plan
// still appears, with Planned false.
type PlanRow struct {
	Category Category
	Planned  bool
	Amount   money.Amount // planned amount
	Actual   money.Amount // actual spending, or savings progress for savings categories
	Notes    string

	Remaining money.Amount // meaningful only for needs/wants
	HasRemain bool
	Progress  float64 // 0..1+, meaningful only when a plan exists
	Overspent bool    // needs/wants only
}

// upsertPlan writes a planned amount for the period's category, replacing any
// existing entry so the UNIQUE(year, month, category) constraint is never hit.
const upsertPlan = `
INSERT INTO planning (year, month, category, amount, notes)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (year, month, category) DO UPDATE SET
	amount = excluded.amount,
	notes  = excluded.notes`

// SetPlan creates or replaces the planned amount for a category in a period.
func (s *Store) SetPlan(p period.Period, category string, amount money.Amount, notes string) (Plan, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return Plan{}, fmt.Errorf("%w: category is required", ErrInvalid)
	}

	active, err := s.CategoryExists(category)
	if err != nil {
		return Plan{}, err
	}
	if !active {
		return Plan{}, fmt.Errorf("%w: category %q is not active", ErrInvalid, category)
	}

	amount = amount.Round()
	if amount < 0 {
		return Plan{}, fmt.Errorf("%w: planned amount cannot be negative", ErrInvalid)
	}

	if _, err := s.db.Exec(upsertPlan, p.Year, p.Month, category, amount.Float(), strings.TrimSpace(notes)); err != nil {
		return Plan{}, fmt.Errorf("set plan for %q in %s: %w", category, p.Label(), err)
	}
	return Plan{Category: category, Amount: amount, Notes: strings.TrimSpace(notes)}, nil
}

// PlansForPeriod returns the planned amounts for a period, keyed by category.
func (s *Store) PlansForPeriod(p period.Period) (map[string]Plan, error) {
	rows, err := s.db.Query(
		`SELECT category, amount, notes FROM planning WHERE year = ? AND month = ?`,
		p.Year, p.Month,
	)
	if err != nil {
		return nil, fmt.Errorf("plans for %s: %w", p.Label(), err)
	}
	defer rows.Close()

	plans := map[string]Plan{}
	for rows.Next() {
		var (
			category string
			amount   float64
			notes    sql.NullString
		)
		if err := rows.Scan(&category, &amount, &notes); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		plans[category] = Plan{
			Category: category, Amount: coerceAmount(amount), Notes: notes.String,
		}
	}
	return plans, rows.Err()
}

// RemovePlan deletes the planned amount for a category in a period.
func (s *Store) RemovePlan(p period.Period, category string) error {
	_, err := s.db.Exec(
		`DELETE FROM planning WHERE year = ? AND month = ? AND category = ?`,
		p.Year, p.Month, category,
	)
	if err != nil {
		return fmt.Errorf("remove plan for %q: %w", category, err)
	}
	return nil
}

// HasPlans reports whether any planned amount exists for a period.
func (s *Store) HasPlans(p period.Period) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM planning WHERE year = ? AND month = ?`, p.Year, p.Month,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("has plans for %s: %w", p.Label(), err)
	}
	return n > 0, nil
}

// bucketTypes returns the transaction types that count as progress toward a
// category's target. This is the one place the rule lives, so planning and the
// dashboard cannot disagree; see design.md - Savings progress counts transfers.
func bucketTypes(b Bucket) []string {
	if b == BucketSavings {
		return []string{string(TypeTransfer)}
	}
	return []string{string(TypeExpense)}
}

// ActualForCategory sums the amounts that count toward a category's target in a
// period: expenses for needs and wants, transfers for savings including
// investment-flagged ones. Soft-deleted transactions and rows dated outside the
// period are excluded.
func (s *Store) ActualForCategory(category string, b Bucket, p period.Period) (money.Amount, error) {
	types, args := inClause(bucketTypes(b))

	start, end := bounds(p)
	query := `SELECT ROUND(COALESCE(SUM(amount), 0), 2)
		FROM transactions
		WHERE deleted_at IS NULL
		  AND category = ?
		  AND type IN ` + types + `
		  AND trx_time >= ? AND trx_time < ?`

	params := []any{category}
	params = append(params, args...)
	params = append(params, start, end)

	var total float64
	if err := s.db.QueryRow(query, params...).Scan(&total); err != nil {
		return 0, fmt.Errorf("actual for %q in %s: %w", category, p.Label(), err)
	}
	return coerceAmount(total), nil
}

// ActualsForPeriod sums progress for every category in a period in one pass,
// selecting on bucket so savings and spending categories are measured the same
// way everywhere.
func (s *Store) ActualsForPeriod(p period.Period) (map[string]money.Amount, error) {
	start, end := bounds(p)
	rows, err := s.db.Query(
		`SELECT t.category, ROUND(COALESCE(SUM(t.amount), 0), 2)
		 FROM transactions t
		 JOIN category c ON c.key = t.category
		 WHERE t.deleted_at IS NULL
		   AND t.trx_time >= ? AND t.trx_time < ?
		   AND t.type = CASE c.buckets WHEN 'savings' THEN 'transfer' ELSE 'expense' END
		 GROUP BY t.category`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("actuals for %s: %w", p.Label(), err)
	}
	defer rows.Close()

	actuals := map[string]money.Amount{}
	for rows.Next() {
		var (
			category string
			total    float64
		)
		if err := rows.Scan(&category, &total); err != nil {
			return nil, fmt.Errorf("scan actual: %w", err)
		}
		actuals[category] = coerceAmount(total)
	}
	return actuals, rows.Err()
}

// PeriodIncome totals income recorded inside a period.
func (s *Store) PeriodIncome(p period.Period) (money.Amount, error) {
	return s.periodTotal(p, TypeIncome)
}

// PeriodSpending totals expenses recorded inside a period. Transfers are not
// spending, so they never appear here.
func (s *Store) PeriodSpending(p period.Period) (money.Amount, error) {
	return s.periodTotal(p, TypeExpense)
}

func (s *Store) periodTotal(p period.Period, typ TransactionType) (money.Amount, error) {
	start, end := bounds(p)
	var total float64
	err := s.db.QueryRow(
		`SELECT ROUND(COALESCE(SUM(amount), 0), 2) FROM transactions
		 WHERE deleted_at IS NULL AND type = ? AND trx_time >= ? AND trx_time < ?`,
		string(typ), start, end,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("%s total for %s: %w", typ, p.Label(), err)
	}
	return coerceAmount(total), nil
}

// PlanRows builds the planning page view for a period: every active category
// with its plan, actual, remaining amount and progress.
//
// needs and wants are limits, so remaining is planned minus actual and may be
// negative; savings are targets, so progress is measured toward the target and
// there is no remaining amount.
func (s *Store) PlanRows(p period.Period) ([]PlanRow, error) {
	categories, err := s.ListCategories()
	if err != nil {
		return nil, err
	}
	plans, err := s.PlansForPeriod(p)
	if err != nil {
		return nil, err
	}
	actuals, err := s.ActualsForPeriod(p)
	if err != nil {
		return nil, err
	}

	rows := make([]PlanRow, 0, len(categories))
	for _, c := range categories {
		row := PlanRow{Category: c}
		if plan, ok := plans[c.Key]; ok {
			row.Planned = true
			row.Amount = plan.Amount
			row.Notes = plan.Notes
		}
		row.Actual = actuals[c.Key]
		row = derive(row)
		rows = append(rows, row)
	}
	return rows, nil
}

// derive fills in Remaining, Progress and Overspent for a plan row.
func derive(row PlanRow) PlanRow {
	if !row.Planned || row.Amount <= 0 {
		return row
	}
	if row.Category.Bucket == BucketSavings {
		row.Progress = float64(row.Actual) / float64(row.Amount)
		if row.Progress > 1 {
			row.Progress = 1
		}
		return row
	}
	row.Remaining = row.Amount.Sub(row.Actual)
	row.HasRemain = true
	row.Overspent = row.Remaining < 0
	row.Progress = float64(row.Actual) / float64(row.Amount)
	if row.Progress > 1 {
		row.Progress = 1
	}
	return row
}

// RemainingPlanned sums the remaining amount across the period's needs and
// wants categories. The result is negative when the period is overspent overall.
func (s *Store) RemainingPlanned(p period.Period) (money.Amount, bool, error) {
	rows, err := s.PlanRows(p)
	if err != nil {
		return 0, false, err
	}
	total := money.Amount(0)
	planned := false
	for _, r := range rows {
		if r.HasRemain {
			planned = true
			total = total.Add(r.Remaining)
		}
	}
	return total.Round(), planned, nil
}
