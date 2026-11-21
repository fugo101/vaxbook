package engine

import (
	"fmt"

	"github.com/fugo101/vaxbook/ruleset"
)

// Date is a civil calendar date with no time and no zone. The zero value is
// "no date" (IsZero) and cannot be encoded.
//
// Internally it is a day ordinal (0001-01-01 = 1) computed with integer
// civil-calendar arithmetic, so it never depends on the time package's zone
// or normalisation rules.
type Date struct {
	days int32
}

// civilEpoch is daysFromCivil(1, 1, 1) - 1, so 0001-01-01 has ordinal 1.
const civilEpoch = -719163

// daysFromCivil returns the number of days from 1970-01-01 to y-m-d in the
// proleptic Gregorian calendar (H. Hinnant's days_from_civil).
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := y
	if era < 0 {
		era -= 399
	}
	era /= 400
	yoe := y - era*400
	mp := m + 9
	if m > 2 {
		mp = m - 3
	}
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// civilFromDays is the inverse of daysFromCivil.
func civilFromDays(z int) (y, m, d int) {
	z += 719468
	era := z
	if era < 0 {
		era -= 146096
	}
	era /= 146097
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if mp >= 10 {
		m = mp - 9
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func lastDay(y, m int) int {
	switch m {
	case 2:
		if isLeap(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// NewDate returns the date y-m-d. Years run from 1 to 9999.
func NewDate(y, m, d int) (Date, error) {
	if y < 1 || y > 9999 || m < 1 || m > 12 || d < 1 || d > lastDay(y, m) {
		return Date{}, fmt.Errorf("invalid date %04d-%02d-%02d", y, m, d)
	}
	return fromCivil(y, m, d), nil
}

func fromCivil(y, m, d int) Date {
	return Date{days: int32(daysFromCivil(y, m, d) - civilEpoch)} //nolint:gosec // years 1..9999 fit in int32
}

// ParseDate parses exactly "YYYY-MM-DD": ten ASCII characters, no sign, no
// time, no surrounding space, and a day that exists in the calendar.
func ParseDate(s string) (Date, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return Date{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	num := func(part string) (int, bool) {
		n := 0
		for i := range len(part) {
			c := part[i]
			if c < '0' || c > '9' {
				return 0, false
			}
			n = n*10 + int(c-'0')
		}
		return n, true
	}
	y, ok1 := num(s[0:4])
	m, ok2 := num(s[5:7])
	day, ok3 := num(s[8:10])
	if !ok1 || !ok2 || !ok3 {
		return Date{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	dt, err := NewDate(y, m, day)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: no such day", s)
	}
	return dt, nil
}

// IsZero reports whether d is the zero Date ("no date").
func (d Date) IsZero() bool { return d.days == 0 }

// YMD returns the year, month (1–12) and day of month.
func (d Date) YMD() (y, m, day int) { return civilFromDays(int(d.days) + civilEpoch) }

// String returns "YYYY-MM-DD", or "0000-00-00" for the zero Date.
func (d Date) String() string {
	if d.IsZero() {
		return "0000-00-00"
	}
	y, m, day := d.YMD()
	return fmt.Sprintf("%04d-%02d-%02d", y, m, day)
}

// AddDays returns d + n days.
func (d Date) AddDays(n int) Date { return Date{days: d.days + int32(n)} } //nolint:gosec // spans are capped at 999 units

// AddMonths returns d + n months, keeping the day of month and clamping it to
// the last day of the target month (2026-01-31 + 1 month = 2026-02-28; SAD
// §5.1).
//
// Month addition is not associative: AddMonths(AddMonths(d, a), b) can be
// earlier than AddMonths(d, a+b) when the intermediate month is shorter
// (2026-01-31 +1m +1m = 2026-03-28, +2m = 2026-03-31). Always add a span to
// its anchor in one step.
func (d Date) AddMonths(n int) Date {
	y, m, day := d.YMD()
	total := y*12 + (m - 1) + n
	ty, tm := floorDiv(total, 12), total-floorDiv(total, 12)*12+1
	return fromCivil(ty, tm, min(day, lastDay(ty, tm)))
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// AddSpan returns d + s. A week is 7 days and a year is 12 months.
func (d Date) AddSpan(s ruleset.Span) Date {
	switch s.Unit {
	case ruleset.Days:
		return d.AddDays(s.N)
	case ruleset.Weeks:
		return d.AddDays(7 * s.N)
	case ruleset.Months:
		return d.AddMonths(s.N)
	case ruleset.Years:
		return d.AddMonths(12 * s.N)
	}
	panic(fmt.Sprintf("engine: invalid span unit %q", rune(s.Unit)))
}

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int {
	switch {
	case d.days < o.days:
		return -1
	case d.days > o.days:
		return 1
	}
	return 0
}

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.days < o.days }

// After reports whether d is after o.
func (d Date) After(o Date) bool { return d.days > o.days }

// MarshalText implements encoding.TextMarshaler. The zero Date is an error,
// so a bug never emits an empty date.
func (d Date) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return nil, fmt.Errorf("engine: cannot encode the zero Date")
	}
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler with ParseDate.
func (d *Date) UnmarshalText(b []byte) error {
	p, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = p
	return nil
}

func maxDate(a, b Date) Date {
	if a.After(b) {
		return a
	}
	return b
}

func minDate(a, b Date) Date {
	if a.Before(b) {
		return a
	}
	return b
}
