package engine_test

import (
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of "certainly out of age" from the earliest possible dose
// dates (PR #31 review, owner decision 2026-11-24): a candidate is certainly
// out only when it is out even from R.Lo of every imprecise dose date
// involved, with no EQ20c stand-in date. Otherwise the result is ⚪ with
// OUT_OF_AGE_UNCERTAIN{numbers} (the positions whose exact date would
// decide), and no next dose is drawn from R.Lo (P1).

func outOfAgeUncertain(numbers ...int) engine.Reason {
	return engine.Reason{Code: engine.ReasonOutOfAgeUncertain, Params: engine.ReasonParams{Numbers: numbers}}
}

var outOfAgeReason = engine.Reason{Code: engine.ReasonOutOfAge}

// twoDoseQ is a generic 2-dose series of disease q: dose 1 minAge 9y, due
// birth + 9y; dose 2 the given minInterval, due previous + 1m;
// maxAgeComplete 15y.
func twoDoseQ(t testing.TB, id, product, interval string) ruleset.Series {
	t.Helper()
	return qSeries(t, id, product, "", "P15Y", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
		rule(t, "", interval, "", ruleset.FromPreviousDose, "P1M"),
	})
}

// threeDoseQ is a generic 3-dose series of disease q: dose 1 minAge 9y, due
// birth + 9y; doses 2 and 3 minInterval 4w, due previous + 1m, dose 3 also
// with the given minIntervalFromFirst ("" for none); maxAgeComplete 15y.
func threeDoseQ(t testing.TB, fromFirst string) ruleset.Series {
	t.Helper()
	return qSeries(t, "q-1", "", "", "P15Y", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P4W", fromFirst, ruleset.FromPreviousDose, "P1M"),
	})
}

// TestImpreciseDoseOutOfAgeUncertain (T1–T3): twoDoseQ with 4w, birth
// 2010-01-15, so L = 2025-01-14; today 2025-01-01.
//   - r1 in 2024-12 (R = [12-01, 12-31]): r1 counted (≥ 9y@b, < 15y@b).
//     F = max(01-01, 12-31 + 4w = 2025-01-28) > L → out. From R.Lo: F^lo =
//     max(01-01, 12-01 + 4w = 12-29) = 01-01 ≤ L → not certain. Exact birth,
//     so only the dose date decides → ⚪ [OUT_OF_AGE_UNCERTAIN{numbers: [1]}],
//     est. (The branch gave outOfAge.)
//   - r1 on 12-01: E = 12-29, Du = 2025-01-01, O = 01-08; F = 01-01 ≤ L_2 =
//     L; nothing lowered → 🟠 {2, 12-29, 01-01, 01-08}, no reasons.
//   - r1 on 12-31: F = 01-28 > L, certain → outOfAge [OUT_OF_AGE], not est.
//   - Dose 2 minInterval 8w, r1 in 2024-12: F^lo = 12-01 + 8w = 2025-01-26
//     > L → still certain → outOfAge [OUT_OF_AGE], est.
func TestImpreciseDoseOutOfAgeUncertain(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, twoDoseQ(t, "q-1", "", "P4W")))
	birth := dayDate("2010-01-15")
	t.Run("month dose", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", monthDate("2024-12"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeUncertain(1)},
		})
	})
	t.Run("earliest day of the month", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", dayDate("2024-12-01"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketDue, NextDose: nextDose(t, 2, "2024-12-29", "2025-01-01", "2025-01-08"),
			SeriesUsed: []string{"q-1"},
		})
	})
	t.Run("latest day of the month", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", dayDate("2024-12-31"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"q-1"}, Reasons: []engine.Reason{outOfAgeReason},
		})
	})
	t.Run("certain from the earliest day", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(nil, twoDoseQ(t, "q-1", "", "P8W")))
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", monthDate("2024-12"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeReason},
		})
	})
}

// TestStandInOutOfAgeUncertain (T4, T5): threeDoseQ with
// minIntervalFromFirst 5m, birth 2010-01-01, so L = 2024-12-31.
//   - r1 #1 undated, r2 #2 on 2024-08-28, today 09-01. r2's interval to the
//     undated r1 is waived → INTERVAL_UNVERIFIABLE{r2, 2}. k = 3, and r2 is
//     the EQ20c stand-in for dose 1: E = max(r2 + 4w = 09-25, r2 + 5m =
//     2025-01-28) = 01-28 > L → out. The real dose 1 is earlier than r2 by
//     an unknown amount, so the stand-in term is dropped from the earliest
//     possible date: F^lo = max(09-01, 09-25) = 09-25 ≤ L → not certain →
//     ⚪ [INTERVAL_UNVERIFIABLE{r2, 2}, OUT_OF_AGE_UNCERTAIN{numbers: [1]}],
//     est. (The branch gave outOfAge.)
//   - r1 #1 on 2024-07-31 instead: E = max(09-25, 07-31 + 5m = 12-31) =
//     12-31, Du = max(E, r2 + 1m) = 12-31, O = 2025-01-07; F = 12-31 ≤ L_3 =
//     L → O lowered to 12-31 → 🟡 {3, 12-31, 12-31, 12-31} with
//     DUE_CAPPED_BY_MAX_AGE{2024-12-31}.
//   - Implied dose 1 (r2 #2 alone): the same, after
//     IMPLIED_EARLIER_DOSES{numbers: [1]}.
//   - r2 on 2024-12-20, today 12-21: F^lo = max(12-21, 12-20 + 4w =
//     2025-01-17) > L → certain → outOfAge [INTERVAL_UNVERIFIABLE{r2, 2},
//     OUT_OF_AGE], est.
func TestStandInOutOfAgeUncertain(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, threeDoseQ(t, "P5M")))
	birth := dayDate("2010-01-01")
	r1 := numbered(doseOf("q", "r1", unknownDate, ""), 1)
	unverifiable := engine.Reason{Code: engine.ReasonIntervalUnverifiable, Params: engine.ReasonParams{Dose: "r2", Number: 2}}
	t.Run("undated dose 1", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2024-09-01", birth, r1, numbered(doseOf("q", "r2", dayDate("2024-08-28"), ""), 2))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{unverifiable, outOfAgeUncertain(1)},
		})
	})
	t.Run("dated dose 1", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2024-09-01", birth,
			numbered(doseOf("q", "r1", dayDate("2024-07-31"), ""), 1), numbered(doseOf("q", "r2", dayDate("2024-08-28"), ""), 2))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 3, "2024-12-31", "2024-12-31", "2024-12-31"),
			SeriesUsed: []string{"q-1"}, Reasons: []engine.Reason{capped("2024-12-31")},
		})
	})
	t.Run("implied dose 1", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2024-09-01", birth, numbered(doseOf("q", "r2", dayDate("2024-08-28"), ""), 2))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{1}}}, unverifiable, outOfAgeUncertain(1)},
		})
	})
	t.Run("certain without the stand-in", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2024-12-21", birth, r1, numbered(doseOf("q", "r2", dayDate("2024-12-20"), ""), 2))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{unverifiable, outOfAgeReason},
		})
	})
}

// TestOutOfAgeUncertainPositions (T6): threeDoseQ, birth 2010-01-15, so L =
// 2025-01-14; today 2025-01-01.
//   - No minIntervalFromFirst. r1 on 2024-06-01, r2 in 2024-12 (its
//     interval to r1 holds from R.Lo). k = 3: F = 12-31 + 4w = 2025-01-28 >
//     L; F^lo = max(01-01, 12-01 + 4w = 12-29) = 01-01 ≤ L → ⚪
//     [OUT_OF_AGE_UNCERTAIN{numbers: [2]}]: only dose 2's date decides.
//   - Dose 3 minIntervalFromFirst 7m and r1 in 2024-06: F = max(01-28,
//     06-30 + 7m = 2025-01-30) = 01-30 > L; F^lo = max(01-01, 12-29, 06-01 +
//     7m = 2025-01-01) = 01-01 ≤ L → numbers [1, 2].
func TestOutOfAgeUncertainPositions(t *testing.T) {
	t.Parallel()
	birth := dayDate("2010-01-15")
	t.Run("latest dose", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(nil, threeDoseQ(t, "")))
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", dayDate("2024-06-01"), ""), doseOf("q", "r2", monthDate("2024-12"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeUncertain(2)},
		})
	})
	t.Run("latest and first dose", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(nil, threeDoseQ(t, "P7M")))
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", monthDate("2024-06"), ""), doseOf("q", "r2", monthDate("2024-12"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeUncertain(1, 2)},
		})
	})
}

// TestOutOfAgeUncertainCandidates (T7): products p (p-1 = twoDoseQ with 8w)
// and q (q-1 = twoDoseQ with 4w), birth 2010-01-15, so L = 2025-01-14 for
// both; today 2025-01-01.
//   - r1 in 2024-12 with no product: both candidates. p-1: F^lo = 12-01 +
//     8w = 2025-01-26 > L → certainly out. q-1: out, not certain (as in
//     TestImpreciseDoseOutOfAgeUncertain). Every candidate out, none
//     complete, not all certain → ⚪ [SAFEST_SERIES{p, q},
//     OUT_OF_AGE_UNCERTAIN{numbers: [1]}], est.
//   - r1 on 12-01 with no product: p-1 out (F = 01-26), q-1 in (F = 01-01)
//     → some out → ⚪ [SAFEST_SERIES, CHECK_PRODUCT_NAME], as before.
//   - r1 in 2024-12 with product p: p-1 alone, certainly out → outOfAge
//     [OUT_OF_AGE], est.
func TestOutOfAgeUncertainCandidates(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot([]string{"p", "q"}, twoDoseQ(t, "p-1", "p", "P8W"), twoDoseQ(t, "q-1", "q", "P4W")))
	birth := dayDate("2010-01-15")
	both := safest("p", "p-1", "q", "q-1")
	t.Run("month dose, no product", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", monthDate("2024-12"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"p-1", "q-1"}, Estimated: true,
			Reasons: []engine.Reason{both, outOfAgeUncertain(1)},
		})
	})
	t.Run("day dose, no product", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", dayDate("2024-12-01"), ""))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"p-1", "q-1"},
			Reasons: []engine.Reason{both, {Code: engine.ReasonCheckProductName}},
		})
	})
	t.Run("month dose, product p", func(t *testing.T) {
		t.Parallel()
		got := assessmentOf(t, e, "q", "2025-01-01", birth, doseOf("q", "r1", monthDate("2024-12"), "p"))
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"p-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeReason},
		})
	})
}

// TestOutOfAgeUncertainBothCauses (T8): twoDoseQ with 4w, birth 2010-01
// (month), r1 in 2024-12, today 2024-12-10 (R = [12-01, 12-10]). L from
// b_old = 2010-01-01 is 2024-12-31 at both ends.
//   - Older end: F = 12-10 + 4w = 2025-01-07 > L → out; F^lo = 12-01 + 4w =
//     12-29 ≤ L → not certain, and only the dose date decides → ⚪
//     [OUT_OF_AGE_UNCERTAIN{numbers: [1]}].
//   - Later end (01-31): Limit^b = 2025-01-30 ≥ F, and Limit = 12-31 ≥ F^lo,
//     so both the birth date and the dose date are needed → ⚪
//     [BIRTH_DATE_IMPRECISE{fullBirthDate}, OUT_OF_AGE_UNCERTAIN{numbers:
//     [1]}].
//   - The ends disagree → ⚪ [BIRTH_DATE_IMPRECISE{fullBirthDate},
//     OUT_OF_AGE_UNCERTAIN{numbers: [1]}], est.
func TestOutOfAgeUncertainBothCauses(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, twoDoseQ(t, "q-1", "", "P4W")))
	got := assessmentOf(t, e, "q", "2024-12-10", monthDate("2010-01"), doseOf("q", "r1", monthDate("2024-12"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{fullBirthDate, outOfAgeUncertain(1)},
	})
}

// TestOutOfAgeUncertainKeepsDataUnknown (T9): threeDoseQ with no
// minIntervalFromFirst, birth 2010-01-15, so L = 2025-01-14; r1 on
// 2024-11-20, r2 in 2024-12, today 2025-01-01. r2 is possibly before r1 + 4w
// = 12-18 → DOSE_POSSIBLY_INVALID{[minInterval], r2, 2}, counted. k = 3: F =
// 12-31 + 4w = 2025-01-28 > L → out; F^lo = 12-29 ∨ today = 01-01 ≤ L → not
// certain, so not layer 5: the data ⚪ stays, with no new reason. (The branch
// gave outOfAge. If r2 was given by 12-17 it is invalid, and dose 2 can be
// repeated by 01-14.)
func TestOutOfAgeUncertainKeepsDataUnknown(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil, threeDoseQ(t, "")))
	got := assessmentOf(t, e, "q", "2025-01-01", dayDate("2010-01-15"), doseOf("q", "r1", dayDate("2024-11-20"), ""), doseOf("q", "r2", monthDate("2024-12"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{possiblyInvalid("r2", 2, engine.CheckMinInterval)},
	})
}

// TestRotaTeqMonthDoseCounterparts: the exact-date counterparts of golden
// f13-rotateq-month-dose-out-of-age-uncertain (illustrative-max-age, birth
// 2026-01-01; rota-rotateq L = 32w@b − 1 = 08-12). r1 rotateq on 03-01,
// today 08-01, and r2 rotateq:
//   - on 07-01: dose 3 E = 07-29, Du = max(E, 6m@b = 07-01) = 07-29, O =
//     08-05; F = 08-01 ≤ L_3 = L, nothing lowered → 🟠 {3, 07-29, 07-29,
//     08-05}, no reasons;
//   - on 07-31: F = 08-28 > L, certain → outOfAge [OUT_OF_AGE].
func TestRotaTeqMonthDoseCounterparts(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	r1 := rota("r1", dayDate("2026-03-01"), "rotateq")
	got := assessmentOf(t, e, "rota", "2026-08-01", birthRota, r1, rota("r2", dayDate("2026-07-01"), "rotateq"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketDue, NextDose: nextDose(t, 3, "2026-07-29", "2026-07-29", "2026-08-05"),
		SeriesUsed: []string{"rota-rotateq"}, Products: []string{"rotateq"},
	})
	got = assessmentOf(t, e, "rota", "2026-08-01", birthRota, r1, rota("r2", dayDate("2026-07-31"), "rotateq"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Block: engine.BlockOutOfAge, SeriesUsed: []string{"rota-rotateq"}, Reasons: []engine.Reason{outOfAgeReason},
	})
}

// TestNumberedOverlapCertainFromLargestLo: the earliest possible date of
// the latest dose in time is the largest R.Lo of the dated doses, not R.Lo
// of the last record in walk order. In-test series q-1: dose 1 minAge 9y,
// due birth + 9y; dose 2 minAge 9y only (or minInterval 4w), due previous
// + 1m; dose 3 minInterval 8w, due previous + 1m; maxAgeComplete 15y.
// Birth 2010-01-15, so L = 2025-01-14; today 2025-01-01. r1 #1 in 2024-12
// (R = [12-01, 12-31]), r2 #2 in 2024 (R = [2024-01-01, 12-31]): the ranges
// overlap and the numbers order them, so r2 is walked last.
//   - Dose 2 minAge only: r1 counted (≥ 9y, < 15y), r2 counted. k = 3: F =
//     12-31 + 8w = 2025-02-25 > L → out. r2 is given on or after r1, so the
//     latest dose is on or after 12-01: F^lo = max(01-01, 12-01 + 8w =
//     2025-01-26) > L → certain → outOfAge [OUT_OF_AGE], est. (R.Lo of r2
//     alone gave F^lo = max(01-01, 2024-02-26) = 01-01 ≤ L → ⚪
//     OUT_OF_AGE_UNCERTAIN.)
//   - Dose 2 minInterval 4w: r2 is possibly before r1 + 4w →
//     DOSE_POSSIBLY_INVALID{[minInterval], r2, 2}, counted. Counted view as
//     above, certain → layer 5: outOfAge [DOSE_POSSIBLY_INVALID{…},
//     OUT_OF_AGE], est.
func TestNumberedOverlapCertainFromLargestLo(t *testing.T) {
	t.Parallel()
	year2024 := engine.RecalledDate{Value: "2024", Precision: engine.Year}
	series := func(interval string) ruleset.Series {
		return qSeries(t, "q-1", "", "", "P15Y", []ruleset.DoseRule{
			rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
			rule(t, "P9Y", interval, "", ruleset.FromPreviousDose, "P1M"),
			rule(t, "", "P8W", "", ruleset.FromPreviousDose, "P1M"),
		})
	}
	doses := []engine.DoseRecord{numbered(doseOf("q", "r1", monthDate("2024-12"), ""), 1), numbered(doseOf("q", "r2", year2024, ""), 2)}
	t.Run("minimum age only", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(nil, series("")))
		got := assessmentOf(t, e, "q", "2025-01-01", dayDate("2010-01-15"), doses...)
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{outOfAgeReason},
		})
	})
	t.Run("possibly too early", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(nil, series("P4W")))
		got := assessmentOf(t, e, "q", "2025-01-01", dayDate("2010-01-15"), doses...)
		checkAssessment(t, got, engine.Assessment{
			Disease: "q", Block: engine.BlockOutOfAge, SeriesUsed: []string{"q-1"}, Estimated: true,
			Reasons: []engine.Reason{possiblyInvalid("r2", 2, engine.CheckMinInterval), outOfAgeReason},
		})
	})
}
