package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// ProblemCode is a code from the closed catalog of input problems.
type ProblemCode string

// The input problem catalog (S3a).
const (
	// Required: a required value is missing, or an optional string member
	// is present but empty (product, freeProduct).
	Required ProblemCode = "REQUIRED"
	// InvalidValue: a value is outside its enum, including the level-4
	// precision of the other field ("none" on a dose record, "unknown" on
	// the birth date). Params: value.
	InvalidValue ProblemCode = "INVALID_VALUE"
	// UnsupportedPrecision: a date precision this engine version cannot
	// evaluate yet. Params: precision. Every precision is evaluable since
	// S2, so no input gives it; the code stays in the catalog.
	UnsupportedPrecision ProblemCode = "UNSUPPORTED_PRECISION"
	// InvalidDate: a value that is not a parseable date for its precision
	// (YYYY-MM-DD, YYYY-MM, YYYY), including an empty or absent value. On a
	// dose record it is record-scoped.
	InvalidDate ProblemCode = "INVALID_DATE"
	// UnexpectedValue: a value given with a precision that has none
	// (unknown on a dose record, none on the birth date). On a dose record it
	// is record-scoped.
	UnexpectedValue ProblemCode = "UNEXPECTED_VALUE"
	// OutOfRange: a date whose period is not inside [1900-01-01,
	// 2199-12-31], or a dose number outside 1..999. A dose date out of range
	// is record-scoped; a dose number is not.
	OutOfRange ProblemCode = "OUT_OF_RANGE"
	// DateInFuture: a birth or dose date whose whole period is after today.
	// On a dose record it is record-scoped.
	DateInFuture ProblemCode = "DATE_IN_FUTURE"
	// DateBeforeBirth: a dose date whose whole period is before the whole
	// birth-date range. Record-scoped.
	DateBeforeBirth ProblemCode = "DATE_BEFORE_BIRTH"
	// UnknownReference: a doseNumbers key that is not one of the record's
	// diseases. Params: disease.
	UnknownReference ProblemCode = "UNKNOWN_REFERENCE"
	// UnknownDisease: a dose record names a disease the rule set does not
	// have. Params: disease.
	UnknownDisease ProblemCode = "UNKNOWN_DISEASE"
	// DuplicateID: two dose records share an ID. Params: id.
	DuplicateID ProblemCode = "DUPLICATE_ID"
	// DuplicateValue: a dose record lists a disease twice. Params: disease.
	DuplicateValue ProblemCode = "DUPLICATE_VALUE"
	// UnknownProduct: a dose record names a product the rule set does not
	// have. Record-scoped (issue #14, Q35). Params: product. Added in S3a.
	UnknownProduct ProblemCode = "UNKNOWN_PRODUCT"
	// MutuallyExclusive: a dose record has both product and freeProduct.
	// Path: freeProduct. Added in S3a.
	MutuallyExclusive ProblemCode = "MUTUALLY_EXCLUSIVE"
)

// Scope says what a problem stops.
type Scope string

// Scopes.
const (
	// ScopeInput: Evaluate returns an *InputError instead of an Output.
	ScopeInput Scope = "input"
	// ScopeRecord: a problem with a dose record's date value (INVALID_DATE,
	// UNEXPECTED_VALUE, OUT_OF_RANGE, DATE_IN_FUTURE, DATE_BEFORE_BIRTH) or
	// its product (UNKNOWN_PRODUCT). Evaluate still assesses the profile:
	// every disease of that record becomes ⚪ with a reason of the same code
	// (A9, EQ27).
	ScopeRecord Scope = "record"
)

// Problem is one input problem. Path is a JSON pointer (RFC 6901) into the
// Input. Scope is Go-only for now; serializing it later is additive.
type Problem struct {
	Code   ProblemCode   `json:"code"`
	Path   string        `json:"path"`
	Params ProblemParams `json:"params,omitzero"`
	Scope  Scope         `json:"-"`
}

// ProblemParams holds every problem parameter; each code uses the members
// its doc lists. Like ReasonParams, members are typed, in alphabetical
// JSON-name order and omitzero, and the struct grows additively.
type ProblemParams struct {
	Disease   string `json:"disease,omitzero"`
	ID        string `json:"id,omitzero"`
	Precision string `json:"precision,omitzero"`
	Product   string `json:"product,omitzero"`
	Value     string `json:"value,omitzero"`
}

func (p Problem) String() string {
	s := fmt.Sprintf("%s at %s", p.Code, p.Path)
	if p.Params != (ProblemParams{}) {
		s += " " + paramString("disease", p.Params.Disease, "id", p.Params.ID, "precision", p.Params.Precision,
			"product", p.Params.Product, "value", p.Params.Value)
	}
	return s
}

// paramString formats name/value pairs, skipping empty values.
func paramString(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			parts = append(parts, pairs[i]+"="+strconv.Quote(pairs[i+1]))
		}
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// InputError is returned by Evaluate for an input it cannot assess:
// Evaluate returns *InputError iff some problem has ScopeInput. It holds
// exactly the ScopeInput problems.
type InputError struct {
	Problems []Problem
}

func (e *InputError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.String()
	}
	return "invalid engine input: " + strings.Join(parts, "; ")
}
