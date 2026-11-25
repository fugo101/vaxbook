package engine_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of the S2 chain gap search (issue #35, Q98, Q99, Q103): the
// last trailing undated slot S at position n may lie before the uncounted
// unnumbered dated doses U1 < … < Um at the end of the walk, in the gap
// after Uk (or after P, the latest dated dose that is counted or numbered,
// or birth) when Uk's step-2 position q_k < n. The gap fits when S's
// earliest date, chained from Uk through positions q_k + 1 … n − 1, is on
// or before U(k+1).Hi (on it only when S walks first that day,
// TestGapBoundSameDay); S's minInterval is then at most possible. Birth
// 2026-01-01 (exact) everywhere, so 6w@b = 02-12 and 15w@b = 04-16;
// minimal-single dose 2 and illustrative-vn pertussis doses 2–3 need 4w.

var birthGap = dayDate("2026-01-01")

func single(id, date string) engine.DoseRecord {
	if date == "" {
		return doseOf("single", id, unknownDate, "")
	}
	return doseOf("single", id, dayDate(date), "")
}

// gapAssessment is the assessment of a minimal-single profile.
func gapAssessment(t *testing.T, today string, doses ...engine.DoseRecord) engine.Assessment {
	t.Helper()
	return assessmentOf(t, loadEngine(t, "minimal-single"), "single", today, birthGap, doses...)
}

func singleUnknown(reasons ...engine.Reason) engine.Assessment {
	return engine.Assessment{Disease: "single", Bucket: engine.BucketUnknown, SeriesUsed: []string{"single-series"}, Estimated: true, Reasons: reasons}
}

var complete = engine.Reason{Code: engine.ReasonComplete}

// TestTrailingSlotBeforeUncountedDose (issue #35 repro 2, Q87's shape):
// illustrative-max-age, today 08-28. d0 rotateq 07-11 (past 15w@b, so
// DOSE_AFTER_MAX_AGE at 1, uncounted), u rotateq #3 undated, so dose 2 is
// implied.
//   - rotateq, as walked: d0 → implied 2 ≥ 08-08 → u ≥ 09-05 > today. Gap 0
//     (before d0, q_0 = 0 < 3): implied 1 ≥ 02-12, implied 2 ≥ 03-12, u ≥
//     04-09 ≤ d0.Hi → fits. Definite threshold 04-09 ≤ today, possible
//     09-05 > today → DOSE_POSSIBLY_INVALID{minInterval} on u, counted, which
//     keeps 0c's possible maxAgeComplete (Q62) in the same reason.
//   - rotarix (2 doses, u beyond the series) carries implied dose 2's
//     possible maxAgeComplete on u, as on main; d0's reasons are main's.
//
// u counted at 3 → k = 4 > 3 → COMPLETE. Main gave rotateq's u
// DOSE_INVALID{minInterval, u, 3} and no COMPLETE.
func TestTrailingSlotBeforeUncountedDose(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	got := assessmentOf(t, e, "rota", "2026-08-28", birthGap,
		rota("d0", dayDate("2026-07-11"), "rotateq"), numbered(rota("u", unknownDate, "rotateq"), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{
			{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{2}}},
			rotaRefs,
			afterMaxAge("d0", 1, engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete),
			afterMaxAge("d0", 1, engine.CheckMaxAgeStart),
			possiblyInvalid("u", 3, engine.CheckMaxAgeComplete),
			possiblyInvalid("u", 3, engine.CheckMinInterval, engine.CheckMaxAgeComplete),
			complete,
		},
	})
}

// TestGapBound (issue #35, Q99 (a)): gap 1 (after r1, before r2) fits only
// when S's earliest date there is on or before r2.Hi. minimal-single, today
// 03-09, u #2 undated; r1 and r2 are before 6w@b (DOSE_INVALID{minAge} at
// 1, uncounted), and step 2 gives r1 position 1, r2 position 3.
//   - Fitting half: r1 01-12, r2 02-11. Gap 0 needs dose 1 ≥ 02-12, so u ≥
//     03-12 > r1. Gap 1: u ≥ r1 + 4w = 02-09 ≤ r2.Hi → fits. As walked: r2 +
//     4w = 03-11 > today → DOSE_POSSIBLY_INVALID{minInterval, u, 2},
//     counted, so k = 3 > 2 → COMPLETE. (Main: DOSE_INVALID, no COMPLETE.)
//   - Failing half (guard, as on main): r1 01-19 → u ≥ 02-16 > r2.Hi 02-11,
//     no gap fits → DOSE_INVALID{minInterval, u, 2}. Under Q99 (b) (skip the
//     uncounted doses, no implied positions or bound) it would be possible.
func TestGapBound(t *testing.T) {
	t.Parallel()
	u := numbered(single("u", ""), 2)
	checkAssessment(t, gapAssessment(t, "2026-03-09", single("r1", "2026-01-12"), single("r2", "2026-02-11"), u), singleUnknown(
		invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
		possiblyInvalid("u", 2, engine.CheckMinInterval), complete))
	checkAssessment(t, gapAssessment(t, "2026-03-09", single("r1", "2026-01-19"), single("r2", "2026-02-11"), u), singleUnknown(
		invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
		invalidDose("u", 2, engine.CheckMinInterval)))
}

// TestGapSearchKeepsRescue (issue #35 Q103 (a)): a definite reason that a
// placement's walk gives a slot that the gap search made possible does not
// count against the placement in S4b 0b's rescue. illustrative-max-age, all
// rotateq, today 07-09: d1 01-18 (< 6w@b → DOSE_INVALID{minAge} at 1), d0
// 07-01 (past 15w@b at 1), u0 undated with no number (floating), u1 #3
// undated.
//   - Step 2 gives d1 position 1 and d0 position 2 (3 is u1's). Base walk:
//     u1 trails d0 at 3, d0 + 4w = 07-29 > today. Gap 0 needs dose 1 ≥
//     02-12 > d1. Gap 1 (after d1, q_1 = 1 < 3): dose 2 ≥ d1 + 4w = 02-15,
//     u1 ≥ 03-15 ≤ d0.Hi → fits. So u1 is lowered to possible.
//   - Rescue: u0 in front of d0 counts d0 at 2 (possible maxAgeStart from u0
//     at 1). That walk gives u1, behind d0 at 3, DOSE_INVALID{minInterval}
//     (d0 + 4w > today); u1 is lowered in the base walk, so this does not
//     count against the placement and d0 is rescued.
//   - forceLater does not keep u1's definite either: u1 is forced counted
//     with DOSE_POSSIBLY_INVALID{minInterval, u1, 3}.
//   - u0 then fits (beyond the series): k = 5 > 3 → COMPLETE.
//
// u0 on 02-12 with u1 on 04-09 gives [DOSE_INVALID{minAge, d1, 1},
// COMPLETE]. Main: [DOSE_INVALID{d1}, DOSE_POSSIBLY_INVALID{maxAgeStart,
// d0, 1}, DOSE_INVALID{minInterval, u1, 3}, PREVIOUS_DOSE_DATE_UNKNOWN{u0}];
// with the gap search but without Q103, d0 loses its rescue and becomes
// DOSE_AFTER_MAX_AGE{maxAgeStart, d0, 1}.
func TestGapSearchKeepsRescue(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	got := assessmentOf(t, e, "rota", "2026-07-09", birthGap,
		rota("d1", dayDate("2026-01-18"), "rotateq"), rota("d0", dayDate("2026-07-01"), "rotateq"),
		rota("u0", unknownDate, "rotateq"), numbered(rota("u1", unknownDate, "rotateq"), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("d1", 1, engine.CheckMinAge),
			possiblyInvalid("d0", 1, engine.CheckMaxAgeStart),
			possiblyInvalid("u1", 3, engine.CheckMinInterval),
			complete,
		},
	})
}

// TestGapUsesStepTwoPositions (issue #35 In scope 1): q_k is Uk's step-2
// position, which skips the positions that numbered level-4 slots reserve.
// In-test series q-1: dose 1 minAge 6w, doses 2 and 3 minInterval 4w, dose
// 4 minInterval 2w. Today 04-25: p 02-12, v #2 undated, u1 04-01, u2 04-20,
// s #4 undated. Step 2: p 1, u1 3, u2 5 (2 and 4 reserved).
//   - Walk: p counts at 1; v at 2 ≥ 03-12; u1 at 3 needs 04-09 →
//     DOSE_INVALID{minInterval, u1, 3}; u2 at 3 needs u1 + 4w = 04-29 →
//     DOSE_INVALID{minInterval, u2, 3}. s trails u2 at 4: u2 + 2w = 05-04 >
//     today.
//   - Gap 0 (after p, q_0 = 1): positions 2 and 3 between, s ≥ 04-09 + 2w =
//     04-23 > u1.Hi → no fit. Gap 1 (after u1, q_1 = 3 < 4, nothing
//     between): s ≥ u1 + 2w = 04-15 ≤ u2.Hi → fits; s dated 04-15 is valid
//     at 4. → DOSE_POSSIBLY_INVALID{minInterval, s, 4}, counted, k = 5 >
//     4 → COMPLETE.
//
// Giving the U's positions P + 1, P + 2 (u1 2, u2 3) would put position 3
// between u1 and s, s ≥ 05-13 > u2.Hi, and keep s DOSE_INVALID, as main.
func TestGapUsesStepTwoPositions(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P2W", "", ruleset.FromPreviousDose, "P1M"),
	})))
	got := assessmentOf(t, e, "q", "2026-04-25", birthGap,
		doseOf("q", "p", dayDate("2026-02-12"), ""), numbered(doseOf("q", "v", unknownDate, ""), 2),
		doseOf("q", "u1", dayDate("2026-04-01"), ""), doseOf("q", "u2", dayDate("2026-04-20"), ""),
		numbered(doseOf("q", "s", unknownDate, ""), 4))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("u1", 3, engine.CheckMinInterval),
			invalidDose("u2", 3, engine.CheckMinInterval),
			possiblyInvalid("s", 4, engine.CheckMinInterval),
			complete,
		},
	})
}

// TestGapImpliedPositions (issue #35, guard): positions q_k + 1 … n − 1
// lie between Uk and S. illustrative-vn pertussis, today 02-16: r1 01-09
// and r2 02-09 (< 6w@b, DOSE_INVALID{minAge} at 1), u #3 undated. Gap 1
// (after r1, q_1 = 1) needs dose 2 between r1 and u: u ≥ r1 + 4w + 4w =
// 03-06 > r2.Hi; gap 0 needs dose 1 ≥ 02-12 > r1 → u stays
// DOSE_INVALID{minInterval, u, 3}, as on main.
func TestGapImpliedPositions(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-vn")
	got := assessmentOf(t, e, "pertussis", "2026-02-16", birthGap,
		doseOf("pertussis", "r1", dayDate("2026-01-09"), ""), doseOf("pertussis", "r2", dayDate("2026-02-09"), ""),
		numbered(doseOf("pertussis", "u", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "pertussis", Bucket: engine.BucketUnknown, SeriesUsed: []string{"pertussis-primary"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
			invalidDose("u", 3, engine.CheckMinInterval),
			{Code: engine.ReasonAlternativePlans, Params: engine.ReasonParams{Series: "pertussis-primary"}},
		},
	})
}

// TestGapSearchKeepsTrueDefinites (issue #35, guards that pass on main):
// minimal-single, u #2 undated.
//   - One uncounted dose, today 02-22: r2 02-10. Gap 0 needs dose 1 ≥ 02-12,
//     u ≥ 03-12 > r2; after r2, u ≥ 03-10 > today → DOSE_INVALID.
//   - Counted predecessor, today 02-22: r1 02-12 counts at 1, so there is no
//     gap; r1 + 4w = 03-12 > today → DOSE_INVALID.
//   - Floating dose, today 02-22: r1 01-09, r2 02-10 and u with no number.
//     The gap search covers fixed slots only (pinned by
//     TestGapSearchFixedSlotsOnly); S4b 0b's search is unchanged →
//     PREVIOUS_DOSE_DATE_UNKNOWN{u}.
//   - q_k ≥ n, today 02-20: r1 01-05, r2 01-10, r3 02-09 (all < 6w@b), so
//     step 2 gives 1, 3 and 4. Gap 0: u ≥ 03-12 > r1. Gap 1 (q_1 = 1):
//     u ≥ r1 + 4w = 02-02 > r2.Hi. Gap 2 is not tried: q_2 = 3 ≥ 2, and u #2
//     dated between r2 and r3 is a NUMBERING_CONFLICT. After r3, u ≥ 03-09 >
//     today → DOSE_INVALID. Trying gap 2 (u ≥ 02-07 ≤ r3.Hi) would demote a
//     definite that holds on every allowed day.
func TestGapSearchKeepsTrueDefinites(t *testing.T) {
	t.Parallel()
	u := numbered(single("u", ""), 2)
	checkAssessment(t, gapAssessment(t, "2026-02-22", single("r2", "2026-02-10"), u), singleUnknown(
		invalidDose("r2", 1, engine.CheckMinAge), invalidDose("u", 2, engine.CheckMinInterval)))
	checkAssessment(t, gapAssessment(t, "2026-02-22", single("r1", "2026-02-12"), u), singleUnknown(
		invalidDose("u", 2, engine.CheckMinInterval)))
	checkAssessment(t, gapAssessment(t, "2026-02-22", single("r1", "2026-01-09"), single("r2", "2026-02-10"), single("u", "")), singleUnknown(
		invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
		engine.Reason{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "u"}}))
	checkAssessment(t, gapAssessment(t, "2026-02-20", single("r1", "2026-01-05"), single("r2", "2026-01-10"), single("r3", "2026-02-09"), u), singleUnknown(
		invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge), invalidDose("r3", 1, engine.CheckMinAge),
		invalidDose("u", 2, engine.CheckMinInterval)))
}

// TestGapSearchStopsAtNumberedDose (issue #35 In scope 1, guard that passes
// on main): P is the latest dated dose that is counted or numbered, so an
// uncounted numbered dated dose ends the scan for the U's. Its number fixes
// its position, and S numbered above it cannot lie before it. In-test series
// q-1 (maxAgeComplete 10w = 03-12, so Validate warns
// SERIES_CANNOT_COMPLETE): dose 1 minAge 6w, doses 2 and 3 minInterval 4w.
// Today 04-10: u1 01-09 (before 6w → DOSE_INVALID{minAge, u1, 1},
// uncounted), n #2 03-20 (past 10w → DOSE_AFTER_MAX_AGE{maxAgeComplete, n,
// 2}, uncounted), s #3 undated.
//   - n is P, so there are no U's and no gap: s trails n, n + 4w = 04-17 >
//     today → DOSE_INVALID{minInterval, s, 3}, and s's earliest date is past
//     10w → DOSE_AFTER_MAX_AGE{maxAgeComplete, s, 3}. s #3 dated before n #2
//     is a NUMBERING_CONFLICT, so no day clears it.
//
// Scanning past n would take u1 as U1 and n as U2: gap 1 (after u1) gives
// s ≥ u1 + 4w + 4w = 03-06 ≤ n.Hi, fits, and would drop the certain
// DOSE_INVALID on s.
func TestGapSearchStopsAtNumberedDose(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P10W", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
	})), ruleset.SeriesCannotComplete)
	got := assessmentOf(t, e, "q", "2026-04-10", birthGap,
		doseOf("q", "u1", dayDate("2026-01-09"), ""), numbered(doseOf("q", "n", dayDate("2026-03-20"), ""), 2),
		numbered(doseOf("q", "s", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("u1", 1, engine.CheckMinAge),
			afterMaxAge("n", 2, engine.CheckMaxAgeComplete),
			invalidDose("s", 3, engine.CheckMinInterval),
			afterMaxAge("s", 3, engine.CheckMaxAgeComplete),
		},
	})
}

// TestGapSearchLinear: the gap search scans the uncounted dated doses
// before the trailing run once per walk, so many of them stay linear,
// with the rescue and floating fit walks on top. illustrative-max-age,
// today 07-10: n rota records from 05-01 (past 15w@b, uncounted), u #3
// undated and two floating doses.
func TestGapSearchLinear(t *testing.T) {
	if testing.Short() || runtime.GOOS == "js" {
		t.Skip("timing test")
	}
	today, first := mustDate(t, "2026-07-10"), mustDate(t, "2026-05-01")
	e := loadEngine(t, "illustrative-max-age")
	assertLinear(t, 2000, func(n int) time.Duration {
		doses := make([]engine.DoseRecord, n, n+3)
		for i := range doses {
			doses[i] = rota(fmt.Sprintf("r%05d", i), dayDate(first.AddDays(i%60).String()), "")
		}
		doses = append(doses, numbered(rota("u", unknownDate, ""), 3), rota("f0", unknownDate, ""), rota("f1", unknownDate, ""))
		in := engine.Input{Today: today, Profile: engine.Profile{Birth: birthGap}, Doses: doses}
		began := time.Now()
		if _, err := e.Evaluate(in); err != nil {
			t.Fatal(err)
		}
		return time.Since(began)
	})
}

// TestGapSearchKeepsMaxAgeDefinite (issue #35 In scope 1, Q103): the gap
// search lowers only the minInterval term of the last slot, so a slot that
// it does not make counted, because a maximum age (0a, 0c) still fails
// definitely, keeps that DOSE_AFTER_MAX_AGE through S4b 0b's rescue. In-test
// series q-1 (maxAgeStart 15w = 04-16, maxAgeComplete 18w = 05-07, so
// Validate warns SERIES_CANNOT_COMPLETE): dose 1 minAge 6w, dose 2
// minInterval 4w, dose 3 minAge 19w (05-14) and minInterval 4w. Today
// 05-25: d1 01-18, dd 04-20, u3 05-20, u0 undated with no number
// (floating), s #3 undated. Step 2: d1 1, dd 2, u3 4 (3 is s's).
//   - Walk: d1 < 6w → DOSE_INVALID{minAge, d1, 1}; dd and u3 are past
//     15w at 1, uncounted. s trails u3 at 3: u3 + 4w = 06-17 > today. Gap 2
//     (after dd, q_2 = 2): s ≥ max(dd + 4w, 19w) = 05-18 ≤ u3.Hi → fits, so
//     its minInterval is at most possible. But 19w is past 18w, so s fails
//     maxAgeComplete on every day: DOSE_AFTER_MAX_AGE, not counted.
//   - Rescue: u0 in front of dd counts dd at 2, so dd is forced counted
//     with DOSE_POSSIBLY_INVALID{maxAgeStart, dd, 1}. The redone walk counts
//     u3 at 2 (dd + 4w = 05-18 ≤ u3), so s trails a counted dose: no gap,
//     u3 + 4w > today → DOSE_INVALID{minInterval, s, 3}, and
//     DOSE_AFTER_MAX_AGE{maxAgeComplete, s, 3}, which the rescuing walk
//     gives s too, so s is not forced.
//   - u0 fits beyond the series: k = 2 + 1 + 1 = 4 > 3 → COMPLETE.
//
// This is main's result. Adding s to the slots the gap search demoted
// would force it, and its certain DOSE_AFTER_MAX_AGE would become
// DOSE_POSSIBLY_INVALID{minInterval, maxAgeComplete, s, 3}.
func TestGapSearchKeepsMaxAgeDefinite(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "P15W", "P18W", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "P19W", "P4W", "", ruleset.FromPreviousDose, "P1M"),
	})), ruleset.SeriesCannotComplete)
	got := assessmentOf(t, e, "q", "2026-05-25", birthGap,
		doseOf("q", "d1", dayDate("2026-01-18"), ""), doseOf("q", "dd", dayDate("2026-04-20"), ""),
		doseOf("q", "u3", dayDate("2026-05-20"), ""), doseOf("q", "u0", unknownDate, ""),
		numbered(doseOf("q", "s", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("d1", 1, engine.CheckMinAge),
			possiblyInvalid("dd", 1, engine.CheckMaxAgeStart),
			invalidDose("s", 3, engine.CheckMinInterval),
			afterMaxAge("s", 3, engine.CheckMaxAgeComplete),
			complete,
		},
	})
}

// TestGapBoundSameDay (issue #35, Q99 (a), F05.10): when S's earliest date
// in gap k is U(k+1)'s own day, S dated that day lies before U(k+1) only
// when it walks first on that day, that is when U(k+1)'s step-2 position
// q_(k+1) is above n (orderSameDay gives the unnumbered U(k+1) the next
// free position). Otherwise U(k+1) takes a position before S, S's
// interval is measured from U(k+1), and that day is the gap after U(k+1).
//   - Fits: minimal-single, today 03-08, r1 01-12 and r2 02-09 (< 6w@b,
//     DOSE_INVALID{minAge} at 1), u #2 undated. Step 2: r1 1, r2 3 (2 is
//     u's), so q_2 = 3 > 2. Gap 1: u ≥ r1 + 4w = 02-09 = r2's day → fits;
//     u dated 02-09 walks at 2 before r2 (then at 3, beyond the series) and
//     is valid. As walked: r2 + 4w = 03-09 > today →
//     DOSE_POSSIBLY_INVALID{minInterval, u, 2}, counted, k = 3 > 2 →
//     COMPLETE.
//   - Does not fit: illustrative-vn hpv (dose 1 minAge 9y, dose 2
//     minInterval 4w, dose 3 minInterval 12w and minIntervalFromFirst 5m),
//     birth 2017-01-01, today 2025-10-01: r1 2025-06-01 and r2 2025-09-21
//     (both before 9y, DOSE_INVALID{minAge} at 1), u #3 undated. Step 2: r1
//     1, r2 2 (3 is u's), so q_2 = 2 < 3. Gap 0 needs dose 1 ≥ 9y > today.
//     Gap 1 (q_1 = 1): dose 2 ≥ r1 + 4w = 06-29, u ≥ 06-29 + 12w = 09-21 =
//     r2's day; u dated 09-21 walks after r2, which takes position 2, and
//     needs r2 + 12w. Behind r2: r2 + 12w = 12-14 > today. No day clears u
//     → DOSE_INVALID{minInterval, u, 3}, as on main.
func TestGapBoundSameDay(t *testing.T) {
	t.Parallel()
	checkAssessment(t, gapAssessment(t, "2026-03-08", single("r1", "2026-01-12"), single("r2", "2026-02-09"), numbered(single("u", ""), 2)), singleUnknown(
		invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
		possiblyInvalid("u", 2, engine.CheckMinInterval), complete))
	e := loadEngine(t, "illustrative-vn")
	got := assessmentOf(t, e, "hpv", "2025-10-01", dayDate("2017-01-01"),
		doseOf("hpv", "r1", dayDate("2025-06-01"), ""), doseOf("hpv", "r2", dayDate("2025-09-21"), ""),
		numbered(doseOf("hpv", "u", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "hpv", Bucket: engine.BucketUnknown, SeriesUsed: []string{"hpv-generic"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
			invalidDose("u", 3, engine.CheckMinInterval),
		},
	})
}

// TestGapBoundSameDayImplied (issue #35, Q99 (a), F05.10): when the last
// trailing slot S is implied, the record a day can be given to is its
// carrier, the level-4 dose #N beyond the series. Dated on U(k+1)'s day,
// the carrier walks before U(k+1) only when q_(k+1) > N (orderSameDay);
// otherwise U(k+1) takes a position before it, and the implied slots
// between them are chained from U(k+1), which is the next gap. So the
// same-day rule reads the carrier's number, not the implied slot's.
//   - minimal-anchors (dose 1 minAge 9y, dose 2 minInterval 4w, dose 3
//     minInterval 12w), today 07-12: d0 02-25 and d1 06-17 (before 9y,
//     DOSE_INVALID{minAge} at 1, uncounted), u #4 undated (beyond the
//     3-dose series), so dose 3 is implied and u carries it. Step 2: d0 1,
//     d1 2 (4 is u's), so q_2 = 2 < 4. As walked: d1 + 12w = 09-09 >
//     today. Gap 0 needs dose 1 ≥ 9y > today. Gap 1 (q_1 = 1): dose 2 ≥
//     d0 + 4w = 03-25, dose 3 ≥ 03-25 + 12w = 06-17 = d1's day; u dated
//     06-17 walks after d1, which takes position 2, and dose 3 then needs
//     d1 + 12w. No day clears u → DOSE_INVALID{minInterval, u, 4}, as on
//     main. The implied dose 3 stays counted, so k = 4 > 3 → COMPLETE.
//   - In-test series q-1 (maxAgeStart 10w = 03-12): dose 1 minAge 6w,
//     doses 2 and 3 minInterval 4w. Today 04-20: u1 02-01 (before 6w →
//     DOSE_INVALID{minAge, u1, 1}), u2 03-29 (past 10w →
//     DOSE_AFTER_MAX_AGE{maxAgeStart, u2, 1}), both uncounted; s #4
//     undated, so dose 3 is implied and s carries it. Step 2: u1 1, u2 2,
//     so q_2 = 2 < 4. As walked: u2 + 4w = 04-26 > today. Gap 0: dose 3 ≥
//     02-12 + 4w + 4w = 04-09 > u1.Hi. Gap 1 (q_1 = 1): dose 2 ≥ 03-01,
//     dose 3 ≥ 03-29 = u2's day, where s walks after u2 → no fit →
//     DOSE_INVALID{minInterval, s, 4} and COMPLETE, as on main.
//
// Reading the implied slot's own number (0) as "no same-day order" would
// fit gap 1 on that day and demote both to DOSE_POSSIBLY_INVALID, although
// no day clears them.
func TestGapBoundSameDayImplied(t *testing.T) {
	t.Parallel()
	implied3 := engine.Reason{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{3}}}
	e := loadEngine(t, "minimal-anchors")
	got := assessmentOf(t, e, "anchored", "2026-07-12", birthGap,
		doseOf("anchored", "d0", dayDate("2026-02-25"), ""), doseOf("anchored", "d1", dayDate("2026-06-17"), ""),
		numbered(doseOf("anchored", "u", unknownDate, ""), 4))
	checkAssessment(t, got, engine.Assessment{
		Disease: "anchored", Bucket: engine.BucketUnknown, SeriesUsed: []string{"anchored-series"}, Estimated: true,
		Reasons: []engine.Reason{
			implied3,
			invalidDose("d0", 1, engine.CheckMinAge), invalidDose("d1", 1, engine.CheckMinAge),
			invalidDose("u", 4, engine.CheckMinInterval),
			complete,
		},
	})
	e = newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "P10W", "", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
	})))
	got = assessmentOf(t, e, "q", "2026-04-20", birthGap,
		doseOf("q", "u1", dayDate("2026-02-01"), ""), doseOf("q", "u2", dayDate("2026-03-29"), ""),
		numbered(doseOf("q", "s", unknownDate, ""), 4))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			implied3,
			invalidDose("u1", 1, engine.CheckMinAge),
			afterMaxAge("u2", 1, engine.CheckMaxAgeStart),
			invalidDose("s", 4, engine.CheckMinInterval),
			complete,
		},
	})
}

// TestGapZeroChecksFromFirst (issue #35, Q99 (a), F05.10): gap 0 chains from
// birth through position 1, so when the walk has no dose 1 (U1, at step-2
// position 1, is uncounted), dose 1 of that gap is the chain's own position
// 1, and minIntervalFromFirst of the later positions is measured from it,
// as for an undated dose 1 in the walk (EQ20b). In-test series q-1
// (maxAgeStart 15w = 04-16): dose 1 minAge 6w, dose 2 minInterval 4w, dose
// 3 minInterval 4w and minIntervalFromFirst 16w. Today 06-10: u1 05-01
// (past 15w → DOSE_AFTER_MAX_AGE{maxAgeStart, u1, 1}, uncounted), s #3
// undated, so dose 2 is implied.
//   - As walked: u1 → implied 2 ≥ 05-29 → s ≥ 06-26 > today.
//   - Gap 0 (q_0 = 0 < 3): dose 1 ≥ 02-12, dose 2 ≥ 03-12, s ≥ max(04-09,
//     02-12 + 16w = 06-04) = 06-04 > u1.Hi → does not fit. s dated 04-09 to
//     04-30 fails minIntervalFromFirst from implied dose 1, and from 05-01
//     on it lies behind u1 → no day clears s.
//
// → DOSE_INVALID{minInterval, s, 3}, as on main. Without dose 1 in the gap
// chain, s ≥ 04-09 ≤ u1 would fit and demote it.
func TestGapZeroChecksFromFirst(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "P15W", "", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P4W", "P16W", ruleset.FromPreviousDose, "P1M"),
	})))
	got := assessmentOf(t, e, "q", "2026-06-10", birthGap,
		doseOf("q", "u1", dayDate("2026-05-01"), ""), numbered(doseOf("q", "s", unknownDate, ""), 3))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{2}}},
			afterMaxAge("u1", 1, engine.CheckMaxAgeStart),
			invalidDose("s", 3, engine.CheckMinInterval),
		},
	})
}

// TestGapSearchKeepsFloatingFit (issue #35 Q103 (a)): a definite reason
// that a placement's walk gives a slot that the gap search made counted
// does not count against the placement in S4b 0b's floating fit either.
// In-test series q-1 (maxAgeStart 8w = 02-26): dose 1 minAge 6w (02-12),
// dose 2 minInterval 4w, doses 3 and 4 minInterval 1w. Today 04-10: u1
// 03-01 and u2 04-06 (both past 8w at 1: DOSE_AFTER_MAX_AGE{maxAgeStart}),
// s #3 undated, f undated with no number (floating). Step 2: u1 1, u2 2.
//   - Walk: s trails u2 at 3, u2 + 1w = 04-13 > today. Gap 0 needs dose 1 ≥
//     02-12, dose 2 ≥ 03-12, s ≥ 03-19 > u1. Gap 1 (after u1, q_1 = 1):
//     dose 2 ≥ u1 + 4w = 03-29, s ≥ 04-05 < u2.Hi → fits, threshold 04-05
//     ≤ today → DOSE_POSSIBLY_INVALID{minInterval, s, 3}, counted.
//   - Floating fit: f after everything is at 4 behind s, u2 + 1w + 1w =
//     04-20 > today → DOSE_INVALID on f: does not fit. f in front of u1
//     (0b point; its chain, 02-12, lies on or before u1): f at 1, u1 at 2
//     fails 02-12 + 4w = 03-12 (its own reason, and no rescue), u2 counts
//     at 2 (u1 + 4w = 03-29). s then trails a counted dose and gets
//     DOSE_INVALID{minInterval} there (u2 + 1w > today), which does not
//     count against the placement, so f fits.
//   - k = 3 + 1 + 1 = 5 > 4 → COMPLETE.
//
// Without the exemption, f would fit nowhere and get DOSE_INVALID.
func TestGapSearchKeepsFloatingFit(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "P8W", "", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P1W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P1W", "", ruleset.FromPreviousDose, "P1M"),
	})))
	got := assessmentOf(t, e, "q", "2026-04-10", birthGap,
		doseOf("q", "u1", dayDate("2026-03-01"), ""), doseOf("q", "u2", dayDate("2026-04-06"), ""),
		numbered(doseOf("q", "s", unknownDate, ""), 3), doseOf("q", "f", unknownDate, ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			afterMaxAge("u1", 1, engine.CheckMaxAgeStart),
			afterMaxAge("u2", 1, engine.CheckMaxAgeStart),
			possiblyInvalid("s", 3, engine.CheckMinInterval),
			complete,
		},
	})
}

// TestGapBoundRange (issue #35, Q99 (a)): the gap fits up to U(k+1).Hi, so
// an uncounted dose with a month-precise date bounds the gap at its last
// day. In-test series q-1: dose 1 minAge 20w (05-21), dose 2 minInterval 8w.
// Today 04-20: r1 01-20 and r2 2026-03 (both before 20w → DOSE_INVALID{minAge}
// at 1, uncounted), u #2 undated. Step 2: r1 1, r2 3 (2 is u's).
//   - Gap 0 needs dose 1 ≥ 05-21 > r1. Gap 1 (after r1, q_1 = 1): u ≥ r1 +
//     8w = 03-17, after r2.Lo 03-01 but ≤ r2.Hi 03-31 → fits (r2 on 03-31
//     with u on 03-17 is valid).
//   - As walked: definite r2.Lo + 8w = 04-26 > today, possible r2.Hi + 8w =
//     05-26. The definite threshold is lowered to 03-17 ≤ today →
//     DOSE_POSSIBLY_INVALID{minInterval, u, 2}, counted, k = 3 > 2 →
//     COMPLETE.
func TestGapBoundRange(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "", []ruleset.DoseRule{
		rule(t, "P20W", "", "", ruleset.FromBirth, "P6M"),
		rule(t, "", "P8W", "", ruleset.FromPreviousDose, "P2M"),
	})))
	got := assessmentOf(t, e, "q", "2026-04-20", birthGap,
		doseOf("q", "r1", dayDate("2026-01-20"), ""), doseOf("q", "r2", monthDate("2026-03"), ""),
		numbered(doseOf("q", "u", unknownDate, ""), 2))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{
			invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge),
			possiblyInvalid("u", 2, engine.CheckMinInterval),
			complete,
		},
	})
}
