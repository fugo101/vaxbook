package engine

import (
	"errors"
	"fmt"
)

// Precision is how precisely a user remembers a date (GLOSSARY: Date
// precision, F04.2).
type Precision string

// Precisions. S1 evaluates Day only; the others are reserved for S2 and
// rejected by Validate.
const (
	Day     Precision = "day"     // level 1: exact day
	Month   Precision = "month"   // level 2: month and year
	Year    Precision = "year"    // level 3: year only
	Unknown Precision = "unknown" // level 4: given, time unknown (dose records)
	None    Precision = "none"    // birth date not entered
)

// RecalledDate is a date as the user remembers it (GLOSSARY: Recalled date).
// Value is "YYYY-MM-DD" for precision Day ("YYYY-MM" and "YYYY" arrive with
// Month and Year in S2). It is kept as raw text and any string decodes, so an
// unparseable value reaches the engine and is reported as INVALID_DATE (A9):
// ⚪ on a dose record, an input error on the birth date. An absent value is
// the empty string. JSON null and non-string values are decode errors.
type RecalledDate struct {
	Value     string    `json:"value,omitempty"`
	Precision Precision `json:"precision"`
}

var errUnsupportedPrecision = errors.New("precision not supported yet")

// Range returns the dates the recalled date may fall on. In S1 only Day is
// supported; it gives an exact Range.
func (r RecalledDate) Range() (Range, error) {
	if r.Precision != Day {
		return Range{}, fmt.Errorf("%w: %q", errUnsupportedPrecision, r.Precision)
	}
	dt, err := ParseDate(r.Value)
	if err != nil {
		return Range{}, err
	}
	return Exact(dt), nil
}
