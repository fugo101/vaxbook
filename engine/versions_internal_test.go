package engine

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// versionTimelines returns windowed disease timelines for the version
// properties: minimal-effective-dating's ed, and an in-test bracketed
// group k-a [6w, 9m) (dose 2 at 4w, maxAgeComplete 3y) and k-b [9m, —)
// (8w, 2y) until 2026-01-01, then k-2026 (8w, 2y) and a product series
// k-p for product p (4w) from then.
func versionTimelines(t *testing.T) []diseaseTimeline {
	t.Helper()
	ed := gapFixture{name: "minimal-effective-dating", disease: "ed"}.diseaseOf(t)
	dose2 := func(interval string) string {
		return `{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}, {"minInterval": "` + interval + `", "recommended": {"from": "previousDose", "offset": "P1M"}}`
	}
	series := func(id, extra, interval string) string {
		return `{"id": "` + id + `", "disease": "k", "stage": "childhood", ` + extra + `"plans": [{"id": "a", "doses": [` + dose2(interval) + `]}]}`
	}
	doc := `{"version": "v", "illustrative": true, "diseases": [{"id": "k", "name": "K"}],
	  "products": [{"id": "p", "name": "P", "diseases": ["k"], "live": false}], "series": [` + strings.Join([]string{
		series("k-a", `"startAge": {"min": "P6W", "max": "P9M"}, "maxAgeComplete": "P3Y", "validTo": "2026-01-01", `, "P4W"),
		series("k-b", `"startAge": {"min": "P9M"}, "maxAgeComplete": "P2Y", "validTo": "2026-01-01", `, "P8W"),
		series("k-2026", `"maxAgeComplete": "P2Y", "validFrom": "2026-01-01", `, "P8W"),
		series("k-p", `"product": "p", "validFrom": "2026-01-01", `, "P4W"),
	}, ", ") + `]}`
	s, err := ruleset.Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return []diseaseTimeline{ed, e.diseases[0]}
}

// TestPropertyVersionsOnlyHelp (issue #29 In scope 3): for a fixed
// position, dose range, predecessor (dated or behind a run), dose 1 and
// dose-1 range, the verdict with versions (datedVerdict) is never worse
// than today's c alone (verdictAt). Birth dates, ranges and products are
// random, and the ranges often cross a boundary. The whole-walk
// counterexample is TestVersionsNotMonotoneOverWalk.
func TestPropertyVersionsOnlyHelp(t *testing.T) {
	t.Parallel()
	helped := 0
	for _, tl := range versionTimelines(t) {
		rng := rand.New(rand.NewPCG(83, uint64(len(tl.id))))
		base := d(t, "2024-01-01")
		for range 20000 {
			day := func(n int) Date { return base.AddDays(rng.IntN(n)) }
			span := func(lo Date) Range { return Range{Lo: lo, Hi: lo.AddDays(rng.IntN(3) * rng.IntN(25))} }
			today := day(1000)
			dz := tl.at(today)
			if len(dz.series) == 0 {
				continue
			}
			s := dz.series[rng.IntN(len(dz.series))]
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
			product := []string{"", "p", "x"}[rng.IntN(3)]
			vc := s.verdictAt(pos, b, x, pd, first)
			v := s.datedVerdict(pos, b, x, pd, first, firstBr, product)
			if v.sev() > vc.sev() {
				t.Fatalf("%s %s: versions made pos %d on %v worse: %+v, c alone %+v", tl.id, s.id, pos, x, v, vc)
			}
			if v.sev() < vc.sev() {
				helped++
			}
		}
	}
	if helped == 0 {
		t.Fatal("no version ever helped; the property is vacuous")
	}
}
