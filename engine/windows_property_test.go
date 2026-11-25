package engine_test

import (
	"bytes"
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

// windowedVariant gives a third of s's series a random validity window
// (from only, to only, or both) and splits another third into two
// successive versions at a random date: the series ends there and a copy
// with the same ID and "-v2" starts there, its minimum intervals doubled or
// halved, so that a dose can meet the rule in effect on its date. Bounds
// are drawn in [lo, hi]. ok is false when Validate rejects the variant. A
// variant may leave a disease with no series in effect on some days.
func windowedVariant(rng *rand.Rand, s *ruleset.Snapshot, lo, hi engine.Date) (*ruleset.Snapshot, bool) {
	out := cloneSnapshot(s)
	span := daysBetween(lo, hi)
	date := func(d engine.Date) *ruleset.Date {
		y, m, dd := d.YMD()
		return &ruleset.Date{Year: y, Month: m, Day: dd}
	}
	n := len(out.Series)
	for i := range n {
		a, b := lo.AddDays(rng.IntN(span)), lo.AddDays(rng.IntN(span))
		if b.Before(a) {
			a, b = b, a
		}
		if a == b {
			b = b.AddDays(1)
		}
		switch rng.IntN(3) {
		case 0:
		case 1:
			switch rng.IntN(3) {
			case 0:
				out.Series[i].ValidFrom = date(a)
			case 1:
				out.Series[i].ValidTo = date(b)
			default:
				out.Series[i].ValidFrom, out.Series[i].ValidTo = date(a), date(b)
			}
		default:
			v2 := out.Series[i]
			v2.ID += "-v2"
			v2.Plans = slices.Clone(v2.Plans)
			for j := range v2.Plans {
				v2.Plans[j].Doses = slices.Clone(v2.Plans[j].Doses)
				for k, d := range v2.Plans[j].Doses {
					if d.MinInterval == nil {
						continue
					}
					m := *d.MinInterval
					if rng.IntN(2) == 0 {
						m.N *= 2
					} else {
						m.N /= 2
					}
					v2.Plans[j].Doses[k].MinInterval = &m
				}
			}
			out.Series[i].ValidTo, v2.ValidFrom = date(a), date(a)
			out.Series = append(out.Series, v2)
		}
	}
	return out, len(ruleset.Rejects(ruleset.Validate(out))) == 0
}

// rdate converts an engine date to a validity bound.
func rdate(d engine.Date) *ruleset.Date {
	y, m, dd := d.YMD()
	return &ruleset.Date{Year: y, Month: m, Day: dd}
}

// historySpan returns the earliest date of in's history for disease that
// #29's covering windows must contain: the earliest day of its earliest
// dated dose (today with none), or, when the disease has an undated
// record, the earliest day of the birth date, since the trailing slot's
// envelope reads the segments from b_old on (Q105). unbounded is set for an
// undated record with no birth date: the envelope then reads every segment
// up to today.
func historySpan(in engine.Input, disease string) (lower engine.Date, unbounded bool) {
	lower = in.Today
	undated := false
	for _, r := range in.Doses {
		if !slices.Contains(r.Diseases, disease) {
			continue
		}
		if lo, _, ok := testPeriod(r.Date, in.Today); ok && lo.Before(lower) {
			lower = lo
		} else if !ok && r.Date.Precision == engine.Unknown {
			undated = true
		}
	}
	if !undated {
		return lower, false
	}
	lo, _, ok := testPeriod(in.Profile.Birth, in.Today)
	if !ok {
		return lower, true
	}
	if lo.Before(lower) {
		lower = lo
	}
	return lower, false
}

// TestPropertyWindowsCoveringHistoryChangeNothing (issue #29): windows that
// contain every relevant date change nothing. Each random input of each
// windowless fixture is evaluated again on a variant where every series of
// a disease gets its own random window containing every date from the
// history's start (historySpan: the earliest dose date, today with no
// dose; from birth when the disease has an undated record, with no lower
// bound when there is no birth date either) to the latest overdue date the
// windowless output shows (today with none). Every dose range and every
// shown date then lies in one segment, so no version arises and the
// envelope reads today's series only: the output must be byte-equal to the
// windowless one. A counter fails the test when no window starts after the
// birth date's earliest day, the case a lower bound at birth would miss.
func TestPropertyWindowsCoveringHistoryChangeNothing(t *testing.T) {
	t.Parallel()
	compared, afterBirth := 0, 0
	for _, fx := range fixtures(t) {
		if windowed(fx.snap) {
			continue
		}
		rng := rand.New(rand.NewPCG(67, uint64(len(fx.name))))
		for range propertyRuns / 2 {
			in := randomInput(t, rng, fx.snap)
			out, want := evaluate(t, fx.engine, in)
			upper := in.Today
			for _, a := range out.Assessments {
				if a.NextDose != nil && a.NextDose.Overdue.After(upper) {
					upper = a.NextDose.Overdue
				}
			}
			v := cloneSnapshot(fx.snap)
			birth, _, hasBirth := testPeriod(in.Profile.Birth, in.Today)
			for i := range v.Series {
				lower, unbounded := historySpan(in, v.Series[i].Disease)
				if !unbounded && rng.IntN(4) != 0 {
					from := lower.AddDays(-rng.IntN(400))
					v.Series[i].ValidFrom = rdate(from)
					if hasBirth && from.After(birth) {
						afterBirth++
					}
				}
				if rng.IntN(4) != 0 {
					v.Series[i].ValidTo = rdate(upper.AddDays(1 + rng.IntN(400)))
				}
			}
			e := newEngine(t, v, warnCodes(fx.snap)...)
			if _, got := evaluate(t, e, in); !bytes.Equal(got, want) {
				t.Fatalf("%s: covering windows changed the output\nwindowless %s\nwindowed   %s\ninput %+v", fx.name, want, got, in)
			}
			compared++
		}
	}
	t.Logf("%d inputs compared, %d windows from after the birth date", compared, afterBirth)
	if compared == 0 || afterBirth == 0 {
		t.Fatalf("nothing compared (%d) or no window starts after the birth date (%d)", compared, afterBirth)
	}
}

// warnCodes returns the Warn codes Validate gives s, for newEngine.
func warnCodes(s *ruleset.Snapshot) []ruleset.ProblemCode {
	var out []ruleset.ProblemCode
	for _, p := range ruleset.Validate(s) {
		out = append(out, p.Code)
	}
	return out
}

// TestPropertyIdenticalCopiesChangeNothing (issue #29 In scope 3): a series
// re-issued with identical rules changes nothing. Each random input of
// each windowless fixture is evaluated again on a variant where one random
// series ends on a date inside the input's history and an identical copy
// ("<id>-copy") starts that day. The output must equal the windowless one
// once the copy's ID is read as the original's (in seriesUsed and in the
// reasons' series and candidates). This is what the better-of dose verdict
// buys: without c's own verdict on the whole range, the parts could turn a
// possible verdict into a definite one.
//
// That needs a cut inside an imprecise dose range that crosses a maximum
// age, which random inputs seldom give. So a fixture with maximum ages
// (ageLimits) also gets len(limits) × propertyRuns / 2 near-limit inputs
// (nearLimitInput), half the cuts fall inside an imprecise dose range of
// the disease, and a counter fails the test when no cut falls inside a
// dose range of a copied series with a maximum age.
//
// Only a series alone in its product group is copied, which narrows #29's
// property. Copying one of a bracketed group makes the group's other
// series versions in the older segment, and with no dose 1 counted #29 In
// scope 3 (Q93) matches a bracketed version "possibly" whatever the dose's
// own date, so the copy can soften a definite failure
// (TestIdenticalCopyInBracketedGroup pins the case). The two decided rules
// conflict there; the narrowing awaits an owner decision.
func TestPropertyIdenticalCopiesChangeNothing(t *testing.T) {
	t.Parallel()
	compared, inside := 0, 0
	for _, fx := range fixtures(t) {
		if windowed(fx.snap) {
			continue
		}
		rng := rand.New(rand.NewPCG(71, uint64(len(fx.name))))
		limits := ageLimits(fx.snap)
		for run := range propertyRuns/2 + len(limits)*propertyRuns/2 {
			var in engine.Input
			if run < propertyRuns/2 {
				in = randomInput(t, rng, fx.snap)
			} else {
				in = nearLimitInput(t, rng, fx.snap, limits)
			}
			_, want := evaluate(t, fx.engine, in)
			var alone []int
			for i, sr := range fx.snap.Series {
				if !slices.ContainsFunc(fx.snap.Series, func(o ruleset.Series) bool {
					return o.ID != sr.ID && o.Disease == sr.Disease && (o.Product == nil) == (sr.Product == nil) && (o.Product == nil || *o.Product == *sr.Product)
				}) {
					alone = append(alone, i)
				}
			}
			if len(alone) == 0 {
				continue
			}
			i := alone[rng.IntN(len(alone))]
			orig := fx.snap.Series[i]
			lower, _ := historySpan(in, orig.Disease)
			cut := lower.AddDays(1 + rng.IntN(daysBetween(lower, in.Today)+1))
			// Half the time the cut falls inside an imprecise dose range of
			// the disease, where c's verdict on the whole range and the
			// parts' can differ.
			var ranges [][2]engine.Date
			for _, r := range in.Doses {
				if lo, hi, ok := testPeriod(r.Date, in.Today); ok && slices.Contains(r.Diseases, orig.Disease) && lo.Before(hi) {
					ranges = append(ranges, [2]engine.Date{lo, hi})
				}
			}
			if len(ranges) > 0 && rng.IntN(2) == 0 {
				x := ranges[rng.IntN(len(ranges))]
				cut = x[0].AddDays(1 + rng.IntN(daysBetween(x[0], x[1])))
			}
			if len(ageLimits(&ruleset.Snapshot{Series: []ruleset.Series{orig}})) > 0 &&
				slices.ContainsFunc(ranges, func(x [2]engine.Date) bool { return x[0].Before(cut) && !cut.After(x[1]) }) {
				inside++
			}
			v := cloneSnapshot(fx.snap)
			cp := v.Series[i]
			cp.ID += "-copy"
			v.Series[i].ValidTo, cp.ValidFrom = rdate(cut), rdate(cut)
			v.Series = slices.Insert(v.Series, i+1, cp)
			e := newEngine(t, v, warnCodes(v)...)
			out, _ := evaluate(t, e, in)
			for j := range out.Assessments {
				a := &out.Assessments[j]
				for k, id := range a.SeriesUsed {
					if id == cp.ID {
						a.SeriesUsed[k] = orig.ID
					}
				}
				for k := range a.Reasons {
					p := &a.Reasons[k].Params
					if p.Series == cp.ID {
						p.Series = orig.ID
					}
					for c := range p.Candidates {
						if p.Candidates[c].Series == cp.ID {
							p.Candidates[c].Series = orig.ID
						}
					}
				}
			}
			got, err := engine.EncodeOutput(out)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s: an identical copy of %s from %s changed the output\nwindowless %s\nwith copy  %s\ninput %+v", fx.name, orig.ID, cut, want, got, in)
			}
			compared++
		}
	}
	t.Logf("%d inputs compared, %d cut inside a dose range of a series with a maximum age", compared, inside)
	if compared == 0 || inside == 0 {
		t.Fatalf("nothing compared (%d) or no cut inside a dose range of a series with a maximum age (%d)", compared, inside)
	}
}
