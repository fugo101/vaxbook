package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// ProblemCode is a code from the closed catalog of input problems.
type ProblemCode string

// The input problem catalog (S1).
const (
	// Required: a required value is missing.
	Required ProblemCode = "REQUIRED"
	// InvalidValue: a value is outside its enum, including the level-4
	// precision of the other field ("none" on a dose record, "unknown" on
	// the birth date). Params: value.
	InvalidValue ProblemCode = "INVALID_VALUE"
	// UnsupportedPrecision: a date precision this engine version cannot
	// evaluate yet. Params: precision.
	UnsupportedPrecision ProblemCode = "UNSUPPORTED_PRECISION"
	// InvalidDate: a value that is not a parseable date, including an empty
	// or absent value. On a dose record it is record-scoped.
	InvalidDate ProblemCode = "INVALID_DATE"
	// OutOfRange: a date outside [1900-01-01, 2199-12-31]. On a dose record
	// it is record-scoped.
	OutOfRange ProblemCode = "OUT_OF_RANGE"
	// DateInFuture: a birth or dose date after today. On a dose record it is
	// record-scoped.
	DateInFuture ProblemCode = "DATE_IN_FUTURE"
	// DateBeforeBirth: a dose date before the birth date. Record-scoped.
	DateBeforeBirth ProblemCode = "DATE_BEFORE_BIRTH"
	// UnknownDisease: a dose record names a disease the rule set does not
	// have. Params: disease.
	UnknownDisease ProblemCode = "UNKNOWN_DISEASE"
	// DuplicateID: two dose records share an ID. Params: id.
	DuplicateID ProblemCode = "DUPLICATE_ID"
	// DuplicateValue: a dose record lists a disease twice. Params: disease.
	DuplicateValue ProblemCode = "DUPLICATE_VALUE"
)

// Scope says what a problem stops.
type Scope string

// Scopes.
const (
	// ScopeInput: Evaluate returns an *InputError instead of an Output.
	ScopeInput Scope = "input"
	// ScopeRecord: a problem with a dose record's date value (INVALID_DATE,
	// OUT_OF_RANGE, DATE_IN_FUTURE, DATE_BEFORE_BIRTH). Evaluate still
	// assesses the profile: every disease of that record becomes ⚪ with a
	// reason of the same code (A9, EQ27).
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
	Value     string `json:"value,omitzero"`
}

func (p Problem) String() string {
	s := fmt.Sprintf("%s at %s", p.Code, p.Path)
	if p.Params != (ProblemParams{}) {
		s += " " + paramString("disease", p.Params.Disease, "id", p.Params.ID, "precision", p.Params.Precision, "value", p.Params.Value)
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
