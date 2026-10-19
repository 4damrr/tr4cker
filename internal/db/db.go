// Package db opens and migrates the SQLite database used by tr4cker.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// TimeFormat is the layout used by every timestamp column: trx_time,
// created_at and deleted_at. It sorts and compares correctly as text, so
// range predicates against it work without parsing, and it matches the
// bounds produced by internal/period.
const TimeFormat = "2006-01-02 15:04:05"

// External is the reserved account key standing for money outside the user's
// tracked accounts. It is never a row in account; see design.md.
const External = "external"

// Open opens the SQLite database at path, creating the directory, applying the
// schema and seeding starter data when needed.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// One connection is what makes a multi-statement sql.Tx safe here without
	// a busy-retry layer.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

// statements is applied in order. Money columns are REAL and rounded to two
// decimals by the write paths, per openspec/specs/database-design.md.
//
// account_from and account_to deliberately carry no foreign key: `external`
// is a reserved literal that is never a row in account. Referential validity
// for accounts is enforced in internal/store instead.
//
// The expenses table from the boilerplate is dropped rather than migrated;
// see design.md - Migration Plan.
var statements = []string{
	`CREATE TABLE IF NOT EXISTS category (
		key        TEXT PRIMARY KEY,
		buckets    TEXT NOT NULL CHECK (buckets IN ('needs','wants','savings')),
		deleted_at TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS account (
		key        TEXT PRIMARY KEY,
		balance    REAL NOT NULL DEFAULT 0,
		notes      TEXT,
		deleted_at TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS transactions (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		amount        REAL NOT NULL,
		type          TEXT NOT NULL CHECK (type IN ('income','expense','transfer')),
		category      TEXT NOT NULL REFERENCES category(key),
		trx_time      TEXT NOT NULL,
		is_investment INTEGER NOT NULL DEFAULT 0 CHECK (is_investment IN (0,1)),
		account_from  TEXT NOT NULL,
		account_to    TEXT NOT NULL,
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		deleted_at    TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS planning (
		month    INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
		year     INTEGER NOT NULL,
		category TEXT NOT NULL REFERENCES category(key),
		amount   REAL NOT NULL DEFAULT 0,
		notes    TEXT,
		UNIQUE (year, month, category)
	)`,
	`CREATE TABLE IF NOT EXISTS to_buy (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT NOT NULL REFERENCES category(key),
		name     TEXT NOT NULL,
		priority INTEGER NOT NULL CHECK (priority IN (1,2,3)),
		qty      INTEGER NOT NULL DEFAULT 1,
		price    REAL NOT NULL DEFAULT 0,
		url      TEXT,
		notes    TEXT,
		status   TEXT NOT NULL DEFAULT 'to_buy' CHECK (status IN ('to_buy','bought','canceled'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_trx_trx_time ON transactions (trx_time)`,
	`CREATE INDEX IF NOT EXISTS idx_trx_category ON transactions (category)`,
	`CREATE INDEX IF NOT EXISTS idx_trx_account_from ON transactions (account_from)`,
	`CREATE INDEX IF NOT EXISTS idx_trx_account_to ON transactions (account_to)`,
	`CREATE INDEX IF NOT EXISTS idx_trx_deleted_at ON transactions (deleted_at)`,
	`CREATE INDEX IF NOT EXISTS idx_planning_period ON planning (year, month)`,
	`CREATE INDEX IF NOT EXISTS idx_to_buy_status ON to_buy (status)`,
	`DROP TABLE IF EXISTS expenses`,
}

var starterCategories = []struct{ key, bucket string }{
	{"Food", "needs"},
	{"Transport", "needs"},
	{"Bills", "needs"},
	{"Entertainment", "wants"},
	{"Shopping", "wants"},
	{"Emergency", "savings"},
	{"Investment", "savings"},
}

var starterAccounts = []struct{ key, notes string }{
	{"BRI", "primary bank account"},
	{"Cash", "physical cash"},
	{"Gopay", ""},
	{"ShopeePay", ""},
}

func migrate(conn *sql.DB) error {
	for _, stmt := range statements {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	for _, c := range starterCategories {
		if _, err := conn.Exec(
			`INSERT OR IGNORE INTO category (key, buckets) VALUES (?, ?)`, c.key, c.bucket,
		); err != nil {
			return fmt.Errorf("seed category %s: %w", c.key, err)
		}
	}
	for _, a := range starterAccounts {
		if _, err := conn.Exec(
			`INSERT OR IGNORE INTO account (key, balance, notes) VALUES (?, 0, ?)`, a.key, a.notes,
		); err != nil {
			return fmt.Errorf("seed account %s: %w", a.key, err)
		}
	}
	return nil
}
