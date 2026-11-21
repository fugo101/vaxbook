package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fugo101/vaxbook/ruleset"
)

// ErrUnsupported reports a valid rule set shape that this engine slice does
// not evaluate yet (S1: exactly one series per disease).
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
	minAge, minInterval, buffer optSpan
	from                        ruleset.Anchor
	offset                      ruleset.Span
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
			return nil, fmt.Errorf("%w: disease %q has %d series, S1 needs exactly one", ErrUnsupported, dz.ID, len(found))
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
		for _, r := range p.Doses {
			pm.doses = append(pm.doses, doseRule{
				minAge:      optional(r.MinAge),
				minInterval: optional(r.MinInterval),
				buffer:      optional(r.Buffer),
				from:        r.Recommended.From,
				offset:      r.Recommended.Offset,
			})
		}
		m.plans = append(m.plans, pm)
	}
	return m
}

// givenDose is a dose record as one disease sees it.
type givenDose struct {
	id      string
	date    Range
	invalid bool // the date cannot be parsed (A9)
}

// Evaluate assesses every disease of the rule set for one profile, in the
// rule set's disease order. It is pure: no clock reads, no I/O, and it never
// coerces input values.
//
// An input with any problem from Validate gives an *InputError holding those
// problems, except an unparseable dose date: that record's diseases become
// ⚪ INVALID_DATE instead (A9).
func (e *Engine) Evaluate(in Input) (Output, error) {
	var fatal []Problem
	for _, p := range e.Validate(in) {
		if p.Code == InvalidDate && strings.HasPrefix(p.Path, "/doses/") && strings.HasSuffix(p.Path, "/date/value") {
			continue
		}
		fatal = append(fatal, p)
	}
	if len(fatal) > 0 {
		return Output{}, &InputError{Problems: fatal}
	}
	birth, err := in.Profile.Birth.Range()
	if err != nil {
		return Output{}, fmt.Errorf("engine: birth date passed validation but has no range: %w", err)
	}
	byDisease := make(map[string][]givenDose, len(e.diseases))
	for _, r := range in.Doses {
		g := givenDose{id: r.ID}
		if g.date, err = r.Date.Range(); err != nil {
			g.invalid = true
		}
		for _, dz := range r.Diseases {
			byDisease[dz] = append(byDisease[dz], g)
		}
	}
	out := Output{RuleSetVersion: e.version, Today: in.Today, Assessments: make([]Assessment, 0, len(e.diseases))}
	for _, dz := range e.diseases {
		// S1 has exact birth dates only (Lo == Hi). S2 runs the steps at both
		// ends of the range and compares the results (SAD §5.3 step 4).
		out.Assessments = append(out.Assessments, dz.assess(birth.Hi, in.Today, byDisease[dz.id]))
	}
	return out, nil
}

// assess runs SAD §5.3 for one disease at one end of the birth-date range.
func (dz diseaseModel) assess(birth, today Date, doses []givenDose) Assessment {
	a := Assessment{Disease: dz.id}

	// Step 1: an unparseable dose date makes the disease ⚪ (A9).
	var bad []string
	for _, g := range doses {
		if g.invalid {
			bad = append(bad, g.id)
		}
	}
	if len(bad) > 0 {
		slices.Sort(bad)
		a.Bucket = BucketUnknown
		for _, id := range bad {
			a.Reasons = append(a.Reasons, Reason{Code: ReasonInvalidDate, Params: ReasonParams{Dose: id}})
		}
		return a
	}

	// Step 2: time order. The ID breaks ties so the order never depends on
	// how the caller stored the records.
	doses = slices.Clone(doses)
	slices.SortFunc(doses, func(x, y givenDose) int {
		if c := x.date.Lo.Compare(y.date.Lo); c != 0 {
			return c
		}
		if c := x.date.Hi.Compare(y.date.Hi); c != 0 {
			return c
		}
		return strings.Compare(x.id, y.id)
	})

	// Step 5: dose validity.
	counted, reasons := dz.series.walk(birth, doses)
	a.Reasons = reasons
	dataUnknown := len(reasons) > 0 // layer 7: dose validity ⚪

	// Step 6: complete.
	if len(counted) >= dz.series.length {
		a.Bucket = BucketComplete
		a.Reasons = append(a.Reasons, Reason{Code: ReasonComplete})
		if dataUnknown {
			a.Bucket = BucketUnknown
		}
		return a
	}

	// Step 7: next dose, merged over alternative plans.
	nd := dz.series.next(birth, doses, counted)
	a.NextDose = &nd
	if len(dz.series.plans) > 1 {
		a.Reasons = append(a.Reasons, Reason{Code: ReasonAlternativePlans, Params: ReasonParams{Series: dz.series.id}})
	}

	// Step 9: bucket; step 10: layer 7 masks it, its reasons stay.
	bucket, block, reason := classify(today, nd, len(counted) > 0)
	if reason != "" {
		a.Reasons = append(a.Reasons, Reason{Code: reason})
	}
	if dataUnknown {
		a.Bucket = BucketUnknown
		return a
	}
	a.Bucket, a.Block = bucket, block
	return a
}
