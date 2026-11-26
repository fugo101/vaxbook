package engine_test

import (
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property tests of tolerance (issue #29 In scope 6, S4c).

// cloneSnapshot returns a deep enough copy of s to change its series' rules
// (spans are values; the series, plans and dose rules are copied).
func cloneSnapshot(s *ruleset.Snapshot) *ruleset.Snapshot {
	out := *s
	out.Series = slices.Clone(s.Series)
	for i := range out.Series {
		out.Series[i].Plans = slices.Clone(out.Series[i].Plans)
		for j := range out.Series[i].Plans {
			out.Series[i].Plans[j].Doses = slices.Clone(out.Series[i].Plans[j].Doses)
		}
	}
	return &out
}

// toleranceVariant gives random dose rules of s a tolerance of P0D–P6D or
// P1W. ok is false when Validate rejects the variant.
func toleranceVariant(rng *rand.Rand, s *ruleset.Snapshot) (*ruleset.Snapshot, bool) {
	return toleranceVariantOf(rng, s, 3, 0)
}

// toleranceVariantOf gives each dose rule of s, with probability 1/every,
// a tolerance of P<least>D–P6D or P1W. ok is false when Validate rejects
// the variant.
func toleranceVariantOf(rng *rand.Rand, s *ruleset.Snapshot, every, least int) (*ruleset.Snapshot, bool) {
	out := cloneSnapshot(s)
	for i := range out.Series {
		for j := range out.Series[i].Plans {
			for k := range out.Series[i].Plans[j].Doses {
				if rng.IntN(every) != 0 {
					continue
				}
				tol := ruleset.Span{N: least + rng.IntN(7-least), Unit: ruleset.Days}
				if rng.IntN(8) == 0 {
					tol = ruleset.Span{N: 1, Unit: ruleset.Weeks}
				}
				out.Series[i].Plans[j].Doses[k].Tolerance = &tol
			}
		}
	}
	return out, len(ruleset.Rejects(ruleset.Validate(out))) == 0
}

// withoutTolerance returns s with every tolerance removed.
func withoutTolerance(s *ruleset.Snapshot) *ruleset.Snapshot {
	out := cloneSnapshot(s)
	for i := range out.Series {
		for j := range out.Series[i].Plans {
			for k := range out.Series[i].Plans[j].Doses {
				out.Series[i].Plans[j].Doses[k].Tolerance = nil
			}
		}
	}
	return out
}

// sameCounting reports whether two result views count the same records at
// the same positions in the same candidates, as walked and in the counted
// view.
func sameCounting(a, b engine.ResultView) bool {
	same := func(x, y []engine.CandidateView) bool {
		return slices.EqualFunc(x, y, func(p, q engine.CandidateView) bool {
			return p.Series == q.Series && p.K == q.K && p.Complete == q.Complete && maps.Equal(p.Counted, q.Counted)
		})
	}
	return same(a.Walked, b.Walked) && same(a.Counted, b.Counted)
}

// capDays returns the cap reasons of a, with their last days.
func capDays(a engine.Assessment) []engine.Reason {
	return slices.DeleteFunc(slices.Clone(a.Reasons), func(r engine.Reason) bool {
		return r.Code != engine.ReasonDueCappedByMaxAge && r.Code != engine.ReasonSeriesMayNotComplete
	})
}

// TestPropertyToleranceNeverChangesDates (EQ27, P1): a tolerance moves no
// next-dose date or limit. Each input is evaluated on a tolerance variant
// of a fixture (minimal-tolerance as it is) and with every tolerance
// removed. When both count the same records at the same positions in every
// candidate (ResultViews), every candidate's per-plan earliest dates are
// equal, and when both results show a next dose its earliest, due and
// overdue dates and its cap reasons with their last days (from C_c,k) are
// equal. With the same k only, a first-dose-anchored date can differ: a
// tolerance can make another record dose 1 (TestToleranceNotMonotoneOverWalk).
//
// Random ages seldom reach a maximum age, so a fixture with one
// (ageLimits) also gets len(limits) × propertyRuns inputs drawn near a
// limit (nearLimitInput), as TestPropertyCapP1 draws them, on variants
// with a tolerance of P1D–P1W on every rule, and a counter fails the test
// when no pair of capped next doses (a cap reason in the result with
// tolerance) is compared: C_c,k and its last days would go unchecked.
func TestPropertyToleranceNeverChangesDates(t *testing.T) {
	t.Parallel()
	compared, differ, capped := 0, 0, 0
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(97, uint64(len(fx.name))))
		limits := ageLimits(fx.snap)
		for i := range propertyRuns + len(limits)*propertyRuns {
			snap := fx.snap
			if fx.name != "minimal-tolerance" {
				// Near a limit, a tolerance on every rule, so that it would
				// move C_c,k if it leaked into the chain.
				every, least := 3, 0
				if i >= propertyRuns {
					every, least = 1, 1
				}
				v, ok := toleranceVariantOf(rng, fx.snap, every, least)
				if !ok {
					continue
				}
				snap = v
			}
			with, err := engine.New(snap)
			if err != nil {
				t.Fatal(err)
			}
			without, err := engine.New(withoutTolerance(snap))
			if err != nil {
				t.Fatal(err)
			}
			var in engine.Input
			switch {
			case i >= propertyRuns:
				in = nearLimitInput(t, rng, fx.snap, limits)
			case i%2 == 0:
				in = randomDayInput(t, rng, fx.snap)
			default:
				in = randomInput(t, rng, fx.snap)
			}
			outW, _ := evaluate(t, with, in)
			outO, _ := evaluate(t, without, in)
			viewW, err := engine.ResultViews(with, in)
			if err != nil {
				t.Fatal(err)
			}
			viewO, err := engine.ResultViews(without, in)
			if err != nil {
				t.Fatal(err)
			}
			for j, a := range outW.Assessments {
				checkAssessmentShape(t, fx.name, a)
				b := outO.Assessments[j]
				vw, vo := viewW[a.Disease], viewO[a.Disease]
				if !sameCounting(vw, vo) {
					differ++
					continue
				}
				compared++
				for c := range vw.Walked {
					if !slices.Equal(vw.Walked[c].Earliest, vo.Walked[c].Earliest) {
						t.Fatalf("%s %s: a tolerance moved %s's earliest dates %v to %v\ninput %+v", fx.name, a.Disease,
							vw.Walked[c].Series, vo.Walked[c].Earliest, vw.Walked[c].Earliest, in)
					}
				}
				if a.NextDose == nil || b.NextDose == nil {
					continue
				}
				if *a.NextDose != *b.NextDose || !slices.EqualFunc(capDays(a), capDays(b), func(x, y engine.Reason) bool {
					return x.Code == y.Code && x.Params.LastDay == y.Params.LastDay
				}) {
					t.Fatalf("%s %s: a tolerance moved the next dose\nwith    %+v\nwithout %+v\ninput %+v", fx.name, a.Disease, a, b, in)
				}
				if len(capDays(a)) > 0 {
					capped++
				}
			}
		}
	}
	t.Logf("%d assessments compared, %d with a different counting, %d capped next doses compared", compared, differ, capped)
	if differ == 0 {
		t.Error("no tolerance changed which records count; the generator is too weak (" + strconv.Itoa(compared) + " compared)")
	}
	if capped == 0 {
		t.Error("no capped next dose compared; C_c,k and its last days go unchecked (" + strconv.Itoa(compared) + " compared)")
	}
}
