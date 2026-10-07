package web

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Expense struct {
	ID          int64
	Description string
	Amount      float64
	CreatedAt   string
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	page, err := s.expensePage()
	if err != nil {
		log.Printf("list expenses: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.render(w, "layout", page)
}

func (s *Server) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	amount, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("amount")), 64)
	if err != nil || description == "" || amount == 0 {
		http.Error(w, "description and a non-zero amount are required", http.StatusBadRequest)
		return
	}

	if _, err := insertExpense(s.db, description, amount); err != nil {
		log.Printf("insert expense: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.renderExpenseList(w)
}

func (s *Server) handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := deleteExpense(s.db, id); err != nil {
		log.Printf("delete expense: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.renderExpenseList(w)
}

func (s *Server) renderExpenseList(w http.ResponseWriter) {
	page, err := s.expensePage()
	if err != nil {
		log.Printf("list expenses: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.render(w, "expense-list", page)
}

func (s *Server) expensePage() (map[string]any, error) {
	expenses, err := listExpenses(s.db)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Title":    "Home",
		"Expenses": expenses,
		"Total":    sumAmounts(expenses),
	}, nil
}

func sumAmounts(expenses []Expense) float64 {
	var total float64
	for _, e := range expenses {
		total += e.Amount
	}
	return total
}
