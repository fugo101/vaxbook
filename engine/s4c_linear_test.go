package engine_test

import (
	"fmt"
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// TestVersionsLinear (issue #29 In scope 3): the versions add a constant
// amount of work per dose (cuts × versions) and the envelope one gap search
// and one check per walk, so many doses stay linear. vb-old (until
// 2025-09-01) and vb-new (from then, maxAgeStart 4m), three doses at 4w,
// birth 2025-01-01, today 2025-09-20: p 2025-08-25 counts under vb-old
// only, n records from 09-02 fail dose 2 under both versions (each judged
// with the versions), and s #3 undated trails them (the gap search and the
// envelope scan them).
func TestVersionsLinear(t *testing.T) {
	if testing.Short() || runtime.GOOS == "js" {
		t.Skip("timing test")
	}
	three := func(id, maxStart string) ruleset.Series {
		return qSeries(t, id, "", maxStart, "", []ruleset.DoseRule{
			rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
			rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
			rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		})
	}
	e := newEngine(t, oneDiseaseSnapshot(nil, ws(t, three("vb-old", ""), "", "2025-09-01", "", ""), ws(t, three("vb-new", "P4M"), "2025-09-01", "", "", "")))
	today, first := mustDate(t, "2025-09-20"), mustDate(t, "2025-09-02")
	assertLinear(t, 2000, func(n int) time.Duration {
		doses := []engine.DoseRecord{qDose("p", "2025-08-25")}
		for i := range n {
			doses = append(doses, qDose(fmt.Sprintf("r%05d", i), first.AddDays(i%10).String()))
		}
		doses = append(doses, numbered(qDose("s", ""), 3), qDose("f0", ""))
		in := engine.Input{Today: today, Profile: engine.Profile{Birth: dayDate("2025-01-01")}, Doses: doses}
		began := time.Now()
		if _, err := e.Evaluate(in); err != nil {
			t.Fatal(err)
		}
		return time.Since(began)
	})
}

// TestToleranceGapSearchLinear (issue #29 In scope 6): the tolerant gap
// search stays linear, as TestGapSearchLinear's shape on minimal-tolerance:
// today 02-22, n records from 01-02 (below 6w − 4d = 02-08, uncounted), u
// #2 undated and two floating doses.
func TestToleranceGapSearchLinear(t *testing.T) {
	if testing.Short() || runtime.GOOS == "js" {
		t.Skip("timing test")
	}
	today, first := mustDate(t, "2026-02-22"), mustDate(t, "2026-01-02")
	e := loadEngine(t, "minimal-tolerance")
	assertLinear(t, 2000, func(n int) time.Duration {
		doses := make([]engine.DoseRecord, n, n+3)
		for i := range doses {
			doses[i] = tolDose(fmt.Sprintf("r%05d", i), dayDate(first.AddDays(i%30).String()))
		}
		doses = append(doses, numbered(tolDose("u", unknownDate), 2), tolDose("f0", unknownDate), tolDose("f1", unknownDate))
		in := engine.Input{Today: today, Profile: engine.Profile{Birth: birthTol}, Doses: doses}
		began := time.Now()
		if _, err := e.Evaluate(in); err != nil {
			t.Fatal(err)
		}
		return time.Since(began)
	})
}

// TestCompileManySeriesInEffect (issue #29 In scope 2–3): compiling a
// disease's validity segments and versions stays of low degree in the
// number of series in effect at once, so that a valid snapshot with many
// products compiles quickly in the browser too. Three shapes, each with
// n and 2n series (two doses each):
//   - staggered: product p<k>'s series is in effect from 2020-01-01 + 7k
//     days, with no end, so the segments k..n all hold it;
//   - re-issued: p<k> has five series, each in effect for one year from
//     2020-01-01 + m years + 7k days (m = 0..4), so every product is in
//     effect at once and its boundaries are staggered;
//   - one bracketed group: generic series g<k>, startAge [k months, k+1
//     months), in effect from 2020-01-01 + 7k days, with no end, so each
//     segment adds the group's last bracket and changes groupMax for all.
//
// The bytes engine.New allocates must grow at most 12× when n doubles (a
// cubic cost grows 8×), and so must the time of the staggered shape (best
// of three, from n = 64), whose tier tables are as long as the catalog;
// the other shapes' time follows their bytes. The version compile used to build a tier map per pair of
// series of two segments, a fifth power; then a version list per series
// and segment, a fourth power for one group, and compared tier maps
// product by product, a fourth power in time for the staggered shape.
func TestCompileManySeriesInEffect(t *testing.T) {
	if testing.Short() || runtime.GOOS == "js" {
		t.Skip("allocation and timing test")
	}
	doses := []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
	}
	start := mustDate(t, "2020-01-01")
	shapes := []struct {
		name  string
		timeN int // 0: no time check
		snap  func(n int) *ruleset.Snapshot
	}{
		{"staggered", 64, func(n int) *ruleset.Snapshot {
			var products []string
			var series []ruleset.Series
			for k := range n {
				p := fmt.Sprintf("p%d", k)
				products = append(products, p)
				series = append(series, ws(t, qSeries(t, p+"-s", p, "", "", doses), start.AddDays(7*k).String(), "", "", ""))
			}
			return oneDiseaseSnapshot(products, series...)
		}},
		{"re-issued", 0, func(n int) *ruleset.Snapshot {
			var products []string
			var series []ruleset.Series
			for k := range n {
				p := fmt.Sprintf("p%d", k)
				products = append(products, p)
				for m := range 5 {
					base := start.AddDays(7 * k)
					from, to := base.AddSpan(ruleset.Span{N: m, Unit: ruleset.Years}), base.AddSpan(ruleset.Span{N: m + 1, Unit: ruleset.Years})
					series = append(series, ws(t, qSeries(t, fmt.Sprintf("%s-%d", p, m), p, "", "", doses), from.String(), to.String(), "", ""))
				}
			}
			return oneDiseaseSnapshot(products, series...)
		}},
		{"one bracketed group", 0, func(n int) *ruleset.Snapshot {
			var series []ruleset.Series
			for k := range n {
				series = append(series, ws(t, qSeries(t, fmt.Sprintf("g%d", k), "", "", "", doses), start.AddDays(7*k).String(), "",
					fmt.Sprintf("P%dM", k), fmt.Sprintf("P%dM", k+1)))
			}
			return oneDiseaseSnapshot(nil, series...)
		}},
	}
	for _, sh := range shapes {
		allocated := func(n int) uint64 {
			s := sh.snap(n)
			if rejects := ruleset.Rejects(ruleset.Validate(s)); len(rejects) > 0 {
				t.Fatalf("%s: %v", sh.name, rejects)
			}
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			if _, err := engine.New(s); err != nil {
				t.Fatal(err)
			}
			runtime.ReadMemStats(&after)
			return after.TotalAlloc - before.TotalAlloc
		}
		const n = 16
		small, large := allocated(n), allocated(2*n)
		t.Logf("%s: n=%d: %d bytes, 2n: %d bytes (%.1f×)", sh.name, n, small, large, float64(large)/float64(small))
		if large > 12*small {
			t.Errorf("%s: twice the products allocated %.1f× as much (%d vs %d bytes)", sh.name, float64(large)/float64(small), large, small)
		}
		if sh.timeN == 0 {
			continue
		}
		best := func(n int) time.Duration {
			s := sh.snap(n)
			b := time.Duration(math.MaxInt64)
			for range 3 {
				began := time.Now()
				if _, err := engine.New(s); err != nil {
					t.Fatal(err)
				}
				b = min(b, time.Since(began))
			}
			return b
		}
		fast, slow := best(sh.timeN), best(2*sh.timeN)
		t.Logf("%s: n=%d: %s, 2n: %s (%.1f×)", sh.name, sh.timeN, fast, slow, float64(slow)/float64(fast))
		if slow > 12*fast {
			t.Errorf("%s: twice the series took %.1f× as long to compile (%s vs %s)", sh.name, float64(slow)/float64(fast), slow, fast)
		}
	}
}
