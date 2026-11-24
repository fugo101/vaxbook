package engine_test

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property-test oracles for the maximum ages of the next dose (issue #28,
// S4b), computed from the snapshot with Date.AddSpan alone. Limits are last
// allowed days: one day before the exclusive limit date.

// lastDayLimit is an oracle limit: none (ok false) or a last allowed day.
type lastDayLimit struct {
	day engine.Date
	ok  bool
}

func (x lastDayLimit) min(y lastDayLimit) lastDayLimit {
	switch {
	case !y.ok:
		return x
	case !x.ok || y.day.Before(x.day):
		return y
	}
	return x
}

// exceededBy reports whether a dose on d is past the limit.
func (x lastDayLimit) exceededBy(d engine.Date) bool { return x.ok && d.After(x.day) }

// spanLimit is from + span − 1 day, or none without a span.
func spanLimit(from engine.Date, span *ruleset.Span) lastDayLimit {
	if span == nil {
		return lastDayLimit{}
	}
	return lastDayLimit{day: from.AddSpan(*span).AddDays(-1), ok: true}
}

// groupLastMax returns the max of the last start-age bracket of sr's product
// group (by min; Validate makes a group's brackets disjoint), or nil.
func groupLastMax(s *ruleset.Snapshot, sr ruleset.Series) *ruleset.Span {
	var last *ruleset.Series
	for i, o := range s.Series {
		if o.Disease != sr.Disease || (o.Product == nil) != (sr.Product == nil) || o.Product != nil && *o.Product != *sr.Product {
			continue
		}
		if last == nil || last.StartAge != nil && o.StartAge != nil && last.StartAge.Max != nil && ruleset.SpanAtMost(*last.StartAge.Max, o.StartAge.Min) {
			last = &s.Series[i]
		}
	}
	if last == nil || last.StartAge == nil {
		return nil
	}
	return last.StartAge.Max
}

// startLimitOf is dose 1's start limit S_c: maxAgeStart from ageBirth and the
// group's last bracket max from bracketBirth.
func startLimitOf(s *ruleset.Snapshot, sr ruleset.Series, ageBirth, bracketBirth engine.Date) lastDayLimit {
	return spanLimit(ageBirth, sr.MaxAgeStart).min(spanLimit(bracketBirth, groupLastMax(s, sr)))
}

// doseOnePlans returns dose 1's (earliest, due) in each plan of sr for a
// person born on b with no dose given: earliest = max(b, b + minAge), due =
// max(earliest, b + offset) (dose 1 counts from birth).
func doseOnePlans(sr ruleset.Series, b engine.Date) (earliest, due []engine.Date) {
	for _, p := range sr.Plans {
		r := p.Doses[0]
		e := b
		if r.MinAge != nil {
			e = maxOf(e, b.AddSpan(*r.MinAge))
		}
		du := e
		if r.Recommended.From == ruleset.FromBirth {
			du = maxOf(e, b.AddSpan(r.Recommended.Offset))
		}
		earliest, due = append(earliest, e), append(due, du)
	}
	return earliest, due
}

func maxOf(a, b engine.Date) engine.Date {
	if b.After(a) {
		return b
	}
	return a
}

// noDoseOracle recomputes the no-dose candidates of one disease with A3's
// removal (issue #28 In scope 1, Q65, Q72) for a birth range starting on lo.
type noDoseOracle struct {
	s       *ruleset.Snapshot
	disease string
	lo      engine.Date
	today   engine.Date
}

// raw is S3b's choice (noDoseChoice) at b, before A3.
func (o noDoseOracle) raw(b engine.Date) []string { return noDoseChoice(o.s, o.disease, b, o.today) }

// subStart returns the older end of the start-age sub-range holding b: the
// first day from which every day up to b has b's raw choice.
func (o noDoseOracle) subStart(b engine.Date) engine.Date {
	want := o.raw(b)
	d := b
	for d.After(o.lo) && slices.Equal(o.raw(d.AddDays(-1)), want) {
		d = d.AddDays(-1)
	}
	return d
}

// series returns the snapshot series with the given IDs.
func (o noDoseOracle) series(ids []string) []ruleset.Series {
	var out []ruleset.Series
	for _, sr := range o.s.Series {
		if slices.Contains(ids, sr.ID) {
			out = append(out, sr)
		}
	}
	return out
}

// filter applies A3 at birth date b, with b_s the older end of its
// sub-range: a candidate is removed when its own F_c (today, or its most
// lenient plan's earliest date) is past Limit_c,1 = min(L_c, S_c) from
// b_old = lo and b_s; then, until stable, when the merged F (from each one's
// plans not past its limit, A7) is past a candidate's limit. certain: every
// candidate removed is past its limit at b itself.
func (o noDoseOracle) filter(ids []string, b, bs engine.Date) (kept []string, certain bool) {
	type cand struct {
		id         string
		limit, own lastDayLimit
		earliest   engine.Date // A7's choice over the kept plans
	}
	var left []cand
	certain = true
	for _, sr := range o.series(ids) {
		limit := spanLimit(o.lo, sr.MaxAgeComplete).min(startLimitOf(o.s, sr, o.lo, bs))
		own := spanLimit(b, sr.MaxAgeComplete).min(startLimitOf(o.s, sr, b, b))
		es, dues := doseOnePlans(sr, b)
		f := maxOf(o.today, slices.MinFunc(es, engine.Date.Compare))
		if limit.exceededBy(f) {
			certain = certain && own.exceededBy(f)
			continue
		}
		c := cand{id: sr.ID, limit: limit, own: own}
		bestDue, first := engine.Date{}, true
		for i, e := range es {
			if limit.exceededBy(maxOf(o.today, e)) {
				continue
			}
			if first || dues[i].Before(bestDue) || dues[i] == bestDue && e.After(c.earliest) {
				c.earliest, bestDue, first = e, dues[i], false
			}
		}
		left = append(left, c)
	}
	for {
		f := o.today
		for _, c := range left {
			f = maxOf(f, c.earliest)
		}
		n := len(left)
		left = slices.DeleteFunc(left, func(c cand) bool { return c.limit.exceededBy(f) })
		if len(left) == n {
			break
		}
	}
	for _, c := range left {
		kept = append(kept, c.id)
	}
	return kept, certain
}

// choice returns the candidates at birth date b after A3, with Q72's
// fallback to every group when the non-retired choice is all removed, and
// whether the removal is certain at b when none is left.
func (o noDoseOracle) choice(b engine.Date) (kept []string, certain bool) {
	retired := func(sr ruleset.Series) bool {
		return sr.Product != nil && slices.ContainsFunc(o.s.Products, func(p ruleset.Product) bool { return p.ID == *sr.Product && p.Retired })
	}
	raw := o.raw(b)
	if len(raw) == 0 {
		return nil, true
	}
	bs := o.subStart(b)
	kept, certain = o.filter(raw, b, bs)
	all := noDoseChoice(&ruleset.Snapshot{Products: nil, Series: o.s.Series}, o.disease, b, o.today)
	if len(kept) == 0 && !slices.ContainsFunc(o.series(raw), retired) && !slices.Equal(all, raw) {
		kept, certain = o.filter(all, b, bs)
	}
	return kept, certain
}

// seriesByID returns the snapshot series with that ID.
func seriesByID(s *ruleset.Snapshot, id string) ruleset.Series {
	return s.Series[slices.IndexFunc(s.Series, func(sr ruleset.Series) bool { return sr.ID == id })]
}

// remainingFit reports whether, with dose k of sr on d, doses k+1…n each on
// max(previous + minInterval, dose 1 + minIntervalFromFirst, late +
// minAge) under the most lenient plan for that dose end on or before last.
// first is dose 1's date when k > 1.
func remainingFit(sr ruleset.Series, k int, d, late, first, last engine.Date) bool {
	if k == 1 {
		first = d
	}
	prev := d
	for j := k + 1; j <= len(sr.Plans[0].Doses); j++ {
		var best engine.Date
		for i, p := range sr.Plans {
			r := p.Doses[j-1]
			x := prev
			if r.MinInterval != nil {
				x = prev.AddSpan(*r.MinInterval)
			}
			if r.MinIntervalFromFirst != nil {
				x = maxOf(x, first.AddSpan(*r.MinIntervalFromFirst))
			}
			if r.MinAge != nil {
				x = maxOf(x, late.AddSpan(*r.MinAge))
			}
			if i == 0 || x.Before(best) {
				best = x
			}
		}
		prev = best
	}
	return !prev.After(last)
}

// TestPropertyCapP1 (P1, EQ15; issue #28 In scope 3), on the near-limit
// inputs of the fixtures with maximum ages or start-age brackets: for every
// emitted next dose, with F = max(today, earliest), and L_k and Lim
// recomputed from the snapshot over the candidates at k (ResultViews) —
// limits from the older end of the birth range (S's bracket term from the
// older end of the later end's sub-range with no dose given), C_c,k by a
// day-by-day scan of the forward chain from L_c down —
//   - F ≤ L_k ⇒ due and overdue ≤ L_k;
//   - L_k < F ≤ Lim ⇒ SERIES_MAY_NOT_COMPLETE{lastDay: Lim};
//   - F > Lim never comes with an emitted next dose.
//
// It also checks P3 (a capped next dose is never 🔴), and that the inputs
// reach both cap reasons.
func TestPropertyCapP1(t *testing.T) {
	t.Parallel()
	seen := map[engine.ReasonCode]int{}
	for _, fx := range fixtures(t) {
		limits := ageLimits(fx.snap)
		if len(limits) == 0 {
			continue
		}
		rng := rand.New(rand.NewPCG(83, uint64(len(fx.name))))
		for range len(limits) * propertyRuns {
			in := nearLimitInput(t, rng, fx.snap, limits)
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok {
				continue
			}
			out, _ := evaluate(t, fx.engine, in)
			views, err := engine.ResultViews(fx.engine, in)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				for _, c := range []engine.ReasonCode{engine.ReasonDueCappedByMaxAge, engine.ReasonSeriesMayNotComplete} {
					if hasCode(a, c) {
						seen[c]++
						if a.Bucket == engine.BucketOverdue {
							t.Fatalf("%s %s: %s with 🔴 (P3): %+v", fx.name, a.Disease, c, a)
						}
					}
				}
				nd := a.NextDose
				if nd == nil {
					continue
				}
				bs := lo
				var dated []engine.DoseRecord
				for _, r := range in.Doses {
					if slices.Contains(r.Diseases, a.Disease) && r.Date.Precision != engine.Unknown {
						dated = append(dated, r)
					}
				}
				if !slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) }) {
					bs = noDoseOracle{s: fx.snap, disease: a.Disease, lo: lo, today: in.Today}.subStart(hi)
				}
				// Dose 1's R.Hi (step 7's first anchor): with no blocking
				// reason every in-series record counts, so it is the
				// earliest dated record in time.
				var first, firstLo engine.Date
				for i, r := range dated {
					if rlo, rhi, _ := testPeriod(r.Date, in.Today); i == 0 || rlo.Before(firstLo) {
						first, firstLo = rhi, rlo
					}
				}
				f := maxOf(in.Today, nd.Earliest)
				var lk, lim lastDayLimit
				lkOK, any := true, false // lkOK: L_k ≥ F (else only L_k < F is known)
				for _, cv := range views[a.Disease].Walked {
					if cv.Complete || cv.K != nd.Number {
						continue
					}
					sr := seriesByID(fx.snap, cv.Series)
					lc := spanLimit(lo, sr.MaxAgeComplete)
					var sc lastDayLimit
					if nd.Number == 1 {
						sc = startLimitOf(fx.snap, sr, lo, bs)
					}
					l := lc.min(sc)
					if !l.ok {
						continue
					}
					any = true
					lim = lim.min(l)
					ck := lc // C_c,n = L_c, and absent without L_c
					if lc.ok && nd.Number < len(sr.Plans[0].Doses) {
						ck.ok = false
						for d := lc.day; !d.Before(f); d = d.AddDays(-1) {
							if remainingFit(sr, nd.Number, d, hi, first, lc.day) {
								ck = lastDayLimit{day: d, ok: true}
								break
							}
						}
						if !ck.ok {
							lkOK = false // C_c,k < F
						}
					}
					lk = lk.min(ck.min(sc))
				}
				if !any {
					continue
				}
				fail := func(msg string) {
					t.Helper()
					t.Fatalf("%s %s: %s: F %s, L_k %+v (≥ F: %v), Lim %+v\nassessment %+v\ninput %+v", fx.name, a.Disease, msg, f, lk, lkOK, lim, a, in)
				}
				switch {
				case lim.exceededBy(f):
					fail("a next dose past Lim")
				case lkOK && lk.ok && !lk.exceededBy(f):
					if nd.Due.After(lk.day) || nd.Overdue.After(lk.day) {
						fail("due or overdue after L_k")
					}
				default:
					want := engine.Reason{Code: engine.ReasonSeriesMayNotComplete, Params: engine.ReasonParams{LastDay: lim.day}}
					if !slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool { return reflect.DeepEqual(r, want) }) {
						fail("L_k < F ≤ Lim without SERIES_MAY_NOT_COMPLETE{Lim}")
					}
				}
			}
		}
	}
	for _, c := range []engine.ReasonCode{engine.ReasonDueCappedByMaxAge, engine.ReasonSeriesMayNotComplete} {
		if seen[c] == 0 {
			t.Errorf("near-limit inputs never reach %s", c)
		}
	}
}

// s4bInputs draws the inputs of the S4b properties for one fixture: the
// near-limit inputs (a fixture without a limit gives none), half of them
// with their doses removed, so that A3's no-dose removal is reached too.
func s4bInputs(t *testing.T, fx fixture, seed uint64, each func(in engine.Input)) {
	t.Helper()
	limits := ageLimits(fx.snap)
	if len(limits) == 0 {
		return
	}
	rng := rand.New(rand.NewPCG(seed, uint64(len(fx.name))))
	for i := range len(limits) * propertyRuns {
		in := nearLimitInput(t, rng, fx.snap, limits)
		if i%2 == 1 {
			in.Doses = []engine.DoseRecord{}
		}
		each(in)
	}
}

// TestPropertyNoDoseNoProductDisagreement (A3, issue #28 In scope 1): with
// no dose of the disease, the result never has CHECK_PRODUCT_NAME or
// MIXED_PRODUCTS: the candidates already out of age, or past the merged F,
// are removed before step 7. The no-dose inputs must reach A3's removal
// (a SAFEST_SERIES-free result on a fixture whose no-dose choice has
// several series) and outOfAge.
func TestPropertyNoDoseNoProductDisagreement(t *testing.T) {
	t.Parallel()
	removed, outOfAge := 0, 0
	for _, fx := range fixtures(t) {
		s4bInputs(t, fx, 89, func(in engine.Input) {
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				if slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) }) {
					continue
				}
				if productDisagreement(a) {
					t.Fatalf("%s %s: no dose, but %+v\ninput %+v", fx.name, a.Disease, a, in)
				}
				if a.Block == engine.BlockOutOfAge {
					outOfAge++
				}
				if ok && len(noDoseChoice(fx.snap, a.Disease, hi, in.Today)) > len(a.SeriesUsed) && a.Block != engine.BlockOutOfAge {
					o := noDoseOracle{s: fx.snap, disease: a.Disease, lo: lo, today: in.Today}
					if kept, _ := o.choice(hi); len(kept) > 0 {
						removed++
					}
				}
			}
		})
	}
	if removed == 0 || outOfAge == 0 {
		t.Fatalf("A3 removed a candidate %d times and gave outOfAge %d times; want both", removed, outOfAge)
	}
}

// TestPropertyOutOfAgeNoCompleteCandidate (A1, P1; issue #28 In scope 2):
// a complete candidate never yields outOfAge. For every outOfAge with doses
// given, each series of seriesUsed evaluated alone (a snapshot that keeps
// only that series for the disease, as TestPropertyMergeCoversCandidates
// does) is never ✅ COMPLETE.
func TestPropertyOutOfAgeNoCompleteCandidate(t *testing.T) {
	t.Parallel()
	checked := 0
	for _, fx := range fixtures(t) {
		alone := map[string]*engine.Engine{}
		single := func(id string) *engine.Engine {
			if e, ok := alone[id]; ok {
				return e
			}
			s := loadSnapshot(t, fx.name)
			disease := seriesByID(s, id).Disease
			s.Series = slices.DeleteFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == disease && sr.ID != id })
			e, err := engine.New(s)
			if err != nil {
				t.Fatalf("%s with %s alone: %v", fx.name, id, err)
			}
			alone[id] = e
			return e
		}
		s4bInputs(t, fx, 97, func(in engine.Input) {
			out, _ := evaluate(t, fx.engine, in)
			for j, a := range out.Assessments {
				if a.Block != engine.BlockOutOfAge || a.SeriesUsed == nil {
					continue
				}
				checked++
				for _, id := range a.SeriesUsed {
					o, _ := evaluate(t, single(id), in)
					if b := o.Assessments[j]; b.Bucket == engine.BucketComplete && hasCode(b, engine.ReasonComplete) {
						t.Fatalf("%s %s: outOfAge, but %s alone is complete\noutOfAge %+v\nalone %+v\ninput %+v", fx.name, a.Disease, id, a, b, in)
					}
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no near-limit input gave outOfAge with doses given: the property is vacuous")
	}
}

// TestPropertyOutOfAgeFromCountedView (issue #28, Q64, Q85): OUT_OF_AGE with
// doses given never comes from an uncounted dose. For every outOfAge with
// doses given, the counted view of the final result (ResultViews; the
// latest evaluated date's) has no complete candidate, and each incomplete
// one is certainly out at that date, recomputed from the snapshot with the
// limits at that date b: its own F_c = max(today, its most lenient plan's
// earliest date) is past Limit^b = min(b + maxAgeComplete, and for dose 1
// b + maxAgeStart and b + the group's last bracket max) − 1 day, or step 7
// stopped on PREVIOUS_DOSE_DATE_UNKNOWN and today is past b +
// maxAgeComplete − 1 day (EQ11).
func TestPropertyOutOfAgeFromCountedView(t *testing.T) {
	t.Parallel()
	checked := 0
	for _, fx := range fixtures(t) {
		s4bInputs(t, fx, 101, func(in engine.Input) {
			_, b, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok {
				return
			}
			out, _ := evaluate(t, fx.engine, in)
			views, err := engine.ResultViews(fx.engine, in)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range out.Assessments {
				if a.Block != engine.BlockOutOfAge || a.SeriesUsed == nil {
					continue
				}
				checked++
				for _, cv := range views[a.Disease].Counted {
					sr := seriesByID(fx.snap, cv.Series)
					limit := spanLimit(b, sr.MaxAgeComplete)
					certain := false
					switch {
					case cv.Complete:
					case cv.StoppedOnUnknown:
						certain = limit.exceededBy(in.Today)
					case len(cv.Earliest) > 0:
						if cv.K == 1 {
							limit = limit.min(startLimitOf(fx.snap, sr, b, b))
						}
						certain = limit.exceededBy(maxOf(in.Today, slices.MinFunc(cv.Earliest, engine.Date.Compare)))
					}
					if !certain {
						t.Fatalf("%s %s: outOfAge, but in the counted view %+v is not certainly out at %s\nassessment %+v\ninput %+v", fx.name, a.Disease, cv, b, a, in)
					}
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no near-limit input gave outOfAge with doses given: the property is vacuous")
	}
}

// TestPropertyImpreciseOutOfAgeEveryDay (issue #28, F05.4a, Q65): outOfAge
// for a month- or year-precision birth date implies outOfAge at every exact
// birth date in the (today-capped) range. Only outOfAge results are
// expanded day by day. The near-limit inputs get a month- or
// year-precision birth date.
func TestPropertyImpreciseOutOfAgeEveryDay(t *testing.T) {
	t.Parallel()
	checked := 0
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(103, uint64(len(fx.name))))
		s4bInputs(t, fx, 107, func(in engine.Input) {
			if in.Profile.Birth.Precision == engine.None {
				return
			}
			b, err := engine.ParseDate(in.Profile.Birth.Value[:7] + "-01")
			if err != nil {
				t.Fatal(err)
			}
			in.Profile.Birth = engine.RecalledDate{Value: b.String()[:7], Precision: engine.Month}
			if rng.IntN(3) == 0 {
				in.Profile.Birth = engine.RecalledDate{Value: b.String()[:4], Precision: engine.Year}
			}
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok || slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool {
				rlo, _, dated := testPeriod(r.Date, in.Today)
				return dated && rlo.Before(hi)
			}) {
				return // a dose before a birth day would be a record problem there
			}
			out, _ := evaluate(t, fx.engine, in)
			for i, a := range out.Assessments {
				if a.Block != engine.BlockOutOfAge {
					continue
				}
				checked++
				for d := lo; !d.After(hi); d = d.AddDays(1) {
					exact := cloneInput(in)
					exact.Profile.Birth = engine.RecalledDate{Value: d.String(), Precision: engine.Day}
					o, _ := evaluate(t, fx.engine, exact)
					if o.Assessments[i].Block != engine.BlockOutOfAge {
						t.Fatalf("%s %s: outOfAge for birth %v, but not on %s: %+v\ninput %+v", fx.name, a.Disease, in.Profile.Birth, d, o.Assessments[i], in)
					}
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no imprecise birth date gave outOfAge: the property is vacuous")
	}
}
