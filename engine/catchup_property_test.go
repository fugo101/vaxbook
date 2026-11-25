package engine_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property tests of catch-up (issue #29 In scope 5, S4c).

// catchUpVariant gives random series of s a catchUpAfterAge of P1Y–P5Y. ok
// is false when Validate rejects the variant (a Warn is kept).
func catchUpVariant(rng *rand.Rand, s *ruleset.Snapshot) (*ruleset.Snapshot, bool) {
	out := cloneSnapshot(s)
	for i := range out.Series {
		if rng.IntN(3) == 0 {
			continue
		}
		age := ruleset.Span{N: 1 + rng.IntN(5), Unit: ruleset.Years}
		out.Series[i].CatchUpAfterAge = &age
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
