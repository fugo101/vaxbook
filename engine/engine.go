package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/fugo101/vaxbook/ruleset"
)

// ErrUnsupported reports a valid rule set shape that this engine slice does
// not evaluate yet. In S3a: a disease with no series, and a product group
// (the series of one disease with the same product, or its generic series)
// with more than one series, which S3b tells apart by start age.
var ErrUnsupported = errors.New("engine: rule set shape not supported yet")

// Engine evaluates inputs against one rule set snapshot. New copies what it
// needs from the snapshot, so an Engine is immutable and safe for concurrent
// use.
type Engine struct {
	version  string
	diseases []diseaseModel
	known    map[string]bool
	products map[string]bool // every catalog product ID, retired ones included
}

type diseaseModel struct {
	id     string
	series []seriesModel // in rule-set order; candidates index it
	all    []int         // every series
	// tier maps each non-retired catalog product that lists the disease to
	// the candidates of a dose with it (SAD §5.3 step 3, A2): its own
	// series, else the generic series, else every series. A dose with any
	// other product, or none, has every series (EQ14, Q36).
	tier map[string][]int
	// noDose are the candidates when no dose is given: every series but
	// those of retired products, or every series when none is left (Q38).
	noDose []int
	// catalog are the non-retired products that list the disease, and
	// genericProducts those of them with no series of their own, in
	// snapshot order. A generic candidate offers genericProducts.
	catalog, genericProducts []string
}

type seriesModel struct {
	id      string
	product string // "" for a generic series
	retired bool   // the series' product is retired
	length  int    // doses per plan; every plan has the same count (EQ5)
	plans   []planModel
}

type planModel struct {
	id    string
	doses []doseRule
}

type optSpan struct {
	span ruleset.Span
	ok   bool
}

func optional(s *ruleset.Span) optSpan {
	if s == nil {
		return optSpan{}
	}
	return optSpan{span: *s, ok: true}
}

type doseRule struct {
	minAge, minInterval, minFromFirst, buffer optSpan
	// minAgeImplied: minAge is absent or implied by dose 1's minimum age and
	// the intervals (ruleset.MinAgeImplied), so it needs no birth date.
	minAgeImplied bool
	from          ruleset.Anchor
	offset        ruleset.Span
	// birthGap is offset_k − offset_{k−1} when both milestones count from
	// birth in the same unit family; without a birth date the due date is
	// dose k−1 + birthGap (EQ19).
	birthGap optSpan
}

// New validates a snapshot and prepares an Engine for it. A snapshot that
// ruleset.Validate rejects gives a *ruleset.RejectedError; a valid shape this
// slice cannot evaluate gives an error wrapping ErrUnsupported.
func New(s *ruleset.Snapshot) (*Engine, error) {
	if s == nil {
		return nil, errors.New("engine: nil rule set snapshot")
	}
	if rejects := ruleset.Rejects(ruleset.Validate(s)); len(rejects) > 0 {
		return nil, &ruleset.RejectedError{Problems: rejects}
	}
	e := &Engine{version: s.Version, known: make(map[string]bool, len(s.Diseases)), products: make(map[string]bool, len(s.Products))}
	for _, p := range s.Products {
		e.products[p.ID] = true
	}
	for _, dz := range s.Diseases {
		m, err := compileDisease(s, dz.ID)
		if err != nil {
			return nil, err
		}
		e.known[dz.ID] = true
		e.diseases = append(e.diseases, m)
	}
	return e, nil
}

// compileDisease prepares the series of one disease and its candidate
// tiers (SAD §5.3 step 3).
func compileDisease(s *ruleset.Snapshot, disease string) (diseaseModel, error) {
	retired := map[string]bool{}
	for _, p := range s.Products {
		retired[p.ID] = p.Retired
	}
	m := diseaseModel{id: disease, tier: map[string][]int{}}
	own := map[string][]int{} // product → its series for the disease
	var generic []int
	for _, sr := range s.Series {
		if sr.Disease != disease {
			continue
		}
		sm := compileSeries(sr)
		sm.retired = retired[sm.product]
		i := len(m.series)
		m.series = append(m.series, sm)
		m.all = append(m.all, i)
		if sm.product == "" {
			generic = append(generic, i)
		} else {
			own[sm.product] = append(own[sm.product], i)
		}
		if !sm.retired {
			m.noDose = append(m.noDose, i)
		}
	}
	switch {
	case len(m.series) == 0:
		return m, fmt.Errorf("%w: disease %q has no series", ErrUnsupported, disease)
	case len(generic) > 1:
		return m, fmt.Errorf("%w: disease %q: more than one generic series", ErrUnsupported, disease)
	}
	for _, sm := range m.series {
		if sm.product != "" && len(own[sm.product]) > 1 {
			return m, fmt.Errorf("%w: disease %q: product %q has more than one series", ErrUnsupported, disease, sm.product)
		}
	}
	if len(m.noDose) == 0 {
		m.noDose = m.all
	}
	for _, p := range s.Products {
		if p.Retired || !slices.Contains(p.Diseases, disease) {
			continue
		}
		m.catalog = append(m.catalog, p.ID)
		switch {
		case len(own[p.ID]) > 0:
			m.tier[p.ID] = own[p.ID]
		case len(generic) > 0:
			m.tier[p.ID] = generic
			m.genericProducts = append(m.genericProducts, p.ID)
		default:
			m.tier[p.ID] = m.all
		}
	}
	return m, nil
}

func compileSeries(sr ruleset.Series) seriesModel {
	m := seriesModel{id: sr.ID, length: len(sr.Plans[0].Doses)}
	if sr.Product != nil {
		m.product = *sr.Product
	}
	for _, p := range sr.Plans {
		pm := planModel{id: p.ID}
		for k, r := range p.Doses {
			d := doseRule{
				minAge:        optional(r.MinAge),
				minInterval:   optional(r.MinInterval),
				minFromFirst:  optional(r.MinIntervalFromFirst),
				buffer:        optional(r.Buffer),
				minAgeImplied: ruleset.MinAgeImplied(p, k+1),
				from:          r.Recommended.From,
				offset:        r.Recommended.Offset,
			}
			if k > 0 && r.Recommended.From == ruleset.FromBirth && p.Doses[k-1].Recommended.From == ruleset.FromBirth {
				d.birthGap = spanGap(p.Doses[k-1].Recommended.Offset, r.Recommended.Offset)
			}
			pm.doses = append(pm.doses, d)
		}
		m.plans = append(m.plans, pm)
	}
	return m
}

// spanGap returns b − a when both spans are in the same unit family (months
// and years, or days and weeks).
func spanGap(a, b ruleset.Span) optSpan {
	months := func(s ruleset.Span) (int, bool) {
		switch s.Unit {
		case ruleset.Months:
			return s.N, true
		case ruleset.Years:
			return 12 * s.N, true
		}
		return 0, false
	}
	days := func(s ruleset.Span) (int, bool) {
		switch s.Unit {
		case ruleset.Days:
			return s.N, true
		case ruleset.Weeks:
			return 7 * s.N, true
		}
		return 0, false
	}
	if ma, ok := months(a); ok {
		if mb, ok := months(b); ok {
			return optSpan{span: ruleset.Span{N: mb - ma, Unit: ruleset.Months}, ok: true}
		}
	}
	if da, ok := days(a); ok {
		if db, ok := days(b); ok {
			return optSpan{span: ruleset.Span{N: db - da, Unit: ruleset.Days}, ok: true}
		}
	}
	return optSpan{}
}

// givenDose is a dose record as one disease sees it.
type givenDose struct {
	id        string
	date      Range // today-capped; zero when not dated
	dated     bool  // precision day, month or year; false for level 4
	imprecise bool  // precision other than day
	number    int   // the user-entered dose number for this disease, 0 if none
	// problems are the codes of the record's record-scoped problems, in
	// path order (step 1).
	problems []ReasonCode
	product  string // the catalog product ID, "" for none or free text
	free     bool   // an off-catalog product (freeProduct)
}

// birthRange is the birth date as the engine uses it (EQ17): its period
// capped at today, or no birth date.
type birthRange struct {
	r         Range
	known     bool
	precision Precision
}

// Evaluate assesses every disease of the rule set for one profile, in the
// rule set's disease order. It is pure: no clock reads, no I/O, and it never
// coerces input values.
//
// It returns an *InputError holding the ScopeInput problems of Validate when
// there are any. A ScopeRecord problem (a dose date that is unparseable, out
// of range, after today or before birth, a level-4 dose with a value, or a
// product the rule set does not have) makes every disease of that record ⚪
// with a reason of the same code instead (A9, EQ27, issue #14 Q35).
func (e *Engine) Evaluate(in Input) (Output, error) {
	v := e.validate(in)
	var fatal []Problem
	for _, p := range v.problems {
		if p.Scope == ScopeInput {
			fatal = append(fatal, p)
		}
	}
	if len(fatal) > 0 {
		return Output{}, &InputError{Problems: fatal}
	}
	birth := birthRange{precision: in.Profile.Birth.Precision}
	period, dated, err := in.Profile.Birth.Period()
	if err != nil {
		return Output{}, fmt.Errorf("engine: birth date passed validation but has no period: %w", err)
	}
	if dated {
		birth.r, birth.known = period.CapAt(in.Today), true
	}
	byDisease := make(map[string][]givenDose, len(e.diseases))
	for i, r := range in.Doses {
		g := givenDose{id: r.ID, imprecise: r.Date.Precision != Day, free: r.FreeProduct != nil}
		if r.Product != nil {
			g.product = *r.Product
		}
		if codes, ok := v.records[i]; ok {
			for _, c := range codes {
				g.problems = append(g.problems, ReasonCode(c))
			}
		} else {
			period, dated, err := r.Date.Period()
			if err != nil {
				return Output{}, fmt.Errorf("engine: dose date passed validation but has no period: %w", err)
			}
			if dated {
				g.date, g.dated = period.CapAt(in.Today), true
			}
		}
		for _, dz := range r.Diseases {
			gd := g
			gd.number = int(r.DoseNumbers[dz])
			byDisease[dz] = append(byDisease[dz], gd)
		}
	}
	out := Output{RuleSetVersion: e.version, Today: in.Today, Assessments: make([]Assessment, 0, len(e.diseases))}
	for _, dz := range e.diseases {
		out.Assessments = append(out.Assessments, dz.assess(birth, in.Today, byDisease[dz.id]))
	}
	return out, nil
}

// assess runs SAD §5.3 for one disease.
func (dz diseaseModel) assess(birth birthRange, today Date, doses []givenDose) Assessment {
	a := Assessment{Disease: dz.id}

	// Step 1 (EQ20d): estimated when the birth date or a dose date of the
	// disease is not day-precise; step 2 adds implied doses and step 6
	// PRODUCT_NOT_IN_CATALOG.
	a.Estimated = birth.precision != Day || slices.ContainsFunc(doses, func(g givenDose) bool { return g.imprecise })

	// Step 1: a dose record whose date is unparseable, out of range, after
	// today or before birth, a level-4 dose with a value, or a record with a
	// product the rule set does not have, makes the disease ⚪ (A9, EQ27,
	// Q35). Steps 2–9 are skipped. The ⚪ is a data ⚪ (step 10 layer 7)
	// with the record reasons, sorted by record ID and then in path order;
	// S3a has no higher layer, so it returns here. A slice that adds layers
	// 1–5 must let them win over it and keep these reasons as masked
	// reasons (SAD §5.3 steps 1 and 10).
	var bad []givenDose
	for _, g := range doses {
		if len(g.problems) > 0 {
			bad = append(bad, g)
		}
	}
	if len(bad) > 0 {
		slices.SortFunc(bad, func(x, y givenDose) int { return strings.Compare(x.id, y.id) })
		a.Bucket = BucketUnknown
		for _, g := range bad {
			for _, c := range g.problems {
				a.Reasons = append(a.Reasons, Reason{Code: c, Params: ReasonParams{Dose: g.id}})
			}
		}
		return a
	}

	// Step 2: positions. The blocking outcomes do not depend on the series,
	// so they are reported once; a step-2 ⚪ skips steps 3–9, like a step-1
	// ⚪.
	arr := arrange(doses)
	if arr.blocked() {
		a.Bucket, a.Reasons = BucketUnknown, arr.reasons
		return a
	}
	// With no dose and no birth date, step 7 stops with NO_BIRTH_DATE
	// whatever the series (EQ19), so no candidates are chosen.
	if len(doses) == 0 && !birth.known {
		a.Bucket, a.Reasons = BucketUnknown, []Reason{{Code: ReasonNoBirthDate}}
		return a
	}

	// Step 3: the candidate series. Implied doses are unknown-product doses
	// (Q30); whether there are any is decided from the records alone, with
	// no series length.
	implied := len(arr.forLength(math.MaxInt).implied) > 0
	ev := evaluation{dz: dz, cands: dz.candidates(doses, implied), today: today}
	longest := 0
	refs := make([]SeriesRef, 0, len(ev.cands))
	for _, c := range ev.cands {
		s := dz.series[c]
		a.SeriesUsed = append(a.SeriesUsed, s.id)
		refs = append(refs, SeriesRef{Product: s.product, Series: s.id})
		longest = max(longest, s.length)
		ev.pls = append(ev.pls, arr.forLength(s.length))
	}
	// IMPLIED_EARLIER_DOSES once, capped at the longest candidate: for one
	// series, the S2 output.
	if numbers := arr.forLength(longest).implied; len(numbers) > 0 {
		a.Estimated = true
		a.Reasons = append(a.Reasons, Reason{Code: ReasonImpliedEarlierDoses, Params: ReasonParams{Numbers: numbers}})
	}
	// SAFEST_SERIES: with doses given the candidates do not depend on the
	// birth end, so it is emitted once, before step 4; with no dose given
	// it is a bucket reason of each end.
	if len(refs) > 1 {
		safest := Reason{Code: ReasonSafestSeries, Params: ReasonParams{Candidates: refs}}
		if len(doses) > 0 {
			a.Reasons = append(a.Reasons, safest)
		} else {
			ev.safest = &safest
		}
	}
	for _, g := range doses {
		if g.free {
			ev.free = append(ev.free, g.id)
		}
	}
	slices.Sort(ev.free)

	// Step 4: steps 5–9 at each end of the birth-date range, or once.
	var res endResult
	switch {
	case !birth.known:
		res = ev.at(birthEnd{})
	case birth.r.IsExact():
		res = ev.at(birthEnd{date: birth.r.Lo, known: true})
	default:
		hint := HintFullBirthDate
		if birth.precision == Year {
			hint = HintBirthMonth
		}
		res = compareEnds(ev.at(birthEnd{date: birth.r.Lo, known: true}), ev.at(birthEnd{date: birth.r.Hi, known: true}), hint)
	}
	a.Reasons = append(a.Reasons, res.reasons...)
	a.Bucket, a.Block = res.bucket, res.block
	if res.bucket != BucketUnknown && res.hasNext {
		nd := res.next
		a.NextDose = &nd
		a.Products = res.products
	}
	if slices.ContainsFunc(a.Reasons, func(r Reason) bool { return r.Code == ReasonProductNotInCatalog }) {
		a.Estimated = true
	}
	return a
}
