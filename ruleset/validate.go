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
	products := v.products(s.Products, diseases)
	series := make(map[string]bool, len(s.Series))
	f := startAgeFindings(s.Series)
	for i, sr := range s.Series {
		p := "/series/" + strconv.Itoa(i)
		if f.required[i] {
			params := ProblemParams{Disease: sr.Disease}
			if sr.Product != nil {
				params.Product = *sr.Product
			}
			v.reject(StartAgeRequired, p, params)
		}
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
		if sr.Product != nil {
			pp := p + "/product"
			switch id := *sr.Product; {
			case !v.id(pp, id):
			case products[id] == nil:
				v.reject(UnknownReference, pp, ProblemParams{Product: id})
			case diseases[sr.Disease] && !products[id][sr.Disease]:
				v.reject(ProductDiseaseMismatch, pp, ProblemParams{Disease: sr.Disease, Product: id})
			}
		}
		switch sr.Stage {
		case Childhood, Adult:
		case "":
			v.reject(Required, p+"/stage", ProblemParams{})
		default:
			v.reject(InvalidValue, p+"/stage", ProblemParams{Value: string(sr.Stage)})
		}
		if sa := sr.StartAge; sa != nil {
			v.startAge(p+"/startAge", *sa, f.overlaps[i])
		}
		v.plans(p, sr.Plans, f.warnings(sr, i))
	}
	return v.problems
}

// startAge checks one start-age bracket: its spans, then START_AGE_EMPTY
// and the START_AGE_OVERLAP problems found by startAgeFindings. A bracket
// with an invalid span gets only the span problem, so no check reasons on a
// span the engine's date arithmetic could not handle.
func (v *validator) startAge(p string, sa StartAge, overlaps []string) {
	if !validBracket(&sa) {
		switch {
		case sa.Min == Span{}:
			v.reject(Required, p+"/min", ProblemParams{})
		case !sa.Min.valid():
			v.reject(InvalidValue, p+"/min", ProblemParams{Value: sa.Min.String()})
		}
		v.optionalSpan(p+"/max", sa.Max)
		return
	}
	if sa.Max != nil && !spanLess(sa.Min, *sa.Max) {
		v.reject(StartAgeEmpty, p, ProblemParams{})
	}
	for _, id := range overlaps {
		v.reject(StartAgeOverlap, p, ProblemParams{ID: id})
	}
}

// validBracket reports whether a start-age bracket is present with a
// non-zero valid min and, when present, a valid max.
func validBracket(sa *StartAge) bool {
	return sa != nil && sa.Min != Span{} && sa.Min.valid() && (sa.Max == nil || sa.Max.valid())
}

// startAgeFinding holds the cross-series start-age checks of a snapshot
// (EQ26, issue #23), by series index; Validate reports them in document
// order of their paths.
type startAgeFinding struct {
	// required: the series has no startAge and its product group has more
	// than one series.
	required []bool
	// overlaps are the IDs of the earlier series of the same group whose
	// brackets may overlap this one's, in rule-set order.
	overlaps [][]string
	// reference is, per series, the dose-1 minimum age of plan 0 of the
	// group's first series, which every plan of the group is compared with
	// (FIRST_DOSE_MIN_AGE_DIFFERS), and refID that series' ID. It is nil
	// when the group has one series, or when that minimum age is absent or
	// invalid or that series has an invalid bracket: a Reject is reported
	// then.
	reference []*Span
	refID     []string
}

// startAgeFindings groups the series by product group: the series of one
// disease with the same product, or its generic series (issue #14). A
// present empty product forms its own group (it is REQUIRED anyway).
func startAgeFindings(series []Series) startAgeFinding {
	type key struct {
		disease, product string
		generic          bool
	}
	groups := map[key][]int{}
	keys := make([]key, len(series))
	for i, sr := range series {
		k := key{disease: sr.Disease, generic: sr.Product == nil}
		if sr.Product != nil {
			k.product = *sr.Product
		}
		keys[i] = k
		groups[k] = append(groups[k], i)
	}
	n := len(series)
	f := startAgeFinding{required: make([]bool, n), overlaps: make([][]string, n), reference: make([]*Span, n), refID: make([]string, n)}
	for i, sr := range series {
		g := groups[keys[i]]
		if len(g) < 2 {
			continue
		}
		f.required[i] = sr.StartAge == nil
		if first := series[g[0]]; (first.StartAge == nil || validBracket(first.StartAge)) &&
			len(first.Plans) > 0 && len(first.Plans[0].Doses) > 0 {
			if a := first.Plans[0].Doses[0].MinAge; a != nil && a.valid() {
				f.reference[i], f.refID[i] = a, first.ID
			}
		}
		if !validBracket(sr.StartAge) {
			continue
		}
		for _, e := range g {
			if e >= i {
				break
			}
			if validBracket(series[e].StartAge) && mayOverlap(*series[e].StartAge, *sr.StartAge) {
				f.overlaps[i] = append(f.overlaps[i], series[e].ID)
			}
		}
	}
	return f
}

// mayOverlap reports whether two brackets may share an age for some month
// length: they are disjoint only when one's max is at most the other's min
// for every month length. Two open-ended brackets always overlap.
func mayOverlap(a, b StartAge) bool {
	return (a.Max == nil || !SpanAtMost(*a.Max, b.Min)) && (b.Max == nil || !SpanAtMost(*b.Max, a.Min))
}

// firstDoseWarnings are the dose-1 Warns of one series (issue #23), checked
// per plan just before dose 1's own problems: they sit at dose 1's minAge,
// which is present and valid whenever they apply, so no other problem of
// that dose comes before them in document order.
type firstDoseWarnings struct {
	bracket   *StartAge // nil: no valid bracket, no FIRST_DOSE_MIN_AGE_ABOVE_START_AGE
	reference *Span     // nil: no FIRST_DOSE_MIN_AGE_DIFFERS
	refID     string
}

// warnings returns the dose-1 Warns that apply to series i. A series with
// an invalid bracket gets neither: only its span problem.
func (f startAgeFinding) warnings(sr Series, i int) firstDoseWarnings {
	if sr.StartAge != nil && !validBracket(sr.StartAge) {
		return firstDoseWarnings{}
	}
	return firstDoseWarnings{bracket: sr.StartAge, reference: f.reference[i], refID: f.refID[i]}
}

// dose1 emits the dose-1 Warns of plan pl at path p (dose 1's minAge).
func (v *validator) dose1(p string, pl Plan, w firstDoseWarnings) {
	if len(pl.Doses) == 0 {
		return
	}
	a := pl.Doses[0].MinAge
	if a == nil || !a.valid() {
		return
	}
	if w.bracket != nil && !SpanAtMost(*a, w.bracket.Min) {
		v.warn(FirstDoseMinAgeAboveStartAge, p, ProblemParams{})
	}
	if w.reference != nil && !spanEqual(*a, *w.reference) {
		v.warn(FirstDoseMinAgeDiffers, p, ProblemParams{ID: w.refID})
	}
}

// products checks the product catalog and returns the diseases each usable
// product ID lists. For a repeated ID the first product wins.
func (v *validator) products(products []Product, diseases map[string]bool) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(products))
	for i, pr := range products {
		p := "/products/" + strconv.Itoa(i)
		listed := make(map[string]bool, len(pr.Diseases))
		if v.id(p+"/id", pr.ID) {
			if out[pr.ID] != nil {
				v.reject(DuplicateID, p+"/id", ProblemParams{ID: pr.ID})
			} else {
				out[pr.ID] = listed
			}
		}
		if pr.Name == "" {
			v.reject(Required, p+"/name", ProblemParams{})
		}
		if len(pr.Diseases) == 0 {
			v.reject(Empty, p+"/diseases", ProblemParams{})
		}
		for j, d := range pr.Diseases {
			dp := p + "/diseases/" + strconv.Itoa(j)
			switch {
			case d == "":
				v.reject(Required, dp, ProblemParams{})
			case !diseases[d]:
				v.reject(UnknownReference, dp, ProblemParams{Disease: d})
			case listed[d]:
				v.reject(DuplicateValue, dp, ProblemParams{Disease: d})
			}
			listed[d] = true
		}
	}
	return out
}

type validator struct {
	problems []Problem
}

func (v *validator) reject(code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Severity: Reject, Path: path, Params: params})
}

func (v *validator) warn(code ProblemCode, path string, params ProblemParams) {
	v.problems = append(v.problems, Problem{Code: code, Severity: Warn, Path: path, Params: params})
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

func (v *validator) plans(seriesPath string, plans []Plan, w firstDoseWarnings) {
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
		v.dose1(p+"/doses/0/minAge", pl, w)
		for k, d := range pl.Doses {
			v.dose(p+"/doses/"+strconv.Itoa(k), k == 0, d)
		}
	}
}

func (v *validator) dose(p string, first bool, d DoseRule) {
	// A7: a minimum age or a minimum interval (to the previous dose or to
	// dose 1); dose 1 has no earlier dose, so only a minimum age counts for it.
	if d.MinAge == nil && (first || (d.MinInterval == nil && d.MinIntervalFromFirst == nil)) {
		v.reject(DoseRuleNoMinimum, p, ProblemParams{})
	}
	if first && d.MinInterval != nil {
		v.reject(FirstDoseHasInterval, p+"/minInterval", ProblemParams{})
	}
	if first && d.MinIntervalFromFirst != nil {
		v.reject(FirstDoseHasInterval, p+"/minIntervalFromFirst", ProblemParams{})
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
	v.optionalSpan(p+"/minIntervalFromFirst", d.MinIntervalFromFirst)
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
