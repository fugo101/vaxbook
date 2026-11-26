package engine_test

import (
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of tolerance outside the goldens (issue #29 In scope 6, Q79,
// Q80, Q90, S4c): a dose rule's tolerance lowers its minimum thresholds when
// given doses are validated (check, the step-5 chain, #35's gap search),
// never a next-dose date or limit. minimal-tolerance: birth 2026-01-01,
// dose 1 minAge 6w (02-12, with its 4-day tolerance 02-08), dose 2
// minInterval 4w with a 4-day tolerance.

var birthTol = dayDate("2026-01-01")

// tolerant returns r with tolerance tol.
func tolerant(t testing.TB, r ruleset.DoseRule, tol string) ruleset.DoseRule {
	t.Helper()
	r.Tolerance = spanOf(t, tol)
	return r
}

func tolDose(id string, date engine.RecalledDate) engine.DoseRecord {
	return doseOf("tol", id, date, "")
}

// TestToleranceAtBothEnds: r1 02-08, r2 in 2026-03, today 03-06 (R = [03-01,
// 03-06]). The threshold with the tolerance is 02-08 + 4w − 4d = 03-04 at
// both ends: R.Hi meets it, R.Lo does not → DOSE_POSSIBLY_INVALID{minInterval}
// and COMPLETE. Without the interval's tolerance R.Hi 03-06 < 03-08 →
// DOSE_INVALID.
func TestToleranceAtBothEnds(t *testing.T) {
	t.Parallel()
	got := assessmentOf(t, loadEngine(t, "minimal-tolerance"), "tol", "2026-03-06", birthTol,
		tolDose("r1", dayDate("2026-02-08")), tolDose("r2", monthDate("2026-03")))
	checkAssessment(t, got, engine.Assessment{
		Disease: "tol", Bucket: engine.BucketUnknown, SeriesUsed: []string{"tol-1"}, Estimated: true,
		Reasons: []engine.Reason{possiblyInvalid("r2", 2, engine.CheckMinInterval), complete},
	})
}

// TestToleranceNotMonotoneOverWalk (In scope 6): r1 02-10, r2 02-20, today
// 03-20. With the tolerance r1 counts (≥ 02-08) and r2 needs 02-10 + 24d =
// 03-06 → DOSE_INVALID{minInterval, r2, 2}. Without it r1 fails 6w (02-12)
// and r2 counts as dose 1: a tolerance can make another record dose 1, so
// a verdict over a whole walk is not monotone in it.
func TestToleranceNotMonotoneOverWalk(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "minimal-tolerance")
	doses := []engine.DoseRecord{tolDose("r1", dayDate("2026-02-10")), tolDose("r2", dayDate("2026-02-20"))}
	got := assessmentOf(t, newEngine(t, s), "tol", "2026-03-20", birthTol, doses...)
	checkAssessment(t, got, engine.Assessment{
		Disease: "tol", Bucket: engine.BucketUnknown, SeriesUsed: []string{"tol-1"},
		Reasons: []engine.Reason{invalidDose("r2", 2, engine.CheckMinInterval)},
	})
	for i := range s.Series[0].Plans[0].Doses {
		s.Series[0].Plans[0].Doses[i].Tolerance = nil
	}
	got = assessmentOf(t, newEngine(t, s), "tol", "2026-03-20", birthTol, doses...)
	checkAssessment(t, got, engine.Assessment{
		Disease: "tol", Bucket: engine.BucketUnknown, SeriesUsed: []string{"tol-1"},
		Reasons: []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge)},
	})
}

// TestToleranceNotInLastAllowedDay (Q90, EQ27): C_c,k is computed without
// tolerance. t-3: maxAgeComplete 1y (L = 2025-12-31), dose 1 minAge 6w, dose
// 2 minInterval 4w due previous + 8w, dose 3 minInterval 8w with a 4-day
// tolerance. Birth 2025-01-01, r1 2025-10-01, today 10-05: dose 2 E = 10-29,
// Du = 11-26; C_2 = L − 8w = 11-05 (with the tolerance it would be 11-09);
// F = 10-29 ≤ L_2 → due and overdue capped at 11-05, 🟡 with
// DUE_CAPPED_BY_MAX_AGE{2025-11-05}: the output without the tolerance.
func TestToleranceNotInLastAllowedDay(t *testing.T) {
	t.Parallel()
	for _, tol := range []string{"", "P4D"} {
		d3 := rule(t, "", "P8W", "", ruleset.FromPreviousDose, "P8W")
		if tol != "" {
			d3 = tolerant(t, d3, tol)
		}
		e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "t-3", "", "", "P1Y", []ruleset.DoseRule{
			rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
			rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P8W"),
			d3,
		})))
		got := assessmentOf(t, e, "q", "2025-10-05", dayDate("2025-01-01"), doseOf("q", "r1", dayDate("2025-10-01"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2025-10-29", "2025-11-05", "2025-11-05"),
			SeriesUsed: []string{"t-3"}, Reasons: []engine.Reason{capped("2025-11-05")},
		})
	}
}

// TestSlotMaxUsesTolerantChain (Q90 (a)): an undated slot's maximum-age
// range comes from the tolerant chain. t-max: maxAgeComplete 1y (limit
// 2026-01-01), dose 2 minInterval 4w with a 4-day tolerance. Birth
// 2025-01-01, r1 2025-12-06, u2 level 4 #2, today 2026-02-01: u2's range
// starts 12-06 + 4w − 4d = 12-30, before the limit → ✅ COMPLETE, est. A
// tolerance-free range would start 2026-01-03 → DOSE_AFTER_MAX_AGE, though
// a dated dose on 12-30 counts.
func TestSlotMaxUsesTolerantChain(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "t-max", "", "", "P1Y", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		tolerant(t, rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"), "P4D"),
	})))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-01-01"),
		doseOf("q", "r1", dayDate("2025-12-06"), ""), numbered(doseOf("q", "u2", unknownDate, ""), 2))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketComplete, SeriesUsed: []string{"t-max"}, Estimated: true,
		Reasons: []engine.Reason{complete},
	})
}

// TestSameDayRuleReadsTolerantDate (#35, Q90): the gap search's same-day
// rule compares the tolerant end with U(k+1).Hi. h-1: dose 1 minAge 9y,
// dose 2 minInterval 4w, dose 3 minInterval 12w with a 4-day tolerance.
// Birth 2017-01-01; r1 2025-06-01 and r2 (both below 9y: DOSE_INVALID{minAge}
// at 1, uncounted), u #3 undated, today 2025-10-01. Gap 1 (after r1, q = 1):
// dose 2 ≥ 06-29, u ≥ 06-29 + 12w − 4d = 09-17.
//   - r2 09-17: a tie that r2 (step 2) walks first, so the gap does not fit
//     → DOSE_INVALID{minInterval, u, 3}.
//   - r2 09-18: the gap fits, threshold 09-17 ≤ today →
//     DOSE_POSSIBLY_INVALID{minInterval, u, 3}, counted → COMPLETE.
//
// Tolerance-free the end is 09-21: DOSE_INVALID in both.
func TestSameDayRuleReadsTolerantDate(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "h-1", "", "", "", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		tolerant(t, rule(t, "", "P12W", "", ruleset.FromPreviousDose, "P3M"), "P4D"),
	})))
	for _, tc := range []struct {
		r2   string
		last engine.Reason
	}{
		{"2025-09-17", invalidDose("u", 3, engine.CheckMinInterval)},
		{"2025-09-18", possiblyInvalid("u", 3, engine.CheckMinInterval)},
	} {
		got := assessmentOf(t, e, "q", "2025-10-01", dayDate("2017-01-01"), doseOf("q", "r1", dayDate("2025-06-01"), ""),
			doseOf("q", "r2", dayDate(tc.r2), ""), numbered(doseOf("q", "u", unknownDate, ""), 3))
		want := []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge), tc.last}
		if tc.last.Code == engine.ReasonDosePossiblyInvalid {
			want = append(want, complete)
		}
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"h-1"}, Estimated: true, Reasons: want,
		})
	}
}

// TestGapZeroDoseOneTolerant (#35, Q90): gap 0's own dose 1 is b + minAge −
// tol, as the walk's undated dose 1 is. TestGapZeroChecksFromFirst's q-1
// (maxAgeStart 15w = 04-16; dose 1 minAge 6w, dose 2 minInterval 4w, dose 3
// minInterval 4w and minIntervalFromFirst 16w) with a 4-day tolerance on
// doses 1 and 3. Birth 2026-01-01, u1 05-30 (past 15w), s #3, today 06-10.
// Gap 0: dose 1 ≥ 02-08, dose 2 ≥ 03-08, s ≥ max(03-08 + 4w − 4d, 02-08 +
// 16w − 4d) = 05-27 < u1 → fits, threshold max(03-08, 04-01) ≤ today →
// DOSE_POSSIBLY_INVALID{minInterval, s, 3}, COMPLETE. With dose 1 at 02-12 s
// would need 05-31 > u1: DOSE_INVALID.
func TestGapZeroDoseOneTolerant(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "P15W", "", []ruleset.DoseRule{
		tolerant(t, rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"), "P4D"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		tolerant(t, rule(t, "", "P4W", "P16W", ruleset.FromPreviousDose, "P1M"), "P4D"),
	})))
	got := assessmentOf(t, e, "q", "2026-06-10", birthGap,
		doseOf("q", "u1", dayDate("2026-05-30"), ""), numbered(doseOf("q", "s", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{2}}},
			afterMaxAge("u1", 1, engine.CheckMaxAgeStart),
			possiblyInvalid("s", 3, engine.CheckMinInterval),
			complete,
		},
	})
}
