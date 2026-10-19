// Package store holds the repositories that read and write tr4cker's SQLite
// tables. Handlers in internal/web depend on this package; it never depends on
// them.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/4damrr/tr4cker/internal/money"
	"github.com/4damrr/tr4cker/internal/period"
)

// Errors returned by the repositories. Handlers map these onto HTTP status
// codes, so they stay coarse on purpose.
var (
	ErrNotFound  = errors.New("store: not found")
	ErrDuplicate = errors.New("store: already exists")
	ErrInvalid   = errors.New("store: invalid")
)

// Store is the entry point to every repository.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by conn.
func New(conn *sql.DB) *Store { return &Store{db: conn} }

// DB exposes the underlying handle for callers that need their own transaction.
func (s *Store) DB() *sql.DB { return s.db }

// FormatTime renders t in the layout used by every timestamp column.
func FormatTime(t time.Time) string { return t.Format(timeFormat) }

// ParseTime reads a timestamp column back into a time.Time.
func ParseTime(s string) (time.Time, error) {
	return time.ParseInLocation(timeFormat, s, time.Local)
}

// timeFormat must match db.TimeFormat; it is duplicated here so store does not
// depend on the db package. TestTimeFormatMatches guards the two.
const timeFormat = "2006-01-02 15:04:05"

// bounds renders a period's inclusive start and exclusive end for use in a
// SQL range predicate.
func bounds(p period.Period) (start, end string) {
	return FormatTime(p.Start()), FormatTime(p.End())
}

// inClause renders a set of filter values as a placeholder list, or "NULL"
// when the set is empty so the caller can omit the predicate entirely.
func inClause(values []string) (string, []any) {
	if len(values) == 0 {
		return "NULL", nil
	}
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = v
	}
	return "(" + strings.Join(placeholders, ",") + ")", args
}

// amountColumns renders a SELECT list of money columns, rounding each in SQL so
// aggregates arrive already rounded to two decimals.
func amountColumns(cols ...string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = fmt.Sprintf("ROUND(COALESCE(%s, 0), 2) AS %s", c, c)
	}
	return strings.Join(out, ", ")
}

// coerceAmount rounds a value read from a REAL column.
func coerceAmount(v float64) money.Amount { return money.New(v) }

// nowString is the current local time in the timestamp layout.
func nowString() string { return time.Now().Format(timeFormat) }
