package engine_test

import (
	"fmt"
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
