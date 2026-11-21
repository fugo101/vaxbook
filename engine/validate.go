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
// import run it before Evaluate; an empty result means Evaluate succeeds.
func (e *Engine) Validate(in Input) []Problem {
	var v problems
	todayOK := false
	switch {
	case in.Today.IsZero():
		v.add(Required, "/today", nil)
	case !inInputRange(in.Today):
		v.add(OutOfRange, "/today", nil)
	default:
		todayOK = true
	}
	birth, birthOK := v.recalled("/profile/birth", in.Profile.Birth, true)
	if birthOK && todayOK && birth.Lo.After(in.Today) {
		v.add(DateInFuture, "/profile/birth/value", nil)
		birthOK = false
	}
	ids := make(map[string]bool, len(in.Doses))
	for i, r := range in.Doses {
		p := "/doses/" + strconv.Itoa(i)
		switch {
		case r.ID == "":
			v.add(Required, p+"/id", nil)
		case ids[r.ID]:
			v.add(DuplicateID, p+"/id", map[string]string{"id": r.ID})
		}
		ids[r.ID] = true
		if len(r.Diseases) == 0 {
			v.add(Required, p+"/diseases", nil)
		}
		seen := make(map[string]bool, len(r.Diseases))
		for j, dz := range r.Diseases {
			dp := p + "/diseases/" + strconv.Itoa(j)
			switch {
			case !e.known[dz]:
				v.add(UnknownDisease, dp, map[string]string{"disease": dz})
			case seen[dz]:
				v.add(DuplicateValue, dp, map[string]string{"disease": dz})
			}
			seen[dz] = true
		}
		date, ok := v.recalled(p+"/date", r.Date, false)
		switch {
		case !ok:
		case todayOK && date.Lo.After(in.Today):
			v.add(DateInFuture, p+"/date/value", nil)
		case birthOK && date.Hi.Before(birth.Lo):
			v.add(DateBeforeBirth, p+"/date/value", nil)
		}
	}
	return v
}

type problems []Problem

func (v *problems) add(code ProblemCode, path string, params map[string]string) {
	*v = append(*v, Problem{Code: code, Path: path, Params: params})
}

// recalled checks a RecalledDate at path and returns its range when usable.
func (v *problems) recalled(path string, r RecalledDate, birth bool) (Range, bool) {
	switch r.Precision {
	case Day:
	case "":
		v.add(Required, path+"/precision", nil)
		return Range{}, false
	case Month, Year:
		v.add(UnsupportedPrecision, path+"/precision", map[string]string{"precision": string(r.Precision)})
		return Range{}, false
	case Unknown, None:
		// Level 4 differs by field (F03, GLOSSARY "Date precision"): a dose
		// record has "unknown" for an unremembered date, and a birth date
		// has "none" for not entered. The other field's value is invalid in
		// every engine version, not merely unsupported yet.
		if (r.Precision == None) != birth {
			v.add(InvalidValue, path+"/precision", map[string]string{"value": string(r.Precision)})
			return Range{}, false
		}
		v.add(UnsupportedPrecision, path+"/precision", map[string]string{"precision": string(r.Precision)})
		return Range{}, false
	default:
		v.add(InvalidValue, path+"/precision", map[string]string{"value": string(r.Precision)})
		return Range{}, false
	}
	rng, err := r.Range()
	if err != nil {
		v.add(InvalidDate, path+"/value", nil)
		return Range{}, false
	}
	if !inInputRange(rng.Lo) || !inInputRange(rng.Hi) {
		v.add(OutOfRange, path+"/value", nil)
		return Range{}, false
	}
	return rng, true
}
