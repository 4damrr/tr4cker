package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/4damrr/tr4cker/internal/db"
	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
	"github.com/4damrr/tr4cker/internal/store"
)

// projectRoot walks up from the package directory to the module root so the
// tests read the real templates and static files.
func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "templates", "layout.html")); err == nil {
			return dir
		}
	}
	t.Fatal("could not locate the module root (templates/layout.html not found)")
	return ""
}

// newTestServer builds a Server over a fresh temporary database with the starter
// rows cleared, so each test controls its own accounts and categories.
func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()

	root := projectRoot(t)
	t.Setenv("WEB_ROOT", root)

	conn, err := db.Open(filepath.Join(t.TempDir(), "tr4cker.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	srv, err := New(conn)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	st := store.New(conn)
	for _, stmt := range []string{`DELETE FROM account`, `DELETE FROM category`} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("clear %q: %v", stmt, err)
		}
	}
	return srv, st
}

// get performs a plain page request.
func get(t *testing.T, srv *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// htmx performs a request that looks like it came from HTMX.
func htmx(t *testing.T, srv *Server, method, target string, form url.Values, hxTarget string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	if hxTarget != "" {
		req.Header.Set("HX-Target", hxTarget)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// seed builds a small known dataset.
func seed(t *testing.T, st *store.Store) {
	t.Helper()
	for _, a := range []struct {
		key     string
		balance float64
	}{{"BRI", 2500000}, {"Gopay", 500000}} {
		if _, err := st.CreateAccount(a.key, money.New(a.balance), ""); err != nil {
			t.Fatalf("seed account %s: %v", a.key, err)
		}
	}
	for _, c := range []struct {
		key    string
		bucket store.Bucket
	}{{"Food", store.BucketNeeds}, {"Fun", store.BucketWants}, {"Emergency", store.BucketSavings}} {
		if _, err := st.CreateCategory(c.key, c.bucket); err != nil {
			t.Fatalf("seed category %s: %v", c.key, err)
		}
	}
}

// TestAllPageTemplatesParse is the task 7.1 guard: every page template must
// parse and execute without a template error.
func TestAllPageTemplatesParse(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	targets := []struct {
		name   string
		target string
	}{
		{"dashboard", "/"},
		{"transactions", "/transactions"},
		{"transaction entry", "/transactions/new"},
		{"planning", "/planning"},
		{"accounts", "/accounts"},
		{"to buy", "/to-buy"},
	}

	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, srv, tc.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200 (body: %s)", tc.target, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "template error") {
				t.Errorf("GET %s rendered a template error", tc.target)
			}
			if !strings.Contains(body, "</html>") {
				t.Errorf("GET %s did not render the full shell", tc.target)
			}
		})
	}
}

// TestEveryPageRendersNavigation is the task 7.2 guard.
func TestEveryPageRendersNavigation(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	for _, target := range []string{"/", "/transactions", "/transactions/new", "/planning", "/accounts", "/to-buy"} {
		t.Run(target, func(t *testing.T) {
			body := get(t, srv, target).Body.String()
			for _, label := range []string{"Dashboard", "Transactions", "Planning", "To Buy", "Accounts"} {
				if !strings.Contains(body, ">"+label+"<") {
					t.Errorf("GET %s is missing the %q nav link", target, label)
				}
			}
			if !strings.Contains(body, `class="nav"`) {
				t.Errorf("GET %s is missing the nav element", target)
			}
		})
	}
}

func TestActiveNavItemIsMarked(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	tests := []struct {
		target string
		active string
	}{
		{"/", "dashboard"},
		{"/transactions", "transactions"},
		{"/planning", "planning"},
		{"/accounts", "accounts"},
		{"/to-buy", "tobuy"},
	}

	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			body := get(t, srv, tc.target).Body.String()
			if !strings.Contains(body, `nav-link is-active`) {
				t.Errorf("GET %s marked no nav item active", tc.target)
			}
			if !strings.Contains(body, `href="`+activeHref(tc.active)+`"`) {
				t.Errorf("GET %s did not link the active item %q", tc.target, tc.active)
			}
		})
	}
}

func activeHref(nav string) string {
	switch nav {
	case "dashboard":
		return "/"
	case "transactions":
		return "/transactions"
	case "planning":
		return "/planning"
	case "accounts":
		return "/accounts"
	case "tobuy":
		return "/to-buy"
	}
	return "/"
}

func TestDashboardShowsPeriodBalanceAndFigures(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	if _, err := st.SetPlan(period.Now(), "Food", money.New(1500000), ""); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(900000), Type: store.TypeExpense, Category: "Food",
		At:          period.Now().Start().Add(48 * time.Hour),
		AccountFrom: "BRI", AccountTo: db.External,
	}); err != nil {
		t.Fatalf("expense: %v", err)
	}

	body := get(t, srv, "/").Body.String()

	if !strings.Contains(body, period.Now().Label()) {
		t.Errorf("dashboard is missing the period label %q", period.Now().Label())
	}
	// The 900000 expense has already left BRI, so the balances are net.
	if !strings.Contains(body, "Rp 2,100,000") {
		t.Error("dashboard is missing the total balance Rp 2,100,000")
	}
	for _, want := range []string{"Rp 1,600,000", "Rp 500,000"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing the per-account balance %s", want)
		}
	}
	if !strings.Contains(body, "Rp 900,000") {
		t.Error("dashboard is missing the period spending Rp 900,000")
	}
	if !strings.Contains(body, "Rp 600,000") {
		t.Error("dashboard is missing the remaining planned amount Rp 600,000")
	}
}

func TestDashboardShowsNoPlanMessageWithoutPlans(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "No plan") {
		t.Error("dashboard should report that the period has no plan")
	}
}

func TestDashboardOmitsDecimalPartWhenZero(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	body := get(t, srv, "/").Body.String()
	if strings.Contains(body, "Rp 3,000,000.00") {
		t.Error("a whole total should render without a decimal part")
	}
}

func TestDashboardTotalEqualsTheListedBalances(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "Rp 3,000,000") {
		t.Fatal("expected a whole total of Rp 3,000,000")
	}
	if !strings.Contains(body, "Rp 2,500,000") || !strings.Contains(body, "Rp 500,000") {
		t.Error("the listed balances do not add up to the shown total")
	}
}

func TestSoftDeletedTransactionLeavesTheDashboard(t *testing.T) {
	srv, st := newTestServer(t)
	seed(t, st)

	created, err := st.CreateTransaction(store.NewTransaction{
		Amount: money.New(25000), Type: store.TypeExpense, Category: "Food",
		At: time.Now(), AccountFrom: "BRI", AccountTo: db.External,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(get(t, srv, "/").Body.String(), "Food") {
		t.Fatal("the transaction should appear on the dashboard")
	}

	if err := st.DeleteTransaction(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if strings.Contains(get(t, srv, "/").Body.String(), "Rp 25,000") {
		t.Error("a soft-deleted transaction still appears on the dashboard")
	}
}
