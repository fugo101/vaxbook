package engine

import (
	"fmt"
)

// Precision is how precisely a user remembers a date (GLOSSARY: Date
// precision, F04.2).
type Precision string

// Precisions. Day, Month and Year are evaluable for both fields since S2;
// Unknown is a dose-record level and None a birth-date level.
const (
	Day     Precision = "day"     // level 1: exact day
	Month   Precision = "month"   // level 2: month and year
	Year    Precision = "year"    // level 3: year only
	Unknown Precision = "unknown" // level 4: given, time unknown (dose records)
	None    Precision = "none"    // birth date not entered
)

// RecalledDate is a date as the user remembers it (GLOSSARY: Recalled date).
// Value is "YYYY-MM-DD" for precision Day, "YYYY-MM" for Month and "YYYY" for
// Year, and absent for Unknown and None. It is kept as raw text and any
// string decodes, so an unparseable value reaches the engine and is reported
// as INVALID_DATE (A9): ⚪ on a dose record, an input error on the birth
// date. An absent value is the empty string. JSON null and non-string values
// are decode errors.
type RecalledDate struct {
	Value     string    `json:"value,omitempty"`
	Precision Precision `json:"precision"`
}

// Period returns the calendar period r names, not yet capped at today: Day
// gives [d, d], Month [first day, last day] and Year [Jan 1, Dec 31]. dated
// is false, with no error, for Unknown and None. A value that does not parse
// for its precision, or a precision outside the enum, is an error.
func (r RecalledDate) Period() (p Range, dated bool, err error) {
	switch r.Precision {
	case Day:
		d, err := ParseDate(r.Value)
		if err != nil {
			return Range{}, false, err
		}
		return Exact(d), true, nil
	case Month:
		y, m, err := parseYearMonth(r.Value)
		if err != nil {
			return Range{}, false, err
		}
		return Range{Lo: fromCivil(y, m, 1), Hi: fromCivil(y, m, lastDay(y, m))}, true, nil
	case Year:
		y, err := parseYear(r.Value)
		if err != nil {
			return Range{}, false, err
		}
		return Range{Lo: fromCivil(y, 1, 1), Hi: fromCivil(y, 12, 31)}, true, nil
	case Unknown, None:
		return Range{}, false, nil
	}
	return Range{}, false, fmt.Errorf("invalid precision %q", r.Precision)
}

// digits parses s as unsigned ASCII decimal digits.
func digits(s string) (int, bool) {
	n := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, len(s) > 0
}

// parseYear parses exactly "YYYY", years 0001 to 9999.
func parseYear(s string) (int, error) {
	y, ok := digits(s)
	if len(s) != 4 || !ok || y < 1 {
		return 0, fmt.Errorf("invalid year %q: want YYYY", s)
	}
	return y, nil
}

// parseYearMonth parses exactly "YYYY-MM", years 0001 to 9999, months 01
// to 12.
func parseYearMonth(s string) (y, m int, err error) {
	if len(s) != 7 || s[4] != '-' {
		return 0, 0, fmt.Errorf("invalid month %q: want YYYY-MM", s)
	}
	y, err = parseYear(s[:4])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid month %q: want YYYY-MM", s)
	}
	m, ok := digits(s[5:])
	if !ok || m < 1 || m > 12 {
		return 0, 0, fmt.Errorf("invalid month %q: want YYYY-MM", s)
	}
	return y, m, nil
}
