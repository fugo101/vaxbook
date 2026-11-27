package engine

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// versionTimelines returns windowed disease timelines for the version
// properties: minimal-effective-dating's ed, and an in-test disease k with
// a bracketed group k-a [6w, 9m) (dose 2 at 4w, maxAgeComplete 3y) and k-b
// [9m, —) (8w, 2y) and a three-dose series k-q-old for product q (4w,
// then 4w) until 2026-01-01, then k-2026 (8w, 2y), a product series k-p
// for product p (10w) and a two-dose k-q-new for q (10w) from then. The
// lenient older series exercise each rule that keeps a version out: Q93's
// "possibly" bracket match (k-a), Q94's tiers (k-a under k-p for a dose
// with no product) and Q96's last dose (k-q-old under k-q-new). Two more
// diseases have versions whose verdict depends on the birth date (Q107): x
// with x-old (dose 1 minAge 4w, dose 2 at 4w) until 2026-01-01, then x-new
// (6w, 4w); and g with g-2025 (one dose, minAge 9m, maxAgeComplete 2y)
// until 2026-01-01, then g-2026 (4w, 8m). A last disease m has m-old (one
// dose, minAge 38w, maxAgeComplete 40w) until 2026-01-01, then m-new
// (minAge 1y): m-old passes a dose for two weeks of birth dates only, so a
// birth range can fail it at both ends, its minimum age at b_late and its
// maximum age at b_old, and pass it between (Q107). No series of these
// timelines is in effect in two segments, so no twin arises (Q109).
func versionTimelines(t *testing.T) []diseaseTimeline {
	t.Helper()
	ed := gapFixture{name: "minimal-effective-dating", disease: "ed"}.diseaseOf(t)
	doses := func(intervals ...string) string {
		out := `{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}`
		for _, iv := range intervals {
			out += `, {"minInterval": "` + iv + `", "recommended": {"from": "previousDose", "offset": "P1M"}}`
		}
		return out
	}
	series := func(id, extra string, intervals ...string) string {
		return `{"id": "` + id + `", "disease": "k", "stage": "childhood", ` + extra + `"plans": [{"id": "a", "doses": [` + doses(intervals...) + `]}]}`
	}
	x := func(id, minAge, extra string) string {
		return `{"id": "` + id + `", "disease": "x", "stage": "childhood", ` + extra + `"plans": [{"id": "a", "doses": [` +
			`{"minAge": "` + minAge + `", "recommended": {"from": "birth", "offset": "P2M"}}, ` +
			`{"minInterval": "P4W", "recommended": {"from": "previousDose", "offset": "P4W"}}]}]}`
	}
	g := func(id, minAge, complete, extra string) string {
		return `{"id": "` + id + `", "disease": "g", "stage": "childhood", "maxAgeComplete": "` + complete + `", ` + extra + `"plans": [{"id": "a", "doses": [` +
			`{"minAge": "` + minAge + `", "recommended": {"from": "birth", "offset": "P9M"}}]}]}`
	}
	m := func(id, minAge, complete, extra string) string {
		return strings.Replace(g(id, minAge, complete, extra), `"disease": "g"`, `"disease": "m"`, 1)
	}
	doc := `{"version": "v", "illustrative": true, "diseases": [{"id": "k", "name": "K"}, {"id": "x", "name": "X"}, {"id": "g", "name": "G"}, {"id": "m", "name": "M"}],
	  "products": [{"id": "p", "name": "P", "diseases": ["k"], "live": false}, {"id": "q", "name": "Q", "diseases": ["k"], "live": false}], "series": [` + strings.Join([]string{
		series("k-a", `"startAge": {"min": "P6W", "max": "P9M"}, "maxAgeComplete": "P3Y", "validTo": "2026-01-01", `, "P4W"),
		series("k-b", `"startAge": {"min": "P9M"}, "maxAgeComplete": "P2Y", "validTo": "2026-01-01", `, "P8W"),
		series("k-q-old", `"product": "q", "validTo": "2026-01-01", `, "P4W", "P4W"),
		series("k-2026", `"maxAgeComplete": "P2Y", "validFrom": "2026-01-01", `, "P8W"),
		series("k-p", `"product": "p", "validFrom": "2026-01-01", `, "P10W"),
		series("k-q-new", `"product": "q", "validFrom": "2026-01-01", `, "P10W"),
		x("x-old", "P4W", `"validTo": "2026-01-01", `),
		x("x-new", "P6W", `"validFrom": "2026-01-01", `),
		g("g-2025", "P9M", "P2Y", `"validTo": "2026-01-01", `),
		g("g-2026", "P4W", "P8M", `"validFrom": "2026-01-01", `),
		m("m-old", "P38W", "P40W", `"validTo": "2026-01-01", `),
		strings.Replace(m("m-new", "P1Y", "P5Y", `"validFrom": "2026-01-01", `), `"maxAgeComplete": "P5Y", `, "", 1),
	}, ", ") + `]}`
	s, err := ruleset.Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return append([]diseaseTimeline{ed}, e.diseases...)
}

// segmentOf returns the index of the segment of tl that holds day.
func segmentOf(tl diseaseTimeline, day Date) int {
	i := 0
	for i+1 < len(tl.starts) && !day.Before(tl.starts[i+1]) {
		i++
	}
	return i
}

// qualifying calls f with each series d of day y's segment j ≠ i that may
// judge a dose of product on y at position pos under c (series ci of
// segment i, today's), read from the segments' models rather than from the
// compiled versions (issue #29 In scope 3), and how its bracket matches:
//   - Q94: d is in the tier of the dose's catalog product in j, and d is in
//     c's group or c is in that product's tier today, neither tier the
//     every-series fall-through; a dose with no product or another product
//     only meets c's group, and of it only the series in the tier in j of
//     every catalog product (Q116; vacuous with no catalog product);
//   - Q96: d has at least pos doses, and pos is d's last when it is c's;
//   - Q93, Q95, Q106: certain when d has no bracket or is alone in its
//     group in j, or, with a birth date, every birth date and dose-1 day
//     put the age at dose 1 in d's reach [floor, max) in j; possible when
//     some do, or with no birth date. Dose 1's range is y itself for pos 1,
//     else firstBr, else [b_old, y]: dose 1 cannot come after y. A series
//     that cannot hold it is skipped.
//
// It does nothing when y lies in today's segment.
func qualifying(tl diseaseTimeline, i, ci, pos int, b birthEnd, y Date, firstBr *Range, product string, f func(d seriesModel, certain bool)) {
	c := tl.models[i].series[ci]
	j := segmentOf(tl, y)
	if j == i {
		return
	}
	mi, mj := tl.models[i], tl.models[j]
	doseOne := Range{Lo: y, Hi: y}
	switch {
	case pos == 1:
	case firstBr != nil:
		doseOne = *firstBr
	default:
		doseOne = Range{Lo: b.older(), Hi: y}
	}
	for di, d := range mj.series {
		sameGroup := d.product == c.product
		applies := sameGroup
		for _, p := range mj.catalog {
			applies = applies && slices.Contains(mj.tier[p], di)
		}
		if _, catalog := mj.tier[product]; catalog {
			applies = slices.Contains(mj.tier[product], di) &&
				(sameGroup || !mj.fallsThrough[product] && !mi.fallsThrough[product] && slices.Contains(mi.tier[product], ci))
		}
		if !applies || d.length < pos || pos == c.length && d.length != pos {
			continue
		}
		br := d.start
		certain := !br.ok || len(mj.groups[mj.groupOf[di]].series) == 1
		if !certain && b.known {
			certain = (!br.floor.ok || !doseOne.Lo.Before(b.later().AddSpan(br.floor.span))) &&
				(!br.max.ok || doseOne.Hi.Before(b.older().AddSpan(br.max.span)))
			if !certain && !br.mayContain(doseOne, Range{Lo: b.older(), Hi: b.later()}) {
				continue
			}
		}
		f(d, certain)
	}
}

// birthEnds returns b_old and b_late, or b with no birth date.
func birthEnds(b birthEnd) []birthEnd {
	if b.known {
		return []birthEnd{{date: b.older(), known: true}, {date: b.later(), known: true}}
	}
	return []birthEnd{b}
}

// countsOnDay is the counting oracle of TestPropertyVersionsOnlyHelp: a
// dose on day y meets c at b, or meets at both ends of the birth range (at
// b with no birth date) a series that certainly qualifies for it
// (qualifying).
func countsOnDay(tl diseaseTimeline, i, ci, pos int, b birthEnd, y Date, pd predecessor, first, firstBr *Range, product string) bool {
	day := Range{Lo: y, Hi: y}
	if tl.models[i].series[ci].verdictAt(pos, b, day, pd, first).sev() == verdictOK {
		return true
	}
	counts := false
	qualifying(tl, i, ci, pos, b, y, firstBr, product, func(d seriesModel, certain bool) {
		if !certain || counts {
			return
		}
		ok := true
		for _, e := range birthEnds(b) {
			ok = ok && d.verdictAt(pos, e, day, pd, first).sev() == verdictOK
		}
		counts = ok
	})
	return counts
}

// passesOnDay is the definite oracle of TestPropertyVersionsOnlyHelp: some
// series that qualifies for a dose on day y, certainly or possibly, passes
// it for some birth date of the range, each day of it tried (at b with no
// birth date), so the dose is not certainly invalid there (Q93, Q107).
func passesOnDay(tl diseaseTimeline, i, ci, pos int, b birthEnd, y Date, pd predecessor, first, firstBr *Range, product string) (string, Date, bool) {
	day := Range{Lo: y, Hi: y}
	id, at, passes := "", Date{}, false
	qualifying(tl, i, ci, pos, b, y, firstBr, product, func(d seriesModel, _ bool) {
		births := []birthEnd{b}
		if b.known {
			births = births[:0]
			for x := b.older(); !x.After(b.later()); x = x.AddDays(1) {
				births = append(births, birthEnd{date: x, known: true})
			}
		}
		for _, e := range births {
			if !passes && d.verdictAt(pos, e, day, pd, first).sev() < verdictDefinite {
				id, at, passes = d.id, e.date, true
			}
		}
	})
	return id, at, passes
}

// TestPropertyVersionsOnlyHelp (issue #29 In scope 3): for a fixed
// position, dose range, predecessor (dated or behind a run), dose 1 and
// dose-1 range, the verdict with versions (datedVerdict)
//   - is never worse than today's c alone (verdictAt). datedVerdict takes
//     the better of the two, so this holds by construction and guards the
//     construction;
//   - is OK only when every day of the range is counted by c or by a
//     series of that day's segment that certainly qualifies for the dose
//     (countsOnDay, an oracle read from the segments' models). A part is
//     OK only under one rule for all of it, so this is weaker than the
//     engine's rule, and it is the P1 direction: a version that wrongly
//     applies (Q94), wrongly counts a last dose (Q96) or is matched
//     certainly where only possibly (Q93) makes a dose count that this
//     oracle does not;
//   - is definite, on a range within one segment, only when no series that
//     qualifies for the dose, certainly or possibly, passes any day of the
//     range for any birth date of the range (passesOnDay, Q93, Q107): a
//     version that passes at one end only, or only between the ends (m-old),
//     must leave the verdict possible;
//   - for a dose with no product or another product, is never better than
//     the verdict for the same dose of any catalog product (P1, SAD §5.7,
//     Q116), and a counter fails when versions never help such a dose
//     (diseases x, g and m have no catalog product, so Q94 still applies
//     there; k's catalog products p and q have different tiers in each
//     segment, so no version helps such a dose of k).
//
// Birth dates, ranges and products are random, and the ranges often cross
// a boundary. The whole-walk counterexample is
// TestVersionsNotMonotoneOverWalk.
func TestPropertyVersionsOnlyHelp(t *testing.T) {
	t.Parallel()
	helped, counted, definite, unknownHelped := 0, 0, 0, 0
	for _, tl := range versionTimelines(t) {
		rng := rand.New(rand.NewPCG(83, uint64(len(tl.id))))
		base := d(t, "2024-01-01")
		for range 20000 {
			day := func(n int) Date { return base.AddDays(rng.IntN(n)) }
			span := func(lo Date) Range { return Range{Lo: lo, Hi: lo.AddDays(rng.IntN(3) * rng.IntN(25))} }
			today := day(1000)
			i := segmentOf(tl, today)
			dz := tl.models[i]
			if len(dz.series) == 0 {
				continue
			}
			ci := rng.IntN(len(dz.series))
			s := dz.series[ci]
			var b birthEnd
			if rng.IntN(6) != 0 {
				lo := day(400)
				b = birthEnd{date: lo, known: true}
				if rng.IntN(3) == 0 {
					b.old, b.late = lo, lo.AddDays(rng.IntN(300))
					if rng.IntN(2) == 0 {
						b.date = b.late
					}
				}
			}
			pos := 1 + rng.IntN(s.length)
			x := span(day(900))
			var from *Range
			var run []item
			if pos > 1 {
				r := span(day(800))
				from = &r
				if rng.IntN(3) == 0 {
					run = []item{{pos: pos - 1}}
				}
			}
			pd := s.predecessor(b, from, run, nil)
			var first, firstBr *Range
			if pos > 1 && rng.IntN(3) != 0 {
				r := span(day(600))
				first, firstBr = &r, &r
				if rng.IntN(2) == 0 {
					firstBr = &Range{Lo: b.older(), Hi: r.Hi}
				}
			}
			product := []string{"", "p", "q", "x"}[rng.IntN(4)]
			vc := s.verdictAt(pos, b, x, pd, first)
			v := s.datedVerdict(pos, b, x, pd, first, firstBr, product)
			if v.sev() > vc.sev() {
				t.Fatalf("%s %s: versions made pos %d on %v worse: %+v, c alone %+v", tl.id, s.id, pos, x, v, vc)
			}
			if v.sev() < vc.sev() {
				helped++
			}
			// Unknown is never less safe (P1, Q116), per dose: a dose with no
			// product or another product could be any catalog product.
			if _, catalog := dz.tier[product]; !catalog {
				if v.sev() < vc.sev() {
					unknownHelped++
				}
				for _, p := range dz.catalog {
					if vp := s.datedVerdict(pos, b, x, pd, first, firstBr, p); v.sev() < vp.sev() {
						t.Fatalf("%s %s: pos %d on %v (birth %+v, first %v, dose-1 range %v): product %q gives %+v, better than product %q's %+v",
							tl.id, s.id, pos, x, b, first, firstBr, product, v, p, vp)
					}
				}
			}
			if v.sev() == verdictDefinite && segmentOf(tl, x.Lo) == segmentOf(tl, x.Hi) {
				definite++
				for y := x.Lo; !y.After(x.Hi); y = y.AddDays(1) {
					if id, at, ok := passesOnDay(tl, i, ci, pos, b, y, pd, first, firstBr, product); ok {
						t.Fatalf("%s %s: pos %d on %v (product %q, birth %+v, first %v, dose-1 range %v) is definite, but %s passes it on %s for birth date %s",
							tl.id, s.id, pos, x, product, b, first, firstBr, id, y, at)
					}
				}
			}
			if v.sev() != verdictOK || vc.sev() == verdictOK {
				continue
			}
			counted++
			for y := x.Lo; !y.After(x.Hi); y = y.AddDays(1) {
				if !countsOnDay(tl, i, ci, pos, b, y, pd, first, firstBr, product) {
					t.Fatalf("%s %s: pos %d on %v (product %q, birth %+v, first %v, dose-1 range %v) counts with versions, but no qualifying series counts it on %s",
						tl.id, s.id, pos, x, product, b, first, firstBr, y)
				}
			}
		}
	}
	t.Logf("versions helped %d times (%d for a dose with no product or another product), made %d doses count; %d definite verdicts checked",
		helped, unknownHelped, counted, definite)
	if helped == 0 || unknownHelped == 0 || counted == 0 || definite == 0 {
		t.Fatal("no version ever helped a dose of a catalog product or one outside the catalog, or made a dose count, or no verdict was definite; the property is vacuous")
	}
}
