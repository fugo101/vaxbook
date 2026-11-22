package engine

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Dates the engine accepts. Spans are capped at 999 units, so computed dates
// keep four-digit years.
var (
	minInputDate = fromCivil(1900, 1, 1)
	maxInputDate = fromCivil(2199, 12, 31)
)

func inInputRange(d Date) bool { return !d.Before(minInputDate) && !d.After(maxInputDate) }

// Validate reports every problem in an input, in traversal order: /today,
// /profile/birth, then each dose record in index order. The server and guest
// import run it before Evaluate, and saving a record (F04.4) is blocked by
// any problem. Evaluate succeeds iff no problem has ScopeInput; a
// ScopeRecord problem makes its record's diseases ⚪ instead.
func (e *Engine) Validate(in Input) []Problem { return e.validate(in).problems }

// validation is the result of validate: every problem, and the codes of the
// record-scoped problems of each dose record that has some, in path order.
type validation struct {
	problems []Problem
	records  map[int][]ProblemCode // dose record index → its ScopeRecord problems
}

func (e *Engine) validate(in Input) validation {
	v := validation{records: map[int][]ProblemCode{}}
	todayOK := false
	switch {
	case in.Today.IsZero():
		v.add(Required, "/today", ProblemParams{})
	case !inInputRange(in.Today):
		v.add(OutOfRange, "/today", ProblemParams{})
	default:
		todayOK = true
	}
	// The birth date is a range for month and year precision (EQ17); the
	// dose checks below compare against its older end.
	birth, birthDated, birthOK := v.recalled(-1, "/profile/birth", in.Profile.Birth)
	birthOK = birthOK && birthDated
	if birthOK && todayOK && birth.Lo.After(in.Today) {
		v.add(DateInFuture, "/profile/birth/value", ProblemParams{})
		birthOK = false
	}
	ids := make(map[string]bool, len(in.Doses))
	for i, r := range in.Doses {
		p := "/doses/" + strconv.Itoa(i)
		switch {
		case r.ID == "":
			v.add(Required, p+"/id", ProblemParams{})
		case ids[r.ID]:
			v.add(DuplicateID, p+"/id", ProblemParams{ID: r.ID})
		}
		ids[r.ID] = true
		if len(r.Diseases) == 0 {
			v.add(Required, p+"/diseases", ProblemParams{})
		}
		seen := make(map[string]bool, len(r.Diseases))
		for j, dz := range r.Diseases {
			dp := p + "/diseases/" + strconv.Itoa(j)
			switch {
			case !e.known[dz]:
				v.add(UnknownDisease, dp, ProblemParams{Disease: dz})
			case seen[dz]:
				v.add(DuplicateValue, dp, ProblemParams{Disease: dz})
			}
			seen[dz] = true
		}
		// The product: a catalog ID (retired ones included) or free text,
		// not both. An ID the rule set does not have is record-scoped: it
		// comes from version skew between a cached client and the rule set
		// (issue #14, Q35).
		switch {
		case r.Product == nil:
		case *r.Product == "":
			v.add(Required, p+"/product", ProblemParams{})
		case !e.products[*r.Product]:
			v.record(i, UnknownProduct, p+"/product", ProblemParams{Product: *r.Product})
		}
		if r.FreeProduct != nil && *r.FreeProduct == "" {
			v.add(Required, p+"/freeProduct", ProblemParams{})
		}
		if r.Product != nil && r.FreeProduct != nil {
			v.add(MutuallyExclusive, p+"/freeProduct", ProblemParams{})
		}
		// A dose date is a problem only when its whole period is after today
		// or before the whole birth-date range; a partial overlap is not
		// (issue #13). The engine then caps the period at today.
		period, dated, ok := v.recalled(i, p+"/date", r.Date)
		switch {
		case !ok || !dated:
		case todayOK && period.Lo.After(in.Today):
			v.value(i, DateInFuture, p+"/date/value")
		case birthOK && period.Hi.Before(birth.Lo):
			v.value(i, DateBeforeBirth, p+"/date/value")
		}
		for _, dz := range slices.Sorted(maps.Keys(r.DoseNumbers)) {
			np := p + "/doseNumbers/" + escapePointer(dz)
			switch n := r.DoseNumbers[dz]; {
			case !seen[dz]:
				v.add(UnknownReference, np, ProblemParams{Disease: dz})
			case n < 1 || n > maxDoseNumber:
				v.add(OutOfRange, np, ProblemParams{})
			}
		}
	}
	return v
}

// escapePointer escapes one reference token of a JSON pointer (RFC 6901).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// add records a problem that stops evaluation.
func (v *validation) add(code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Path: path, Params: params, Scope: ScopeInput})
}

// value records a problem with the date value of dose record i, or of the
// birth date for i < 0. On a dose record it is record-scoped (at most one
// per date): Evaluate makes that record's diseases ⚪.
func (v *validation) value(i int, code ProblemCode, path string) {
	if i < 0 {
		v.add(code, path, ProblemParams{})
		return
	}
	v.record(i, code, path, ProblemParams{})
}

// record records a record-scoped problem of dose record i: Evaluate makes
// that record's diseases ⚪ with a reason of the same code.
func (v *validation) record(i int, code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Path: path, Params: params, Scope: ScopeRecord})
	v.records[i] = append(v.records[i], code)
}

// recalled checks the RecalledDate at path, of dose record i or of the
// birth date for i < 0. ok reports that it is usable; dated that it names a
// period (day, month or year), which is then returned, not yet capped at
// today. A usable undated date is a level-4 dose (unknown) or no birth date
// (none).
func (v *validation) recalled(i int, path string, r RecalledDate) (period Range, dated, ok bool) {
	birth := i < 0
	switch r.Precision {
	case Day, Month, Year:
	case "":
		v.add(Required, path+"/precision", ProblemParams{})
		return Range{}, false, false
	case Unknown, None:
		// Level 4 differs by field (F03, GLOSSARY "Date precision"): a dose
		// record has "unknown" for an unremembered date, and a birth date
		// has "none" for not entered. The other field's value is invalid.
		if (r.Precision == None) != birth {
			v.add(InvalidValue, path+"/precision", ProblemParams{Value: string(r.Precision)})
			return Range{}, false, false
		}
		if r.Value != "" {
			v.value(i, UnexpectedValue, path+"/value")
			return Range{}, false, false
		}
		return Range{}, false, true
	default:
		v.add(InvalidValue, path+"/precision", ProblemParams{Value: string(r.Precision)})
		return Range{}, false, false
	}
	period, _, err := r.Period()
	if err != nil {
		v.value(i, InvalidDate, path+"/value")
		return Range{}, false, false
	}
	if !inInputRange(period.Lo) || !inInputRange(period.Hi) {
		v.value(i, OutOfRange, path+"/value")
		return Range{}, false, false
	}
	return period, true, true
}
