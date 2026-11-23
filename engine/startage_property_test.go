package engine_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property tests of start-age brackets (issue #23).

// TestPropertyStartAgeCertainWithExactDates: AGE_AT_START_UNCERTAIN never
// appears when the birth date and every dose date are day-precise, no dose
// is level 4 or implied (randomDayInput has no dose numbers), and
// ruleset.Validate gives the snapshot no problem, Warn included
// (history-and-model-8, Q43). Every bracket of a group then counts the same
// record as dose 1, and Validate makes the brackets disjoint.
func TestPropertyStartAgeCertainWithExactDates(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		if len(ruleset.Validate(fx.snap)) > 0 {
			continue
		}
		rng := rand.New(rand.NewPCG(47, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomDayInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				if hasCode(a, engine.ReasonAgeAtStartUncertain) {
					t.Fatalf("%s %s: exact dates give AGE_AT_START_UNCERTAIN\nassessment %+v\ninput %+v", fx.name, a.Disease, a, in)
				}
			}
		}
	}
}

// TestPropertyNoDoseBrackets checks the brackets chosen with no dose given
// against an oracle computed from the snapshot with Date.AddSpan alone
// (EQ10, A5, Q24, Q33, Q50 (b)). With a birth date and no dose of the
// disease:
//   - every series in seriesUsed (the later end's list, Q40) can still be
//     started at the later end of the birth-date range (no max, or birth +
//     max after today), and every bracket of its product group before it
//     (by min) cannot: it is the first open bracket of its group;
//   - outOfAge means that at the later end no series of the disease,
//     retired ones included, can be started (so retiring a product never
//     causes it, Q50 (b));
//   - OUT_OF_AGE inside a ⚪ comes from the older end, where the same
//     holds, while the later end has some series left.
//
// Every no-dose assessment also goes through checkCandidates, which checks
// each end's SAFEST_SERIES against noDoseChoice at that end.
//
// Half the inputs have their doses removed and a month- or year-precision
// birth date, so that the two ends often choose different brackets. Half of
// those then move today to just after a bracket's max is reached at the
// older end (birthLo + max + 0–14 days), the window where the older end has
// left that bracket, is due on the next one, and the later end is still in
// it: only there does a two-end ⚪ whose ends chose different lists arise
// (the golden start-age-groups-ends-differ-disagree pins one date). The
// minimal-start-age-groups fixture then reaches outOfAge at both ends and at
// the older end only, and with ends that chose different lists both
// agreement and a two-end ⚪; the counts guard against a generator change
// making this vacuous.
func TestPropertyNoDoseBrackets(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(53, uint64(len(fx.name))))
		reached := map[string]int{}
		var maxes []ruleset.Span
		for _, sr := range fx.snap.Series {
			if sr.StartAge != nil && sr.StartAge.Max != nil {
				maxes = append(maxes, *sr.StartAge.Max)
			}
		}
		for i := range 2 * propertyRuns {
			in := randomInput(t, rng, fx.snap)
			if i%2 == 1 {
				in.Doses = []engine.DoseRecord{}
				if b, err := engine.ParseDate(in.Profile.Birth.Value); err == nil && in.Profile.Birth.Precision == engine.Day {
					in.Profile.Birth = engine.RecalledDate{Value: b.String()[:7], Precision: engine.Month}
					if rng.IntN(2) == 0 {
						in.Profile.Birth = engine.RecalledDate{Value: b.String()[:4], Precision: engine.Year}
					}
				}
				if lo, _, ok := testPeriod(in.Profile.Birth, in.Today); ok && i%4 == 3 && len(maxes) > 0 {
					in.Today = lo.AddSpan(maxes[rng.IntN(len(maxes))]).AddDays(rng.IntN(15))
				}
			}
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok {
				continue
			}
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				if slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) }) {
					continue
				}
				fail := func(format string, args ...any) {
					t.Helper()
					t.Fatalf("%s %s: "+format+"\nassessment %+v\ninput %+v", append(append([]any{fx.name, a.Disease}, args...), a, in)...)
				}
				checkCandidates(t, fx, in, a)
				var series []ruleset.Series
				for _, sr := range fx.snap.Series {
					if sr.Disease == a.Disease {
						series = append(series, sr)
					}
				}
				noneOpen := func(b engine.Date) bool {
					return !slices.ContainsFunc(series, func(sr ruleset.Series) bool { return bracketOpen(sr, b, in.Today) })
				}
				for _, sr := range seriesUsed(t, fx.snap, a) {
					if !bracketOpen(sr, hi, in.Today) {
						fail("seriesUsed has %s, whose start age is past at the later end %s", sr.ID, hi)
					}
					for _, o := range series {
						if o.ID != sr.ID && productGroup(t, fx.snap, a.Disease, o.ID) == productGroup(t, fx.snap, a.Disease, sr.ID) &&
							o.StartAge != nil && o.StartAge.Max != nil && ruleset.SpanAtMost(*o.StartAge.Max, sr.StartAge.Min) &&
							bracketOpen(o, hi, in.Today) {
							fail("seriesUsed has %s, but %s of its group comes before it and is still open at %s", sr.ID, o.ID, hi)
						}
					}
				}
				switch {
				case a.Block == engine.BlockOutOfAge:
					reached["outOfAge"]++
					if !noneOpen(hi) {
						fail("outOfAge, but some series can still be started at the later end %s", hi)
					}
				case hasCode(a, engine.ReasonOutOfAge):
					reached["older end outOfAge"]++
					if a.Bucket != engine.BucketUnknown || !noneOpen(lo) || noneOpen(hi) {
						fail("OUT_OF_AGE outside outOfAge: the older end %s must have no series left and the later end %s some", lo, hi)
					}
				case hasCode(a, engine.ReasonBirthDateImprecise):
					reached["two-end ⚪"]++
				}
				if differ := !slices.Equal(noDoseChoice(fx.snap, a.Disease, lo, in.Today), noDoseChoice(fx.snap, a.Disease, hi, in.Today)); differ {
					switch {
					case a.Block == engine.BlockOutOfAge || hasCode(a, engine.ReasonOutOfAge):
					case hasCode(a, engine.ReasonBirthDateImprecise):
						reached["ends differ, ⚪"]++
					default:
						reached["ends differ, agree"]++
					}
				}
			}
		}
		if fx.name == "minimal-start-age-groups" {
			for _, k := range []string{"outOfAge", "older end outOfAge", "two-end ⚪", "ends differ, ⚪", "ends differ, agree"} {
				if reached[k] == 0 {
					t.Errorf("%s: no random input reached %q (%v)", fx.name, k, reached)
				}
			}
		}
	}
}

// TestPropertyValidatedBracketsDisjoint: for a snapshot that ruleset.Validate
// accepts, at most one bracket of a product group contains any exact age
// (EQ26), with containment computed by calendar arithmetic (birth + min ≤
// dose < birth + max, Date.AddSpan). This checks Validate's month rule
// (SAD §5.2) against month-end clamping; it lives in engine because ruleset
// has no calendar arithmetic. Brackets of two or three series are drawn in
// mixed units around shared boundaries, so that boundaries collide, and
// kept only when Validate gives no Reject; engine.New must accept them.
func TestPropertyValidatedBracketsDisjoint(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(59, 1))
	inUnit := func(days int) ruleset.Span {
		for {
			switch rng.IntN(4) {
			case 0:
				if days <= 800 {
					return ruleset.Span{N: days, Unit: ruleset.Days}
				}
			case 1:
				if n := (days + rng.IntN(7)) / 7; n <= 120 {
					return ruleset.Span{N: n, Unit: ruleset.Weeks}
				}
			case 2:
				if n := (days*12 + rng.IntN(365)) / 365; n <= 30 {
					return ruleset.Span{N: n, Unit: ruleset.Months}
				}
			case 3:
				if n := (days + rng.IntN(365)) / 365; n <= 3 {
					return ruleset.Span{N: n, Unit: ruleset.Years}
				}
			}
		}
	}
	base := mustDate(t, "2000-01-01")
	accepted := 0
	for i := range 3000 {
		n := 2 + rng.IntN(2)
		// Boundaries in days, ascending, each written in a random unit for
		// every bracket that uses it.
		days := []int{rng.IntN(60)}
		for range n {
			days = append(days, days[len(days)-1]+1+rng.IntN(400))
		}
		s := &ruleset.Snapshot{Version: "v", Diseases: []ruleset.Disease{{ID: "d", Name: "D"}}}
		for k := range n {
			sa := &ruleset.StartAge{Min: inUnit(days[k])}
			if k < n-1 || rng.IntN(3) > 0 {
				m := inUnit(days[k+1])
				sa.Max = &m
			}
			s.Series = append(s.Series, ruleset.Series{
				ID: fmt.Sprintf("s%d", k), Disease: "d", Stage: ruleset.Childhood, StartAge: sa,
				Plans: []ruleset.Plan{{ID: "p", Doses: []ruleset.DoseRule{{
					MinAge: &ruleset.Span{N: 0, Unit: ruleset.Days}, Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: ruleset.Span{N: 0, Unit: ruleset.Days}},
				}}}},
			})
		}
		rng.Shuffle(len(s.Series), func(a, b int) { s.Series[a], s.Series[b] = s.Series[b], s.Series[a] })
		if len(ruleset.Rejects(ruleset.Validate(s))) > 0 {
			continue
		}
		accepted++
		if _, err := engine.New(s); err != nil {
			t.Fatalf("snapshot %d: Validate accepts it but New fails: %v", i, err)
		}
		for range 40 {
			birth := randomBirth(rng, base)
			dose := birth.AddDays(rng.IntN(days[n] + 400))
			var in []string
			for _, sr := range s.Series {
				sa := sr.StartAge
				if !dose.Before(birth.AddSpan(sa.Min)) && (sa.Max == nil || dose.Before(birth.AddSpan(*sa.Max))) {
					maxText := "—"
					if sa.Max != nil {
						maxText = sa.Max.String()
					}
					in = append(in, fmt.Sprintf("%s [%s, %s)", sr.ID, sa.Min, maxText))
				}
			}
			if len(in) > 1 {
				t.Fatalf("snapshot %d: birth %s, dose %s lies in %d brackets: %v", i, birth, dose, len(in), in)
			}
		}
	}
	if accepted < 300 {
		t.Fatalf("only %d of 3000 random groups were accepted; the generator no longer exercises Validate", accepted)
	}
}

// randomBirth draws a birth date in the 40 years from base, biased to the
// month ends where AddSpan clamps: day 28–31 half the time, and 29 February
// one time in ten.
func randomBirth(rng *rand.Rand, base engine.Date) engine.Date {
	d := base.AddMonths(rng.IntN(480))
	switch n := rng.IntN(10); {
	case n == 0:
		d = base.AddMonths(12 * 4 * rng.IntN(10)).AddMonths(1).AddDays(28) // 29 February of a leap year from 2000
	case n < 5:
		d = d.AddDays(27 + rng.IntN(4))
	default:
		d = d.AddDays(rng.IntN(28))
	}
	return d
}
