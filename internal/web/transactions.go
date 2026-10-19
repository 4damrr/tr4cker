package web

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/store"
)

// transactionsData is the view model for the transactions page.
type transactionsData struct {
	Rows      []transactionRow
	Type      string
	Types     []store.TransactionType
	TypeQuery string
}

func (s *Server) transactionsData(r *http.Request) (transactionsData, error) {
	filter := store.TransactionFilter{}

	typeParam := r.URL.Query().Get("type")
	if store.ValidType(store.TransactionType(typeParam)) {
		filter.Types = []store.TransactionType{store.TransactionType(typeParam)}
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filter.Categories = []string{category}
	}
	if account := r.URL.Query().Get("account"); account != "" {
		filter.Accounts = []string{account}
	}

	list, err := s.store.ListTransactions(filter)
	if err != nil {
		return transactionsData{}, err
	}

	// The filter links carry the whole query so category and account filters
	// survive a type change.
	typeQuery := ""
	if typeParam != "" {
		typeQuery = "?" + r.URL.Query().Encode()
	}

	return transactionsData{
		Rows: newTransactionRows(list),
		Type: typeParam,
		Types: []store.TransactionType{
			store.TypeIncome, store.TypeExpense, store.TypeTransfer,
		},
		TypeQuery: typeQuery,
	}, nil
}

func (s *Server) handleTransactionsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.transactionsData(r)
	if err != nil {
		log.Printf("transactions page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["transactions"], map[string]any{"Transactions": data})
}

// renderTransactionList returns the list region for HTMX updates.
func (s *Server) renderTransactionList(w http.ResponseWriter, r *http.Request) {
	data, err := s.transactionsData(r)
	if err != nil {
		log.Printf("transaction list: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "transaction-list", data)
}

// entryData is the view model for the dedicated entry view, and for editing.
type entryData struct {
	Accounts   []store.Account
	Categories []store.Category
	Types      []store.TransactionType
	Buckets    []store.Bucket
	Form       entryForm
	IsEdit     bool
	EditID     int64
}

// entryForm holds the transaction form's current values.
type entryForm struct {
	Type         store.TransactionType
	TypeValue    string
	Amount       string
	Category     string
	At           time.Time
	AtValue      string
	AccountFrom  string
	AccountTo    string
	IsInvestment bool
}

// defaultEntryForm is the fast path: an expense, now, on the most recently used
// account. See design.md - Transaction entry is a dedicated view.
func (s *Server) defaultEntryForm() entryForm {
	now := time.Now()
	form := entryForm{
		Type:      store.TypeExpense,
		TypeValue: string(store.TypeExpense),
		At:        now,
		AtValue:   datetimeInput(now),
		AccountTo: db.External,
	}
	if recent, err := s.store.RecentTransactions(1); err == nil && len(recent) > 0 {
		form.AccountFrom = recent[0].AccountFrom
		if recent[0].AccountFrom == db.External {
			form.AccountTo = recent[0].AccountTo
		}
	}
	if accounts, err := s.store.ListAccounts(); err == nil && form.AccountFrom == db.External && len(accounts) > 0 {
		form.AccountFrom = accounts[0].Key
		form.AccountTo = db.External
	}
	return form
}

func (s *Server) entryViewData(form entryForm, isEdit bool, id int64) (entryData, error) {
	accounts, err := s.store.ListAccounts()
	if err != nil {
		return entryData{}, err
	}
	categories, err := s.store.ListCategories()
	if err != nil {
		return entryData{}, err
	}
	return entryData{
		Accounts:   accounts,
		Categories: categories,
		Types: []store.TransactionType{
			store.TypeIncome, store.TypeExpense, store.TypeTransfer,
		},
		Buckets: []store.Bucket{store.BucketNeeds, store.BucketWants, store.BucketSavings},
		Form:    form,
		IsEdit:  isEdit,
		EditID:  id,
	}, nil
}

func (s *Server) handleTransactionNewPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.entryViewData(s.defaultEntryForm(), false, 0)
	if err != nil {
		log.Printf("transaction new: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["transactionNew"], map[string]any{"Entry": data})
}

func (s *Server) handleTransactionEditPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	t, err := s.store.GetTransaction(id)
	if err != nil {
		log.Printf("edit transaction %d: %v", id, err)
		http.Error(w, "not found", statusFor(err))
		return
	}

	form := entryForm{
		Type:         t.Type,
		TypeValue:    string(t.Type),
		Amount:       t.Amount.String(),
		Category:     t.Category,
		At:           t.At,
		AtValue:      datetimeInput(t.At),
		AccountFrom:  t.AccountFrom,
		AccountTo:    t.AccountTo,
		IsInvestment: t.IsInvestment,
	}
	data, err := s.entryViewData(form, true, t.ID)
	if err != nil {
		log.Printf("edit page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["transactionNew"], map[string]any{"Entry": data, "Editing": t})
}

// parseTransactionForm reads the entry form into a store input.
func (s *Server) parseTransactionForm(r *http.Request) (store.NewTransaction, error) {
	if err := r.ParseForm(); err != nil {
		return store.NewTransaction{}, err
	}

	amount, err := money.Parse(r.FormValue("amount"))
	if err != nil {
		return store.NewTransaction{}, err
	}

	at, err := parseFormTime(r.FormValue("trx_time"))
	if err != nil {
		return store.NewTransaction{}, err
	}

	return store.NewTransaction{
		Amount:       amount,
		Type:         store.TransactionType(r.FormValue("type")),
		Category:     r.FormValue("category"),
		At:           at,
		IsInvestment: r.FormValue("is_investment") == "on",
		AccountFrom:  r.FormValue("account_from"),
		AccountTo:    r.FormValue("account_to"),
	}, nil
}

// parseFormTime reads a datetime-local value, falling back to now when blank so
// the common case needs no time input at all.
func parseFormTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Now(), nil
}

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	in, err := s.parseTransactionForm(r)
	if err != nil {
		s.failMutation(w, r, err, "parse transaction form")
		return
	}
	if _, err := s.store.CreateTransaction(in); err != nil {
		s.failMutation(w, r, err, "create transaction")
		return
	}
	s.afterTransactionMutation(w, r)
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in, err := s.parseTransactionForm(r)
	if err != nil {
		s.failMutation(w, r, err, "parse transaction form")
		return
	}
	if _, err := s.store.UpdateTransaction(id, in); err != nil {
		s.failMutation(w, r, err, "update transaction")
		return
	}
	s.afterTransactionMutation(w, r)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteTransaction(id); err != nil {
		s.failMutation(w, r, err, "delete transaction")
		return
	}
	s.afterTransactionMutation(w, r)
}

// afterTransactionMutation refreshes whichever region asked for the change: the
// transaction list on the transactions page, or the dashboard figures when the
// request came from the dashboard.
func (s *Server) afterTransactionMutation(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		if r.Header.Get("HX-Target") == "dashboard-figures" {
			s.renderDashboardFigures(w, r)
			return
		}
		s.renderTransactionList(w, r)
		return
	}
	redirectBack(w, r)
}
