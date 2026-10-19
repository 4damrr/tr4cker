package web

import (
	"log"
	"net/http"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/store"
)

// accountsData is the view model for the accounts page, which also carries the
// secondary category management section.
type accountsData struct {
	Accounts   []accountView
	Categories []categoryView
	Buckets    []store.Bucket
	Types      []store.TransactionType
}

func (s *Server) accountsData() (accountsData, error) {
	accounts, err := s.store.ListAccounts()
	if err != nil {
		return accountsData{}, err
	}
	categories, err := s.store.ListCategories()
	if err != nil {
		return accountsData{}, err
	}

	categoryViews := make([]categoryView, 0, len(categories))
	for _, c := range categories {
		categoryViews = append(categoryViews, categoryView{
			Key: c.Key, Bucket: c.Bucket, BucketLabel: bucketLabel(c.Bucket),
		})
	}

	return accountsData{
		Accounts:   newAccountViews(accounts),
		Categories: categoryViews,
		Buckets:    []store.Bucket{store.BucketNeeds, store.BucketWants, store.BucketSavings},
		Types:      []store.TransactionType{store.TypeIncome, store.TypeExpense, store.TypeTransfer},
	}, nil
}

func (s *Server) handleAccountsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.accountsData()
	if err != nil {
		log.Printf("accounts page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["accounts"], map[string]any{"Accounts": data})
}

// renderAccountList returns the account list region, used after mutations.
func (s *Server) renderAccountList(w http.ResponseWriter) {
	data, err := s.accountsData()
	if err != nil {
		log.Printf("account list: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "account-list", data)
}

// renderCategoryList returns the category section region, used after mutations.
func (s *Server) renderCategoryList(w http.ResponseWriter) {
	data, err := s.accountsData()
	if err != nil {
		log.Printf("category list: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "category-list", data)
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	balance, err := money.Parse(r.FormValue("balance"))
	if err != nil {
		http.Error(w, "balance must be a number", http.StatusBadRequest)
		return
	}

	if _, err := s.store.CreateAccount(r.FormValue("key"), balance, r.FormValue("notes")); err != nil {
		s.failMutation(w, r, err, "create account")
		return
	}
	s.finishMutation(w, r, s.renderAccountList)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	balance, err := money.Parse(r.FormValue("balance"))
	if err != nil {
		http.Error(w, "balance must be a number", http.StatusBadRequest)
		return
	}

	if _, err := s.store.UpdateAccount(r.PathValue("key"), balance, r.FormValue("notes")); err != nil {
		s.failMutation(w, r, err, "update account")
		return
	}
	s.finishMutation(w, r, s.renderAccountList)
}

func (s *Server) handleDeactivateAccount(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeactivateAccount(r.PathValue("key")); err != nil {
		s.failMutation(w, r, err, "deactivate account")
		return
	}
	s.finishMutation(w, r, s.renderAccountList)
}

// Category management lives in a secondary section of the accounts page.

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if _, err := s.store.CreateCategory(r.FormValue("key"), store.Bucket(r.FormValue("buckets"))); err != nil {
		s.failMutation(w, r, err, "create category")
		return
	}
	s.finishMutation(w, r, s.renderCategoryList)
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if _, err := s.store.UpdateCategory(r.PathValue("key"), store.Bucket(r.FormValue("buckets"))); err != nil {
		s.failMutation(w, r, err, "update category")
		return
	}
	s.finishMutation(w, r, s.renderCategoryList)
}

func (s *Server) handleDeactivateCategory(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeactivateCategory(r.PathValue("key")); err != nil {
		s.failMutation(w, r, err, "deactivate category")
		return
	}
	s.finishMutation(w, r, s.renderCategoryList)
}
