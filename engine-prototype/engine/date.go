// PROTOTYPE — throwaway. Civil-date helpers for the VaxBook engine prototype.
package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a civil date (no time, no zone). Internally UTC midnight.
type Date struct{ t time.Time }

func D(y, m, d int) Date { return Date{time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)} }

func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

func (d Date) IsZero() bool            { return d.t.IsZero() }
func (d Date) String() string          { return d.t.Format("2006-01-02") }
func (d Date) Before(o Date) bool      { return d.t.Before(o.t) }
func (d Date) After(o Date) bool       { return d.t.After(o.t) }
func (d Date) AddDays(n int) Date      { return Date{d.t.AddDate(0, 0, n)} }
func (d Date) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }
func (d *Date) UnmarshalJSON(b []byte) error {
	p, err := ParseDate(strings.Trim(string(b), `"`))
	if err != nil {
		return err
	}
	*d = p
	return nil
}

// AddMonths keeps the day of month; if the target month is shorter, clamps to its last day.
// 2026-01-31 + 1 month = 2026-02-28.
func (d Date) AddMonths(n int) Date {
	y, m, day := d.t.Date()
	total := int(m) - 1 + n
	q := floorDiv(total, 12)
	y += q
	mm := total - q*12 + 1
	if last := daysIn(y, mm); day > last {
		day = last
	}
	return D(y, mm, day)
}

func daysIn(y, m int) int { return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day() }

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func MaxD(a, b Date) Date {
	if a.After(b) {
		return a
	}
	return b
}

func MinD(a, b Date) Date {
	if a.Before(b) {
		return a
	}
	return b
}

// Span is an age or interval such as "6w", "2m", "15y", "30d". Weeks = 7 days.
type Span struct {
	N    int
	Unit byte // d, w, m, y
}

func ParseSpan(s string) (Span, error) {
	if len(s) < 2 {
		return Span{}, fmt.Errorf("bad span %q", s)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return Span{}, fmt.Errorf("bad span %q", s)
	}
	u := s[len(s)-1]
	switch u {
	case 'd', 'w', 'm', 'y':
	default:
		return Span{}, fmt.Errorf("bad span unit %q", s)
	}
	return Span{n, u}, nil
}

func (s Span) AddTo(d Date) Date {
	switch s.Unit {
	case 'd':
		return d.AddDays(s.N)
	case 'w':
		return d.AddDays(7 * s.N)
	case 'm':
		return d.AddMonths(s.N)
	default:
		return d.AddMonths(12 * s.N)
	}
}

func (s Span) String() string { return strconv.Itoa(s.N) + string(s.Unit) }
func (s Span) MarshalJSON() ([]byte, error) { return []byte(`"` + s.String() + `"`), nil }
func (s *Span) UnmarshalJSON(b []byte) error {
	p, err := ParseSpan(strings.Trim(string(b), `"`))
	if err != nil {
		return err
	}
	*s = p
	return nil
}

// Precision of a remembered date (F04.2 levels 1–4; "none" for an omitted birth date).
type Precision string

const (
	PDay     Precision = "day"     // level 1
	PMonth   Precision = "month"   // level 2
	PYear    Precision = "year"    // level 3
	PUnknown Precision = "unknown" // level 4: given, time unknown
	PNone    Precision = "none"    // birth date not entered
)

// FuzzyDate is a date with precision. Value: "2026-03-15" | "2026-03" | "2026".
type FuzzyDate struct {
	Precision Precision `json:"precision"`
	Value     string    `json:"value,omitempty"`
}

// Range returns the closed interval [lo, hi] the date may fall in.
func (f FuzzyDate) Range() (lo, hi Date, ok bool) {
	switch f.Precision {
	case PDay:
		d, err := ParseDate(f.Value)
		if err != nil {
			return
		}
		return d, d, true
	case PMonth:
		t, err := time.Parse("2006-01", f.Value)
		if err != nil {
			return
		}
		y, m := t.Year(), int(t.Month())
		return D(y, m, 1), D(y, m, daysIn(y, m)), true
	case PYear:
		y, err := strconv.Atoi(f.Value)
		if err != nil {
			return
		}
		return D(y, 1, 1), D(y, 12, 31), true
	}
	return
}
