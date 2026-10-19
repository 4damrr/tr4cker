package web

import (
	"log"
	"net/http"
	"strconv"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/store"
)

// toBuyData is the view model for the to-buy page.
type toBuyData struct {
	Rows       []toBuyView
	Status     store.BuyingStatus
	Statuses   []store.BuyingStatus
	Priorities []store.Priority
	Categories []store.Category
}

func (s *Server) toBuyDataFor(r *http.Request) (toBuyData, error) {
	var status *store.BuyingStatus
	if param := r.URL.Query().Get("status"); store.ValidStatus(store.BuyingStatus(param)) {
		parsed := store.BuyingStatus(param)
		status = &parsed
	}

	items, err := s.store.ListToBuy(status)
	if err != nil {
		return toBuyData{}, err
	}
	categories, err := s.store.ListCategories()
	if err != nil {
		return toBuyData{}, err
	}

	return toBuyData{
		Rows:     newToBuyRows(items),
		Status:   statusOrEmpty(status),
		Statuses: []store.BuyingStatus{store.StatusToBuy, store.StatusBought, store.StatusCanceled},
		Priorities: []store.Priority{
			store.PriorityHigh, store.PriorityMedium, store.PriorityLow,
		},
		Categories: categories,
	}, nil
}

func statusOrEmpty(s *store.BuyingStatus) store.BuyingStatus {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Server) handleToBuyPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.toBuyDataFor(r)
	if err != nil {
		log.Printf("to-buy page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["tobuy"], map[string]any{"ToBuy": data})
}

// renderToBuyList returns the list region, used after mutations.
func (s *Server) renderToBuyList(w http.ResponseWriter, r *http.Request) {
	data, err := s.toBuyDataFor(r)
	if err != nil {
		log.Printf("to-buy list: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "tobuy-list", data)
}

// parseToBuyForm reads the to-buy form into a store input.
func parseToBuyForm(r *http.Request) (store.NewToBuy, error) {
	if err := r.ParseForm(); err != nil {
		return store.NewToBuy{}, err
	}
	price, err := money.Parse(r.FormValue("price"))
	if err != nil {
		return store.NewToBuy{}, err
	}
	priority, err := strconv.Atoi(r.FormValue("priority"))
	if err != nil {
		return store.NewToBuy{}, err
	}
	qty, err := strconv.Atoi(r.FormValue("qty"))
	if err != nil {
		return store.NewToBuy{}, err
	}
	status := store.BuyingStatus(r.FormValue("status"))
	if status == "" {
		status = store.StatusToBuy
	}
	return store.NewToBuy{
		Category: r.FormValue("category"),
		Name:     r.FormValue("name"),
		Priority: store.Priority(priority),
		Qty:      qty,
		Price:    price,
		URL:      r.FormValue("url"),
		Notes:    r.FormValue("notes"),
		Status:   status,
	}, nil
}

func (s *Server) handleCreateToBuy(w http.ResponseWriter, r *http.Request) {
	in, err := parseToBuyForm(r)
	if err != nil {
		s.failMutation(w, r, err, "parse to-buy form")
		return
	}
	if _, err := s.store.CreateToBuy(in); err != nil {
		s.failMutation(w, r, err, "create to-buy item")
		return
	}
	s.finishMutation(w, r, func(w http.ResponseWriter) { s.renderToBuyList(w, r) })
}

func (s *Server) handleUpdateToBuy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in, err := parseToBuyForm(r)
	if err != nil {
		s.failMutation(w, r, err, "parse to-buy form")
		return
	}
	if _, err := s.store.UpdateToBuy(id, in); err != nil {
		s.failMutation(w, r, err, "update to-buy item")
		return
	}
	s.finishMutation(w, r, func(w http.ResponseWriter) { s.renderToBuyList(w, r) })
}

// handleToBuyStatus changes only the status column. It never creates a
// transaction and never moves a balance; see specs/to-buy-list.
func (s *Server) handleToBuyStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	status := store.BuyingStatus(r.FormValue("status"))
	if _, err := s.store.SetToBuyStatus(id, status); err != nil {
		s.failMutation(w, r, err, "set to-buy status")
		return
	}
	s.finishMutation(w, r, func(w http.ResponseWriter) { s.renderToBuyList(w, r) })
}

func (s *Server) handleDeleteToBuy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteToBuy(id); err != nil {
		s.failMutation(w, r, err, "delete to-buy item")
		return
	}
	s.finishMutation(w, r, func(w http.ResponseWriter) { s.renderToBuyList(w, r) })
}
