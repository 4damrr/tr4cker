// Package money holds the single representation of a monetary amount used
// across the application.
//
// Money is a float64 rounded to two decimal places. Every value is rounded when
// it is written and rounded again when it is formatted, per
// openspec/specs/database-design.md - Money Representation.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Amount is a monetary value in rupiah.
type Amount float64

// Errors returned by Parse.
var (
	ErrEmpty   = errors.New("money: amount is empty")
	ErrInvalid = errors.New("money: amount is not a number")
)

const (
	// Decimals is the number of decimal places every amount is rounded to.
	Decimals = 2

	scale = 100.0 // 10^Decimals
)

// Parse reads a user-typed amount and rounds it to two decimal places.
//
// The rounding is exact decimal arithmetic on the input text rather than
// arithmetic on a float, because "25000.505" is not representable as a
// float64: converting it first would round down to 25000.50 and the user
// would silently lose the half rupiah they typed.
func Parse(s string) (Amount, error) {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, " ", "")
	if clean == "" {
		return 0, ErrEmpty
	}
	r, ok := new(big.Rat).SetString(clean)
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrInvalid, s)
	}
	return Amount(roundRat(r)), nil
}

// New rounds v to two decimal places.
func New(v float64) Amount { return Amount(round(v)) }

// round returns v rounded to two decimal places, half away from zero. Negative
// zero is normalised to positive zero so it never formats as "-0".
func round(v float64) float64 {
	out := math.Round(v*scale) / scale
	if out == 0 {
		return 0
	}
	return out
}

// roundRat rounds an exact rational to two decimal places, half away from
// zero, and returns the nearest float64 to that value.
func roundRat(r *big.Rat) float64 {
	scaled := new(big.Rat).Mul(r, big.NewRat(100, 1))

	num := scaled.Num()
	den := scaled.Denom()
	negative := num.Sign() < 0

	quo, rem := new(big.Int).QuoRem(new(big.Int).Abs(num), den, new(big.Int))
	// Half away from zero: round up when the remainder is at least half.
	if new(big.Int).Lsh(rem, 1).Cmp(den) >= 0 {
		quo.Add(quo, big.NewInt(1))
	}
	if negative {
		quo.Neg(quo)
	}

	f, _ := new(big.Rat).SetFrac(quo, big.NewInt(100)).Float64()
	if f == 0 {
		return 0
	}
	return f
}

// Round rounds the amount to two decimal places.
func (a Amount) Round() Amount { return Amount(round(float64(a))) }

// Float returns the amount as a float64 for storage in a REAL column.
func (a Amount) Float() float64 { return round(float64(a)) }

// Add returns a+b rounded to two decimal places.
func (a Amount) Add(b Amount) Amount { return (a + b).Round() }

// Sub returns a-b rounded to two decimal places.
func (a Amount) Sub(b Amount) Amount { return (a - b).Round() }

// Neg returns -a rounded to two decimal places. It is how a recorded movement
// is reversed on update and soft-delete.
func (a Amount) Neg() Amount { return (-a).Round() }

// String formats the amount with thousands separators and at most two decimal
// places. The decimal part is omitted when it is zero, so a whole amount reads
// as "900,000" and a fractional one as "900,000.50".
func (a Amount) String() string {
	fixed := strconv.FormatFloat(float64(a.Round()), 'f', Decimals, 64)
	fixed = strings.TrimSuffix(fixed, "."+strings.Repeat("0", Decimals))

	negative := strings.HasPrefix(fixed, "-")
	body := strings.TrimPrefix(fixed, "-")

	whole, fraction, _ := strings.Cut(body, ".")

	var b strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(digit)
	}
	out := b.String()
	if fraction != "" {
		out += "." + fraction
	}
	if negative && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// IsZero reports whether the amount rounds to zero.
func (a Amount) IsZero() bool { return a.Round() == 0 }
