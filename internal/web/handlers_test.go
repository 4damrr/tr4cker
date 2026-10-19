package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
	"github.com/4damrr/tr4cker/internal/store"
)

// ---------- accounts ----------

func TestCreateAccountRendersTheRefreshedList(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := url.Values{"key": {"Tabungan"}, "balance": {"1200000"}, "notes": {"savings account"}}
	rec := htmx(t, srv, http.MethodPost, "/accounts", form, "account-list")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Tabungan") {
		t.Error("the refreshed list is missing the new account")
	}
	if !strings.Contains(body, "Rp 1,200,000") {
		t.Error("the refreshed list is missing the new balance")
	}

	// The HTMX response is a region, not a whole page.
	if strings.Contains(body, "</html>") {
		t.Error("the HTMX response rendered a whole page instead of the list region")
	}

	stored, err := st.GetAccount("Tabungan")
	if err != nil {
		t.Fatalf("account not stored: %v", err)
	}
	if stored.Balance != money.New(1200000) {
		t.Errorf("stored balance = %s, want 1,200,000", stored.Balance)
	}
}

func TestCreateAccountRejectsDuplicateKey(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := url.Values{"key": {"BRI"}, "balance": {"1000"}, "notes": {""}}
	rec := htmx(t, srv, http.MethodPost, "/accounts", form, "account-list")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already exists") {
		t.Errorf("body = %q, want it to explain the duplicate", rec.Body.String())
	}

	// The original balance is untouched.
	a, err := st.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.Balance != money.New(2500000) {
		t.Errorf("BRI balance = %s, want the original 2,500,000", a.Balance)
	}
}

func TestDeactivateAccountReturnsRefreshedList(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	rec := htmx(t, srv, http.MethodDelete, "/accounts/Gopay", nil, "account-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), ">Gopay<") {
		t.Error("the deactivated account is still in the refreshed list")
	}

	if ok, err := st.AccountExists("Gopay"); err != nil || ok {
		t.Errorf("AccountExists(Gopay) = %v, %v; want false", ok, err)
	}
}

func TestUpdateAccountChangesBalanceAndNotes(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := url.Values{"balance": {"2600000"}, "notes": {"primary checking"}}
	rec := htmx(t, srv, http.MethodPost, "/accounts/BRI", form, "account-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	a, err := st.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.Balance != money.New(2600000) {
		t.Errorf("balance = %s, want 2,600,000", a.Balance)
	}
	if a.Notes != "primary checking" {
		t.Errorf("notes = %q, want %q", a.Notes, "primary checking")
	}
}

// ---------- categories ----------

func TestCreateCategoryInEveryBucketFromTheSecondarySection(t *testing.T) {
	tests := []struct {
		bucket store.Bucket
		label  string
	}{
		{store.BucketNeeds, "Needs"},
		{store.BucketWants, "Wants"},
		{store.BucketSavings, "Savings"},
	}

	for _, tc := range tests {
		t.Run(string(tc.bucket), func(t *testing.T) {
			srv, st := newTestServer(t)
			seed(t, st)

			form := url.Values{"key": {"New-" + string(tc.bucket)}, "buckets": {string(tc.bucket)}}
			rec := htmx(t, srv, http.MethodPost, "/categories", form, "category-list")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "New-"+string(tc.bucket)) {
				t.Error("the refreshed category list is missing the new category")
			}

			got, err := st.GetCategory("New-" + string(tc.bucket))
			if err != nil {
				t.Fatalf("category not stored: %v", err)
			}
			if got.Bucket != tc.bucket {
				t.Errorf("bucket = %q, want %q", got.Bucket, tc.bucket)
			}
		})
	}
}

func TestDeactivateCategoryRemovesItFromTheChoices(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	rec := htmx(t, srv, http.MethodDelete, "/categories/Fun", nil, "category-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), ">Fun<") {
		t.Error("the deactivated category is still offered as a choice")
	}

	// It must also disappear from the transaction entry form's category list.
	entry := get(t, srv, "/transactions/new").Body.String()
	if strings.Contains(entry, `value="Fun"`) {
		t.Error("a deactivated category is still offered on the entry form")
	}
}

func TestCreateCategoryRejectsUnknownBucket(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := url.Values{"key": {"Nonsense"}, "buckets": {"bogus"}}
	rec := htmx(t, srv, http.MethodPost, "/categories", form, "category-list")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ---------- transactions ----------

func expenseForm(amount, category, from string, at time.Time) url.Values {
	return url.Values{
		"type":         {string(store.TypeExpense)},
		"amount":       {amount},
		"category":     {category},
		"trx_time":     {at.Format("2006-01-02T15:04")},
		"account_from": {from},
		"account_to":   {db.External},
	}
}

func TestTransactionEntryRendersDefaults(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	// Give the store a most-recently-used account.
	if _, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(25000), Type: store.TypeExpense, Category: "Food",
		At: time.Now(), AccountFrom: "Gopay", AccountTo: db.External,
	}); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	body := get(t, srv, "/transactions/new").Body.String()

	if !strings.Contains(body, `value="expense" selected`) {
		t.Error("the entry form does not default to type expense")
	}
	if !strings.Contains(body, `name="trx_time" value="`+datetimeInput(time.Now())[:13]) {
		t.Errorf("the entry form does not default the date and time to now")
	}
	if !strings.Contains(body, `value="Gopay" selected`) {
		t.Error("the entry form does not preselect the most recently used account")
	}
	if !strings.Contains(body, "Save transaction") {
		t.Error("the entry form is missing its submit button")
	}
}

func TestSaveTransactionWithNoAmountIsRejected(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := expenseForm("", "Food", "BRI", time.Now())
	rec := htmx(t, srv, http.MethodPost, "/transactions", form, "")

	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200, want a rejection (body: %s)", rec.Body.String())
	}

	listed, err := st.ListTransactions(store.TransactionFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("stored %d transactions after a rejected save, want 0", len(listed))
	}
}

func TestCreateTransactionThroughHTMXRefreshesTheList(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := expenseForm("25000", "Food", "BRI", time.Now())
	rec := htmx(t, srv, http.MethodPost, "/transactions", form, "transaction-list")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rp 25,000") {
		t.Error("the refreshed list is missing the new amount")
	}
	if !strings.Contains(body, "BRI") {
		t.Error("the refreshed list is missing the account")
	}
}

func TestEditTransactionAmountUpdatesTheList(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	created, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(25000), Type: store.TypeExpense, Category: "Food",
		At: time.Now(), AccountFrom: "BRI", AccountTo: db.External,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	form := expenseForm("27000", "Food", "BRI", created.At)
	rec := htmx(t, srv, http.MethodPost, "/transactions/"+strconv.FormatInt(created.ID, 10), form, "transaction-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	got, err := st.GetTransaction(created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Amount != money.New(27000) {
		t.Errorf("amount = %s, want 27,000", got.Amount)
	}
	// Balance follows the edit.
	a, err := st.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if a.Balance != money.New(2500000-27000) {
		t.Errorf("BRI = %s, want 2,473,000", a.Balance)
	}
}

func TestDeleteTransactionReturnsTheRefreshedList(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	created, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(25000), Type: store.TypeExpense, Category: "Food",
		At: time.Now(), AccountFrom: "BRI", AccountTo: db.External,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rec := htmx(t, srv, http.MethodDelete, "/transactions/"+strconv.FormatInt(created.ID, 10), nil, "transaction-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Rp 25,000") {
		t.Error("the soft-deleted transaction is still in the refreshed list")
	}

	// The balance is restored.
	a, err := st.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if a.Balance != money.New(2500000) {
		t.Errorf("BRI = %s, want the original 2,500,000 after the soft-delete", a.Balance)
	}
}

func TestTransactionListFiltersByType(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	records := []store.NewTransaction{
		{Amount: money.New(10000), Type: store.TypeExpense, Category: "Food",
			At: time.Now(), AccountFrom: "BRI", AccountTo: db.External},
		{Amount: money.New(500000), Type: store.TypeIncome, Category: "Food",
			At: time.Now(), AccountFrom: db.External, AccountTo: "BRI"},
	}
	for _, in := range records {
		if _, err := st.CreateTransaction(in); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	body := get(t, srv, "/transactions?type=expense").Body.String()
	if !strings.Contains(body, "Rp 10,000") {
		t.Error("the expense filter is missing the expense")
	}
	if strings.Contains(body, "Rp 500,000") {
		t.Error("the expense filter is showing an income")
	}

	all := get(t, srv, "/transactions").Body.String()
	if !strings.Contains(all, "Rp 10,000") || !strings.Contains(all, "Rp 500,000") {
		t.Error("the unfiltered list is missing a transaction")
	}
}

// ---------- planning ----------

func planForm(category, amount string, p period.Period) url.Values {
	return url.Values{
		"category": {category},
		"amount":   {amount},
		"notes":    {""},
		"year":     {strconv.Itoa(p.Year)},
		"month":    {strconv.Itoa(p.Month)},
	}
}

func TestPlanningPeriodNavigationChangesTheFigures(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	current := period.Now()
	previous := current.Prev()

	if _, err := st.SetPlan(current, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("plan current: %v", err)
	}
	if _, err := st.SetPlan(previous, "Food", money.New(900000), ""); err != nil {
		t.Fatalf("plan previous: %v", err)
	}

	// Current period shows its own planned amount.
	currentBody := get(t, srv, "/planning").Body.String()
	if !strings.Contains(currentBody, "Rp 1,500,000") {
		t.Error("the current period is missing its planned amount")
	}
	if strings.Contains(currentBody, "Rp 900,000") {
		t.Error("the current period is showing the previous period's plan")
	}

	// Navigating back shows the earlier period's figures.
	prevTarget := "/planning?year=" + strconv.Itoa(previous.Year) + "&month=" + strconv.Itoa(previous.Month)
	prevBody := get(t, srv, prevTarget).Body.String()
	if !strings.Contains(prevBody, "Rp 900,000") {
		t.Error("the previous period is missing its planned amount")
	}
	if !strings.Contains(prevBody, previous.Label()) {
		t.Errorf("the previous period page is missing its label %q", previous.Label())
	}
	if strings.Contains(prevBody, "Rp 1,500,000") {
		t.Error("the previous period is showing the current period's plan")
	}
}

func TestInlineAmountEditUpdatesItsRow(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	p := period.Now()
	form := planForm("Food", "1500000", p)
	rec := htmx(t, srv, http.MethodPost, "/planning?year="+strconv.Itoa(p.Year)+"&month="+strconv.Itoa(p.Month), form, "plan-rows")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rp 1,500,000") {
		t.Error("the refreshed row is missing the planned amount")
	}
	if !strings.Contains(body, `id="plan-Food"`) {
		t.Error("the refreshed region is missing the Food row")
	}

	// Changing the amount replaces it rather than adding a second entry.
	form = planForm("Food", "500000", p)
	rec = htmx(t, srv, http.MethodPost, "/planning?year="+strconv.Itoa(p.Year)+"&month="+strconv.Itoa(p.Month), form, "plan-rows")
	if !strings.Contains(rec.Body.String(), "Rp 500,000") {
		t.Error("the edited amount is not shown")
	}
	if strings.Contains(rec.Body.String(), "Rp 1,500,000") {
		t.Error("the previous amount is still shown after the edit")
	}

	plans, err := st.PlansForPeriod(p)
	if err != nil {
		t.Fatalf("plans: %v", err)
	}
	if len(plans) != 1 || plans["Food"].Amount != money.New(500000) {
		t.Errorf("stored plans = %+v, want exactly one entry of 500,000", plans)
	}
}

func TestPlanningPageShowsRemainingAndOverspend(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	p := period.Now()
	if _, err := st.SetPlan(p, "Food", money.New(500000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(900000), Type: store.TypeExpense, Category: "Food",
		At: p.Start().Add(48 * time.Hour), AccountFrom: "BRI", AccountTo: db.External,
	}); err != nil {
		t.Fatalf("expense: %v", err)
	}

	body := get(t, srv, "/planning").Body.String()
	if !strings.Contains(body, "Rp -400,000") {
		t.Error("the planning page is missing the negative remaining amount")
	}
	if !strings.Contains(body, "overspent") {
		t.Error("the planning page is missing the overspent marker")
	}
}

func TestPlanningPageShowsSavingsProgressFromTransfers(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	p := period.Now()
	if _, err := st.SetPlan(p, "Emergency", money.New(1000000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(400000), Type: store.TypeTransfer, Category: "Emergency",
		At: p.Start().Add(24 * time.Hour), AccountFrom: "BRI", AccountTo: "Gopay",
	}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	// An expense in the savings category must not move the target's progress.
	if _, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(900000), Type: store.TypeExpense, Category: "Emergency",
		At: p.Start().Add(25 * time.Hour), AccountFrom: "BRI", AccountTo: db.External,
	}); err != nil {
		t.Fatalf("expense: %v", err)
	}

	body := get(t, srv, "/planning").Body.String()
	if !strings.Contains(body, "Rp 400,000") {
		t.Error("the savings progress is missing the transferred amount")
	}
	if strings.Contains(body, "Rp 900,000") {
		t.Error("an expense in a savings category was counted toward its target")
	}
	if !strings.Contains(body, "40%") {
		t.Error("the savings progress percentage is missing")
	}
}

// ---------- to buy ----------

func toBuyForm(name, category string, priority int) url.Values {
	return url.Values{
		"name": {name}, "category": {category},
		"priority": {strconv.Itoa(priority)}, "qty": {"1"},
		"price": {"1000"}, "url": {""}, "notes": {""},
	}
}

func TestCreateToBuyShowsTheExpectedTotal(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := toBuyForm("Keyboard", "Food", 1)
	form.Set("qty", "3")
	form.Set("price", "125000.55")

	rec := htmx(t, srv, http.MethodPost, "/to-buy", form, "tobuy-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Keyboard") {
		t.Error("the refreshed list is missing the new item")
	}
	if !strings.Contains(body, "Rp 375,001.65") {
		t.Error("the refreshed list is missing the expected total for a fractional price")
	}
}

func TestToBuyStatusChangeUpdatesTheRow(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	created, err := st.CreateToBuy(store.NewToBuy{
		Category: "Food", Name: "Keyboard", Priority: store.PriorityHigh, Qty: 1, Price: money.New(750000),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rec := htmx(t, srv, http.MethodPost, "/to-buy/"+strconv.FormatInt(created.ID, 10)+"/status",
		url.Values{"status": {"bought"}}, "tobuy-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Bought") {
		t.Error("the refreshed row does not show the new status")
	}

	// Marking it bought must not touch transactions or balances.
	listed, err := st.ListTransactions(store.TransactionFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("marking an item bought created %d transactions, want 0", len(listed))
	}
	a, err := st.GetAccount("BRI")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if a.Balance != money.New(2500000) {
		t.Errorf("BRI = %s, want the unchanged 2,500,000", a.Balance)
	}
}

func TestRejectedPriorityLeavesTheListUnchanged(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	form := toBuyForm("Keyboard", "Food", 4)
	rec := htmx(t, srv, http.MethodPost, "/to-buy", form, "tobuy-list")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	items, err := st.ListToBuy(nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("stored %d items after a rejected priority, want 0", len(items))
	}
}

func TestToBuyListFiltersByStatus(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	pending, err := st.CreateToBuy(store.NewToBuy{Category: "Food", Name: "Pending", Priority: store.PriorityHigh, Qty: 1, Price: money.New(1000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	bought, err := st.CreateToBuy(store.NewToBuy{Category: "Food", Name: "Bought", Priority: store.PriorityHigh, Qty: 1, Price: money.New(2000)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := st.SetToBuyStatus(bought.ID, store.StatusBought); err != nil {
		t.Fatalf("mark bought: %v", err)
	}

	body := get(t, srv, "/to-buy?status=to_buy").Body.String()
	if !strings.Contains(body, "Pending") {
		t.Error("the filter dropped the pending item")
	}
	if strings.Contains(body, "Rp 2,000") {
		t.Error("the filter still shows a bought item")
	}
	_ = pending
}

// ---------- HTMX dashboard refresh ----------

func TestDashboardFiguresRefreshAfterARecordedExpense(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	p := period.Now()
	if _, err := st.SetPlan(p, "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}

	form := expenseForm("900000", "Food", "BRI", p.Start().Add(48*time.Hour))
	rec := htmx(t, srv, http.MethodPost, "/transactions", form, "dashboard-figures")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rp 900,000") {
		t.Error("the refreshed dashboard is missing the new spending")
	}
	if !strings.Contains(body, "Rp 600,000") {
		t.Error("the refreshed dashboard is missing the updated remaining planned amount")
	}
	if !strings.Contains(body, "Rp 1,600,000") {
		t.Error("the refreshed dashboard is missing the updated total balance")
	}
	if !strings.Contains(body, `id="dashboard-figures"`) {
		t.Error("the response is not the dashboard figures region")
	}
}

func TestDashboardFiguresRefreshAfterAPlanChange(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	p := period.Now()
	target := "/planning?year=" + strconv.Itoa(p.Year) + "&month=" + strconv.Itoa(p.Month)
	rec := htmx(t, srv, http.MethodPost, target, planForm("Food", "1500000", p), "dashboard-figures")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Rp 1,500,000") {
		t.Error("the refreshed dashboard is missing the new planned amount")
	}
}

// ---------- redirect-after-post ----------

func TestPlainFormPostRedirectsBack(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	body := url.Values{"key": {"Savings"}, "balance": {"500"}, "notes": {""}}
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/accounts")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/accounts" {
		t.Errorf("Location = %q, want /accounts", got)
	}
}
