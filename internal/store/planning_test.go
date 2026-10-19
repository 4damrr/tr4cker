package store

import (
	"testing"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

func TestSetPlanIsAnUpsertPerPeriodAndCategory(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	if _, err := s.SetPlan(p, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("first set: %v", err)
	}

	// A second amount for the same period and category replaces the first and
	// leaves exactly one row, so the UNIQUE constraint is never violated.
	if _, err := s.SetPlan(p, "Food", money.New(500000), "trimmed"); err != nil {
		t.Fatalf("second set: %v", err)
	}

	plans, err := s.PlansForPeriod(p)
	if err != nil {
		t.Fatalf("plans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("got %d plans, want 1", len(plans))
	}
	if plans["Food"].Amount != money.New(500000) {
		t.Errorf("amount = %s, want 500,000", plans["Food"].Amount)
	}
	if plans["Food"].Notes != "trimmed" {
		t.Errorf("notes = %q, want %q", plans["Food"].Notes, "trimmed")
	}
}

func TestSetPlanIsScopedToItsPeriod(t *testing.T) {
	s := fixture(t)
	current := period.New(2026, 11)
	previous := current.Prev()

	if _, err := s.SetPlan(current, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("current: %v", err)
	}
	if _, err := s.SetPlan(previous, "Food", money.New(900000), ""); err != nil {
		t.Fatalf("previous: %v", err)
	}

	cur, err := s.PlansForPeriod(current)
	if err != nil {
		t.Fatalf("current plans: %v", err)
	}
	prev, err := s.PlansForPeriod(previous)
	if err != nil {
		t.Fatalf("previous plans: %v", err)
	}
	if cur["Food"].Amount != money.New(1500000) {
		t.Errorf("current amount = %s, want 1,500,000", cur["Food"].Amount)
	}
	if prev["Food"].Amount != money.New(900000) {
		t.Errorf("previous amount = %s, want 900,000", prev["Food"].Amount)
	}
}

func TestSetPlanValidatesCategory(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	if _, err := s.SetPlan(p, "Nope", money.New(1000), ""); err == nil {
		t.Error("planning an unknown category succeeded, want an error")
	}
	if _, err := s.SetPlan(p, "  ", money.New(1000), ""); err == nil {
		t.Error("planning with an empty category succeeded, want an error")
	}
	if _, err := s.SetPlan(p, "Food", money.New(-1), ""); err == nil {
		t.Error("planning a negative amount succeeded, want an error")
	}
}

func TestSetPlanRoundsFinerInput(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	parsed, err := money.Parse("1500000.756")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := s.SetPlan(p, "Food", parsed, ""); err != nil {
		t.Fatalf("set: %v", err)
	}

	plans, err := s.PlansForPeriod(p)
	if err != nil {
		t.Fatalf("plans: %v", err)
	}
	if got := plans["Food"].Amount.String(); got != "1,500,000.76" {
		t.Errorf("stored amount = %s, want 1,500,000.76", got)
	}
}

func TestActualForCategorySelectsByBucket(t *testing.T) {
	tests := []struct {
		name     string
		category string
		bucket   Bucket
		record   []NewTransaction
		want     string
	}{
		{
			name:     "expense counts toward a needs category",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Food", at(2026, 10, 28, 12)),
			},
			want: "900,000",
		},
		{
			name:     "transfer is not spending for a needs category",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				{Amount: money.New(100000), Type: TypeTransfer, Category: "Food",
					At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
			},
			want: "0",
		},
		{
			name:     "income is not spending for a wants category",
			category: "Fun",
			bucket:   BucketWants,
			record: []NewTransaction{
				{Amount: money.New(500000), Type: TypeIncome, Category: "Fun",
					At: at(2026, 10, 28, 12), AccountFrom: db.External, AccountTo: "BRI"},
			},
			want: "0",
		},
		{
			name:     "transfer counts toward a savings category",
			category: "Emergency",
			bucket:   BucketSavings,
			record: []NewTransaction{
				{Amount: money.New(500000), Type: TypeTransfer, Category: "Emergency",
					At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
			},
			want: "500,000",
		},
		{
			name:     "investment transfer counts toward a savings category",
			category: "Emergency",
			bucket:   BucketSavings,
			record: []NewTransaction{
				{Amount: money.New(2000000), Type: TypeTransfer, Category: "Emergency",
					IsInvestment: true, At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
			},
			want: "2,000,000",
		},
		{
			name:     "expense does not count toward a savings category",
			category: "Emergency",
			bucket:   BucketSavings,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Emergency", at(2026, 10, 28, 12)),
			},
			want: "0",
		},
		{
			name:     "spending on the 25th belongs to the next period",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Food", at(2026, 11, 25, 12)),
			},
			want: "0",
		},
		{
			name:     "spending before the 25th is still this period",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Food", at(2026, 11, 2, 12)),
			},
			want: "900,000",
		},
		{
			name:     "spending on the first day of the period is included",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Food", at(2026, 10, 25, 12)),
			},
			want: "900,000",
		},
		{
			name:     "spending on the last day of the period is included",
			category: "Food",
			bucket:   BucketNeeds,
			record: []NewTransaction{
				expense(t, "900000", "BRI", "Food", at(2026, 11, 24, 12)),
			},
			want: "900,000",
		},
	}

	p := period.New(2026, 11)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			for _, in := range tc.record {
				if _, err := s.CreateTransaction(in); err != nil {
					t.Fatalf("create: %v", err)
				}
			}
			got, err := s.ActualForCategory(tc.category, tc.bucket, p)
			if err != nil {
				t.Fatalf("actual: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("actual = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestActualForCategoryExcludesSoftDeleted(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	created, err := s.CreateTransaction(expense(t, "900000", "BRI", "Food", at(2026, 10, 28, 12)))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.ActualForCategory("Food", BucketNeeds, p)
	if err != nil {
		t.Fatalf("actual: %v", err)
	}
	if got.String() != "900,000" {
		t.Errorf("actual = %s, want 900,000", got)
	}

	if err := s.DeleteTransaction(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err = s.ActualForCategory("Food", BucketNeeds, p)
	if err != nil {
		t.Fatalf("actual: %v", err)
	}
	if got.String() != "0" {
		t.Errorf("actual after soft-delete = %s, want 0", got)
	}
}

func TestDeriveRemainingAndProgress(t *testing.T) {
	tests := []struct {
		name          string
		row           PlanRow
		wantRemaining string
		wantProgress  float64
		wantOverspent bool
		wantHasRemain bool
	}{
		{
			name:          "within the limit",
			row:           PlanRow{Planned: true, Category: Category{Bucket: BucketNeeds}, Amount: money.New(1500000), Actual: money.New(900000)},
			wantRemaining: "600,000",
			wantProgress:  0.6,
			wantHasRemain: true,
		},
		{
			name:          "overspent on a whole amount",
			row:           PlanRow{Planned: true, Category: Category{Bucket: BucketWants}, Amount: money.New(500000), Actual: money.New(900000)},
			wantRemaining: "-400,000",
			wantProgress:  1,
			wantOverspent: true,
			wantHasRemain: true,
		},
		{
			name:          "a fraction over the limit is overspent",
			row:           PlanRow{Planned: true, Category: Category{Bucket: BucketNeeds}, Amount: money.New(500000), Actual: money.New(500000.60)},
			wantRemaining: "-0.60",
			wantProgress:  1,
			wantOverspent: true,
			wantHasRemain: true,
		},
		{
			name:          "plan with no spending",
			row:           PlanRow{Planned: true, Category: Category{Bucket: BucketNeeds}, Amount: money.New(1500000), Actual: money.New(0)},
			wantRemaining: "1,500,000",
			wantProgress:  0,
			wantHasRemain: true,
		},
		{
			name: "spending with no plan has no remaining or progress",
			row:  PlanRow{Planned: false, Category: Category{Bucket: BucketNeeds}, Actual: money.New(900000)},
		},
		{
			name: "a zero planned amount is treated as unplanned",
			row:  PlanRow{Planned: true, Category: Category{Bucket: BucketNeeds}, Amount: money.New(0), Actual: money.New(900000)},
		},
		{
			name:         "savings target met",
			row:          PlanRow{Planned: true, Category: Category{Bucket: BucketSavings}, Amount: money.New(1000000), Actual: money.New(1000000)},
			wantProgress: 1,
		},
		{
			name:         "savings partially funded has no remaining amount",
			row:          PlanRow{Planned: true, Category: Category{Bucket: BucketSavings}, Amount: money.New(1000000), Actual: money.New(400000)},
			wantProgress: 0.4,
		},
		{
			name:         "savings over-funded is capped at full progress",
			row:          PlanRow{Planned: true, Category: Category{Bucket: BucketSavings}, Amount: money.New(1000000), Actual: money.New(1500000)},
			wantProgress: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := derive(tc.row)
			if tc.wantHasRemain {
				if got.Remaining.String() != tc.wantRemaining {
					t.Errorf("Remaining = %s, want %s", got.Remaining, tc.wantRemaining)
				}
				if !got.HasRemain {
					t.Error("HasRemain = false, want true")
				}
			} else if got.HasRemain {
				t.Errorf("HasRemain = true with remaining %s, want false", got.Remaining)
			}
			if got.Progress != tc.wantProgress {
				t.Errorf("Progress = %v, want %v", got.Progress, tc.wantProgress)
			}
			if got.Overspent != tc.wantOverspent {
				t.Errorf("Overspent = %v, want %v", got.Overspent, tc.wantOverspent)
			}
		})
	}
}

func TestPlanRowsCoversEveryActiveCategory(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	if _, err := s.SetPlan(p, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "900000", "BRI", "Food", at(2026, 10, 28, 12))); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.CreateTransaction(expense(t, "40000", "Gopay", "Fun", at(2026, 10, 28, 12))); err != nil {
		t.Fatalf("create unplanned: %v", err)
	}

	rows, err := s.PlanRows(p)
	if err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4 (every active category)", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Category.Key] = true
	}
	for _, key := range []string{"Food", "Transport", "Fun", "Emergency"} {
		if !seen[key] {
			t.Errorf("active category %q missing from the planning rows", key)
		}
	}

	byKey := map[string]PlanRow{}
	for _, r := range rows {
		byKey[r.Category.Key] = r
	}

	food := byKey["Food"]
	if !food.Planned {
		t.Error("Food should be planned")
	}
	if food.Remaining.String() != "600,000" {
		t.Errorf("Food remaining = %s, want 600,000", food.Remaining)
	}

	fun := byKey["Fun"]
	if fun.Planned {
		t.Error("Fun should be unplanned")
	}
	if fun.Actual.String() != "40,000" {
		t.Errorf("Fun actual = %s, want 40,000", fun.Actual)
	}
	if fun.HasRemain {
		t.Error("an unplanned category should have no remaining amount")
	}
}

func TestPeriodIncomeAndSpending(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	records := []NewTransaction{
		{Amount: money.New(5000000), Type: TypeIncome, Category: "Food",
			At: at(2026, 10, 28, 12), AccountFrom: db.External, AccountTo: "BRI"},
		expense(t, "900000", "BRI", "Food", at(2026, 10, 28, 12)),
		{Amount: money.New(100000), Type: TypeTransfer, Category: "Food",
			At: at(2026, 10, 28, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
		expense(t, "700000", "BRI", "Food", at(2026, 11, 25, 12)), // next period
	}
	for _, in := range records {
		if _, err := s.CreateTransaction(in); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	income, err := s.PeriodIncome(p)
	if err != nil {
		t.Fatalf("income: %v", err)
	}
	if income.String() != "5,000,000" {
		t.Errorf("income = %s, want 5,000,000", income)
	}

	spending, err := s.PeriodSpending(p)
	if err != nil {
		t.Fatalf("spending: %v", err)
	}
	if spending.String() != "900,000" {
		t.Errorf("spending = %s, want 900,000 (transfer and next period excluded)", spending)
	}
}

func TestPeriodSpendingRoundsFloatNoise(t *testing.T) {
	s := newTestStore(t)
	mustAccount(t, s, "BRI", 10000000)
	mustCategory(t, s, "Food", BucketNeeds)

	p := period.New(2026, 11)
	// Three thirds of a rupiah accumulate to float noise that must not surface.
	for i := 0; i < 3; i++ {
		if _, err := s.CreateTransaction(expense(t, "300000", "BRI", "Food", at(2026, 10, 28, 12))); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	spending, err := s.PeriodSpending(p)
	if err != nil {
		t.Fatalf("spending: %v", err)
	}
	if spending.String() != "900,000" {
		t.Errorf("spending = %s, want 900,000", spending)
	}
}

func TestRemainingPlanned(t *testing.T) {
	tests := []struct {
		name          string
		plans         map[string]float64
		spending      map[string]float64
		wantRemaining string
		wantPlanned   bool
	}{
		{
			name:          "sums remaining across spending categories",
			plans:         map[string]float64{"Food": 1500000, "Transport": 500000},
			spending:      map[string]float64{"Food": 900000},
			wantRemaining: "1,100,000",
			wantPlanned:   true,
		},
		{
			name:          "overspending drives the total negative",
			plans:         map[string]float64{"Food": 500000},
			spending:      map[string]float64{"Food": 900000},
			wantRemaining: "-400,000",
			wantPlanned:   true,
		},
		{
			name:          "a savings target contributes no remaining amount",
			plans:         map[string]float64{"Emergency": 1000000},
			wantRemaining: "0",
			wantPlanned:   false,
		},
		{
			name:          "period with no plans",
			spending:      map[string]float64{"Food": 900000},
			wantRemaining: "0",
			wantPlanned:   false,
		},
	}

	p := period.New(2026, 11)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			for category, amount := range tc.plans {
				if _, err := s.SetPlan(p, category, money.New(amount), ""); err != nil {
					t.Fatalf("plan %s: %v", category, err)
				}
			}
			for category, amount := range tc.spending {
				in := expense(t, money.New(amount).String(), "BRI", category, at(2026, 10, 28, 12))
				if _, err := s.CreateTransaction(in); err != nil {
					t.Fatalf("spend %s: %v", category, err)
				}
			}

			got, planned, err := s.RemainingPlanned(p)
			if err != nil {
				t.Fatalf("remaining: %v", err)
			}
			if got.String() != tc.wantRemaining {
				t.Errorf("remaining = %s, want %s", got, tc.wantRemaining)
			}
			if planned != tc.wantPlanned {
				t.Errorf("planned = %v, want %v", planned, tc.wantPlanned)
			}
		})
	}
}

func TestRemovePlanAndHasPlans(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	has, err := s.HasPlans(p)
	if err != nil {
		t.Fatalf("has plans: %v", err)
	}
	if has {
		t.Error("a fresh period should have no plans")
	}

	if _, err := s.SetPlan(p, "Food", money.New(1000), ""); err != nil {
		t.Fatalf("set plan: %v", err)
	}
	has, err = s.HasPlans(p)
	if err != nil {
		t.Fatalf("has plans: %v", err)
	}
	if !has {
		t.Error("HasPlans = false after setting a plan, want true")
	}

	if err := s.RemovePlan(p, "Food"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	has, err = s.HasPlans(p)
	if err != nil {
		t.Fatalf("has plans: %v", err)
	}
	if has {
		t.Error("HasPlans = true after removing the only plan, want false")
	}
}

func TestActualsForPeriodMatchesPerCategoryAggregate(t *testing.T) {
	s := fixture(t)
	p := period.New(2026, 11)

	records := []NewTransaction{
		expense(t, "900000", "BRI", "Food", at(2026, 10, 28, 12)),
		expense(t, "40000", "Gopay", "Fun", at(2026, 10, 29, 12)),
		{Amount: money.New(500000), Type: TypeTransfer, Category: "Emergency",
			At: at(2026, 11, 1, 12), AccountFrom: "BRI", AccountTo: "Gopay"},
		expense(t, "700000", "BRI", "Food", at(2026, 11, 25, 12)), // next period
	}
	for _, in := range records {
		if _, err := s.CreateTransaction(in); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	actuals, err := s.ActualsForPeriod(p)
	if err != nil {
		t.Fatalf("actuals: %v", err)
	}
	if got := actuals["Food"].String(); got != "900,000" {
		t.Errorf("Food actual = %s, want 900,000", got)
	}
	if got := actuals["Fun"].String(); got != "40,000" {
		t.Errorf("Fun actual = %s, want 40,000", got)
	}
	if got := actuals["Emergency"].String(); got != "500,000" {
		t.Errorf("Emergency actual = %s, want 500,000 (transfer counts for savings)", got)
	}

	// The batch query must agree with the per-category aggregate.
	for category, bucket := range map[string]Bucket{
		"Food": BucketNeeds, "Fun": BucketWants, "Emergency": BucketSavings,
	} {
		one, err := s.ActualForCategory(category, bucket, p)
		if err != nil {
			t.Fatalf("actual for %s: %v", category, err)
		}
		if one != actuals[category] {
			t.Errorf("%s: per-category = %s, batch = %s; they must agree", category, one, actuals[category])
		}
	}
}
