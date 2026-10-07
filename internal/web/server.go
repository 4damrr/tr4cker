package web

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Server struct {
	db   *sql.DB
	tmpl *template.Template
	mux  *http.ServeMux
}

func New(db *sql.DB) (*Server, error) {
	root := envOr("WEB_ROOT", ".")
	tmpl, err := template.ParseFiles(
		filepath.Join(root, "templates/layout.html"),
		filepath.Join(root, "templates/pages/home.html"),
		filepath.Join(root, "templates/components/expense-row.html"),
		filepath.Join(root, "templates/components/expense-list.html"),
	)
	if err != nil {
		return nil, err
	}

	s := &Server{db: db, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes(root)
	return s, nil
}

func (s *Server) routes(root string) {
	staticDir := filepath.Join(root, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	s.mux.HandleFunc("GET /{$}", s.handleHome)
	s.mux.HandleFunc("POST /expenses", s.handleCreateExpense)
	s.mux.HandleFunc("DELETE /expenses/{id}", s.handleDeleteExpense)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
