// Package period maps calendar dates onto the application's financial periods.
//
// A period runs from the 25th of one month through the 24th of the next and is
// labelled, and stored, by the month it ends in: "25 Oct - 24 Nov 2026" is
// {Year: 2026, Month: 11}. See design.md.
package period

import (
	"fmt"
	"time"
)

// StartDay is the first day of a new financial period.
const StartDay = 25

// EndDay is the last day of a financial period.
const EndDay = 24

// Period is a financial period, identified by the calendar month it ends in.
type Period struct {
	Year  int
	Month int
}

// For returns the financial period containing t.
func For(t time.Time) Period {
	if t.Day() >= StartDay {
		next := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
		return Period{Year: next.Year(), Month: int(next.Month())}
	}
	return Period{Year: t.Year(), Month: int(t.Month())}
}

// Now returns the financial period containing the current time.
func Now() Period { return For(time.Now()) }

// New returns the period ending in the given calendar month. Month is
// normalised into range, so New(2026, 13) is the period ending in January 2027.
func New(year, month int) Period {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	return Period{Year: t.Year(), Month: int(t.Month())}
}

// Start returns the first instant of the period: the 25th of the previous
// month at 00:00:00. time.Date normalises month 0 to December of the year
// before, which is what carries a January period across the year boundary.
func (p Period) Start() time.Time {
	return time.Date(p.Year, time.Month(p.Month)-1, StartDay, 0, 0, 0, 0, time.Local)
}

// End returns the exclusive upper bound of the period: the 25th of the period's
// month at 00:00:00, which is the first instant of the next period.
func (p Period) End() time.Time {
	return p.Start().AddDate(0, 1, 0)
}

// LastDay returns the final calendar day of the period: the 24th of the
// period's month.
func (p Period) LastDay() time.Time { return p.End().AddDate(0, 0, -1) }

// Contains reports whether t falls inside the period.
func (p Period) Contains(t time.Time) bool {
	return !t.Before(p.Start()) && t.Before(p.End())
}

// Next returns the following period.
func (p Period) Next() Period { return New(p.Year, p.Month+1) }

// Prev returns the preceding period.
func (p Period) Prev() Period { return New(p.Year, p.Month-1) }

// IsZero reports whether the period is unset.
func (p Period) IsZero() bool { return p.Year == 0 && p.Month == 0 }

// Label renders the period the way the user reads it, for example
// "25 Oct - 24 Nov 2026".
func (p Period) Label() string {
	start := p.Start()
	last := p.LastDay()
	return fmt.Sprintf("%d %s %s %d %s %d",
		StartDay, start.Format("Jan"), sep,
		last.Day(), last.Format("Jan"),
		p.Year,
	)
}

// sep separates the two halves of a period label.
const sep = "\u2013"
