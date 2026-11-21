package ruleset

import (
	"strconv"
)

// Validate checks the engine-semantics invariants of a snapshot (SAD §5.2)
// and the integrity the engine relies on (IDs, references, enums, anchors).
// It returns every problem, in document order of their paths. A nil or
// empty result means the snapshot is accepted.
func Validate(s *Snapshot) []Problem {
	v := &validator{}
	if s.Version == "" {
		v.reject(Required, "/version", ProblemParams{})
	}
	if len(s.Diseases) == 0 {
		v.reject(Empty, "/diseases", ProblemParams{})
	}
	diseases := make(map[string]bool, len(s.Diseases))
	for i, d := range s.Diseases {
		p := "/diseases/" + strconv.Itoa(i)
		if v.id(p+"/id", d.ID) {
			if diseases[d.ID] {
				v.reject(DuplicateID, p+"/id", ProblemParams{ID: d.ID})
			}
			diseases[d.ID] = true
		}
		if d.Name == "" {
			v.reject(Required, p+"/name", ProblemParams{})
		}
	}
	series := make(map[string]bool, len(s.Series))
	for i, sr := range s.Series {
		p := "/series/" + strconv.Itoa(i)
		if v.id(p+"/id", sr.ID) {
			if series[sr.ID] {
				v.reject(DuplicateID, p+"/id", ProblemParams{ID: sr.ID})
			}
			series[sr.ID] = true
		}
		switch {
		case sr.Disease == "":
			v.reject(Required, p+"/disease", ProblemParams{})
		case !diseases[sr.Disease]:
			v.reject(UnknownReference, p+"/disease", ProblemParams{Disease: sr.Disease})
		}
		switch sr.Stage {
		case Childhood, Adult:
		case "":
			v.reject(Required, p+"/stage", ProblemParams{})
		default:
			v.reject(InvalidValue, p+"/stage", ProblemParams{Value: string(sr.Stage)})
		}
		v.plans(p, sr.Plans)
	}
	return v.problems
}

type validator struct {
	problems []Problem
}

func (v *validator) reject(code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Severity: Reject, Path: path, Params: params})
}

// id reports a missing or malformed ID and returns whether it is usable.
func (v *validator) id(path, id string) bool {
	switch {
	case id == "":
		v.reject(Required, path, ProblemParams{})
		return false
	case !validID(id):
		v.reject(InvalidValue, path, ProblemParams{Value: id})
		return false
	}
	return true
}

// validID matches ^[a-z0-9]+(-[a-z0-9]+)*$.
func validID(id string) bool {
	if id == "" || id[0] == '-' || id[len(id)-1] == '-' {
		return false
	}
	for i := range len(id) {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && id[i-1] != '-':
		default:
			return false
		}
	}
	return true
}

func (v *validator) plans(seriesPath string, plans []Plan) {
	if len(plans) == 0 {
		v.reject(Empty, seriesPath+"/plans", ProblemParams{})
		return
	}
	ids := make(map[string]bool, len(plans))
	for j, pl := range plans {
		p := seriesPath + "/plans/" + strconv.Itoa(j)
		// EQ5: plan 0 sets the dose count of the series.
		if j > 0 && len(pl.Doses) != len(plans[0].Doses) {
			v.reject(PlanDoseCountMismatch, p, ProblemParams{})
		}
		if v.id(p+"/id", pl.ID) {
			if ids[pl.ID] {
				v.reject(DuplicateID, p+"/id", ProblemParams{ID: pl.ID})
			}
			ids[pl.ID] = true
		}
		if len(pl.Doses) == 0 {
			v.reject(Empty, p+"/doses", ProblemParams{})
		}
		for k, d := range pl.Doses {
			v.dose(p+"/doses/"+strconv.Itoa(k), k == 0, d)
		}
	}
}

func (v *validator) dose(p string, first bool, d DoseRule) {
	// A7: a minimum age or a minimum interval; dose 1 has no previous dose,
	// so only a minimum age counts for it.
	if d.MinAge == nil && (first || d.MinInterval == nil) {
		v.reject(DoseRuleNoMinimum, p, ProblemParams{})
	}
	if first && d.MinInterval != nil {
		v.reject(FirstDoseHasInterval, p+"/minInterval", ProblemParams{})
	}
	switch d.Recommended.From {
	case FromBirth:
	case FromFirstDose, FromPreviousDose:
		if first {
			v.reject(FirstDoseAnchorNotBirth, p+"/recommended/from", ProblemParams{})
		}
	case "":
		v.reject(Required, p+"/recommended/from", ProblemParams{})
	default:
		v.reject(InvalidValue, p+"/recommended/from", ProblemParams{Value: string(d.Recommended.From)})
	}
	// Decode enforces span syntax, but a Snapshot built in Go skips it, and
	// the engine's date arithmetic relies on a real unit and 0..MaxSpanN.
	v.optionalSpan(p+"/minAge", d.MinAge)
	v.optionalSpan(p+"/minInterval", d.MinInterval)
	switch o := d.Recommended.Offset; {
	case o == Span{}:
		v.reject(Required, p+"/recommended/offset", ProblemParams{})
	case !o.valid():
		v.reject(InvalidValue, p+"/recommended/offset", ProblemParams{Value: o.String()})
	}
	v.optionalSpan(p+"/buffer", d.Buffer)
}

// optionalSpan reports a present span that is not valid.
func (v *validator) optionalSpan(path string, s *Span) {
	if s != nil && !s.valid() {
		v.reject(InvalidValue, path, ProblemParams{Value: s.String()})
	}
}
