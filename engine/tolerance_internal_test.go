package engine

import (
	"math/rand/v2"
	"testing"
)

// TestPropertyToleranceMonotonePerCheck (issue #29 In scope 6): for a fixed
// position, dose range, predecessor, run of undated slots, dose 1 and gap
// bound, check()'s verdict with tolerance is never worse than with every
// tolerance removed, and the tolerant chain's dates are never later than
// the tolerance-free chain's. On minimal-tolerance, illustrative-vn
// pertussis (with random tolerances) and illustrative-max-age rota. The
// whole-walk counterexample is TestToleranceNotMonotoneOverWalk.
func TestPropertyToleranceMonotonePerCheck(t *testing.T) {
	t.Parallel()
	withTol := func(s seriesModel, rng *rand.Rand) seriesModel {
		s.plans = append([]planModel(nil), s.plans...)
		for i := range s.plans {
			s.plans[i].doses = append([]doseRule(nil), s.plans[i].doses...)
			for k := range s.plans[i].doses {
				if rng.IntN(2) == 0 {
					s.plans[i].doses[k].tol = rng.IntN(8)
				}
			}
		}
		return s
	}
	zero := func(s seriesModel) seriesModel {
		s.plans = append([]planModel(nil), s.plans...)
		for i := range s.plans {
			s.plans[i].doses = append([]doseRule(nil), s.plans[i].doses...)
			for k := range s.plans[i].doses {
				s.plans[i].doses[k].tol = 0
			}
		}
		return s
	}
	checked := 0
	for _, fx := range []gapFixture{
		{name: "minimal-tolerance", disease: "tol"},
		{name: "illustrative-vn", disease: "pertussis"},
		{name: "illustrative-max-age", disease: "rota"},
	} {
		rng := rand.New(rand.NewPCG(79, uint64(len(fx.name))))
		base := fx.diseaseOf(t).only(t)
		for range 20000 {
			s := base.series[rng.IntN(len(base.series))]
			if fx.name != "minimal-tolerance" {
				s = withTol(s, rng)
			}
			plain := zero(s)
			birth := d(t, "2026-01-01")
			b := birthEnd{date: birth, known: rng.IntN(5) != 0}
			day := func() Date { return birth.AddDays(rng.IntN(400)) }
			span := func(lo Date) Range { return Range{Lo: lo, Hi: lo.AddDays(rng.IntN(3) * rng.IntN(20))} }
			pos := 1 + rng.IntN(s.length)
			x := span(day())
			var from *Range
			if rng.IntN(3) != 0 {
				r := span(day())
				from = &r
			}
			var run []item
			for p := 1 + rng.IntN(pos); p < pos; p++ {
				run = append(run, item{pos: p})
			}
			var first *Range
			if rng.IntN(2) == 0 {
				r := span(day())
				first = &r
			}
			var gb gapBound
			switch rng.IntN(3) {
			case 1:
				gb = gapBound{set: true, best: day()}
			case 2:
				gb = gapBound{set: true, open: true}
			}
			vt, _ := s.check(pos, b, x, from, run, first, gb)
			vp, _ := plain.check(pos, b, x, from, run, first, gb)
			if vt > vp {
				t.Fatalf("%s %s: check with tolerance %d is worse than without %d\npos %d b %+v x %v from %v run %v first %v gb %+v",
					fx.name, s.id, vt, vp, pos, b, x, from, run, first, gb)
			}
			if len(run) > 0 {
				tb, tw, tok := s.chain(b, from, run, first, true)
				pb, pw, pok := s.chain(b, from, run, first, false)
				if tok != pok || tok && (tb.After(pb) || tw.After(pw)) {
					t.Fatalf("%s %s: tolerant chain (%v, %v, %v) later than exact (%v, %v, %v)", fx.name, s.id, tb, tw, tok, pb, pw, pok)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("nothing checked")
	}
}
