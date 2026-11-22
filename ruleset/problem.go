package ruleset

import (
	"fmt"
	"strconv"
	"strings"
)

// Severity says whether a Problem rejects the snapshot or only warns.
type Severity string

// Severities.
const (
	Reject Severity = "reject"
	Warn   Severity = "warn"
)

// ProblemCode is a code from the closed catalog of rule set problems.
type ProblemCode string

// The rule set problem catalog (S3a). Renaming a code is a contract change.
const (
	// PlanDoseCountMismatch: a plan has a different number of doses from
	// plan 0 of its series (EQ5). Path: the plan.
	PlanDoseCountMismatch ProblemCode = "PLAN_DOSE_COUNT_MISMATCH"
	// DoseRuleNoMinimum: a dose rule has neither a minimum age nor a minimum
	// interval (minInterval or minIntervalFromFirst), or dose 1 has no minimum
	// age (A7). Path: the dose rule.
	DoseRuleNoMinimum ProblemCode = "DOSE_RULE_NO_MINIMUM"
	// FirstDoseHasInterval: dose 1 has a minimum interval or a
	// minIntervalFromFirst, but there is no earlier dose to measure it from.
	// Path: the interval member.
	FirstDoseHasInterval ProblemCode = "FIRST_DOSE_HAS_INTERVAL"
	// FirstDoseAnchorNotBirth: dose 1's milestone counts from a dose.
	FirstDoseAnchorNotBirth ProblemCode = "FIRST_DOSE_ANCHOR_NOT_BIRTH"
	// Required: a required member is empty.
	Required ProblemCode = "REQUIRED"
	// InvalidValue: a value is outside its enum or pattern. Params: value.
	InvalidValue ProblemCode = "INVALID_VALUE"
	// DuplicateID: an ID is used twice in its scope. Params: id.
	DuplicateID ProblemCode = "DUPLICATE_ID"
	// UnknownReference: a series or product names a disease, or a series
	// names a product, that does not exist. Params: disease or product.
	UnknownReference ProblemCode = "UNKNOWN_REFERENCE"
	// Empty: a snapshot has no diseases, a series has no plans, a plan has
	// no doses or a product has no diseases.
	Empty ProblemCode = "EMPTY"
	// DuplicateValue: a product lists a disease twice. Params: disease.
	// Added in S3a.
	DuplicateValue ProblemCode = "DUPLICATE_VALUE"
	// ProductDiseaseMismatch: a series names a product that does not list
	// the series' disease. Path: the series' product. Params: disease,
	// product. Added in S3a.
	ProductDiseaseMismatch ProblemCode = "PRODUCT_DISEASE_MISMATCH"
)

// Problem is one finding of Validate. Path is a JSON pointer (RFC 6901)
// into the snapshot.
type Problem struct {
	Code     ProblemCode   `json:"code"`
	Severity Severity      `json:"severity"`
	Path     string        `json:"path"`
	Params   ProblemParams `json:"params,omitzero"`
}

// ProblemParams holds every problem parameter; each code uses the members
// its doc lists. Members are typed (IDs and values are strings), in
// alphabetical JSON-name order and omitzero, and the struct grows
// additively.
type ProblemParams struct {
	Disease string `json:"disease,omitzero"`
	ID      string `json:"id,omitzero"`
	Product string `json:"product,omitzero"`
	Value   string `json:"value,omitzero"`
}

func (p Problem) String() string {
	s := fmt.Sprintf("%s %s at %s", p.Severity, p.Code, p.Path)
	var parts []string
	for _, kv := range [][2]string{{"disease", p.Params.Disease}, {"id", p.Params.ID}, {"product", p.Params.Product}, {"value", p.Params.Value}} {
		if kv[1] != "" {
			parts = append(parts, kv[0]+"="+strconv.Quote(kv[1]))
		}
	}
	if len(parts) > 0 {
		s += " {" + strings.Join(parts, " ") + "}"
	}
	return s
}

// RejectedError reports a snapshot that Validate rejects.
type RejectedError struct {
	Problems []Problem
}

func (e *RejectedError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		if p.Severity == Reject {
			parts = append(parts, p.String())
		}
	}
	return "rule set rejected: " + strings.Join(parts, "; ")
}

// Rejects returns the problems with severity Reject.
func Rejects(problems []Problem) []Problem {
	var out []Problem
	for _, p := range problems {
		if p.Severity == Reject {
			out = append(out, p)
		}
	}
	return out
}
