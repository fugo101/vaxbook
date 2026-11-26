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
		if from, to := usableDate(sr.ValidFrom), usableDate(sr.ValidTo); from != nil && to != nil && from.compare(*to) >= 0 {
			v.reject(ValidityEmpty, p, ProblemParams{})
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
		v.maxAges(p, sr)
		v.catchUp(p, sr)
		v.date(p+"/validFrom", sr.ValidFrom)
		v.date(p+"/validTo", sr.ValidTo)
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
// order of their paths. Since S4c every check is between series of one
// product group whose validity windows overlap (windowsOverlap): series that
// are never in effect on the same day never compete for a dose. Intervals
// that overlap pairwise share a day, so the pairwise checks validate the
// group in effect on every day.
type startAgeFinding struct {
	// required: the series has no startAge and another series of its
	// product group overlaps its window.
	required []bool
	// overlaps are the IDs of the earlier series of the same group whose
	// windows overlap this one's and whose brackets may overlap, in
	// rule-set order.
	overlaps [][]string
	// refs are, per series, the dose-1 minimum ages every plan of it is
	// compared with (FIRST_DOSE_MIN_AGE_DIFFERS), in order: plan 0 of every
	// earlier series of the group whose window overlaps, then its own plan
	// 0. A series with an absent or invalid plan-0 dose-1 minAge is left
	// out (a Reject is reported then). It is empty when no other series of
	// the group overlaps the window. Brackets do not matter: the
	// comparison never reads one, so an invalid one skips only that
	// series' own Warns (warnings).
	refs [][]minAgeRef
}

// minAgeRef is a dose-1 minimum age to compare with, and its series' ID.
type minAgeRef struct {
	span Span
	id   string
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
	f := startAgeFinding{required: make([]bool, n), overlaps: make([][]string, n), refs: make([][]minAgeRef, n)}
	for i, sr := range series {
		var others []int // the other series of the group whose windows overlap i's
		for _, e := range groups[keys[i]] {
			if e != i && windowsOverlap(series[e], sr) {
				others = append(others, e)
			}
		}
		if len(others) == 0 {
			continue
		}
		f.required[i] = sr.StartAge == nil
		for _, e := range others {
			if e > i {
				break
			}
			if a, ok := firstMinAge(series[e]); ok {
				f.refs[i] = append(f.refs[i], minAgeRef{span: a, id: series[e].ID})
			}
		}
		if a, ok := firstMinAge(sr); ok {
			f.refs[i] = append(f.refs[i], minAgeRef{span: a, id: sr.ID})
		}
		if !validBracket(sr.StartAge) {
			continue
		}
		for _, e := range others {
			if e > i {
				break
			}
			if validBracket(series[e].StartAge) && mayOverlap(*series[e].StartAge, *sr.StartAge) {
				f.overlaps[i] = append(f.overlaps[i], series[e].ID)
			}
		}
	}
	return f
}

// firstMinAge returns the dose-1 minimum age of plan 0 of a series, when it
// is present and valid.
func firstMinAge(sr Series) (Span, bool) {
	if len(sr.Plans) == 0 || len(sr.Plans[0].Doses) == 0 {
		return Span{}, false
	}
	if a := sr.Plans[0].Doses[0].MinAge; a != nil && a.valid() {
		return *a, true
	}
	return Span{}, false
}

// windowsOverlap reports whether two series' validity windows share a day:
// a.validFrom < b.validTo and b.validFrom < a.validTo, an absent bound
// being unbounded. A zero or invalid bound counts as absent too: it has its
// own Reject, and an unbounded window only adds overlaps.
func windowsOverlap(a, b Series) bool {
	before := func(from, to *Date) bool {
		f, t := usableDate(from), usableDate(to)
		return f == nil || t == nil || f.compare(*t) < 0
	}
	return before(a.ValidFrom, b.ValidTo) && before(b.ValidFrom, a.ValidTo)
}

// usableDate returns d when it is present and a valid date, else nil.
func usableDate(d *Date) *Date {
	if d == nil || !d.valid() {
		return nil
	}
	return d
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
	bracket *StartAge   // nil: no valid bracket, no FIRST_DOSE_MIN_AGE_ABOVE_START_AGE
	refs    []minAgeRef // empty: no FIRST_DOSE_MIN_AGE_DIFFERS
}

// warnings returns the dose-1 Warns that apply to series i. A series with
// an invalid bracket gets neither: only its span problem.
func (f startAgeFinding) warnings(sr Series, i int) firstDoseWarnings {
	if sr.StartAge != nil && !validBracket(sr.StartAge) {
		return firstDoseWarnings{}
	}
	return firstDoseWarnings{bracket: sr.StartAge, refs: f.refs[i]}
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
	for _, r := range w.refs {
		if !spanEqual(*a, r.span) {
			v.warn(FirstDoseMinAgeDiffers, p, ProblemParams{ID: r.id})
			break
		}
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
	v.tolerance(p+"/tolerance", d)
}

// tolerance checks a dose rule's tolerance (issue #29, Q80, Q89): a zero
// Span (Go-built only; JSON "P0D" is a valid zero tolerance) or an invalid
// one is INVALID_VALUE, as for buffer, and a valid one in months or years
// is TOLERANCE_UNIT. A usable tolerance that is not certainly below a
// present, valid minAge, minInterval or minIntervalFromFirst of the same
// rule gets one TOLERANCE_NOT_BELOW_MINIMUM Warn: the check it lowers would
// accept two doses on one day.
func (v *validator) tolerance(path string, d DoseRule) {
	t := d.Tolerance
	switch {
	case t == nil:
		return
	case !t.valid():
		v.reject(InvalidValue, path, ProblemParams{Value: t.String()})
		return
	case t.Unit != Days && t.Unit != Weeks:
		v.reject(ToleranceUnit, path, ProblemParams{Value: t.String()})
		return
	}
	for _, m := range []*Span{d.MinAge, d.MinInterval, d.MinIntervalFromFirst} {
		if m != nil && m.valid() && !spanLess(*t, *m) {
			v.warn(ToleranceNotBelowMinimum, path, ProblemParams{})
			return
		}
	}
}

// catchUp checks a series' catchUpAfterAge (issue #29, Q89): a zero Span
// (Go-built only) is REQUIRED and an invalid one INVALID_VALUE, as for
// maxAgeComplete. A usable one that is not certainly before a usable
// maxAgeComplete gets the CATCH_UP_AFTER_MAX_AGE Warn: the series would be
// out of age before the catch-up age is reached.
func (v *validator) catchUp(p string, sr Series) {
	cu, ok := v.maxAgeSpan(p+"/catchUpAfterAge", sr.CatchUpAfterAge)
	if !ok {
		return
	}
	if complete, ok := usableMaxAge(sr.MaxAgeComplete); ok && !spanLess(cu, complete) {
		v.warn(CatchUpAfterMaxAge, p+"/catchUpAfterAge", ProblemParams{})
	}
}

// date reports a zero (Go-built only) or invalid validity bound.
func (v *validator) date(path string, d *Date) {
	switch {
	case d == nil:
	case *d == Date{}:
		v.reject(Required, path, ProblemParams{})
	case !d.valid():
		v.reject(InvalidValue, path, ProblemParams{Value: d.String()})
	}
}

// optionalSpan reports a present span that is not valid.
func (v *validator) optionalSpan(path string, s *Span) {
	if s != nil && !s.valid() {
		v.reject(InvalidValue, path, ProblemParams{Value: s.String()})
	}
}
