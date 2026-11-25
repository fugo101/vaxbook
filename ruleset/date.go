package ruleset

import (
	"cmp"
	"fmt"
)

// Date is a civil calendar date, such as a series' validFrom or validTo
// (SAD §5.2). Its text form is exactly "YYYY-MM-DD", years 0001 to 9999, a
// day that exists in the proleptic Gregorian calendar: what the schema's
// $defs/Date pattern accepts. The zero Date stands for a missing value, and
// a Go-built Date that is not a calendar day ({2025, 2, 30}, month 13) is
// invalid; Validate reports both. Added in S4c.
//
// The engine converts a Date to its own date type once, when it compiles a
// snapshot. ruleset cannot import engine, so the two parsers are kept equal
// by a test in package engine.
type Date struct {
	Year, Month, Day int
}

// ParseDate parses the text form of a Date: ten ASCII characters, no sign,
// no time, no surrounding space.
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
	d, ok3 := num(s[8:10])
	if !ok1 || !ok2 || !ok3 {
		return Date{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	dt := Date{Year: y, Month: m, Day: d}
	if !dt.valid() {
		return Date{}, fmt.Errorf("invalid date %q: no such day", s)
	}
	return dt, nil
}

// valid reports whether d is a calendar day in years 1 to 9999.
func (d Date) valid() bool {
	return d.Year >= 1 && d.Year <= 9999 && d.Month >= 1 && d.Month <= 12 && d.Day >= 1 && d.Day <= daysIn(d.Year, d.Month)
}

// daysIn returns the number of days of month m of year y (Gregorian leap
// years). It duplicates the engine's month length, which ruleset cannot
// import.
func daysIn(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// compare orders two dates: negative when d is before o, zero when equal.
func (d Date) compare(o Date) int {
	return cmp.Or(cmp.Compare(d.Year, o.Year), cmp.Compare(d.Month, o.Month), cmp.Compare(d.Day, o.Day))
}

// String returns the text form, or a diagnostic form for an invalid Date.
func (d Date) String() string {
	if !d.valid() {
		return fmt.Sprintf("Date{Year:%d, Month:%d, Day:%d}", d.Year, d.Month, d.Day)
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// MarshalText implements encoding.TextMarshaler. An invalid Date (including
// the zero value) is an error, so a bug never emits a malformed date.
func (d Date) MarshalText() ([]byte, error) {
	if !d.valid() {
		return nil, fmt.Errorf("invalid date %s", d)
	}
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Date) UnmarshalText(b []byte) error {
	p, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = p
	return nil
}
