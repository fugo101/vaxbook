package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fugo101/vaxbook/ruleset"
)

// ErrUnsupported reports a valid rule set shape that this engine slice does
// not evaluate yet (S1, S2: exactly one series per disease).
var ErrUnsupported = errors.New("engine: rule set shape not supported yet")

// Engine evaluates inputs against one rule set snapshot. New copies what it
// needs from the snapshot, so an Engine is immutable and safe for concurrent
// use.
type Engine struct {
	version  string
	diseases []diseaseModel
	known    map[string]bool
}

type diseaseModel struct {
	id     string
	series seriesModel
}

type seriesModel struct {
	id     string
	length int // doses per plan; every plan has the same count (EQ5)
	plans  []planModel
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
	e := &Engine{version: s.Version, known: make(map[string]bool, len(s.Diseases))}
	for _, dz := range s.Diseases {
		var found []ruleset.Series
		for _, sr := range s.Series {
			if sr.Disease == dz.ID {
				found = append(found, sr)
			}
		}
		if len(found) != 1 {
			return nil, fmt.Errorf("%w: disease %q has %d series, S2 needs exactly one", ErrUnsupported, dz.ID, len(found))
		}
		e.known[dz.ID] = true
		e.diseases = append(e.diseases, diseaseModel{id: dz.ID, series: compileSeries(found[0])})
	}
	return e, nil
}

func compileSeries(sr ruleset.Series) seriesModel {
	m := seriesModel{id: sr.ID, length: len(sr.Plans[0].Doses)}
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
	problem   ReasonCode
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
// of range, after today or before birth, or a level-4 dose with a value)
// makes every disease of that record ⚪ with a reason of the same code
// instead (A9, EQ27).
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
		g := givenDose{id: r.ID, imprecise: r.Date.Precision != Day}
		if code, ok := v.records[i]; ok {
			g.problem = ReasonCode(code)
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
	// disease is not day-precise; step 2 adds implied doses.
	a.Estimated = birth.precision != Day || slices.ContainsFunc(doses, func(g givenDose) bool { return g.imprecise })

	// Step 1: a dose record whose date is unparseable, out of range, after
	// today or before birth, or a level-4 dose with a value, makes the
	// disease ⚪ (A9, EQ27). Steps 2–9 are skipped. The ⚪ is a data ⚪ (step
	// 10 layer 7) with the record reasons, sorted by record ID; S2 has no
	// higher layer, so it returns here. A slice that adds layers 1–5 must
	// let them win over it and keep these reasons as masked reasons (SAD
	// §5.3 steps 1 and 10).
	var bad []givenDose
	for _, g := range doses {
		if g.problem != "" {
			bad = append(bad, g)
		}
	}
	if len(bad) > 0 {
		slices.SortFunc(bad, func(x, y givenDose) int { return strings.Compare(x.id, y.id) })
		a.Bucket = BucketUnknown
		for _, g := range bad {
			a.Reasons = append(a.Reasons, Reason{Code: g.problem, Params: ReasonParams{Dose: g.id}})
		}
		return a
	}

	// Step 2: positions. A step-2 ⚪ skips steps 5–9, like a step-1 ⚪.
	pl := dz.series.place(doses)
	if pl.blocked() {
		a.Bucket, a.Reasons = BucketUnknown, pl.reasons
		return a
	}
	// Step 3: the candidate series. S2 has one series per disease. With no
	// dose and no birth date, step 7 stops with NO_BIRTH_DATE before any
	// series is chosen.
	if len(doses) > 0 || birth.known {
		a.SeriesUsed = []string{dz.series.id}
	}
	if len(pl.implied) > 0 {
		a.Estimated = true
		a.Reasons = append(a.Reasons, Reason{Code: ReasonImpliedEarlierDoses, Params: ReasonParams{Numbers: pl.implied}})
	}

	// Step 4: steps 5–9 at each end of the birth-date range, or once.
	var res endResult
	switch {
	case !birth.known:
		res = dz.series.evaluate(birthEnd{}, today, pl)
	case birth.r.IsExact():
		res = dz.series.evaluate(birthEnd{date: birth.r.Lo, known: true}, today, pl)
	default:
		hint := HintFullBirthDate
		if birth.precision == Year {
			hint = HintBirthMonth
		}
		res = compareEnds(
			dz.series.evaluate(birthEnd{date: birth.r.Lo, known: true}, today, pl),
			dz.series.evaluate(birthEnd{date: birth.r.Hi, known: true}, today, pl),
			hint)
	}
	a.Reasons = append(a.Reasons, res.reasons...)
	a.Bucket, a.Block = res.bucket, res.block
	if res.bucket != BucketUnknown && res.hasNext {
		nd := res.next
		a.NextDose = &nd
	}
	return a
}

// evaluate runs steps 5–9 at one birth end. Step 10 has one layer in S2:
// any blocking reason makes the result ⚪ (layer 7, data ⚪), and the
// masked bucket's reasons stay.
func (s seriesModel) evaluate(b birthEnd, today Date, pl placement) endResult {
	// Step 5: dose validity.
	w := s.walk(b, today, pl)
	res := endResult{k: w.k, reasons: w.reasons}

	// Step 6: complete.
	if w.k > s.length {
		res.complete = true
		res.bucket = BucketComplete
		res.reasons = append(res.reasons, Reason{Code: ReasonComplete})
		if blocking(res.reasons) {
			res.bucket = BucketUnknown
		}
		return res
	}

	// Step 7: next dose, merged over alternative plans. A ⚪ assessment does
	// not emit it (SAD §5.1): the dates still decide the masked bucket.
	n := s.next(b, w)
	res.reasons = append(res.reasons, n.reasons...)
	if n.stop {
		res.bucket = BucketUnknown
		return res
	}
	if len(s.plans) > 1 {
		res.reasons = append(res.reasons, Reason{Code: ReasonAlternativePlans, Params: ReasonParams{Series: s.id}})
	}

	// Step 9: bucket; step 10: a blocking reason masks it.
	bucket, block, reason := classify(today, n.next, w.k > 1)
	if reason != "" {
		res.reasons = append(res.reasons, Reason{Code: reason})
	}
	res.next, res.hasNext = n.next, true
	if blocking(res.reasons) {
		res.bucket = BucketUnknown
		return res
	}
	res.bucket, res.block = bucket, block
	return res
}
