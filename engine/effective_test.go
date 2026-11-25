package engine_test

import (
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
