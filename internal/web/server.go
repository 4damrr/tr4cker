// Package web serves tr4cker's HTTP routes and renders its templates.
//
// Pages are server-rendered HTML. HTMX requests receive only the region they
// asked for; everything else gets the full shell. See design.md.
package web

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/4damrr/tr4cker/internal/store"
)

// page pairs a page template file with the uniquely named content block it
// defines. The shell calls a single {{block "content"}}, so every page needs its
// own block name; see design.md - Templates get a per-page content block.
type page struct {
	File  string
	Block string
	Nav   string
	Title string

	// tmpl is a clone of the base set in which "content" is defined to call
	// this page's uniquely named block. Go's template package needs a literal
	// name to dispatch, so each page gets its own set.
	tmpl *template.Template
}

var pages = map[string]*page{
	"dashboard":      {File: "pages/dashboard.html", Block: "dashboard-content", Nav: "dashboard", Title: "Dashboard"},
	"transactions":   {File: "pages/transactions.html", Block: "transactions-content", Nav: "transactions", Title: "Transactions"},
	"transactionNew": {File: "pages/transaction_new.html", Block: "transaction-new-content", Nav: "transactions", Title: "New transaction"},
	"planning":       {File: "pages/planning.html", Block: "planning-content", Nav: "planning", Title: "Planning"},
	"tobuy":          {File: "pages/tobuy.html", Block: "tobuy-content", Nav: "tobuy", Title: "To Buy"},
	"accounts":       {File: "pages/accounts.html", Block: "accounts-content", Nav: "accounts", Title: "Accounts"},
}

// Server holds the dependencies shared by every handler.
type Server struct {
	store *store.Store
	base  *template.Template
	mux   *http.ServeMux
}

// New builds a Server. Templates are discovered from the templates/ directory
// under root so adding a page is a file drop rather than a code edit.
func New(conn *sql.DB) (*Server, error) {
	root := envOr("WEB_ROOT", ".")
	base, err := template.New("").Funcs(funcMap).ParseFS(
		templateFiles(root),
		"templates/*.html",
		"templates/pages/*.html",
		"templates/components/*.html",
	)
	if err != nil {
		return nil, err
	}

	s := &Server{store: store.New(conn), base: base, mux: http.NewServeMux()}
	if err := s.buildPageTemplates(); err != nil {
		return nil, err
	}
	s.routes(root)
	return s, nil
}

// buildPageTemplates gives each page its own template set in which "content"
// calls that page's block. Every page file therefore needs a uniquely named
// block, and adding a page is a file drop rather than a code edit.
func (s *Server) buildPageTemplates() error {
	for _, p := range pages {
		set, err := s.base.Clone()
		if err != nil {
			return fmt.Errorf("clone templates for %s: %w", p.Block, err)
		}
		if _, err := set.New("content").Parse(fmt.Sprintf("{{template %q .}}", p.Block)); err != nil {
			return fmt.Errorf("wire %s: %w", p.Block, err)
		}
		p.tmpl = set
	}
	return nil
}

// templateFiles roots template discovery at the project directory, so the
// WEB_ROOT override keeps working and ParseFS can glob the templates tree.
func templateFiles(root string) fs.FS { return os.DirFS(root) }

func (s *Server) routes(root string) {
	staticDir := filepath.Join(root, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	s.mux.HandleFunc("GET /{$}", s.handleDashboard)
	s.mux.HandleFunc("GET /transactions", s.handleTransactionsPage)
	s.mux.HandleFunc("POST /transactions", s.handleCreateTransaction)
	s.mux.HandleFunc("GET /transactions/new", s.handleTransactionNewPage)
	s.mux.HandleFunc("GET /transactions/{id}/edit", s.handleTransactionEditPage)
	s.mux.HandleFunc("POST /transactions/{id}", s.handleUpdateTransaction)
	s.mux.HandleFunc("DELETE /transactions/{id}", s.handleDeleteTransaction)

	s.mux.HandleFunc("GET /planning", s.handlePlanningPage)
	s.mux.HandleFunc("POST /planning", s.handleSetPlan)
	s.mux.HandleFunc("DELETE /planning", s.handleRemovePlan)

	s.mux.HandleFunc("GET /to-buy", s.handleToBuyPage)
	s.mux.HandleFunc("POST /to-buy", s.handleCreateToBuy)
	s.mux.HandleFunc("POST /to-buy/{id}", s.handleUpdateToBuy)
	s.mux.HandleFunc("POST /to-buy/{id}/status", s.handleToBuyStatus)
	s.mux.HandleFunc("DELETE /to-buy/{id}", s.handleDeleteToBuy)

	s.mux.HandleFunc("GET /accounts", s.handleAccountsPage)
	s.mux.HandleFunc("POST /accounts", s.handleCreateAccount)
	s.mux.HandleFunc("POST /accounts/{key}", s.handleUpdateAccount)
	s.mux.HandleFunc("DELETE /accounts/{key}", s.handleDeactivateAccount)

	s.mux.HandleFunc("POST /categories", s.handleCreateCategory)
	s.mux.HandleFunc("POST /categories/{key}", s.handleUpdateCategory)
	s.mux.HandleFunc("DELETE /categories/{key}", s.handleDeactivateCategory)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// renderPage writes the full shell with a page's content block inside it.
func (s *Server) renderPage(w http.ResponseWriter, p *page, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Nav"] = p.Nav
	if _, ok := data["Title"]; !ok {
		data["Title"] = p.Title
	}
	s.execute(w, p.tmpl, "layout", data)
}

// renderRegion writes only the named template, for HTMX partial updates.
func (s *Server) renderRegion(w http.ResponseWriter, name string, data any) {
	s.execute(w, s.base, name, data)
}

func (s *Server) execute(w http.ResponseWriter, t *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// isHTMX reports whether the request came from HTMX, in which case a mutation
// returns just the affected region instead of a whole page.
func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// statusFor maps a store error onto an HTTP status code.
func statusFor(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrDuplicate), errors.Is(err, store.ErrInvalid):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
