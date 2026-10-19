package web

import (
	"log"
	"net/http"

	"github.com/4damrr/tr4cker/internal/period"
	"github.com/4damrr/tr4cker/internal/store"
)

// recentTransactionLimit caps the dashboard's recent transaction list. It is a
// template constant; see design.md - Open Questions.
const recentTransactionLimit = 8

// dashboardDataFor assembles everything the dashboard shows for one period.
func (s *Server) dashboardDataFor(p period.Period) (dashboardData, error) {
	accounts, err := s.store.ListAccounts()
	if err != nil {
		return dashboardData{}, err
	}
	total, err := s.store.TotalBalance()
	if err != nil {
		return dashboardData{}, err
	}
	income, err := s.store.PeriodIncome(p)
	if err != nil {
		return dashboardData{}, err
	}
	spending, err := s.store.PeriodSpending(p)
	if err != nil {
		return dashboardData{}, err
	}
	remaining, hasPlans, err := s.store.RemainingPlanned(p)
	if err != nil {
		return dashboardData{}, err
	}
	planRows, err := s.store.PlanRows(p)
	if err != nil {
		return dashboardData{}, err
	}
	recent, err := s.store.RecentTransactions(recentTransactionLimit)
	if err != nil {
		return dashboardData{}, err
	}

	data := dashboardData{
		Period:           p,
		PeriodLabel:      p.Label(),
		Accounts:         newAccountViews(accounts),
		TotalBalance:     total,
		TotalBalanceText: moneyLabel(total),
		Income:           income,
		IncomeText:       moneyLabel(income),
		Spending:         spending,
		SpendingText:     moneyLabel(spending),
		Remaining:        remaining,
		RemainingText:    moneyLabel(remaining),
		IsOver:           remaining < 0,
		HasPlans:         hasPlans,
		PlanRows:         newPlanRows(planRows),
		Recent:           newTransactionRows(recent),
	}
	for _, r := range data.PlanRows {
		if r.Planned {
			data.PlannedCount++
		}
	}
	return data, nil
}

func newAccountViews(accounts []store.Account) []accountView {
	out := make([]accountView, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, accountView{
			Key:          a.Key,
			Balance:      a.Balance,
			BalanceLabel: moneyLabel(a.Balance),
			Notes:        a.Notes,
		})
	}
	return out
}

// handleDashboard serves the dashboard at the site root.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := s.dashboardDataFor(period.Now())
	if err != nil {
		log.Printf("dashboard: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderPage(w, pages["dashboard"], map[string]any{"Dashboard": data})
}

// renderDashboardFigures returns only the dashboard's figures and recent
// transactions, for HTMX to swap in after a mutation.
func (s *Server) renderDashboardFigures(w http.ResponseWriter, r *http.Request) {
	data, err := s.dashboardDataFor(selectPeriod(r))
	if err != nil {
		log.Printf("dashboard figures: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	s.renderRegion(w, "dashboard-figures", data)
}
