package engine_test

import (
	"fmt"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of effective dating outside the goldens (issue #29 In scope
// 2–4, S4c): the model per validity segment, NO_SERIES_IN_EFFECT and "the
// rule in effect on its dose date or today". Every in-test snapshot passes
// ruleset.Validate with no Reject. Dose 1 has minAge P6W unless stated.

var noSeries = engine.Reason{Code: engine.ReasonNoSeriesInEffect}

// TestNoSeriesInEffect (In scope 4, Q81, Q82): engine.New accepts a disease
// with no series, and its assessment is ⚪ NO_SERIES_IN_EFFECT alone, with
// no seriesUsed, products or next dose, also with no birth date (it is
// decided before the NO_BIRTH_DATE stop) and with doses. A step-2 ⚪ still
// wins over it, as a step-1 ⚪ does.
func TestNoSeriesInEffect(t *testing.T) {
	t.Parallel()
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "q", Name: "Q"}}, Series: []ruleset.Series{}}
	e := newEngine(t, s)
	want := engine.Assessment{Disease: "q", Bucket: engine.BucketUnknown, Reasons: []engine.Reason{noSeries}}
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01")), want)
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01"), doseOf("q", "r1", dayDate("2025-11-01"), "")), want)
	// Step 1's estimated flag stays (birth precision none, a level-4 dose),
	// and IMPLIED_EARLIER_DOSES is not added: no candidate caps it.
	want.Estimated = true
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", engine.RecalledDate{Precision: engine.None}), want)
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01"), numbered(doseOf("q", "u", unknownDate, ""), 3)), want)
	// Two dated doses numbered 1 on different days: NUMBERING_CONFLICT.
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01"),
		numbered(doseOf("q", "r1", dayDate("2025-11-01"), ""), 1), numbered(doseOf("q", "r2", dayDate("2025-12-01"), ""), 1))
	if len(got.Reasons) != 1 || got.Reasons[0].Code != engine.ReasonNumberingConflict {
		t.Errorf("numbering conflict: reasons %v, want NUMBERING_CONFLICT alone", got.Reasons)
	}
}

// TestRulesetDateParsesAsEngineDate: ruleset.ParseDate (validity bounds)
// and engine.ParseDate (input dates) accept exactly the same strings, the
// schema's $defs/Date pattern, and agree on the day (issue #29 Contract).
// ruleset cannot import engine, so the two parsers are written twice.
func TestRulesetDateParsesAsEngineDate(t *testing.T) {
	t.Parallel()
	corpus := []string{"2026-01-01", "0001-01-01", "9999-12-31", "0000-01-01", "2024-02-29", "2025-02-29", "1900-02-29",
		"2000-02-29", "2026-04-30", "2026-04-31", "2026-13-01", "2026-00-01", "2026-01-00", "2026-1-01", "+2026-01-01",
		"2026-01-01 ", " 2026-01-01", "2026/01/01", "20260101", "2026-01-0a", "10000-01-01", "", "2026-12-31T00:00"}
	for y := 1896; y <= 2104; y += 4 {
		corpus = append(corpus, fmt.Sprintf("%04d-02-29", y), fmt.Sprintf("%04d-02-28", y+1), fmt.Sprintf("%04d-02-29", y+1))
	}
	for _, s := range corpus {
		rd, rerr := ruleset.ParseDate(s)
		ed, eerr := engine.ParseDate(s)
		if (rerr == nil) != (eerr == nil) {
			t.Errorf("%q: ruleset.ParseDate error %v, engine.ParseDate error %v", s, rerr, eerr)
			continue
		}
		if rerr != nil {
			continue
		}
		if y, m, dd := ed.YMD(); y != rd.Year || m != rd.Month || dd != rd.Day || rd.String() != ed.String() {
			t.Errorf("%q: ruleset %v, engine %v", s, rd, ed)
		}
	}
}
