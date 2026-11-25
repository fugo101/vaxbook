package engine_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property tests of catch-up (issue #29 In scope 5, S4c).

// catchUpVariant gives random series of s a catchUpAfterAge. A series with
// a maximum age (maxAgeComplete, else maxAgeStart) mostly gets one from
// half of it up to one unit below it, in its unit (a limit of one unit is
// first written in the next smaller one: months, weeks or days), so that
// catch-up meets the cap before the series is out of age; any other gets
// P1Y–P5Y. ok is false
// when Validate rejects the variant (a Warn is kept).
func catchUpVariant(rng *rand.Rand, s *ruleset.Snapshot) (*ruleset.Snapshot, bool) {
	out := cloneSnapshot(s)
	for i := range out.Series {
		if rng.IntN(3) == 0 {
			continue
		}
		sr := &out.Series[i]
		age := ruleset.Span{N: 1 + rng.IntN(5), Unit: ruleset.Years}
		limit := sr.MaxAgeComplete
		if limit == nil {
			limit = sr.MaxAgeStart
		}
		if limit != nil && rng.IntN(4) != 0 {
			age = *limit
			if age.N < 2 {
				switch age.Unit {
				case ruleset.Years:
					age = ruleset.Span{N: 12 * age.N, Unit: ruleset.Months}
				case ruleset.Months:
					age = ruleset.Span{N: 4 * age.N, Unit: ruleset.Weeks}
				case ruleset.Weeks:
					age = ruleset.Span{N: 7 * age.N, Unit: ruleset.Days}
				}
			}
			if age.N >= 2 {
				age.N = (age.N+1)/2 + rng.IntN(age.N/2)
			}
		}
		sr.CatchUpAfterAge = &age
	}
	return out, len(ruleset.Rejects(ruleset.Validate(out))) == 0
}

// TestPropertyCatchUp (Q78, Q91, Q92), with an exact birth date (one
// evaluated date), on every fixture and catch-up variants of it (so that a
// maximum-age fixture meets the cap). The oracle reads F = max(today, the
// result's next-dose earliest date, shown or masked: ResultView.Next), the
// candidates at k (ResultView.Walked) and catchUpAfterAge from the
// snapshot:
//   - a clean result (a shown next dose) has no candidate at k with F ≥ b +
//     catchUpAfterAge;
//   - CATCH_UP_ASK_DOCTOR comes in a ⚪ with no next dose, never with
//     OUT_OF_AGE, an outOfAge block or another blocking reason, and some
//     candidate at k has F ≥ b + catchUpAfterAge.
//
// With an imprecise birth date a step-4 ⚪ can list [BIRTH_DATE_IMPRECISE,
// CATCH_UP_ASK_DOCTOR, OUT_OF_AGE] from two dates, so it is not checked.
func TestPropertyCatchUp(t *testing.T) {
	t.Parallel()
	asked, clean := 0, 0
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(61, uint64(len(fx.name))))
		for i := range propertyRuns {
			snap, e := fx.snap, fx.engine
			if i%2 == 1 {
				v, ok := catchUpVariant(rng, fx.snap)
				if !ok {
					continue
				}
				var err error
				if e, err = engine.New(v); err != nil {
					t.Fatal(err)
				}
				snap = v
			}
			in := randomDayInput(t, rng, fx.snap)
			if limits := ageLimits(fx.snap); len(limits) > 0 && i%3 == 0 {
				in = nearLimitDayInput(t, rng, fx.snap, limits)
			}
			b, _, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok {
				continue
			}
			out, _ := evaluate(t, e, in)
			views, err := engine.ResultViews(e, in)
			if err != nil {
				t.Fatal(err)
			}
			snapOn := snapshotOn(snap, in.Today)
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				v, ok := views[a.Disease]
				if !ok || v.Next == nil {
					if hasCode(a, engine.ReasonCatchUpAskDoctor) {
						t.Fatalf("%s %s: CATCH_UP_ASK_DOCTOR with no next dose, shown or masked\n%+v", fx.name, a.Disease, a)
					}
					continue
				}
				f := slices.MaxFunc([]engine.Date{in.Today, v.Next.Earliest}, engine.Date.Compare)
				trigger := slices.ContainsFunc(v.Walked, func(c engine.CandidateView) bool {
					if c.Complete || c.K != v.Next.Number {
						return false
					}
					sr := seriesByID(snapOn, c.Series)
					return sr.CatchUpAfterAge != nil && !f.Before(b.AddSpan(*sr.CatchUpAfterAge))
				})
				switch {
				case hasCode(a, engine.ReasonCatchUpAskDoctor):
					asked++
					other := slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool {
						return blockingCodes[r.Code] && r.Code != engine.ReasonCatchUpAskDoctor
					})
					if a.NextDose != nil || a.Bucket != engine.BucketUnknown || hasCode(a, engine.ReasonOutOfAge) || other || !trigger {
						t.Fatalf("%s %s: CATCH_UP_ASK_DOCTOR, F %s, trigger %v\nassessment %+v\ninput %+v", fx.name, a.Disease, f, trigger, a, in)
					}
				case a.NextDose != nil:
					clean++
					if trigger {
						t.Fatalf("%s %s: a next dose with F %s past a candidate's catch-up age\nassessment %+v\ninput %+v", fx.name, a.Disease, f, a, in)
					}
				}
			}
		}
	}
	t.Logf("%d caught up, %d clean next doses", asked, clean)
	if asked == 0 || clean == 0 {
		t.Fatal("the generator reaches no catch-up or no clean next dose; the property is vacuous")
	}
}

// TestPropertyCatchUpBirthRange (Q108): with an imprecise birth date, a
// shown next dose never lies past the catch-up age of a birth date in the
// range: F = max(today, earliest) < b_old + catchUpAfterAge. The dates
// driven by a minimum age come from the latest birth date, so without the
// guard at b_old the later end could show such a date while it is clean.
// The fixtures seldom put a minimum age near the catch-up age, so the
// inputs come from an in-test family of cu-x: one dose with minAge 96 to
// 111 weeks, due birth + 30m, catchUpAfterAge 2y, a random birth month and
// today 600 to 799 days after its first day. Counters fail the test when
// no input shows a next dose or none is guarded (⚪ BIRTH_DATE_IMPRECISE
// without CATCH_UP_ASK_DOCTOR).
func TestPropertyCatchUpBirthRange(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(108, 0))
	base := mustDate(t, "2020-01-01")
	shown, guarded := 0, 0
	for range propertyRuns {
		w := 96 + rng.IntN(16)
		sr := qSeries(t, "cu-x", "", "", "", []ruleset.DoseRule{rule(t, fmt.Sprintf("P%dW", w), "", "", ruleset.FromBirth, "P30M")})
		sr.CatchUpAfterAge = spanOf(t, "P2Y")
		e := newEngine(t, oneDiseaseSnapshot(nil, sr))
		y, m, _ := base.AddDays(rng.IntN(6 * 365)).YMD()
		lo, err := engine.NewDate(y, m, 1)
		if err != nil {
			t.Fatal(err)
		}
		today := lo.AddDays(600 + rng.IntN(200))
		a := assessmentOf(t, e, "q", today.String(), monthDate(lo.String()[:7]))
		checkAssessmentShape(t, "cu-x", a)
		switch {
		case a.NextDose != nil:
			shown++
			f := slices.MaxFunc([]engine.Date{today, a.NextDose.Earliest}, engine.Date.Compare)
			if limit := lo.AddSpan(ruleset.Span{N: 2, Unit: ruleset.Years}); !f.Before(limit) {
				t.Fatalf("cu-x minAge %dw, birth %s, today %s: shown next dose with F %s at or past b_old + 2y = %s\n%+v", w, lo.String()[:7], today, f, limit, a)
			}
		case hasCode(a, engine.ReasonBirthDateImprecise) && !hasCode(a, engine.ReasonCatchUpAskDoctor):
			guarded++
		}
	}
	t.Logf("%d shown next doses, %d guarded", shown, guarded)
	if shown == 0 || guarded == 0 {
		t.Fatal("the family reaches no shown next dose or no guarded result; the property is vacuous")
	}
}
