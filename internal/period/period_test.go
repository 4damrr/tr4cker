package period

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.Local)
}

func TestForMapsDatesToPeriods(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want Period
	}{
		{"the 25th opens a new period", date(2026, time.October, 25), Period{2026, 11}},
		{"the 24th closes a period", date(2026, time.October, 24), Period{2026, 10}},
		{"a 31 day month", date(2026, time.January, 25), Period{2026, 2}},
		{"the 31st of a 31 day month is already in the next period", date(2026, time.January, 31), Period{2026, 2}},
		{"a 30 day month", date(2026, time.April, 25), Period{2026, 5}},
		{"the 30th of a 30 day month is already in the next period", date(2026, time.April, 30), Period{2026, 5}},
		{"february in a common year", date(2026, time.February, 25), Period{2026, 3}},
		{"february in a leap year", date(2028, time.February, 25), Period{2028, 3}},
		{"december rolls into january", date(2026, time.December, 25), Period{2027, 1}},
		{"december before the 25th stays put", date(2026, time.December, 24), Period{2026, 12}},
		{"january before the 25th stays put", date(2027, time.January, 10), Period{2027, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := For(tc.in); got != tc.want {
				t.Errorf("For(%s) = %+v, want %+v", tc.in.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestPeriodBounds(t *testing.T) {
	p := Period{2026, 11}

	start := p.Start()
	if got := start.Format("2006-01-02 15:04:05"); got != "2026-10-25 00:00:00" {
		t.Errorf("Start() = %s, want 2026-10-25 00:00:00", got)
	}

	end := p.End()
	if got := end.Format("2006-01-02 15:04:05"); got != "2026-11-25 00:00:00" {
		t.Errorf("End() = %s, want 2026-11-25 00:00:00", got)
	}

	last := p.LastDay()
	if got := last.Format("2006-01-02 15:04:05"); got != "2026-11-24 00:00:00" {
		t.Errorf("LastDay() = %s, want 2026-11-24 00:00:00", got)
	}
}

func TestPeriodContains(t *testing.T) {
	p := Period{2026, 11}

	tests := []struct {
		name string
		in   time.Time
		want bool
	}{
		{"first instant of the period", date(2026, time.October, 25), true},
		{"mid period", date(2026, time.November, 10), true},
		{"last calendar day of the period", date(2026, time.November, 24), true},
		{"the 25th is the next period", date(2026, time.November, 25), false},
		{"the day before the period starts", date(2026, time.October, 24), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.Contains(tc.in); got != tc.want {
				t.Errorf("Contains(%s) = %v, want %v", tc.in.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestPeriodNavigation(t *testing.T) {
	tests := []struct {
		name     string
		in       Period
		wantNext Period
		wantPrev Period
	}{
		{"october ending period", Period{2026, 10}, Period{2026, 11}, Period{2026, 9}},
		{"november ending period", Period{2026, 11}, Period{2026, 12}, Period{2026, 10}},
		{"december rolls over", Period{2026, 12}, Period{2027, 1}, Period{2026, 11}},
		{"january after rollover", Period{2027, 1}, Period{2027, 2}, Period{2026, 12}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Next(); got != tc.wantNext {
				t.Errorf("Next() = %+v, want %+v", got, tc.wantNext)
			}
			if got := tc.in.Prev(); got != tc.wantPrev {
				t.Errorf("Prev() = %+v, want %+v", got, tc.wantPrev)
			}
		})
	}
}

func TestNewNormalisesOutOfRangeMonth(t *testing.T) {
	if got := New(2026, 13); got != (Period{2027, 1}) {
		t.Errorf("New(2026, 13) = %+v, want {2027 1}", got)
	}
	if got := New(2026, 0); got != (Period{2025, 12}) {
		t.Errorf("New(2026, 0) = %+v, want {2025 12}", got)
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name string
		in   Period
		want string
	}{
		{"october to november", Period{2026, 11}, "25 Oct \u2013 24 Nov 2026"},
		{"september to october", Period{2026, 10}, "25 Sep \u2013 24 Oct 2026"},
		{"december to january", Period{2027, 1}, "25 Dec \u2013 24 Jan 2027"},
		{"january to february", Period{2026, 2}, "25 Jan \u2013 24 Feb 2026"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Label(); got != tc.want {
				t.Errorf("Label() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNowIsInsideItsOwnPeriod(t *testing.T) {
	if !Now().Contains(time.Now()) {
		t.Error("Now() does not contain the current time")
	}
}
