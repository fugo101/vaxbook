package engine

import (
	"fmt"
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
	// or absent value. On a dose record it does not stop Evaluate: the
	// record's diseases become ⚪ INVALID_DATE.
	InvalidDate ProblemCode = "INVALID_DATE"
	// OutOfRange: a date outside [1900-01-01, 2199-12-31].
	OutOfRange ProblemCode = "OUT_OF_RANGE"
	// DateInFuture: a birth or dose date after today.
	DateInFuture ProblemCode = "DATE_IN_FUTURE"
	// DateBeforeBirth: a dose date before the birth date.
	DateBeforeBirth ProblemCode = "DATE_BEFORE_BIRTH"
	// UnknownDisease: a dose record names a disease the rule set does not
	// have. Params: disease.
	UnknownDisease ProblemCode = "UNKNOWN_DISEASE"
	// DuplicateID: two dose records share an ID. Params: id.
	DuplicateID ProblemCode = "DUPLICATE_ID"
	// DuplicateValue: a dose record lists a disease twice. Params: disease.
	DuplicateValue ProblemCode = "DUPLICATE_VALUE"
)

// Problem is one input problem. Path is a JSON pointer (RFC 6901) into the
// Input.
type Problem struct {
	Code   ProblemCode       `json:"code"`
	Path   string            `json:"path"`
	Params map[string]string `json:"params,omitempty"`
}

func (p Problem) String() string {
	s := fmt.Sprintf("%s at %s", p.Code, p.Path)
	if len(p.Params) > 0 {
		s += fmt.Sprintf(" %v", p.Params)
	}
	return s
}

// InputError is returned by Evaluate for an input it cannot assess. It holds
// every problem that stops evaluation.
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
