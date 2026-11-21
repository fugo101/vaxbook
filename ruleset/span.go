package ruleset

import (
	"fmt"
	"strconv"
)

// SpanUnit is the unit of a Span.
type SpanUnit byte

// Span units. A week is 7 days and a year is 12 months (SAD §5.1).
const (
	Days   SpanUnit = 'd'
	Weeks  SpanUnit = 'w'
	Months SpanUnit = 'm'
	Years  SpanUnit = 'y'
)

// MaxSpanN is the largest count a Span may hold. It keeps every computed
// date within four-digit years.
const MaxSpanN = 999

// Span is an age, interval, offset or buffer in one unit, written as
// "<n><unit>" such as "6w", "2m", "9y" or "0d" (n in 0..999, no leading
// zeros).
type Span struct {
	N    int
	Unit SpanUnit
}

// ParseSpan parses the compact text form of a Span.
func ParseSpan(s string) (Span, error) {
	if len(s) < 2 || len(s) > 4 {
		return Span{}, fmt.Errorf("invalid span %q", s)
	}
	digits, unit := s[:len(s)-1], SpanUnit(s[len(s)-1])
	if !unit.valid() {
		return Span{}, fmt.Errorf("invalid span %q: unit must be d, w, m or y", s)
	}
	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return Span{}, fmt.Errorf("invalid span %q", s)
		}
	}
	if len(digits) > 1 && digits[0] == '0' {
		return Span{}, fmt.Errorf("invalid span %q: leading zero", s)
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return Span{}, fmt.Errorf("invalid span %q", s)
	}
	return Span{N: n, Unit: unit}, nil
}

func (u SpanUnit) valid() bool {
	switch u {
	case Days, Weeks, Months, Years:
		return true
	}
	return false
}

// String returns the compact text form, or a diagnostic form for an
// invalid Span.
func (s Span) String() string {
	if !s.valid() {
		return fmt.Sprintf("Span{N:%d, Unit:%q}", s.N, rune(s.Unit))
	}
	return strconv.Itoa(s.N) + string(rune(s.Unit))
}

func (s Span) valid() bool { return s.Unit.valid() && s.N >= 0 && s.N <= MaxSpanN }

// MarshalText implements encoding.TextMarshaler. An invalid Span (including
// the zero value) is an error, so a bug never emits a malformed span.
func (s Span) MarshalText() ([]byte, error) {
	if !s.valid() {
		return nil, fmt.Errorf("invalid span %s", s)
	}
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Span) UnmarshalText(b []byte) error {
	p, err := ParseSpan(string(b))
	if err != nil {
		return err
	}
	*s = p
	return nil
}
