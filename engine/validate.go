package engine

import "strconv"

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

// validation is the result of validate: every problem, and the code of the
// record-scoped problem of each dose record that has one.
type validation struct {
	problems []Problem
	records  map[int]ProblemCode // dose record index → its ScopeRecord problem
}

func (e *Engine) validate(in Input) validation {
	v := validation{records: map[int]ProblemCode{}}
	todayOK := false
	switch {
	case in.Today.IsZero():
		v.add(Required, "/today", ProblemParams{})
	case !inInputRange(in.Today):
		v.add(OutOfRange, "/today", ProblemParams{})
	default:
		todayOK = true
	}
	birth, birthOK := v.recalled(-1, "/profile/birth", in.Profile.Birth)
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
		date, ok := v.recalled(i, p+"/date", r.Date)
		switch {
		case !ok:
		case todayOK && date.Lo.After(in.Today):
			v.value(i, DateInFuture, p+"/date/value")
		case birthOK && date.Hi.Before(birth.Lo):
			v.value(i, DateBeforeBirth, p+"/date/value")
		}
	}
	return v
}

// add records a problem that stops evaluation.
func (v *validation) add(code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Path: path, Params: params, Scope: ScopeInput})
}

// value records a problem with the date value of dose record i, or of the
// birth date for i < 0. On a dose record it is record-scoped (at most one
// per record): Evaluate makes that record's diseases ⚪.
func (v *validation) value(i int, code ProblemCode, path string) {
	if i < 0 {
		v.add(code, path, ProblemParams{})
		return
	}
	v.problems = append(v.problems, Problem{Code: code, Path: path, Scope: ScopeRecord})
	v.records[i] = code
}

// recalled checks the RecalledDate at path, of dose record i or of the birth
// date for i < 0, and returns its range when usable.
func (v *validation) recalled(i int, path string, r RecalledDate) (Range, bool) {
	birth := i < 0
	switch r.Precision {
	case Day:
	case "":
		v.add(Required, path+"/precision", ProblemParams{})
		return Range{}, false
	case Month, Year:
		v.add(UnsupportedPrecision, path+"/precision", ProblemParams{Precision: string(r.Precision)})
		return Range{}, false
	case Unknown, None:
		// Level 4 differs by field (F03, GLOSSARY "Date precision"): a dose
		// record has "unknown" for an unremembered date, and a birth date
		// has "none" for not entered. The other field's value is invalid in
		// every engine version, not merely unsupported yet.
		if (r.Precision == None) != birth {
			v.add(InvalidValue, path+"/precision", ProblemParams{Value: string(r.Precision)})
			return Range{}, false
		}
		v.add(UnsupportedPrecision, path+"/precision", ProblemParams{Precision: string(r.Precision)})
		return Range{}, false
	default:
		v.add(InvalidValue, path+"/precision", ProblemParams{Value: string(r.Precision)})
		return Range{}, false
	}
	rng, err := r.Range()
	if err != nil {
		v.value(i, InvalidDate, path+"/value")
		return Range{}, false
	}
	if !inInputRange(rng.Lo) || !inInputRange(rng.Hi) {
		v.value(i, OutOfRange, path+"/value")
		return Range{}, false
	}
	return rng, true
}
