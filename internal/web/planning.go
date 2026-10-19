package web

import (
	"log"
	"net/http"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
	"github.com/4damrr/tr4cker/internal/store"
)

// planningData is the view model for the planning page.
type planningData struct {
	Period       period.Period
	PeriodLabel  string
	PrevQuery    string
	NextQuery    string
	CurrentQuery string
	Rows         []planRowView
	Buckets      []store.Bucket
}

func (s *Server) planningDataFor(p period.Period) (planningData, error) {
	rows, err := s.store.PlanRows(p)
	if err != nil {
		return planningData{}, err
	}
	current := periodQuery(p, withoutPeriod(nil))
	return planningData{
		Period:       p,
		PeriodLabel:  p.Label(),
		PrevQuery:    periodQuery(p.Prev(), withoutPeriod(nil)),
		NextQuery:    periodQuery(p.Next(), withoutPeriod(nil)),
		CurrentQuery: current,
		Rows:         newPlanRowsFor(rows, current),
		Buckets:      []store.Bucket{store.BucketNeeds, store.BucketWants, store.BucketSavings},
	}, nil
}

func (s *Server) handlePlanningPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.planningDataFor(selectPeriod(r))
	if err != nil {
		log.Printf("planning page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["planning"], map[string]any{"Planning": data})
}

// renderPlanRows returns the plan rows region, used after inline amount edits
// and for period navigation over HTMX.
func (s *Server) renderPlanRows(w http.ResponseWriter, r *http.Request) {
	data, err := s.planningDataFor(selectPeriod(r))
	if err != nil {
		log.Printf("plan rows: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "plan-rows", data)
}

func (s *Server) handleSetPlan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.failMutation(w, r, err, "parse plan form")
		return
	}
	amount, err := money.Parse(r.FormValue("amount"))
	if err != nil {
		s.failMutation(w, r, err, "parse planned amount")
		return
	}

	p := selectPeriod(r)
	category := r.FormValue("category")

	amountField := r.FormValue("amount_field")
	notes := r.FormValue("notes")

	if amount.IsZero() && amountField == "amount" {
		if err := s.store.RemovePlan(p, category); err != nil {
			s.failMutation(w, r, err, "remove plan")
			return
		}
		s.finishPlanMutation(w, r)
		return
	}

	if _, err := s.store.SetPlan(p, category, amount, notes); err != nil {
		s.failMutation(w, r, err, "set plan")
		return
	}
	s.finishPlanMutation(w, r)
}

func (s *Server) handleRemovePlan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.failMutation(w, r, err, "parse plan form")
		return
	}
	if err := s.store.RemovePlan(selectPeriod(r), r.FormValue("category")); err != nil {
		s.failMutation(w, r, err, "remove plan")
		return
	}
	s.finishPlanMutation(w, r)
}

// finishPlanMutation refreshes the planning rows, or the dashboard figures when
// the request came from the dashboard.
func (s *Server) finishPlanMutation(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		if r.Header.Get("HX-Target") == "dashboard-figures" {
			s.renderDashboardFigures(w, r)
			return
		}
		s.renderPlanRows(w, r)
		return
	}
	redirectBack(w, r)
}
