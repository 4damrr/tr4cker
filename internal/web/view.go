package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
	"github.com/4damrr/tr4cker/internal/store"
)

// The view models below pre-format everything a template needs, so templates
// stay free of arithmetic and money handling lives in one place.

// bar is a prepared progress bar.
type bar struct {
	Percent int
	Over    bool
	Label   string
}

// newBar converts a 0..1+ progress fraction into a bar capped at 100%.
func newBar(fraction float64, over bool, label string) bar {
	pct := int(fraction * 100)
	switch {
	case pct < 0:
		pct = 0
	case pct > 100:
		pct = 100
	}
	return bar{Percent: pct, Over: over, Label: label}
}

// moneyLabel renders "Rp 900,000" with the decimal part trimmed when zero.
func moneyLabel(a money.Amount) string { return "Rp " + a.String() }

// dateLabel renders a timestamp for list rows.
func dateLabel(t time.Time) string { return t.Format("2 Jan 2006, 15:04") }

// datetimeInput renders a timestamp for an <input type="datetime-local">.
func datetimeInput(t time.Time) string { return t.Format("2006-01-02T15:04") }

// transactionRow is one row in a transaction list.
type transactionRow struct {
	ID           int64
	Amount       money.Amount
	AmountLabel  string
	Type         store.TransactionType
	TypeLabel    string
	Category     string
	At           time.Time
	AtLabel      string
	AccountFrom  string
	AccountTo    string
	Accounts     string
	IsInvestment bool
}

func newTransactionRows(list []store.Transaction) []transactionRow {
	rows := make([]transactionRow, 0, len(list))
	for _, t := range list {
		rows = append(rows, transactionRow{
			ID:           t.ID,
			Amount:       t.Amount,
			AmountLabel:  moneyLabel(t.Amount),
			Type:         t.Type,
			TypeLabel:    typeLabel(t.Type),
			Category:     t.Category,
			At:           t.At,
			AtLabel:      dateLabel(t.At),
			AccountFrom:  t.AccountFrom,
			AccountTo:    t.AccountTo,
			Accounts:     accountPair(t.AccountFrom, t.AccountTo),
			IsInvestment: t.IsInvestment,
		})
	}
	return rows
}

func typeLabel(t store.TransactionType) string {
	switch t {
	case store.TypeIncome:
		return "Income"
	case store.TypeExpense:
		return "Expense"
	case store.TypeTransfer:
		return "Transfer"
	}
	return string(t)
}

func accountPair(from, to string) string {
	if from == "external" || to == "external" {
		if from == "external" {
			return "External → " + to
		}
		return from + " → External"
	}
	return from + " → " + to
}

// planRowView is one category's row on the planning page.
type planRowView struct {
	CurrentQuery  string
	Category      string
	Bucket        store.Bucket
	BucketLabel   string
	Planned       bool
	Amount        money.Amount
	AmountValue   string
	AmountLabel   string
	Actual        money.Amount
	ActualLabel   string
	Notes         string
	HasRemain     bool
	Remaining     money.Amount
	RemainingText string
	Overspent     bool
	Bar           bar
	IsSavings     bool
	ProgressText  string
}

func newPlanRows(rows []store.PlanRow) []planRowView {
	return newPlanRowsFor(rows, "")
}

// newPlanRowsFor builds the planning rows, stamping each with currentQuery so an
// inline amount edit posts back to the period the user is looking at.
func newPlanRowsFor(rows []store.PlanRow, currentQuery string) []planRowView {
	out := make([]planRowView, 0, len(rows))
	for _, r := range rows {
		view := planRowView{
			CurrentQuery: currentQuery,
			Category:     r.Category.Key,
			Bucket:       r.Category.Bucket,
			BucketLabel:  bucketLabel(r.Category.Bucket),
			Planned:      r.Planned,
			Amount:       r.Amount,
			AmountValue:  r.Amount.String(),
			AmountLabel:  moneyLabel(r.Amount),
			Actual:       r.Actual,
			ActualLabel:  moneyLabel(r.Actual),
			Notes:        r.Notes,
			HasRemain:    r.HasRemain,
			Remaining:    r.Remaining,
			Overspent:    r.Overspent,
			IsSavings:    r.Category.Bucket == store.BucketSavings,
		}
		if r.HasRemain {
			view.RemainingText = moneyLabel(r.Remaining)
		}
		if r.Planned {
			view.Bar = newBar(r.Progress, r.Overspent,
				fmt.Sprintf("%s progress for %s", view.BucketLabel, r.Category.Key))
			view.ProgressText = fmt.Sprintf("%d%%", view.Bar.Percent)
		}
		out = append(out, view)
	}
	return out
}

func bucketLabel(b store.Bucket) string {
	switch b {
	case store.BucketNeeds:
		return "Needs"
	case store.BucketWants:
		return "Wants"
	case store.BucketSavings:
		return "Savings"
	}
	return string(b)
}

// accountView is one row on the accounts page.
type accountView struct {
	Key          string
	Balance      money.Amount
	BalanceLabel string
	Notes        string
}

// categoryView is one row in the accounts page's category section.
type categoryView struct {
	Key         string
	Bucket      store.Bucket
	BucketLabel string
}

// toBuyView is one row on the to-buy page.
type toBuyView struct {
	ID           int64
	Name         string
	Category     string
	Priority     store.Priority
	PriorityText string
	Qty          int
	Price        money.Amount
	PriceValue   string
	PriceLabel   string
	Total        money.Amount
	TotalLabel   string
	URL          string
	Notes        string
	Status       store.BuyingStatus
	StatusLabel  string
}

func newToBuyRows(items []store.ToBuyItem) []toBuyView {
	out := make([]toBuyView, 0, len(items))
	for _, i := range items {
		total := i.ExpectedTotal()
		out = append(out, toBuyView{
			ID:           i.ID,
			Name:         i.Name,
			Category:     i.Category,
			Priority:     i.Priority,
			PriorityText: priorityLabel(i.Priority),
			Qty:          i.Qty,
			Price:        i.Price,
			PriceValue:   i.Price.String(),
			PriceLabel:   moneyLabel(i.Price),
			Total:        total,
			TotalLabel:   moneyLabel(total),
			URL:          i.URL,
			Notes:        i.Notes,
			Status:       i.Status,
			StatusLabel:  statusLabel(i.Status),
		})
	}
	return out
}

func priorityLabel(p store.Priority) string {
	switch p {
	case store.PriorityHigh:
		return "High"
	case store.PriorityMedium:
		return "Medium"
	case store.PriorityLow:
		return "Low"
	}
	return "Unknown"
}

func statusLabel(s store.BuyingStatus) string {
	switch s {
	case store.StatusToBuy:
		return "To buy"
	case store.StatusBought:
		return "Bought"
	case store.StatusCanceled:
		return "Canceled"
	}
	return string(s)
}

// dashboardData is the view model for GET /.
type dashboardData struct {
	Title            string
	Period           period.Period
	PeriodLabel      string
	Accounts         []accountView
	TotalBalance     money.Amount
	TotalBalanceText string
	Income           money.Amount
	IncomeText       string
	Spending         money.Amount
	SpendingText     string
	Remaining        money.Amount
	RemainingText    string
	IsOver           bool
	HasPlans         bool
	PlannedCount     int
	ProgressText     string
	PlanRows         []planRowView
	Recent           []transactionRow
}

// selectPeriod reads the ?year= and ?month= query parameters, falling back to
// the period containing today.
func selectPeriod(r *http.Request) period.Period {
	q := r.URL.Query()
	year, month := atoiDefault(q.Get("year"), 0), atoiDefault(q.Get("month"), 0)
	if year == 0 || month < 1 || month > 12 {
		return period.Now()
	}
	return period.New(year, month)
}

// periodQuery builds a query string that keeps the rest of the URL intact while
// pointing at p, so period navigation does not drop the active filter.
func periodQuery(p period.Period, extra url.Values) string {
	q := url.Values{}
	for k, vs := range extra {
		q[k] = vs
	}
	q.Set("year", fmt.Sprint(p.Year))
	q.Set("month", fmt.Sprint(p.Month))
	return "?" + q.Encode()
}

// filteredQuery drops the period parameters, used by forms that post and then
// redirect back to the current period.
func withoutPeriod(v url.Values) url.Values {
	out := url.Values{}
	for k, vals := range v {
		if k == "year" || k == "month" {
			continue
		}
		out[k] = vals
	}
	return out
}

func atoiDefault(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n
}
