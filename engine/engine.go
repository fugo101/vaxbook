package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/fugo101/vaxbook/ruleset"
)

// Engine evaluates inputs against one rule set snapshot. New copies what it
// needs from the snapshot, so an Engine is immutable and safe for concurrent
// use.
type Engine struct {
	version  string
	diseases []diseaseTimeline // per validity segment (issue #29)
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
	// fallsThrough marks the catalog products whose tier is the every-series
	// fall-through (no series of their own and no generic series), which a
	// version of another segment never crosses groups through (issue #29
	// Q94).
	fallsThrough map[string]bool
	// noDose are the candidates when no dose is given: every series but
	// those of retired products, or every series when none is left (Q38).
	noDose []int
	// catalog are the non-retired products that list the disease, and
	// genericProducts those of them with no series of their own, in
	// snapshot order. A generic candidate offers genericProducts.
	catalog, genericProducts []string
	// groups are the product groups of the disease (the series with the
	// same product, or its generic series), in rule-set order of each
	// group's first series; groupOf maps a series to its group. Every
	// candidate list above is a union of whole groups, since a tier is a
	// group or every series and retirement is per product.
	groups  []groupModel
	groupOf []int
	// listing holds every catalog product that lists the disease, retired
	// ones included. It decides only whether a dose's product is named or
	// unknown for CHECK_PRODUCT_NAME and MIXED_PRODUCTS (anyUnknownProduct),
	// and is separate from the candidate tiers (issue #15, Q53).
	listing map[string]bool
}

// groupModel is one product group. Its series are in bracket order: by
// startAge.min, an open-ended bracket last (ruleset.Validate makes the
// brackets of a group of several series disjoint for every month length,
// so the order is total). A group of one series may have no bracket (C8).
// Each bracket's floor is set from this order (compileDisease).
type groupModel struct {
	series []int
}

type seriesModel struct {
	id      string
	product string // "" for a generic series
	retired bool   // the series' product is retired
	length  int    // doses per plan; every plan has the same count (EQ5)
	start   bracket
	plans   []planModel
	// maxStart and maxComplete are the series' maximum ages, absent for no
	// limit (exclusive bounds; SAD §5.2, issue #15). groupMax is the max of
	// the last bracket of the series' product group in bracket order, when
	// it has one: a maximum age to start for dose 1 of every series of the
	// group (A5, Q55). All three are series-level, not per plan.
	maxStart, maxComplete, groupMax optSpan
	// catchUp is catchUpAfterAge, absent for none: a clean next dose whose
	// earliest shown date is at or after birth + catchUp is handed to a
	// doctor (evaluation.catchUp; issue #29 In scope 5).
	catchUp optSpan
	// versions are the series of the disease's other validity segments that
	// may count a dose of this series as the rule in effect on its date
	// (version, issue #29 In scope 3), and cuts their segment bounds,
	// ascending. bare are the same versions with no maximum age, for the
	// walk of the start-age bracket choice (withoutMaxAges). All are nil on
	// a windowless snapshot.
	versions, bare []version
	cuts           []Date
}

// hasMaxAges reports whether the series has any maximum age. Without one,
// no S4a check runs and the walk is S3b's.
func (s seriesModel) hasMaxAges() bool {
	return s.maxStart.ok || s.maxComplete.ok || s.groupMax.ok
}

// withoutMaxAges returns s with no maximum age: the walk of the start-age
// bracket choice (issue #15 In scope 2), so that the series chosen and the
// item counted at position 1 stay S3b's.
func (s seriesModel) withoutMaxAges() seriesModel {
	s.maxStart, s.maxComplete, s.groupMax = optSpan{}, optSpan{}, optSpan{}
	s.versions = s.bare
	return s
}

// startLimit returns dose 1's start limit for a person born on b: the
// earlier of b + maxAgeStart and b + the group's last bracket max, each when
// present (Q55). Spans of different units are compared as dates at b.
func (s seriesModel) startLimit(b Date) (Date, bool) { return s.startLimitAt(b, b) }

// startLimitAt is startLimit with maxAgeStart counted from ageBirth and the
// bracket max from bracketBirth: the next dose's start limit uses b_old and
// b_s (issue #28, Q65, Q73).
func (s seriesModel) startLimitAt(ageBirth, bracketBirth Date) (Date, bool) {
	var lim Date
	ok := false
	for _, t := range []struct {
		m optSpan
		b Date
	}{{s.maxStart, ageBirth}, {s.groupMax, bracketBirth}} {
		if !t.m.ok {
			continue
		}
		if d := t.b.AddSpan(t.m.span); !ok || d.Before(lim) {
			lim, ok = d, true
		}
	}
	return lim, ok
}

// past checks a dose range x against a maximum age whose limit date is lim.
// Maximum ages are exclusive: the check fails definitely when x.Lo ≥ lim,
// possibly when only x.Hi ≥ lim. It mirrors against for the inclusive
// minimums.
func past(x Range, lim Date) verdict {
	switch {
	case !x.Lo.Before(lim):
		return verdictDefinite
	case !x.Hi.Before(lim):
		return verdictPossible
	}
	return verdictOK
}

// maxAges checks a dose on range x at position pos against the maximum ages
// at birth end b (SAD §5.3 step 5, F05.10, EQ7): the start limit for
// position 1 (CheckMaxAgeStart) and maxAgeComplete for positions 1…length
// (CheckMaxAgeComplete). It returns the checks failed definitely and those
// failed possibly, each in catalog order. With no birth date nothing is
// checked (EQ19, A6). For an undated slot x is the range of its earliest
// chain dates (issue #15 In scope 4).
func (s seriesModel) maxAges(pos int, b birthEnd, x Range) (def, poss []Check) {
	if !b.known || pos < 1 || pos > s.length {
		return nil, nil
	}
	add := func(v verdict, c Check) {
		switch v {
		case verdictDefinite:
			def = append(def, c)
		case verdictPossible:
			poss = append(poss, c)
		}
	}
	if pos == 1 {
		if lim, ok := s.startLimit(b.date); ok {
			add(past(x, lim), CheckMaxAgeStart)
		}
	}
	if s.maxComplete.ok {
		add(past(x, b.date.AddSpan(s.maxComplete.span)), CheckMaxAgeComplete)
	}
	return def, poss
}

// bracket is a series' start-age bracket [min, max) of the age at dose 1
// (SAD §5.2, issue #23). ok is false when the series has no startAge: it
// matches every age (C8).
//
// For choosing, a bracket covers its reach [floor, max): floor is the max of
// the bracket before it in its group's order, absent for the first, whose
// reach starts at birth. Validate puts each max at or below the next min,
// so the reaches of a group tile every age below the last max, and a gap
// between two brackets belongs to the one above it (Q33; PR #27).
type bracket struct {
	ok    bool
	min   ruleset.Span
	max   optSpan // absent: open-ended (Q25)
	floor optSpan // matching lower bound: the previous bracket's max in the group's order; absent for the first
}

// mayContain reports whether the bracket's reach possibly contains the age
// at a dose on range d, for a birth date in birth (today-capped): when floor
// is set, d.Hi ≥ birth.Lo + floor, and when max is set, d.Lo < birth.Hi +
// max (SAD §5.3 step 3, issue #23 In scope 4, Q33). Each test is monotone
// in d, so a wider range only passes more of them.
func (br bracket) mayContain(d, birth Range) bool {
	if !br.ok {
		return true
	}
	return (!br.floor.ok || !d.Hi.Before(birth.Lo.AddSpan(br.floor.span))) && (!br.max.ok || d.Lo.Before(birth.Hi.AddSpan(br.max.span)))
}

// openAt reports whether a person born on b can still start the series
// today: no bracket or no max, or b + max after today (EQ10).
func (br bracket) openAt(b, today Date) bool {
	return !br.ok || !br.max.ok || b.AddSpan(br.max.span).After(today)
}

// before reports whether bracket a comes before bracket b in a validated
// group: a has a max that is at most b's min.
func (br bracket) before(b bracket) bool {
	return br.max.ok && ruleset.SpanAtMost(br.max.span, b.min)
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
	// tol is the rule's tolerance in days (weeks × 7; 0 without one): its
	// minimum thresholds are (anchor + span) − tol when a given dose is
	// validated (check, the step-5 chain), never for a next-dose date or
	// limit (F05.10, EQ27; issue #29 In scope 6). Validate allows only days
	// and weeks (Q80).
	tol int
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
// ruleset.Validate rejects gives a *ruleset.RejectedError. Every valid
// snapshot is accepted: since S4c a disease with no series in effect is
// assessed ⚪ NO_SERIES_IN_EFFECT (issue #29, Q81).
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
		tl, err := compileTimeline(s, dz.ID)
		if err != nil {
			return nil, err
		}
		e.known[dz.ID] = true
		e.diseases = append(e.diseases, tl)
	}
	return e, nil
}

// compileDisease prepares the series of one disease and its candidate
// tiers (SAD §5.3 step 3), from the series of the snapshot for which
// inEffect (an index into s.Series) holds: those of one validity segment
// (compileTimeline).
func compileDisease(s *ruleset.Snapshot, disease string, inEffect func(int) bool) diseaseModel {
	retired := map[string]bool{}
	for _, p := range s.Products {
		retired[p.ID] = p.Retired
	}
	m := diseaseModel{id: disease, tier: map[string][]int{}, fallsThrough: map[string]bool{}, listing: map[string]bool{}}
	for _, p := range s.Products {
		if slices.Contains(p.Diseases, disease) {
			m.listing[p.ID] = true
		}
	}
	own := map[string][]int{} // product → its series for the disease
	var generic []int
	group := map[string]int{} // product ("" generic) → index into m.groups
	for j, sr := range s.Series {
		if sr.Disease != disease || !inEffect(j) {
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
		g, ok := group[sm.product]
		if !ok {
			g = len(m.groups)
			group[sm.product] = g
			m.groups = append(m.groups, groupModel{})
		}
		m.groups[g].series = append(m.groups[g].series, i)
		m.groupOf = append(m.groupOf, g)
	}
	for _, g := range m.groups {
		slices.SortStableFunc(g.series, func(a, b int) int {
			switch sa, sb := m.series[a].start, m.series[b].start; {
			case sa.before(sb):
				return -1
			case sb.before(sa):
				return 1
			}
			return 0
		})
		for j := 1; j < len(g.series); j++ {
			m.series[g.series[j]].start.floor = m.series[g.series[j-1]].start.max
		}
		// The group's brackets end at its last bracket's max: a maximum age
		// to start for dose 1 of each of its series (A5, Q55). A non-last
		// bracket's max is no limit: a dose 1 past it is in the reach of the
		// bracket above. Retirement is per product, so the group is whole.
		if last := m.series[g.series[len(g.series)-1]].start; last.ok && last.max.ok {
			for _, i := range g.series {
				m.series[i].groupMax = last.max
			}
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
			m.fallsThrough[p.ID] = true
		}
	}
	return m
}

func compileSeries(sr ruleset.Series) seriesModel {
	m := seriesModel{id: sr.ID, length: len(sr.Plans[0].Doses), maxStart: optional(sr.MaxAgeStart), maxComplete: optional(sr.MaxAgeComplete),
		catchUp: optional(sr.CatchUpAfterAge)}
	if sr.Product != nil {
		m.product = *sr.Product
	}
	if sa := sr.StartAge; sa != nil {
		m.start = bracket{ok: true, min: sa.Min, max: optional(sa.Max)}
	}
	for _, p := range sr.Plans {
		pm := planModel{id: p.ID}
		for k, r := range p.Doses {
			d := doseRule{
				minAge:        optional(r.MinAge),
				minInterval:   optional(r.MinInterval),
				minFromFirst:  optional(r.MinIntervalFromFirst),
				buffer:        optional(r.Buffer),
				tol:           toleranceDays(r.Tolerance),
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

// toleranceDays returns a tolerance in days: n for P<n>D, 7n for P<n>W, 0
// when absent. ruleset.Validate rejects any other unit (TOLERANCE_UNIT).
func toleranceDays(s *ruleset.Span) int {
	switch {
	case s == nil:
		return 0
	case s.Unit == ruleset.Weeks:
		return 7 * s.N
	}
	return s.N
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
	birth, byDisease, err := e.prepare(in)
	if err != nil {
		return Output{}, err
	}
	out := Output{RuleSetVersion: e.version, Today: in.Today, Assessments: make([]Assessment, 0, len(e.diseases))}
	for _, tl := range e.diseases {
		dz := tl.at(in.Today)
		a, _ := dz.assess(birth, in.Today, byDisease[dz.id])
		out.Assessments = append(out.Assessments, a)
	}
	return out, nil
}

// prepare validates an input and converts it to the birth range and the
// dose records of each disease, as Evaluate uses them.
func (e *Engine) prepare(in Input) (birthRange, map[string][]givenDose, error) {
	v := e.validate(in)
	var fatal []Problem
	for _, p := range v.problems {
		if p.Scope == ScopeInput {
			fatal = append(fatal, p)
		}
	}
	if len(fatal) > 0 {
		return birthRange{}, nil, &InputError{Problems: fatal}
	}
	birth := birthRange{precision: in.Profile.Birth.Precision}
	period, dated, err := in.Profile.Birth.Period()
	if err != nil {
		return birthRange{}, nil, fmt.Errorf("engine: birth date passed validation but has no period: %w", err)
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
				return birthRange{}, nil, fmt.Errorf("engine: dose date passed validation but has no period: %w", err)
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
	return birth, byDisease, nil
}

// assess runs SAD §5.3 for one disease. It also returns the result of steps
// 4–9 the assessment is built from (zero when step 1 or 2 stops).
func (dz diseaseModel) assess(birth birthRange, today Date, doses []givenDose) (Assessment, endResult) {
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
	// S3b has no higher layer, so it returns here. A slice that adds layers
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
		return a, endResult{}
	}

	// Step 2: positions. The blocking outcomes do not depend on the series,
	// so they are reported once; a step-2 ⚪ skips steps 3–9, like a step-1
	// ⚪.
	arr := arrange(doses)
	if arr.blocked() {
		a.Bucket, a.Reasons = BucketUnknown, arr.reasons
		return a, endResult{}
	}
	// Step 3, effective dating: with no series in effect today the disease
	// has no rule to judge the history against, even a history complete
	// under a series that has ended (EQ26, issue #29 Q82). It is never
	// dropped: ⚪ NO_SERIES_IN_EFFECT is its only reason, decided before the
	// no-birth-date stop below, with no candidates, so no seriesUsed,
	// products, next dose or IMPLIED_EARLIER_DOSES.
	if len(dz.series) == 0 {
		a.Bucket, a.Reasons = BucketUnknown, []Reason{{Code: ReasonNoSeriesInEffect}}
		return a, endResult{}
	}
	// With no dose and no birth date, step 7 stops with NO_BIRTH_DATE
	// whatever the series (EQ19), so no candidates are chosen.
	if len(doses) == 0 && !birth.known {
		a.Bucket, a.Reasons = BucketUnknown, []Reason{{Code: ReasonNoBirthDate}}
		return a, endResult{}
	}

	// Step 3: the candidate series. Implied doses are unknown-product doses
	// (Q30); whether there are any is decided from the records alone, with
	// no series length. S3a's tiers give a union of product groups; start-age
	// brackets then choose within each group (Q39).
	var at func(birthEnd) endResult
	if len(doses) > 0 {
		// Doses given: the brackets are chosen once, from the age at dose 1,
		// so both birth ends share the candidates.
		implied := len(arr.forLength(math.MaxInt).implied) > 0
		cands, uncertain := dz.byAgeAtDoseOne(arr, dz.candidates(doses, implied), birth, today)
		// IMPLIED_EARLIER_DOSES once, capped at the longest candidate after
		// the bracket choice (issue #23, Q48 (a)): for one series, the S2
		// output.
		longest := 0
		for _, c := range cands {
			longest = max(longest, dz.series[c].length)
		}
		if numbers := arr.forLength(longest).implied; len(numbers) > 0 {
			a.Estimated = true
			a.Reasons = append(a.Reasons, Reason{Code: ReasonImpliedEarlierDoses, Params: ReasonParams{Numbers: numbers}})
		}
		a.Reasons = append(a.Reasons, uncertain...)
		// SAFEST_SERIES: with doses given the candidates do not depend on
		// the birth end, so it is emitted once, before step 4; with no dose
		// given it is a bucket reason of each birth date (evaluate).
		if len(cands) > 1 {
			a.Reasons = append(a.Reasons, Reason{Code: ReasonSafestSeries, Params: ReasonParams{Candidates: dz.refs(cands)}})
		}
		var free []string
		for _, g := range doses {
			if g.free {
				free = append(free, g.id)
			}
		}
		slices.Sort(free)
		ev := dz.evaluate(arr, cands, today, free, false)
		// Every implied dose counts as an unknown product (issue #15 In scope
		// 6, Q30), as in the candidate rule above: one numbered above every
		// candidate's length widens the candidates although
		// IMPLIED_EARLIER_DOSES does not list it.
		ev.unknownProduct = dz.anyUnknownProduct(doses, implied)
		at = ev.at
	} else {
		// No dose given: each birth date chooses its brackets from its own
		// age today (EQ10); a date with none left is outOfAge (Q24). Since
		// S4b the candidates already out of age are removed (A3).
		at = func(b birthEnd) endResult { return dz.noDoseResult(arr, b, today) }
	}

	// Step 4: steps 5–9 at each end of the birth-date range, or once. With
	// no dose given the range is also split where the start-age choice
	// changes, and both ends of every sub-range are evaluated (PR #27).
	// Every evaluated date also carries the whole range's ends, from which
	// the maximum ages of the next dose are computed, and the older end of
	// its sub-range (issue #28, Q65, Q73).
	var res endResult
	switch {
	case !birth.known:
		res = at(birthEnd{})
	case birth.r.IsExact():
		res = at(birthEnd{date: birth.r.Lo, known: true})
	default:
		hint := HintFullBirthDate
		if birth.precision == Year {
			hint = HintBirthMonth
		}
		subs := []Range{birth.r}
		if len(doses) == 0 {
			subs = dz.noDoseSubRanges(birth.r, today)
		}
		var rs []endResult
		for _, sub := range subs {
			for _, p := range slices.Compact([]Date{sub.Lo, sub.Hi}) {
				rs = append(rs, at(birthEnd{date: p, known: true, old: birth.r.Lo, late: birth.r.Hi, start: sub.Lo}))
			}
		}
		res = compareResults(rs, hint)
	}
	a.Reasons = append(a.Reasons, res.reasons...)
	a.Bucket, a.Block = res.bucket, res.block
	// seriesUsed is the final result's candidates: the latest birth date's
	// when the results differ (Q40), none for an outOfAge with no dose given
	// (with doses given the doses fixed the candidates, Q74).
	if res.block != BlockOutOfAge || len(doses) > 0 {
		for _, c := range res.series {
			a.SeriesUsed = append(a.SeriesUsed, dz.series[c].id)
		}
	}
	if res.bucket != BucketUnknown && res.hasNext {
		nd := res.next
		a.NextDose = &nd
		a.Products = res.products
	}
	if slices.ContainsFunc(a.Reasons, func(r Reason) bool { return r.Code == ReasonProductNotInCatalog }) {
		a.Estimated = true
	}
	return a, res
}
