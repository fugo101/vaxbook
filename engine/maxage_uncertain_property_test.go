package engine_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Properties of "certainly out of age" from the earliest possible dose
// dates (PR #31 review, owner decision 2026-11-24; P1).

// recordProblemCodes are the reasons of steps 1 and 2 that a wider dose
// date can bring: a record problem or an unplaced dose.
var recordProblemCodes = []engine.ReasonCode{
	engine.ReasonInvalidDate, engine.ReasonOutOfRange, engine.ReasonDateInFuture, engine.ReasonDateBeforeBirth,
	engine.ReasonUnexpectedValue, engine.ReasonUnknownProduct, engine.ReasonOverlappingEstimatedDates, engine.ReasonNumberingConflict,
}

func hasRecordProblem(a engine.Assessment) bool {
	return slices.ContainsFunc(recordProblemCodes, func(c engine.ReasonCode) bool { return hasCode(a, c) })
}

// TestPropertyWideningDoseNeverOutOfAge: the contrapositive of "a certain
// outOfAge never flips when a dose date is narrowed". When a disease has a
// next dose, widening any one of its day-precise doses to that day's month,
// or to its year, never gives outOfAge, when the widened result has the same
// seriesUsed and no step-1 or step-2 reason. The near-limit inputs must
// reach OUT_OF_AGE_UNCERTAIN at least once.
func TestPropertyWideningDoseNeverOutOfAge(t *testing.T) {
	t.Parallel()
	checked, uncertain := 0, 0
	for _, fx := range fixtures(t) {
		s4bInputs(t, fx, 113, func(in engine.Input) {
			out, _ := evaluate(t, fx.engine, in)
			for j, a := range out.Assessments {
				if a.NextDose == nil {
					continue
				}
				for i, r := range in.Doses {
					if r.Date.Precision != engine.Day || !slices.Contains(r.Diseases, a.Disease) {
						continue
					}
					day := r.Date.Value
					for _, w := range []engine.RecalledDate{{Value: day[:7], Precision: engine.Month}, {Value: day[:4], Precision: engine.Year}} {
						wide := cloneInput(in)
						wide.Doses[i].Date = w
						o, _ := evaluate(t, fx.engine, wide)
						b := o.Assessments[j]
						if !slices.Equal(a.SeriesUsed, b.SeriesUsed) || hasRecordProblem(b) {
							continue
						}
						checked++
						if hasCode(b, engine.ReasonOutOfAgeUncertain) {
							uncertain++
						}
						if b.Block == engine.BlockOutOfAge {
							t.Fatalf("%s %s: next dose %+v with %s on %s, but outOfAge with %v\nexact %+v\nwide  %+v\ninput %+v",
								fx.name, a.Disease, *a.NextDose, r.ID, day, w, a, b, in)
						}
					}
				}
			}
		})
	}
	t.Logf("%d widened doses checked, %d OUT_OF_AGE_UNCERTAIN", checked, uncertain)
	if uncertain == 0 {
		t.Fatalf("%d widened doses checked, none gave OUT_OF_AGE_UNCERTAIN: the property is vacuous", checked)
	}
}

// TestPropertyStandInNeverOutOfAge: dropping dose 1's date never turns a
// next dose into outOfAge, although step 7 then counts minIntervalFromFirst
// from the EQ20c stand-in, a later dose. No fixture has minIntervalFromFirst
// together with maxAgeComplete, so this runs on threeDoseQ with
// minIntervalFromFirst 5m (doses 2 and 3 minInterval 4w; maxAgeComplete
// 15y), with r1 numbered 1 and r2 numbered 2 near the limit: whenever the
// exact input has a next dose, the same input with r1's date unknown is not
// outOfAge. It must reach OUT_OF_AGE_UNCERTAIN at least once.
func TestPropertyStandInNeverOutOfAge(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, threeDoseQ(t, "P5M")))
	rng := rand.New(rand.NewPCG(127, 131))
	checked, uncertain := 0, 0
	for range 10 * propertyRuns {
		birth := mustDate(t, "2010-01-01").AddDays(rng.IntN(366))
		last := birth.AddMonths(180).AddDays(-1)
		d1 := last.AddDays(-rng.IntN(400))
		d2 := d1.AddDays(rng.IntN(240))
		today := d2.AddDays(rng.IntN(120))
		r2 := numbered(doseOf("q", "r2", dayDate(d2.String()), ""), 2)
		exact := assessmentOf(t, e, "q", today.String(), dayDate(birth.String()), numbered(doseOf("q", "r1", dayDate(d1.String()), ""), 1), r2)
		if exact.NextDose == nil {
			continue
		}
		checked++
		got := assessmentOf(t, e, "q", today.String(), dayDate(birth.String()), numbered(doseOf("q", "r1", unknownDate, ""), 1), r2)
		if hasCode(got, engine.ReasonOutOfAgeUncertain) {
			uncertain++
		}
		if got.Block == engine.BlockOutOfAge {
			t.Fatalf("birth %s, r1 %s, r2 %s, today %s: next dose %+v, but outOfAge %+v with r1 undated", birth, d1, d2, today, *exact.NextDose, got)
		}
	}
	t.Logf("%d stand-in inputs checked, %d OUT_OF_AGE_UNCERTAIN", checked, uncertain)
	if uncertain == 0 {
		t.Fatalf("%d stand-in inputs checked, none gave OUT_OF_AGE_UNCERTAIN: the property is vacuous", checked)
	}
}

// TestPropertyOutOfAgeUncertainNeedsImpreciseDose: OUT_OF_AGE_UNCERTAIN
// rests on a dose date only, so it never appears when every dose of the
// disease is day-precise and none is implied (no IMPLIED_EARLIER_DOSES):
// then the earliest and latest possible dates are the same.
func TestPropertyOutOfAgeUncertainNeedsImpreciseDose(t *testing.T) {
	t.Parallel()
	checked := 0
	check := func(fx fixture, in engine.Input) {
		out, _ := evaluate(t, fx.engine, in)
		for _, a := range out.Assessments {
			if hasCode(a, engine.ReasonImpliedEarlierDoses) || slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool {
				return slices.Contains(r.Diseases, a.Disease) && r.Date.Precision != engine.Day
			}) {
				continue
			}
			checked++
			if hasCode(a, engine.ReasonOutOfAgeUncertain) {
				t.Fatalf("%s %s: OUT_OF_AGE_UNCERTAIN with exact dose dates: %+v\ninput %+v", fx.name, a.Disease, a, in)
			}
		}
	}
	for _, fx := range fixtures(t) {
		s4bInputs(t, fx, 137, func(in engine.Input) { check(fx, in) })
		rng := rand.New(rand.NewPCG(139, uint64(len(fx.name))))
		for range propertyRuns {
			check(fx, randomInput(t, rng, fx.snap))
		}
	}
	if checked == 0 {
		t.Fatal("no assessment with exact dose dates: the property is vacuous")
	}
}

// impreciseNearLimitInputs feeds each n inputs on two in-test fixtures
// whose next dose falls near maxAgeComplete from an imprecise latest dose,
// where certainty from R.Hi and from R.Lo differ: twoDoseQ with 4w and
// threeDoseQ with minIntervalFromFirst 5m (maxAgeComplete 15y, so L =
// birth + 15y − 1 day). The fixtures' near-limit inputs seldom reach that
// zone. The latest dose is in the 70 days up to L, at month precision or
// sometimes year; in threeDoseQ the earlier dose is 4w to about 9 months
// before it, at day or month precision; today is up to 45 days after it.
func impreciseNearLimitInputs(t *testing.T, seed uint64, n int, each func(fx fixture, in engine.Input)) {
	t.Helper()
	var fxs []fixture
	for name, sr := range map[string]ruleset.Series{"twoDoseQ": twoDoseQ(t, "q-1", "", "P4W"), "threeDoseQ": threeDoseQ(t, "P5M")} {
		s := oneDiseaseSnapshot(nil, sr)
		fxs = append(fxs, fixture{name: "in-test " + name, snap: s, engine: newEngine(t, s)})
	}
	slices.SortFunc(fxs, func(a, b fixture) int { return strings.Compare(a.name, b.name) })
	rng := rand.New(rand.NewPCG(seed, 149))
	recalled := func(d engine.Date, coarse bool) engine.RecalledDate {
		switch {
		case !coarse:
			return dayDate(d.String())
		case rng.IntN(5) == 0:
			return engine.RecalledDate{Value: d.String()[:4], Precision: engine.Year}
		}
		return monthDate(d.String()[:7])
	}
	for i := range n {
		fx := fxs[i%len(fxs)]
		birth := mustDate(t, "2010-01-01").AddDays(rng.IntN(366))
		last := birth.AddMonths(180).AddDays(-1).AddDays(-rng.IntN(70))
		in := engine.Input{
			Today:   last.AddDays(rng.IntN(45)),
			Profile: engine.Profile{Birth: dayDate(birth.String())},
			Doses:   []engine.DoseRecord{doseOf("q", "r2", recalled(last, true), "")},
		}
		if fx.name == "in-test threeDoseQ" {
			first := last.AddDays(-28 - rng.IntN(240))
			in.Doses = append([]engine.DoseRecord{doseOf("q", "r1", recalled(first, rng.IntN(2) == 0), "")}, in.Doses...)
		}
		each(fx, in)
	}
}
