package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "tr4cker.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestOpenCreatesAllFiveDomainTables(t *testing.T) {
	conn := openTest(t)

	want := []string{"transactions", "category", "account", "planning", "to_buy"}
	for _, table := range want {
		var name string
		err := conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

func TestOpenDropsTheBoilerplateExpensesTable(t *testing.T) {
	conn := openTest(t)

	var name string
	err := conn.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'expenses'`,
	).Scan(&name)
	if err == nil {
		t.Error("expenses table still present after migration")
	}
}

func TestMoneyColumnsAreReal(t *testing.T) {
	conn := openTest(t)

	tests := []struct {
		table string
		cols  []string
	}{
		{"transactions", []string{"amount"}},
		{"account", []string{"balance"}},
		{"planning", []string{"amount"}},
		{"to_buy", []string{"price"}},
	}

	for _, tc := range tests {
		rows, err := conn.Query(`SELECT name, type FROM pragma_table_info(?)`, tc.table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", tc.table, err)
		}
		types := map[string]string{}
		for rows.Next() {
			var name, typ string
			if err := rows.Scan(&name, &typ); err != nil {
				t.Fatalf("scan: %v", err)
			}
			types[name] = typ
		}
		_ = rows.Close()

		for _, col := range tc.cols {
			if got := types[col]; got != "REAL" {
				t.Errorf("%s.%s type = %q, want REAL", tc.table, col, got)
			}
		}
	}
}

func TestCategoryColumnsAreForeignKeys(t *testing.T) {
	conn := openTest(t)

	tests := []struct {
		table string
		col   string
	}{
		{"transactions", "category"},
		{"planning", "category"},
		{"to_buy", "category"},
	}

	for _, tc := range tests {
		rows, err := conn.Query(`SELECT "from", "table" FROM pragma_foreign_key_list(?)`, tc.table)
		if err != nil {
			t.Fatalf("pragma_foreign_key_list(%s): %v", tc.table, err)
		}
		found := false
		for rows.Next() {
			var from, target string
			if err := rows.Scan(&from, &target); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if from == tc.col && target == "category" {
				found = true
			}
		}
		_ = rows.Close()

		if !found {
			t.Errorf("%s.%s has no foreign key to category", tc.table, tc.col)
		}
	}
}

func TestAccountColumnsHaveNoForeignKey(t *testing.T) {
	conn := openTest(t)

	for _, col := range []string{"account_from", "account_to"} {
		rows, err := conn.Query(`SELECT "from", "table" FROM pragma_foreign_key_list('transactions')`)
		if err != nil {
			t.Fatalf("pragma_foreign_key_list: %v", err)
		}
		for rows.Next() {
			var from, target string
			if err := rows.Scan(&from, &target); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if from == col {
				t.Errorf("transactions.%s unexpectedly has a foreign key to %s", col, target)
			}
		}
		_ = rows.Close()
	}
}

func TestSoftDeleteColumnsExist(t *testing.T) {
	conn := openTest(t)

	for _, table := range []string{"transactions", "category", "account"} {
		rows, err := conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", table, err)
		}
		found := false
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if name == "deleted_at" {
				found = true
			}
		}
		_ = rows.Close()

		if !found {
			t.Errorf("%s has no deleted_at column", table)
		}
	}
}

func TestPlanningIsUniquePerYearMonthCategory(t *testing.T) {
	conn := openTest(t)

	_, err := conn.Exec(
		`INSERT INTO planning (month, year, category, amount) VALUES (11, 2026, 'Food', 1500000)`,
	)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO planning (month, year, category, amount) VALUES (11, 2026, 'Food', 500000)`,
	)
	if err == nil {
		t.Error("second insert for the same year/month/category succeeded, want unique violation")
	}
}

func TestStarterDataCoversEveryBucket(t *testing.T) {
	conn := openTest(t)

	rows, err := conn.Query(`SELECT buckets, COUNT(*) FROM category WHERE deleted_at IS NULL GROUP BY buckets`)
	if err != nil {
		t.Fatalf("query seeded categories: %v", err)
	}
	defer rows.Close()

	got := map[string]int{}
	for rows.Next() {
		var bucket string
		var n int
		if err := rows.Scan(&bucket, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[bucket] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	for _, bucket := range []string{"needs", "wants", "savings"} {
		if got[bucket] == 0 {
			t.Errorf("no seeded categories for bucket %q", bucket)
		}
	}
}

func TestStarterAccountsExist(t *testing.T) {
	conn := openTest(t)

	for _, key := range []string{"BRI", "Cash", "Gopay", "ShopeePay"} {
		var balance float64
		err := conn.QueryRow(`SELECT balance FROM account WHERE key = ?`, key).Scan(&balance)
		if err != nil {
			t.Errorf("seeded account %q missing: %v", key, err)
			continue
		}
		if balance != 0 {
			t.Errorf("seeded account %q balance = %v, want 0", key, balance)
		}
	}
}

func TestExternalIsNotAnAccountRow(t *testing.T) {
	conn := openTest(t)

	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM account WHERE key = ?`, External).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Error("external must be a reserved literal, not a row in account")
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tr4cker.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := first.Exec(
		`INSERT INTO account (key, balance) VALUES ('Custom', 1000)`,
	); err != nil {
		t.Fatalf("insert custom account: %v", err)
	}
	_ = first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer second.Close()

	var n int
	if err := second.QueryRow(`SELECT COUNT(*) FROM account WHERE key = 'Custom'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Error("reopening the database lost data")
	}
}
