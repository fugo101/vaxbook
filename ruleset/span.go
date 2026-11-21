package ruleset

import (
	"fmt"
	"strconv"
)

// SpanUnit is the unit of a Span: its ISO 8601 duration designator.
type SpanUnit byte

// Span units. A week is 7 days and a year is 12 months (SAD §5.1).
const (
	Days   SpanUnit = 'D'
	Weeks  SpanUnit = 'W'
	Months SpanUnit = 'M'
	Years  SpanUnit = 'Y'
)

// MaxSpanN is the largest count a Span may hold. It keeps every computed
// date within four-digit years.
const MaxSpanN = 999

// Span is an age, interval, offset or buffer in one unit, written as an
// ISO 8601 duration with one designator: "P6W", "P2M", "P9Y" or "P0D" (n in
// 0..999, no leading zeros, upper case). Months clamp the day of month to
// the month end. Compound durations ("P1Y6M") are not accepted yet: no
// sourced rule needs one, and an exclusive maximum age such as "14 weeks 6
// days" is written as its exclusive bound "P15W".
type Span struct {
	N    int
	Unit SpanUnit
}

// ParseSpan parses the text form of a Span.
func ParseSpan(s string) (Span, error) {
	if len(s) < 3 || len(s) > 5 || s[0] != 'P' {
		return Span{}, fmt.Errorf("invalid span %q: want P<n>D, P<n>W, P<n>M or P<n>Y", s)
	}
	digits, unit := s[1:len(s)-1], SpanUnit(s[len(s)-1])
	if !unit.valid() {
		return Span{}, fmt.Errorf("invalid span %q: designator must be D, W, M or Y", s)
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

// String returns the text form, or a diagnostic form for an invalid Span.
func (s Span) String() string {
	if !s.valid() {
		return fmt.Sprintf("Span{N:%d, Unit:%q}", s.N, rune(s.Unit))
	}
	return "P" + strconv.Itoa(s.N) + string(rune(s.Unit))
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
