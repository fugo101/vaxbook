package engine_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Window helpers of the property tests (issue #29 In scope 2–3, S4c). The
// oracles written before S4c read every series of the snapshot. With
// validity windows the engine uses the series in effect today, and a dose
// can meet the rule in effect on its date (versions) or, for a trailing
// undated slot, the series of the segments from its predecessor on (the
// Q105 envelope). The oracles do not re-implement either: they read the
// snapshot as of today, and an input is checked for a disease only when
// every date from birth (or from the earliest dose with no birth date) to
// today lies in today's segment, where versions cannot arise.

// windowOf converts a series' validity bounds; ok is false for an absent
// bound.
func windowOf(sr ruleset.Series) (from, to engine.Date, hasFrom, hasTo bool) {
	conv := func(d *ruleset.Date) (engine.Date, bool) {
		if d == nil {
			return engine.Date{}, false
		}
		v, err := engine.NewDate(d.Year, d.Month, d.Day)
		if err != nil {
			panic(err)
		}
		return v, true
	}
	from, hasFrom = conv(sr.ValidFrom)
	to, hasTo = conv(sr.ValidTo)
	return from, to, hasFrom, hasTo
}

// inEffect reports whether series sr is in effect on day.
func inEffect(sr ruleset.Series, day engine.Date) bool {
	from, to, hasFrom, hasTo := windowOf(sr)
	return (!hasFrom || !day.Before(from)) && (!hasTo || day.Before(to))
}

// windowed reports whether some series of s has a validity window.
func windowed(s *ruleset.Snapshot) bool {
	return slices.ContainsFunc(s.Series, func(sr ruleset.Series) bool { return sr.ValidFrom != nil || sr.ValidTo != nil })
}

// boundaries returns the validity bounds of disease's series, ascending.
func boundaries(s *ruleset.Snapshot, disease string) []engine.Date {
	var out []engine.Date
	for _, sr := range s.Series {
		if sr.Disease != disease {
			continue
		}
		from, to, hasFrom, hasTo := windowOf(sr)
		if hasFrom {
			out = append(out, from)
		}
		if hasTo {
			out = append(out, to)
		}
	}
	slices.SortFunc(out, engine.Date.Compare)
	return slices.Compact(out)
}

// snapshotOn returns s with only the series in effect on day, their windows
// removed: the snapshot a window-blind oracle reads. A windowless s is
// returned as it is.
func snapshotOn(s *ruleset.Snapshot, day engine.Date) *ruleset.Snapshot {
	if !windowed(s) {
		return s
	}
	out := *s
	out.Series = nil
	for _, sr := range s.Series {
		if inEffect(sr, day) {
			sr.ValidFrom, sr.ValidTo = nil, nil
			out.Series = append(out.Series, sr)
		}
	}
	return &out
}

// historyInToday reports whether every date of in's history for disease,
// from the birth date's earliest day (or the earliest dose date with no
// birth date, or no lower bound with no birth date and an undated record)
// to today, lies in today's validity segment: no boundary b of the disease
// with lower < b ≤ today.
func historyInToday(s *ruleset.Snapshot, disease string, in engine.Input) bool {
	bs := boundaries(s, disease)
	if len(bs) == 0 {
		return true
	}
	lower, bounded := in.Today, true
	if lo, _, ok := testPeriod(in.Profile.Birth, in.Today); ok {
		lower = lo
	} else {
		for _, r := range in.Doses {
			if !slices.Contains(r.Diseases, disease) {
				continue
			}
			if lo, _, ok := testPeriod(r.Date, in.Today); ok {
				if lo.Before(lower) {
					lower = lo
				}
			} else if r.Date.Precision == engine.Unknown {
				bounded = false
			}
		}
	}
	return !slices.ContainsFunc(bs, func(b engine.Date) bool { return (!bounded || lower.Before(b)) && !b.After(in.Today) })
}

// on returns fx with its snapshot as of in.Today (snapshotOn), and whether
// a window-blind oracle may check disease on in (historyInToday).
func (fx fixture) on(in engine.Input) (fixture, func(disease string) bool) {
	if !windowed(fx.snap) {
		return fx, func(string) bool { return true }
	}
	out := fx
	out.snap = snapshotOn(fx.snap, in.Today)
	return out, func(disease string) bool { return historyInToday(fx.snap, disease, in) }
}

// checkSeriesInEffect checks one assessment of in on snapshot s (issue #29
// In scope 4): NO_SERIES_IN_EFFECT appears iff no series of the disease is
// in effect today, unless a step-1 or step-2 ⚪ wins; it is then the only
// reason, with no seriesUsed, products or next dose. seriesUsed names only
// series in effect today.
func checkSeriesInEffect(t testing.TB, name string, s *ruleset.Snapshot, in engine.Input, a engine.Assessment) {
	t.Helper()
	none := !slices.ContainsFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == a.Disease && inEffect(sr, in.Today) })
	stepOneTwo := hasRecordProblem(a) || hasCode(a, engine.ReasonOverlappingEstimatedDates) || hasCode(a, engine.ReasonNumberingConflict)
	if got := hasCode(a, engine.ReasonNoSeriesInEffect); !stepOneTwo && got != none {
		t.Fatalf("%s %s: NO_SERIES_IN_EFFECT %v, no series in effect on %s %v\nassessment %+v\ninput %+v", name, a.Disease, got, in.Today, none, a, in)
	}
	if hasCode(a, engine.ReasonNoSeriesInEffect) && (len(a.Reasons) != 1 || a.SeriesUsed != nil || a.Products != nil || a.NextDose != nil || a.Bucket != engine.BucketUnknown) {
		t.Fatalf("%s %s: NO_SERIES_IN_EFFECT is not alone in a ⚪ with no seriesUsed\nassessment %+v", name, a.Disease, a)
	}
	for _, id := range a.SeriesUsed {
		i := slices.IndexFunc(s.Series, func(sr ruleset.Series) bool { return sr.ID == id })
		if i < 0 || !inEffect(s.Series[i], in.Today) {
			t.Fatalf("%s %s: seriesUsed names %s, not in effect on %s\ninput %+v", name, a.Disease, id, in.Today, in)
		}
	}
}

// TestPropertySeriesInEffect runs checkSeriesInEffect on random inputs of
// every fixture.
func TestPropertySeriesInEffect(t *testing.T) {
	t.Parallel()
	noSeries := 0
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(29, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				checkSeriesInEffect(t, fx.name, fx.snap, in, a)
				if hasCode(a, engine.ReasonNoSeriesInEffect) {
					noSeries++
				}
			}
		}
	}
	if noSeries == 0 {
		t.Fatal("no random input reached NO_SERIES_IN_EFFECT")
	}
}
