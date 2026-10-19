package money

import (
	"math"
	"testing"
)

func TestParseRoundsToTwoDecimals(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Amount
	}{
		{"whole amount", "25000", 25000},
		{"fractional amount is kept", "25000.50", 25000.50},
		{"cents are kept", "0.75", 0.75},
		{"third decimal rounds up", "0.756", 0.76},
		{"third decimal rounds up on a large amount", "25000.505", 25000.51},
		{"float noise collapses", "900000.0000000001", 900000},
		{"grouping separators are accepted", "1,500,000", 1500000},
		{"surrounding spaces are accepted", "  25000.50  ", 25000.50},
		{"negative amount", "-400000", -400000},
		{"negative fraction rounds away from zero", "-0.756", -0.76},
		{"zero", "0", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Parse(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrEmpty},
		{"only spaces", "   ", ErrEmpty},
		{"non-numeric", "abc", ErrInvalid},
		{"trailing letters", "25000abc", ErrInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse(%q) returned no error, want %v", tc.in, tc.want)
			}
			if !errorsIs(err, tc.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tc.in, err, tc.want)
			}
		})
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		in   Amount
		want string
	}{
		{"whole amount has no decimal part", New(900000.0000000001), "900,000"},
		{"fractional amount keeps two decimals", New(900000.5), "900,000.50"},
		{"grouping separators", New(1500000), "1,500,000"},
		{"negative remaining amount", New(-0.6), "-0.60"},
		{"large negative remaining amount", New(-400000), "-400,000"},
		{"zero", New(0), "0"},
		{"negative zero is normalised", Amount(math.Copysign(0, -1)), "0"},
		{"rounded on format", Amount(900000.0000000001), "900,000"},
		{"single decimal is padded", New(0.5), "0.50"},
		{"sub-cent is rounded away", New(0.756), "0.76"},
		{"four digit whole", New(7500), "7,500"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Errorf("Amount(%v).String() = %q, want %q", float64(tc.in), got, tc.want)
			}
		})
	}
}

func TestArithmeticRoundsToTwoDecimals(t *testing.T) {
	amount := New(100000)
	if got := amount.Sub(New(25000.50)); got != New(74999.50) {
		t.Errorf("100000 - 25000.50 = %v, want 74999.50", got)
	}
	if got := Amount(0).Sub(New(0.756)); got != New(-0.76) {
		t.Errorf("0 - 0.756 = %v, want -0.76", got)
	}
	if got := New(0).Add(New(0.33)).Add(New(0.33)).Add(New(0.33)); got != New(0.99) {
		t.Errorf("0 + 0.33 * 3 = %v, want 0.99", got)
	}
	if got := New(74999.50).Add(New(25000.50)); got != New(100000) {
		t.Errorf("74999.50 + 25000.50 = %v, want 100000", got)
	}
}

// TestParseRoundsDifferentlyFromFloatArithmetic documents why Parse does exact
// decimal arithmetic. Some values are not representable as float64, so rounding
// the float would drop the half the user typed; Parse reads the text instead.
func TestParseRoundsDifferentlyFromFloatArithmetic(t *testing.T) {
	const in = "0.145"

	parsed, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", in, err)
	}
	if parsed != New(0.15) {
		t.Errorf("Parse(%q) = %v, want 0.15", in, parsed)
	}
	if fromFloat := New(0.145); fromFloat == parsed {
		t.Errorf("New(0.145) = %v, expected it to differ from Parse(%q) = %v", fromFloat, in, parsed)
	}
}

func TestFloatIsRoundedForStorage(t *testing.T) {
	if got := Amount(25000.505).Float(); got != 25000.51 {
		t.Errorf("Amount(25000.505).Float() = %v, want 25000.51", got)
	}
	parsed, err := Parse("25000.505")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Float() != Amount(25000.505).Float() {
		t.Errorf("Parse and float rounding disagree: %v vs %v", parsed.Float(), Amount(25000.505).Float())
	}
	if got := New(0.756).Float(); got != 0.76 {
		t.Errorf("New(0.756).Float() = %v, want 0.76", got)
	}
}

func TestIsZero(t *testing.T) {
	if !New(0.001).IsZero() {
		t.Error("New(0.001).IsZero() = false, want true (rounds to zero)")
	}
	if New(0.01).IsZero() {
		t.Error("New(0.01).IsZero() = true, want false")
	}
}
