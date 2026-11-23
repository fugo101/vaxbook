package engine_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
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
//   - OUT_OF_AGE inside a ⚪ comes from the oldest sub-range of the birth
//     range (SAD §5.3 step 4), which contains the older end, where the same
//     holds, while the later end has some series left.
//
// Every no-dose assessment also goes through checkCandidates, which checks
// each sub-range's SAFEST_SERIES against noDoseChoice on its days.
//
// Half the inputs have their doses removed and a month- or year-precision
// birth date, so that the two ends often choose different brackets. Half of
// those then move today to just after a bracket's max is reached at the
// older end (birthLo + max + 0–14 days), the window where the older end has
// left that bracket, is due on the next one, and the later end is still in
// it: there a ⚪ whose ends chose different lists arises (the golden
// start-age-groups-ends-differ-disagree pins one date). The
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

// shownClass is what step 4 compares with no dose given (SAD §5.3 step 4):
// the class as the user sees it (✅ ON_TRACK told apart from ✅ complete)
// and the next dose number, 0 when there is none.
func shownClass(a engine.Assessment) (string, int) {
	k := 0
	if a.NextDose != nil {
		k = a.NextDose.Number
	}
	if hasCode(a, engine.ReasonOnTrack) {
		return "onTrack", k
	}
	return visible(a), k
}

// distanceOnlyClass reports whether a class differs from the others in its
// set only by how far away the next dose is (F05.5).
func distanceOnlyClass(c string) bool {
	return c == string(engine.BlockNotYetScheduled) || c == string(engine.BucketUpcoming) || c == "onTrack"
}

// TestPropertyNoDoseBirthRangeOracle checks the no-dose birth-range split
// (SAD §5.3 step 4; PR #27, issue #23) against every day of the birth
// range evaluated as an exact birth date, month-end clamping included.
// With no dose given and a month- or year-precision birth date:
//   - the result is ⚪ BIRTH_DATE_IMPRECISE iff some day disagrees with the
//     latest day: another next dose number, or another class unless both
//     are distance-only (notYetScheduled, 🟡, ✅ ON_TRACK);
//   - otherwise it is the latest day's result: the same class, nextDose,
//     seriesUsed, products and reasons;
//   - a ⚪ has the latest day's seriesUsed, and its hint is fullBirthDate
//     iff the birth date is month-precise or some day is outOfAge while
//     another is not (an end hits an age limit), else birthMonth.
//
// Half the inputs move today to 0–199 days after a bracket's max is reached
// at the older birth end (noDoseImprecise), where the start-age choice
// changes inside the range. Every fixture with a bracketed disease must
// reach an input whose days choose different seriesUsed.
func TestPropertyNoDoseBirthRangeOracle(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		if !slices.ContainsFunc(fx.snap.Series, func(sr ruleset.Series) bool { return sr.StartAge != nil }) {
			continue
		}
		rng := rand.New(rand.NewPCG(71, uint64(len(fx.name))))
		maxes := bracketMaxes(fx.snap)
		varied := 0
		for range 150 {
			in := noDoseImprecise(rng, randomInput(t, rng, fx.snap), maxes, 200)
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok || lo == hi {
				continue
			}
			wide, _ := evaluate(t, fx.engine, in)
			var days []engine.Output // one per birth day, oldest first
			for b := lo; !b.After(hi); b = b.AddDays(1) {
				exact := cloneInput(in)
				exact.Profile.Birth = engine.RecalledDate{Value: b.String(), Precision: engine.Day}
				out, _ := evaluate(t, fx.engine, exact)
				days = append(days, out)
			}
			for i, a := range wide.Assessments {
				fail := func(format string, args ...any) {
					t.Helper()
					t.Fatalf("%s %s: "+format+"\nwide  %+v\ninput %+v", append(append([]any{fx.name, a.Disease}, args...), a, in)...)
				}
				latest := days[len(days)-1].Assessments[i]
				lc, lk := shownClass(latest)
				agree, outOfAge, startable, differ := true, false, false, false
				for _, out := range days {
					d := out.Assessments[i]
					c, k := shownClass(d)
					if sameClass := c == lc || distanceOnlyClass(c) && distanceOnlyClass(lc); k != lk || !sameClass {
						agree = false
					}
					outOfAge = outOfAge || d.Block == engine.BlockOutOfAge
					startable = startable || d.Block != engine.BlockOutOfAge
					differ = differ || !slices.Equal(d.SeriesUsed, latest.SeriesUsed)
				}
				if differ && bracketed(fx.snap, a.Disease) {
					varied++
				}
				if imprecise := hasCode(a, engine.ReasonBirthDateImprecise); imprecise == agree {
					fail("BIRTH_DATE_IMPRECISE is %v, but every day agreeing with the latest one (%s, %d) is %v", imprecise, lc, lk, agree)
				}
				if agree {
					if c, k := shownClass(a); c != lc || k != lk || !reflect.DeepEqual(a.NextDose, latest.NextDose) ||
						!slices.Equal(a.SeriesUsed, latest.SeriesUsed) || !slices.Equal(a.Products, latest.Products) ||
						!reflect.DeepEqual(a.Reasons, latest.Reasons) {
						fail("the days agree, but the result is not the latest day's %+v", latest)
					}
					continue
				}
				want := engine.HintBirthMonth
				if in.Profile.Birth.Precision == engine.Month || outOfAge && startable {
					want = engine.HintFullBirthDate
				}
				if got := reasonsWith(a, engine.ReasonBirthDateImprecise)[0].Params.Hint; got != want {
					fail("hint %v, want %v", got, want)
				}
				if !slices.Equal(a.SeriesUsed, latest.SeriesUsed) {
					fail("⚪ with seriesUsed %v, want the latest day's %v", a.SeriesUsed, latest.SeriesUsed)
				}
			}
		}
		if varied == 0 {
			t.Errorf("%s: no input had birth days choosing different seriesUsed", fx.name)
		}
	}
}

// TestPropertyDoseOneWideningAddsBrackets (P1; SAD §5.3 step 3, Q33, Q49):
// with doses given, narrowing a month- or year-precision dose date to a day
// inside its range never matches a start-age bracket that the wide date did
// not. A bracket's reach runs from the previous bracket's max in its group
// (from birth for the first) to its own max, so a dose-1 range that may fall
// in a bracket or in the gap above it matches both (PR #27).
//
// matched(x) is the union of an assessment's AGE_AT_START_UNCERTAIN
// candidates and its seriesUsed: each group's matches, or its kept last
// bracket when nothing matches. The property needs the same records to be
// dose-1 candidates in both inputs, with the narrow ranges inside the wide
// ones, so it runs on snapshots that Validate gives no problem (one dose-1
// minimum age per group, no Warn) and skips an assessment pair when
// either has no seriesUsed, OVERLAPPING_ESTIMATED_DATES,
// NUMBERING_CONFLICT, or a DOSE_INVALID or DOSE_POSSIBLY_INVALID at
// position 1, or the wide one has any DOSE_POSSIBLY_INVALID.
//
// Half the draws move the chosen dose to the month of the older birth end +
// a random bracket max ± 45 days, so that it straddles a reach bound. On
// minimal-start-age-gap some wide input must match gap-a and gap-b and more
// brackets than its narrowed form.
func TestPropertyDoseOneWideningAddsBrackets(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		if len(ruleset.Validate(fx.snap)) > 0 || !slices.ContainsFunc(fx.snap.Series, func(sr ruleset.Series) bool { return sr.StartAge != nil }) {
			continue
		}
		maxes := bracketMaxes(fx.snap)
		rng := rand.New(rand.NewPCG(79, uint64(len(fx.name))))
		gapPair := 0
		for range 10 * propertyRuns {
			in := randomInput(t, rng, fx.snap)
			var wideDoses []int
			for i, r := range in.Doses {
				if lo, hi, ok := testPeriod(r.Date, in.Today); ok && lo != hi {
					wideDoses = append(wideDoses, i)
				}
			}
			if len(wideDoses) == 0 {
				continue
			}
			i := wideDoses[rng.IntN(len(wideDoses))]
			birthLo, _, birthOK := testPeriod(in.Profile.Birth, in.Today)
			if birthOK && len(maxes) > 0 && rng.IntN(2) == 0 {
				d := birthLo.AddSpan(maxes[rng.IntN(len(maxes))]).AddDays(rng.IntN(91) - 45)
				if !d.After(in.Today) && !d.Before(birthLo) {
					in.Doses[i].Date = engine.RecalledDate{Value: d.String()[:7], Precision: engine.Month}
				}
			}
			lo, hi, ok := testPeriod(in.Doses[i].Date, in.Today)
			if !ok {
				continue
			}
			if birthOK {
				lo = slices.MaxFunc([]engine.Date{lo, birthLo}, engine.Date.Compare)
			}
			if lo.After(hi) {
				continue
			}
			wide, _ := evaluate(t, fx.engine, in)
			narrow := cloneInput(in)
			narrow.Doses[i].Date = engine.RecalledDate{Value: lo.AddDays(rng.IntN(daysBetween(lo, hi) + 1)).String(), Precision: engine.Day}
			got, _ := evaluate(t, fx.engine, narrow)
			for j, a := range wide.Assessments {
				b := got.Assessments[j]
				if !bracketed(fx.snap, a.Disease) || a.SeriesUsed == nil || b.SeriesUsed == nil ||
					unplaced(a) || unplaced(b) || hasCode(a, engine.ReasonDosePossiblyInvalid) {
					continue
				}
				mw, mn := matchedBrackets(a), matchedBrackets(b)
				for id := range mn {
					if !mw[id] {
						t.Fatalf("%s %s: narrowing %s to %s matches %s, which the wide date does not\nwide   %+v\nnarrow %+v\ninput  %+v",
							fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, id, a, b, in)
					}
				}
				if len(mw) > len(mn) && mw["gap-a"] && mw["gap-b"] {
					gapPair++
				}
			}
		}
		if fx.name == "minimal-start-age-gap" && gapPair == 0 {
			t.Errorf("%s: no wide dose-1 range matched both brackets around the gap", fx.name)
		}
	}
}

// unplaced reports whether an assessment's dose-1 candidates may differ
// from a narrower or wider input's: the doses are not placed, or a dose at
// position 1 fails a check.
func unplaced(a engine.Assessment) bool {
	if hasCode(a, engine.ReasonOverlappingEstimatedDates) || hasCode(a, engine.ReasonNumberingConflict) {
		return true
	}
	return slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool {
		return (r.Code == engine.ReasonDoseInvalid || r.Code == engine.ReasonDosePossiblyInvalid) && r.Params.Number == 1
	})
}

// matchedBrackets returns the series an assessment's bracket choice
// matched: its AGE_AT_START_UNCERTAIN candidates and its seriesUsed.
func matchedBrackets(a engine.Assessment) map[string]bool {
	m := map[string]bool{}
	for _, r := range reasonsWith(a, engine.ReasonAgeAtStartUncertain) {
		for _, c := range r.Params.Candidates {
			m[c.Series] = true
		}
	}
	for _, id := range a.SeriesUsed {
		m[id] = true
	}
	return m
}
