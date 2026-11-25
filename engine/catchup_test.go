package engine_test

import (
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of catch-up outside the goldens (issue #29 In scope 5, Q78,
// Q91, Q92, S4c). cu-1 is minimal-catch-up-age's series: catchUpAfterAge
// 2y, dose 1 minAge 12m due birth + 12m, dose 2 minInterval 6m due birth +
// 18m.

var askDoctor = engine.Reason{Code: engine.ReasonCatchUpAskDoctor}

// cuCap is cu-1 with maxAgeComplete 3y and dose 2 due previous + 12m, as a
// series of disease q.
func cuCap(t testing.TB) ruleset.Series {
	t.Helper()
	sr := qSeries(t, "cu-cap", "", "", "P3Y", []ruleset.DoseRule{
		rule(t, "P12M", "", "", ruleset.FromBirth, "P12M"),
		rule(t, "", "P6M", "", ruleset.FromPreviousDose, "P12M"),
	})
	sr.CatchUpAfterAge = spanOf(t, "P2Y")
	return sr
}

// TestCatchUpAfterCap (Q92): cu-cap, birth 2023-01-10 (catch-up from
// 2025-01-10, L = 2026-01-09).
//   - r1 2025-05-01, today 2025-10-01: dose 2 E = 11-01, Du = 2026-05-01; F
//     = 11-01 ≤ L_2 = L → due and overdue capped at 01-09 with
//     DUE_CAPPED_BY_MAX_AGE; F ≥ 2025-01-10 → ⚪, the cap reason kept as a
//     masked reason before CATCH_UP_ASK_DOCTOR.
//   - r1 2024-08-01, today 2024-09-01: E = 2025-02-01 ≥ 2025-01-10 and Du =
//     2025-08-01 is more than 6 months away → ⚪ [CATCH_UP_ASK_DOCTOR,
//     ON_TRACK]: the masked ON_TRACK stays last.
//   - r1 2025-08-01, today 2025-10-01: E = 2026-02-01 > L → outOfAge
//     [OUT_OF_AGE]; layer 5 wins over catch-up.
func TestCatchUpAfterCap(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, cuCap(t)))
	birth := dayDate("2023-01-10")
	unknown := func(reasons ...engine.Reason) engine.Assessment {
		return engine.Assessment{Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"cu-cap"}, Reasons: reasons}
	}
	checkAssessment(t, assessmentOf(t, e, "q", "2025-10-01", birth, doseOf("q", "r1", dayDate("2025-05-01"), "")),
		unknown(capped("2026-01-09"), askDoctor))
	checkAssessment(t, assessmentOf(t, e, "q", "2024-09-01", birth, doseOf("q", "r1", dayDate("2024-08-01"), "")),
		unknown(askDoctor, engine.Reason{Code: engine.ReasonOnTrack}))
	checkAssessment(t, assessmentOf(t, e, "q", "2025-10-01", birth, doseOf("q", "r1", dayDate("2025-08-01"), "")),
		engine.Assessment{Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"cu-cap"}, Reasons: []engine.Reason{outOfAgeReason}})
}

// TestCatchUpLeavesBlockingResult (In scope 5): an as-walked blocking
// result past the catch-up age stays as it is. minimal-catch-up-age, birth
// 2023-01-10, r1 in 2024-01 (R = [01-01, 01-31], 12m = 2024-01-10), today
// 2026-02-10 → ⚪ [DOSE_POSSIBLY_INVALID{minAge, r1, 1}], est, no
// CATCH_UP_ASK_DOCTOR.
func TestCatchUpLeavesBlockingResult(t *testing.T) {
	t.Parallel()
	got := assessmentOf(t, loadEngine(t, "minimal-catch-up-age"), "cu", "2026-02-10", dayDate("2023-01-10"),
		doseOf("cu", "r1", monthDate("2024-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "cu", Bucket: engine.BucketUnknown, SeriesUsed: []string{"cu-1"}, Estimated: true,
		Reasons: []engine.Reason{possiblyInvalid("r1", 1, engine.CheckMinAge)},
	})
}

// TestCatchUpReadsShownEarliestDate (Q91): F is the earliest date the
// result would show, from A7's plan, not the first day any plan allows.
// cu-2p, catchUpAfterAge 2y, plans a ("fast": dose 2 minInterval 6m, due
// previous + 6m) and b ("slow": minInterval 1m, due previous + 12m). Birth
// 2024-01-10 (threshold 2026-01-10), r1 2025-08-01, today 2025-08-15. A7
// shows a: E = Du = 2026-02-01 ≥ threshold → ⚪ [ALTERNATIVE_PLANS{cu-2p},
// CATCH_UP_ASK_DOCTOR], although b validates dose 2 from 2025-09-01.
func TestCatchUpReadsShownEarliestDate(t *testing.T) {
	t.Parallel()
	d1 := rule(t, "P12M", "", "", ruleset.FromBirth, "P12M")
	sr := qSeries(t, "cu-2p", "", "", "",
		[]ruleset.DoseRule{d1, rule(t, "", "P6M", "", ruleset.FromPreviousDose, "P6M")},
		[]ruleset.DoseRule{d1, rule(t, "", "P1M", "", ruleset.FromPreviousDose, "P12M")})
	sr.CatchUpAfterAge = spanOf(t, "P2Y")
	got := assessmentOf(t, newEngine(t, oneDiseaseSnapshot(nil, sr)), "q", "2025-08-15", dayDate("2024-01-10"),
		doseOf("q", "r1", dayDate("2025-08-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"cu-2p"},
		Reasons: []engine.Reason{{Code: engine.ReasonAlternativePlans, Params: engine.ReasonParams{Series: "cu-2p"}}, askDoctor},
	})
}

// TestCatchUpGuardAtOlderBirthEnd (Q108): cu-x, catchUpAfterAge 2y, one
// dose, minAge P103W (721 days), due birth + 30m.
//   - Birth month 2024-01, today 2025-12-01. At 01-01: F = E = 2025-12-22 <
//     2026-01-01 → a clean notYetScheduled. At 01-31: F = E = 2026-01-21, not
//     caught up at 01-31 (2026-01-31) but past b_old's catch-up age
//     (2026-01-01), so that end is ⚪ [BIRTH_DATE_IMPRECISE{fullBirthDate}];
//     the ends differ → ⚪ [BIRTH_DATE_IMPRECISE{fullBirthDate}], est. It no
//     longer shows 2026-01-21, a date past the catch-up age of a birth date
//     in the range.
//   - The same on 2026-01-01: at 01-01 F = today is caught up; at 01-31 the
//     guard → ⚪ [BIRTH_DATE_IMPRECISE{fullBirthDate}, CATCH_UP_ASK_DOCTOR],
//     est, as before.
//   - Birth 2024-01-01, today 2025-12-01: unchanged, notYetScheduled {1,
//     2025-12-22, 2026-07-01, 2026-07-08}.
func TestCatchUpGuardAtOlderBirthEnd(t *testing.T) {
	t.Parallel()
	sr := qSeries(t, "cu-x", "", "", "", []ruleset.DoseRule{rule(t, "P103W", "", "", ruleset.FromBirth, "P30M")})
	sr.CatchUpAfterAge = spanOf(t, "P2Y")
	e := newEngine(t, oneDiseaseSnapshot(nil, sr))
	used := []string{"cu-x"}
	month := monthDate("2024-01")
	checkAssessment(t, assessmentOf(t, e, "q", "2025-12-01", month), unknownWith(used, true, fullBirthDate))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-01-01", month), unknownWith(used, true, fullBirthDate, askDoctor))
	checkAssessment(t, assessmentOf(t, e, "q", "2025-12-01", dayDate("2024-01-01")), engine.Assessment{
		Disease: "q", Block: engine.BlockNotYetScheduled, NextDose: nextDose(t, 1, "2025-12-22", "2026-07-01", "2026-07-08"), SeriesUsed: used,
	})
}
