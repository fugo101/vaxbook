package engine_test

import (
	"fmt"
	"reflect"
	"slices"
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

// ws returns sr with the validity window [from, to) ("" for an absent
// bound) and, when min is set, the start-age bracket [min, max) (max ""
// for open-ended).
func ws(t testing.TB, sr ruleset.Series, from, to, min, max string) ruleset.Series {
	t.Helper()
	date := func(s string) *ruleset.Date {
		if s == "" {
			return nil
		}
		d, err := ruleset.ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	sr.ValidFrom, sr.ValidTo = date(from), date(to)
	if min != "" {
		sr.StartAge = &ruleset.StartAge{Min: *spanOf(t, min)}
		if max != "" {
			sr.StartAge.Max = spanOf(t, max)
		}
	}
	return sr
}

// twoDose is a series of disease q: dose 1 minAge minAge due birth + 2m,
// dose 2 minInterval interval due previous + interval; product "" is
// generic.
func twoDose(t testing.TB, id, product, minAge, interval string) ruleset.Series {
	t.Helper()
	return qSeries(t, id, product, "", "", []ruleset.DoseRule{
		rule(t, minAge, "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", interval, "", ruleset.FromPreviousDose, interval),
	})
}

func qDose(id, date string) engine.DoseRecord {
	switch {
	case date == "":
		return doseOf("q", id, unknownDate, "")
	case len(date) == 7:
		return doseOf("q", id, monthDate(date), "")
	}
	return doseOf("q", id, dayDate(date), "")
}

func pqDose(id, date, product string) engine.DoseRecord {
	return doseOf("q", id, dayDate(date), product)
}

func unknownWith(series []string, est bool, reasons ...engine.Reason) engine.Assessment {
	return engine.Assessment{Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: series, Estimated: est, Reasons: reasons}
}

func completeWith(series []string, est bool, reasons ...engine.Reason) engine.Assessment {
	return engine.Assessment{Disease: "q", Bucket: engine.BucketComplete, SeriesUsed: series, Estimated: est, Reasons: append(reasons, complete)}
}

// TestVersionPerPart (Q77): a dose range that crosses a boundary is judged
// per part. minimal-effective-dating with the boundary moved to
// 2025-12-15: ed r1 2025-11-01, r2 in 2025-12, today 2026-02-01. The part
// [12-01, 12-14] is valid under ed-2025 (11-29); [12-15, 12-31] is under
// ed-2026 only, which needs 12-27 → possible → ⚪
// [DOSE_POSSIBLY_INVALID{minInterval, r2, 2}, COMPLETE], est. A version
// applied to the whole range would count r2 cleanly.
func TestVersionPerPart(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "minimal-effective-dating")
	cut := ruleset.Date{Year: 2025, Month: 12, Day: 15}
	s.Series[0].ValidTo, s.Series[1].ValidFrom = &cut, &cut
	got := assessmentOf(t, newEngine(t, s), "ed", "2026-02-01", dayDate("2025-09-01"),
		doseOf("ed", "r1", dayDate("2025-11-01"), ""), doseOf("ed", "r2", monthDate("2025-12"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "ed", Bucket: engine.BucketUnknown, SeriesUsed: []string{"ed-2026"}, Estimated: true,
		Reasons: []engine.Reason{possiblyInvalid("r2", 2, engine.CheckMinInterval), complete},
	})
}

// TestVersionBracketThreeWay (Q93): k-a [6w, 9m) with dose 2 at 4w and k-b
// [9m, —) at 8w until 2026-01-01; k-2026 (no bracket) at 8w from then.
// Birth 2024-05-15 (9m = 2025-02-15), r2 2025-03-28, today 2026-02-01.
//   - r1 in 2025-02: k-a possibly holds dose 1 and accepts r2 (4w from
//     02-01 = 03-01), k-b fails it → k-2026's definite failure is softened
//     → ⚪ [DOSE_POSSIBLY_INVALID{minInterval, r2, 2}, COMPLETE], est.
//   - r1 2025-02-10: k-a certainly holds it → ✅ COMPLETE.
//   - r1 2025-02-20: k-a cannot hold it, k-b fails r2 → ⚪ DOSE_INVALID.
//   - No birth date, r1 2025-02-20: k-a possibly holds it → ⚪
//     [DOSE_POSSIBLY_INVALID, COMPLETE], est.
func TestVersionBracketThreeWay(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "k-a", "", "P6W", "P4W"), "", "2026-01-01", "P6W", "P9M"),
		ws(t, twoDose(t, "k-b", "", "P6W", "P8W"), "", "2026-01-01", "P9M", ""),
		ws(t, twoDose(t, "k-2026", "", "P6W", "P8W"), "2026-01-01", "", "", ""),
	))
	birth := dayDate("2024-05-15")
	used := []string{"k-2026"}
	r2 := qDose("r2", "2025-03-28")
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, qDose("r1", "2025-02"), r2),
		unknownWith(used, true, possiblyInvalid("r2", 2, engine.CheckMinInterval), complete))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, qDose("r1", "2025-02-10"), r2), completeWith(used, false))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, qDose("r1", "2025-02-20"), r2),
		unknownWith(used, false, invalidDose("r2", 2, engine.CheckMinInterval)))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", engine.RecalledDate{Precision: engine.None}, qDose("r1", "2025-02-20"), r2),
		unknownWith(used, true, possiblyInvalid("r2", 2, engine.CheckMinInterval), complete))
}

// TestVersionUndatedDoseOne (Q93): u-a [6w, 9m) with maxAgeComplete 3y and
// u-b [9m, —) with 2y until 2026-01-01, then u-2026 with 2y; dose 2 at 4w.
// Birth 2022-09-01, u1 level 4 #1, r2 2025-03-01 #2 (2.5 years), today
// 2026-02-01. u-2026's 2y fails r2 definitely. Dose 1 lies in [birth, r2],
// so u-a only possibly holds it, and its 3y accepts r2 → ⚪
// [DOSE_POSSIBLY_INVALID{maxAgeComplete, r2, 2}, COMPLETE], est. Reading
// dose 1 at the chain's b + 6w would make u-a certain and r2 count.
func TestVersionUndatedDoseOne(t *testing.T) {
	t.Parallel()
	with := func(sr ruleset.Series, complete string) ruleset.Series {
		sr.MaxAgeComplete = spanOf(t, complete)
		return sr
	}
	e := newEngine(t, oneDiseaseSnapshot(nil,
		with(ws(t, twoDose(t, "u-a", "", "P6W", "P4W"), "", "2026-01-01", "P6W", "P9M"), "P3Y"),
		with(ws(t, twoDose(t, "u-b", "", "P6W", "P4W"), "", "2026-01-01", "P9M", ""), "P2Y"),
		with(ws(t, twoDose(t, "u-2026", "", "P6W", "P4W"), "2026-01-01", "", "", ""), "P2Y"),
	))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2022-09-01"), numbered(qDose("u1", ""), 1), numbered(qDose("r2", "2025-03-01"), 2))
	checkAssessment(t, got, unknownWith([]string{"u-2026"}, true, possiblyInvalid("r2", 2, engine.CheckMaxAgeComplete), complete))
}

// TestVersionFinalDose (Q96): dose 1 minAge 9y everywhere. h-9-14 [9y,
// 15y): dose 2 at 5m. h-15-45 [15y, 45y): dose 2 at 4w, dose 3 at 12w (due
// dose 1 + 6m). Both until 2026-01-01; h-2026 from then, as h-9-14 with no
// bracket. Birth 2010-06-15, r1 2025-07-01 (15y), r2 2025-08-01.
//   - Today 2025-12-01: h-15-45 → 🟡 {3, 2025-10-24, 2026-01-01,
//     2026-01-08}. (S4d, #36, turns it ⚪ RULE_CHANGE_PENDING.)
//   - Today 2026-02-01: h-2026 needs 5m; h-15-45 accepts r2 but has 3 doses,
//     so r2 would be its dose 2, not a last dose → ⚪ DOSE_INVALID. Without
//     the rule: ✅ COMPLETE.
func TestVersionFinalDose(t *testing.T) {
	t.Parallel()
	nineY := rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y")
	h914 := qSeries(t, "h-9-14", "", "", "", []ruleset.DoseRule{nineY, rule(t, "", "P5M", "", ruleset.FromPreviousDose, "P6M")})
	h1545 := qSeries(t, "h-15-45", "", "", "", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P15Y"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P2M"),
		rule(t, "", "P12W", "", ruleset.FromFirstDose, "P6M"),
	})
	h2026 := h914
	h2026.ID = "h-2026"
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, h914, "", "2026-01-01", "P9Y", "P15Y"),
		ws(t, h1545, "", "2026-01-01", "P15Y", "P45Y"),
		ws(t, h2026, "2026-01-01", "", "", ""),
	))
	birth := dayDate("2010-06-15")
	doses := []engine.DoseRecord{qDose("r1", "2025-07-01"), qDose("r2", "2025-08-01")}
	checkAssessment(t, assessmentOf(t, e, "q", "2025-12-01", birth, doses...), engine.Assessment{
		Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 3, "2025-10-24", "2026-01-01", "2026-01-08"), SeriesUsed: []string{"h-15-45"},
	})
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, doses...),
		unknownWith([]string{"h-2026"}, false, invalidDose("r2", 2, engine.CheckMinInterval)))
}

// TestVersionBirthRange (In scope 3, Q107): a version is judged at both
// ends of the birth range. g-2025 (until 2026-01-01): one dose, minAge 9m,
// maxAgeComplete 2y. g-2026 (from then): minAge 4w, maxAgeComplete 8m. r1
// 2025-08-15, today 2026-02-01.
//   - Birth in 2024: at the 2024-01-01 end g-2026's 8m fails r1 definitely;
//     g-2025 passes r1 at b_old 2024-01-01 (9m = 2024-10-01, 2y =
//     2026-01-01) and fails its 9m at b_late 2024-12-31 (2025-09-30), so
//     it passes at one end only and softens the failure (Q107): ⚪
//     [DOSE_POSSIBLY_INVALID{maxAgeComplete, r1, 1}, COMPLETE]. At the
//     2024-12-31 end g-2026 counts it: ✅. The ends differ → ⚪
//     [BIRTH_DATE_IMPRECISE{fullBirthDate}, COMPLETE,
//     DOSE_POSSIBLY_INVALID{maxAgeComplete, r1, 1}], est. Judged at b
//     alone, g-2025 would count r1 at 2024-01-01 although a birth date
//     between fails both; judged by its worse end it would make the failure
//     definite although birth 2024-01-01 counts r1.
//   - Birth 2024-12-01 (8.5 months at r1): past g-2026's 8m, below g-2025's
//     9m → ⚪ DOSE_AFTER_MAX_AGE.
//   - Birth 2024-01-01: past today's 8m, within g-2025's rules → ✅.
func TestVersionBirthRange(t *testing.T) {
	t.Parallel()
	one := func(id, minAge, complete string) ruleset.Series {
		return qSeries(t, id, "", "", complete, []ruleset.DoseRule{rule(t, minAge, "", "", ruleset.FromBirth, "P9M")})
	}
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, one("g-2025", "P9M", "P2Y"), "", "2026-01-01", "", ""),
		ws(t, one("g-2026", "P4W", "P8M"), "2026-01-01", "", "", ""),
	))
	used := []string{"g-2026"}
	r1 := qDose("r1", "2025-08-15")
	after := afterMaxAge("r1", 1, engine.CheckMaxAgeComplete)
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", engine.RecalledDate{Value: "2024", Precision: engine.Year}, r1),
		unknownWith(used, true, fullBirthDate, complete, possiblyInvalid("r1", 1, engine.CheckMaxAgeComplete)))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2024-12-01"), r1), unknownWith(used, false, after))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2024-01-01"), r1), completeWith(used, false))
}

// TestVersionGroupMaxOfItsSegment (In scope 3): a version's maximum ages are
// its own segment's. 2025: m-a [6w, 9m), m-b [9m, 2y); 2026: m-a2 [6w, 9m),
// m-b2 [9m, 18m); one dose each. Birth 2023-09-01, r1 2025-05-01 (20
// months), today 2026-02-01: no bracket matches and C3 picks m-b2, whose
// 18m start limit r1 fails; m-b holds 20 months with its limit 2y → ✅
// COMPLETE. With today's groupMax: DOSE_AFTER_MAX_AGE.
func TestVersionGroupMaxOfItsSegment(t *testing.T) {
	t.Parallel()
	one := func(id string) ruleset.Series {
		return qSeries(t, id, "", "", "", []ruleset.DoseRule{rule(t, "P6W", "", "", ruleset.FromBirth, "P2M")})
	}
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, one("m-a"), "", "2026-01-01", "P6W", "P9M"),
		ws(t, one("m-b"), "", "2026-01-01", "P9M", "P2Y"),
		ws(t, one("m-a2"), "2026-01-01", "", "P6W", "P9M"),
		ws(t, one("m-b2"), "2026-01-01", "", "P9M", "P18M"),
	))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2023-09-01"), qDose("r1", "2025-05-01"))
	checkAssessment(t, got, completeWith([]string{"m-b2"}, false))
}

// TestVersionBetterOfPartsAndWhole (In scope 3): p-a (until 2025-11-12) and
// p-b (from then) have identical rules: maxAgeComplete 8m, dose 2 at 6m.
// Birth 2025-03-10 (8m = 11-10), r1 2025-05-15 (+6m = 11-15), r2 in
// 2025-11, today 2026-02-01. On the whole range r2 is possibly invalid by
// both checks, as with one windowless series → ⚪
// [DOSE_POSSIBLY_INVALID{minInterval, maxAgeComplete, r2, 2}, COMPLETE].
// The parts alone are definite ([11-01, 11-11] fails 11-15, [11-12, 11-30]
// the 11-10 limit) and would give DOSE_INVALID.
func TestVersionBetterOfPartsAndWhole(t *testing.T) {
	t.Parallel()
	pp := func(id string) ruleset.Series {
		return qSeries(t, id, "", "", "P8M", []ruleset.DoseRule{
			rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
			rule(t, "", "P6M", "", ruleset.FromBirth, "P7M"),
		})
	}
	e := newEngine(t, oneDiseaseSnapshot(nil, ws(t, pp("p-a"), "", "2025-11-12", "", ""), ws(t, pp("p-b"), "2025-11-12", "", "", "")))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-03-10"), qDose("r1", "2025-05-15"), qDose("r2", "2025-11"))
	checkAssessment(t, got, unknownWith([]string{"p-b"}, true,
		possiblyInvalid("r2", 2, engine.CheckMinInterval, engine.CheckMaxAgeComplete), complete))
}

// TestVersionsNotMonotoneOverWalk (In scope 3): e-2025 (until 2026-01-01)
// with dose 1 minAge 4w and e-2026 (from then) with 6w, both dose 2 at 4w.
// Birth 2025-09-01, r1 2025-10-06, r2 2025-10-20, today 2026-02-01. r1
// counts under e-2025's 4w, so r2 is dose 2, 2 weeks later →
// DOSE_INVALID{minInterval, r2, 2}. With e-2026 alone r1 fails 6w and r2
// counts as dose 1 → DOSE_INVALID{minAge, r1, 1}.
func TestVersionsNotMonotoneOverWalk(t *testing.T) {
	t.Parallel()
	e2025 := ws(t, twoDose(t, "e-2025", "", "P4W", "P4W"), "", "2026-01-01", "", "")
	e2026 := ws(t, twoDose(t, "e-2026", "", "P6W", "P4W"), "2026-01-01", "", "", "")
	birth := dayDate("2025-09-01")
	doses := []engine.DoseRecord{qDose("r1", "2025-10-06"), qDose("r2", "2025-10-20")}
	used := []string{"e-2026"}
	checkAssessment(t, assessmentOf(t, newEngine(t, oneDiseaseSnapshot(nil, e2025, e2026)), "q", "2026-02-01", birth, doses...),
		unknownWith(used, false, invalidDose("r2", 2, engine.CheckMinInterval)))
	checkAssessment(t, assessmentOf(t, newEngine(t, oneDiseaseSnapshot(nil, e2026)), "q", "2026-02-01", birth, doses...),
		unknownWith(used, false, invalidDose("r1", 1, engine.CheckMinAge)))
}

// TestVersionReachInItsSegment (Q95): l-old [9m, —) with no window, dose 2
// at 4w; l-new [6w, 9m) from 2026-01-01, dose 2 at 8w. Birth 2024-06-01, r1
// 2025-01-01 (7 months), r2 2025-02-05, today 2026-02-01: in 2025 l-old was
// alone in its group and held every age (C8), so its 4w counts r2 → ✅,
// seriesUsed l-new. With [min, max) as written: DOSE_INVALID.
func TestVersionReachInItsSegment(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "l-old", "", "P6W", "P4W"), "", "", "P9M", ""),
		ws(t, twoDose(t, "l-new", "", "P6W", "P8W"), "2026-01-01", "", "P6W", "P9M"),
	))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2024-06-01"), qDose("r1", "2025-01-01"), qDose("r2", "2025-02-05"))
	checkAssessment(t, got, completeWith([]string{"l-new"}, false))
}

// TestVersionInBracketChoice (In scope 3): the start-age bracket choice
// walks with the versions. o-1 (no bracket, until 2026-01-01): dose 1 minAge
// 4w, dose 2 at 4w. n-a [6w, 9m) and n-b [9m, —) from 2026-01-01, both
// dose 1 minAge 6w and dose 2 at 8w. Birth 2025-01-01, r1 2025-02-05 (5
// weeks, valid only under o-1), r2 2025-10-10, today 2026-02-01: r1 is dose
// 1's record, so n-a's bracket holds it → ✅, seriesUsed n-a. Without
// versions the walk reads r2 as dose 1 and picks n-b.
func TestVersionInBracketChoice(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "o-1", "", "P4W", "P4W"), "", "2026-01-01", "", ""),
		ws(t, twoDose(t, "n-a", "", "P6W", "P8W"), "2026-01-01", "", "P6W", "P9M"),
		ws(t, twoDose(t, "n-b", "", "P6W", "P8W"), "2026-01-01", "", "P9M", ""),
	))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-01-01"), qDose("r1", "2025-02-05"), qDose("r2", "2025-10-10"))
	checkAssessment(t, got, completeWith([]string{"n-a"}, false))
}

// TestVersionEndsGapSearch (#35): vb-old (until 2025-09-01) and vb-new (from
// then, maxAgeStart 4m): dose 1 minAge 6w, doses 2–3 at 4w. Birth
// 2025-01-01, p 2025-08-25, u 2025-09-05, s level 4 #3, today 2025-09-20.
// Only vb-old counts p (past 4m), so p ends the gap search's scan: u needs
// p + 4w → DOSE_INVALID{minInterval, u, 2}, and s, chained from p through
// dose 2 (10-20 > u), fits no gap → DOSE_INVALID{minInterval, s, 3}. With
// today's c alone gap 0 would fit (s ≥ 04-09 < p).
func TestVersionEndsGapSearch(t *testing.T) {
	t.Parallel()
	three := func(id, maxStart string) ruleset.Series {
		return qSeries(t, id, "", maxStart, "", []ruleset.DoseRule{
			rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
			rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
			rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		})
	}
	e := newEngine(t, oneDiseaseSnapshot(nil, ws(t, three("vb-old", ""), "", "2025-09-01", "", ""), ws(t, three("vb-new", "P4M"), "2025-09-01", "", "", "")))
	got := assessmentOf(t, e, "q", "2025-09-20", dayDate("2025-01-01"), qDose("p", "2025-08-25"), qDose("u", "2025-09-05"), numbered(qDose("s", ""), 3))
	checkAssessment(t, got, unknownWith([]string{"vb-new"}, true, invalidDose("u", 2, engine.CheckMinInterval), invalidDose("s", 3, engine.CheckMinInterval)))
}

// TestVersionTiers (Q94): products p and q; the *-2025 series end
// 2026-01-01 and the *-2026 series start then. Birth 2025-09-01, today
// 2026-02-01. Dose 1 minAge 6w unless stated.
func TestVersionTiers(t *testing.T) {
	t.Parallel()
	birth := dayDate("2025-09-01")
	old := func(sr ruleset.Series) ruleset.Series { return ws(t, sr, "", "2026-01-01", "", "") }
	cur := func(sr ruleset.Series) ruleset.Series { return ws(t, sr, "2026-01-01", "", "", "") }
	p2025, q2025, q2026 := old(twoDose(t, "p-2025", "p", "P6W", "P4W")), old(twoDose(t, "q-2025", "q", "P8W", "P4W")), cur(twoDose(t, "q-2026", "q", "P8W", "P8W"))
	pq := []string{"p", "q"}
	t.Run("expiry", func(t *testing.T) {
		t.Parallel()
		// p's own series expired; today's c for a p dose is the generic
		// g-2026, and p-2025 is in p's tier on the dose date.
		e := newEngine(t, oneDiseaseSnapshot(pq, p2025, q2025, q2026, cur(twoDose(t, "g-2026", "", "P8W", "P8W"))))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-11-01", "p"), pqDose("r2", "2025-12-01", "p")),
			completeWith([]string{"g-2026"}, false))
		// Under q-2026 the p dose r1 (7 weeks) meets q's rules only (S3a);
		// under g-2026 it meets p-2025's 6w, and the q dose r2 then meets
		// g-2026 only (q-2026 is q's tier today).
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-10-20", "p"), pqDose("r2", "2025-11-20", "q")),
			unknownWith([]string{"q-2026", "g-2026"}, false, safest("q", "q-2026", "", "g-2026"),
				invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 2, engine.CheckMinInterval)))
	})
	t.Run("fall-through", func(t *testing.T) {
		t.Parallel()
		// No generic series: p falls through to every series today, which
		// no version crosses groups through.
		e := newEngine(t, oneDiseaseSnapshot(pq, p2025, q2025, q2026))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-10-20", "p"), pqDose("r2", "2025-11-20", "q")),
			unknownWith([]string{"q-2026"}, false, invalidDose("r1", 1, engine.CheckMinAge)))
	})
	t.Run("introduction", func(t *testing.T) {
		t.Parallel()
		// A p dose given while the generic series covered p meets it under
		// p's new series.
		e := newEngine(t, oneDiseaseSnapshot(pq, old(twoDose(t, "g-2025", "", "P6W", "P4W")), cur(twoDose(t, "p-2026", "p", "P6W", "P8W"))))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-11-01", "p"), pqDose("r2", "2025-12-01", "p")),
			completeWith([]string{"p-2026"}, false))
	})
	t.Run("own groups", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot(pq, p2025, cur(twoDose(t, "p-2026", "p", "P6W", "P8W")),
			old(twoDose(t, "q-2025", "q", "P6W", "P3W")), cur(twoDose(t, "q-2026", "q", "P6W", "P8W"))))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-11-01", "q"), pqDose("r2", "2025-11-26", "q")),
			completeWith([]string{"q-2026"}, false))
		both := []string{"p-2026", "q-2026"}
		want := unknownWith(both, false, safest("p", "p-2026", "q", "q-2026"), invalidDose("r2", 2, engine.CheckMinInterval))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-11-01", "p"), pqDose("r2", "2025-12-01", "q")), want)
		// No product: every series' tier, so each version meets its own
		// group only; q-2025's 3w is not applied under p-2026.
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", birth, pqDose("r1", "2025-11-01", ""), pqDose("r2", "2025-11-26", "")), want)
	})
}

// TestEnvelopeInGapZero (Q105 (b)): w-old (until 2025-09-01) and w-new (from
// then): maxAgeStart 2m, dose 1 minAge 6w, dose 2 at 4w / 8w. Birth
// 2025-06-01 (6w = 07-13, 2m = 08-01), r1 2025-08-20 (past 2m under both:
// DOSE_AFTER_MAX_AGE{maxAgeStart, r1, 1}), u #2 undated, today 2025-10-01.
// Under w-new u needs r1 + 8w = 10-15 and gap 0 needs 07-13 + 8w = 09-07 >
// r1. There is no P, so the envelope reads the segments from birth; with
// w-old's 4w gap 0 fits (08-10 < r1) → ⚪ [DOSE_AFTER_MAX_AGE{r1},
// DOSE_POSSIBLY_INVALID{minInterval, u, 2}, COMPLETE], est. Under Q105 (a)
// u is DOSE_INVALID, though u dated 08-10 gives ✅.
func TestEnvelopeInGapZero(t *testing.T) {
	t.Parallel()
	w := func(id, interval string) ruleset.Series {
		sr := twoDose(t, id, "", "P6W", interval)
		sr.MaxAgeStart = spanOf(t, "P2M")
		return sr
	}
	e := newEngine(t, oneDiseaseSnapshot(nil, ws(t, w("w-old", "P4W"), "", "2025-09-01", "", ""), ws(t, w("w-new", "P8W"), "2025-09-01", "", "", "")))
	got := assessmentOf(t, e, "q", "2025-10-01", dayDate("2025-06-01"), qDose("r1", "2025-08-20"), numbered(qDose("u", ""), 2))
	checkAssessment(t, got, unknownWith([]string{"w-new"}, true,
		afterMaxAge("r1", 1, engine.CheckMaxAgeStart), possiblyInvalid("u", 2, engine.CheckMinInterval), complete))
}

// TestEnvelopeReadsSegmentsFromP (Q105 (b)): br-a [6w, 9m) from 2024-01-01,
// br-b [9m, —) from 2025-06-01, dose 2 at 4w / 8w. Birth 2024-06-01, r1
// 2025-07-01 (13 months: br-b), u #2 undated, today 2025-08-10: u needs r1
// + 8w = 08-26. P is r1, and br-a's lone segment [2024-01-01, 2025-06-01)
// ends before it, so the envelope adds nothing → ⚪ [DOSE_INVALID{minInterval,
// u, 2}], est, as without windows. Read from birth, br-a (C8 there) would
// demote u, although u ≥ r1 can never lie in that segment.
func TestEnvelopeReadsSegmentsFromP(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "br-a", "", "P6W", "P4W"), "2024-01-01", "", "P6W", "P9M"),
		ws(t, twoDose(t, "br-b", "", "P6W", "P8W"), "2025-06-01", "", "P9M", ""),
	))
	got := assessmentOf(t, e, "q", "2025-08-10", dayDate("2024-06-01"), qDose("r1", "2025-07-01"), numbered(qDose("u", ""), 2))
	checkAssessment(t, got, unknownWith([]string{"br-b"}, true, invalidDose("u", 2, engine.CheckMinInterval)))
}

// TestIdenticalCopyInBracketedGroup (Q106, Q109): an identical re-issue of
// one series of a bracketed group changes nothing. On
// minimal-start-age-gap-late (gap-a [1y, 2y) dose 2 at 8w, gap-b [3y, 5y)
// dose 2 at 4w, gap-c [5y, —), dose 1 minAge 1y in each), birth
// 2023-08-06, r1 2023-09-20 (below every minAge), r4 #2 2023-10-29, today
// 2024-03-09, both give the windowless output: r1 fails its minimum age,
// and r4 fails gap-a's 8 weeks from r1 (11-15): ⚪ [DOSE_INVALID{minAge,
// r1, 1}, DOSE_INVALID{minInterval, r4, 2}], seriesUsed [gap-a], then
// [gap-a-copy] once gap-a ends on 2024-02-25 and an identical gap-a-copy
// starts then.
//
// Two rules each keep gap-b, in effect in the older segment, from counting
// r4 under gap-a-copy. With no dose 1 counted, dose 1 lies in [birth, r4]
// = [2023-08-06, 2023-10-29] (Q106), under 3 months old, which gap-b's
// reach there, [2y, 5y), cannot hold. And gap-b has a twin in effect today,
// itself, so it is not a version at all (Q109).
func TestIdenticalCopyInBracketedGroup(t *testing.T) {
	t.Parallel()
	base := loadSnapshot(t, "minimal-start-age-gap-late")
	copied := identicalCopy(base, slices.IndexFunc(base.Series, func(sr ruleset.Series) bool { return sr.ID == "gap-a" }), mustDate(t, "2024-02-25"))
	doses := []engine.DoseRecord{doseOf("gap", "r1", dayDate("2023-09-20"), ""), numbered(doseOf("gap", "r4", dayDate("2023-10-29"), ""), 2)}
	want := []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r4", 2, engine.CheckMinInterval)}
	for _, c := range []struct {
		name   string
		snap   *ruleset.Snapshot
		series string
	}{
		{"windowless", base, "gap-a"},
		{"identical copy", copied, "gap-a-copy"},
	} {
		a := assessmentOf(t, newEngine(t, c.snap, warnCodes(c.snap)...), "gap", "2024-03-09", dayDate("2023-08-06"), doses...)
		if a.Bucket != engine.BucketUnknown || !slices.Equal(a.SeriesUsed, []string{c.series}) || !reflect.DeepEqual(a.Reasons, want) {
			t.Errorf("%s: %+v\nwant ⚪ seriesUsed [%s] reasons %v", c.name, a, c.series, want)
		}
	}
}

// TestVersionPassesAtOneBirthEnd (Q107): a version that passes a dose at
// one birth end only counts like a possible match. x-old (until
// 2026-01-01) has dose 1 minAge P4W, x-new (from then) P6W, each dose 2 at
// 4w due previous + 4w. r1 2025-10-06, today 2026-02-01:
//   - birth 2025-09: today's x-new fails r1 at both ends (6w from 09-01 =
//     10-13, from 09-30 = 11-11). x-old passes it at b_old 09-01 (09-29) and
//     fails it at b_late 09-30 (10-28), so it softens the failure: at both
//     step-4 ends ⚪ [DOSE_POSSIBLY_INVALID{minAge, r1, 1}], est,
//     seriesUsed [x-new]. Judged by its worse end it would leave a definite
//     DOSE_INVALID that birth 2025-09-01 contradicts;
//   - birth 2025-09-01: x-old counts r1, the rule in effect on its date, so
//     dose 2 is due 4 weeks later: 🔴 {2, 2025-11-03, 2025-11-03,
//     2025-11-10}.
func TestVersionPassesAtOneBirthEnd(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "x-old", "", "P4W", "P4W"), "", "2026-01-01", "", ""),
		ws(t, twoDose(t, "x-new", "", "P6W", "P4W"), "2026-01-01", "", "", "")))
	used := []string{"x-new"}
	r1 := qDose("r1", "2025-10-06")
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", monthDate("2025-09"), r1),
		unknownWith(used, true, possiblyInvalid("r1", 1, engine.CheckMinAge)))
	checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01"), r1), engine.Assessment{
		Disease: "q", Bucket: engine.BucketOverdue, NextDose: nextDose(t, 2, "2025-11-03", "2025-11-03", "2025-11-10"), SeriesUsed: used,
	})
}

// TestVersionNoDoseOneCounted (Q106): with no dose 1 counted, dose 1's
// range for the bracket match is [b_old, R.Hi of the dose being judged].
// Birth 2024-05-15, r1 2024-06-01 (below 6 weeks in every series:
// DOSE_INVALID{minAge, r1, 1}), r2 #2 2024-07-10, today 2026-02-01. Dose 1
// lies in [05-15, 07-10], at most 8 weeks old.
//   - k-a [6w, 9m) with dose 2 at 4w and k-b [9m, —) at 8w until
//     2026-01-01, k-2026 (no bracket) at 8w from then. k-a is first in its
//     group, so its reach is (—, 9m): it certainly holds dose 1, and its 4
//     weeks from r1 (06-29) pass r2 → ⚪ [DOSE_INVALID{minAge, r1, 1},
//     COMPLETE], seriesUsed [k-2026]. Read as "possible", k-a would only
//     soften k-2026's failure (07-27) to DOSE_POSSIBLY_INVALID.
//   - The same with the intervals swapped (k2-a at 8w, k2-b at 4w, k2-2026
//     at 8w): k2-b's reach [9m, —) cannot hold dose 1, so it is not a
//     version, and k2-a fails r2 → ⚪ [DOSE_INVALID{minAge, r1, 1},
//     DOSE_INVALID{minInterval, r2, 2}], seriesUsed [k2-2026]. Read as
//     "possible", k2-b would soften r2 and count it.
func TestVersionNoDoseOneCounted(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		prefix, aIv, bIv string
		want             []engine.Reason
	}{
		{"k", "P4W", "P8W", []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge), complete}},
		{"k2", "P8W", "P4W", []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge), invalidDose("r2", 2, engine.CheckMinInterval)}},
	} {
		e := newEngine(t, oneDiseaseSnapshot(nil,
			ws(t, twoDose(t, c.prefix+"-a", "", "P6W", c.aIv), "", "2026-01-01", "P6W", "P9M"),
			ws(t, twoDose(t, c.prefix+"-b", "", "P6W", c.bIv), "", "2026-01-01", "P9M", ""),
			ws(t, twoDose(t, c.prefix+"-2026", "", "P6W", "P8W"), "2026-01-01", "", "", ""),
		))
		got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2024-05-15"), qDose("r1", "2024-06-01"), numbered(qDose("r2", "2024-07-10"), 2))
		checkAssessment(t, got, unknownWith([]string{c.prefix + "-2026"}, false, c.want...))
	}
}

// TestTwinIsNotAVersion (Q109): a series of another segment with a twin in
// effect today (the same product, compiled rules, reach and tiers) is not a
// version, so an identical re-issue changes nothing also where Q106 and
// Q107 alone would let the original count a dose. Each case copies one
// series from a cut (identicalCopy) and must give the windowless output,
// the copy's ID read as the original's.
//   - illustrative-max-age, rota-rotavin (alone in its group) copied from
//     2002-03-12; birth year 2002, r0 rotavin 2002-03-05, today 2002-04-01.
//     Windowless: r0 meets the 6-week minimum age at b_old 2002-01-01
//     (02-12) but not at the later end → ⚪
//     [BIRTH_DATE_IMPRECISE{birthMonth}, DOSE_INVALID{minAge, r0, 1}], est.
//     As a version, the original would pass r0 at b_old only and soften
//     the later end's definite failure to DOSE_POSSIBLY_INVALID (Q107),
//     though that failure holds for that birth date.
//   - minimal-start-age-gap-late, gap-b copied from 1993-06-04; birth year
//     1991, r0 1993-04-23, r1 1993-05-28, today 1993-06-30. Dose 1 is 1.3
//     to 2.3 years old, so gap-a and gap-b are both candidates, and r1
//     fails gap-a's 8 weeks from r0 (06-18) → ⚪
//     [AGE_AT_START_UNCERTAIN{gap-a, gap-b}, SAFEST_SERIES{gap-a, gap-b},
//     DOSE_INVALID{minInterval, r1, 2}], est. As a version of gap-a, the
//     original gap-b (reach [2y, 5y) behind gap-a) possibly holds dose 1
//     and its 4 weeks (05-21) pass r1, so it would soften r1 to
//     DOSE_POSSIBLY_INVALID and count it.
func TestTwinIsNotAVersion(t *testing.T) {
	t.Parallel()
	refs := []engine.SeriesRef{{Series: "gap-a"}, {Series: "gap-b"}}
	for _, c := range []struct {
		fixture, disease, series, cut, today string
		birth                                engine.RecalledDate
		doses                                []engine.DoseRecord
		want                                 engine.Assessment
	}{
		{"illustrative-max-age", "rota", "rota-rotavin", "2002-03-12", "2002-04-01", engine.RecalledDate{Value: "2002", Precision: engine.Year},
			[]engine.DoseRecord{doseOf("rota", "r0", dayDate("2002-03-05"), "rotavin")},
			engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotavin"}, Estimated: true, Reasons: []engine.Reason{
				{Code: engine.ReasonBirthDateImprecise, Params: engine.ReasonParams{Hint: engine.HintBirthMonth}}, invalidDose("r0", 1, engine.CheckMinAge)}}},
		{"minimal-start-age-gap-late", "gap", "gap-b", "1993-06-04", "1993-06-30", engine.RecalledDate{Value: "1991", Precision: engine.Year},
			[]engine.DoseRecord{doseOf("gap", "r0", dayDate("1993-04-23"), ""), doseOf("gap", "r1", dayDate("1993-05-28"), "")},
			engine.Assessment{Disease: "gap", Bucket: engine.BucketUnknown, SeriesUsed: []string{"gap-a", "gap-b"}, Estimated: true, Reasons: []engine.Reason{
				{Code: engine.ReasonAgeAtStartUncertain, Params: engine.ReasonParams{Candidates: refs}},
				{Code: engine.ReasonSafestSeries, Params: engine.ReasonParams{Candidates: refs}},
				invalidDose("r1", 2, engine.CheckMinInterval)}}},
	} {
		base := loadSnapshot(t, c.fixture)
		checkAssessment(t, assessmentOf(t, newEngine(t, base, warnCodes(base)...), c.disease, c.today, c.birth, c.doses...), c.want)
		copied := identicalCopy(base, slices.IndexFunc(base.Series, func(sr ruleset.Series) bool { return sr.ID == c.series }), mustDate(t, c.cut))
		got := assessmentOf(t, newEngine(t, copied, warnCodes(copied)...), c.disease, c.today, c.birth, c.doses...)
		readCopyAsOriginal(&got, c.series+"-copy", c.series)
		checkAssessment(t, got, c.want)
	}
}

// TestVersionPassesBetweenBirthEnds (Q107): a version that fails a dose at
// both birth ends, its minimum age at b_late and its maximum age at b_old,
// still passes it for the birth dates between, so it counts like a
// possible match: judged with its minimums at b_old and its maximum ages at
// b_late, it passes. One dose each, today 2026-02-01:
//   - g-old minAge 9m, maxAgeComplete 12m, due 9m, until 2026-01-01; g-new
//     minAge 2y, due 2y, from then. r1 2025-07-15. Birth year 2024: g-new
//     fails r1 at both ends (18.5 and 6.5 months old); g-old fails its 12
//     months at 2024-01-01 and its 9 months at 2024-12-31, so it softens
//     the failure → ⚪ [DOSE_POSSIBLY_INVALID{minAge, r1, 1}, COMPLETE],
//     est, seriesUsed [g-new]. Birth 2024-09-01 (10.5 months at r1): g-old
//     counts r1 → ✅ COMPLETE.
//   - h-old minAge 38w, maxAgeComplete 40w until 2026-01-01; h-new minAge
//     1y from then. r1 2025-10-15. Birth month 2025-01: h-old fails its 40
//     weeks at 01-01 (41 weeks old) and its 38 weeks at 01-31 (36 weeks) →
//     ⚪ [DOSE_POSSIBLY_INVALID{minAge, r1, 1}, COMPLETE], est. Birth
//     2025-01-15 (39 weeks) → ✅ COMPLETE.
func TestVersionPassesBetweenBirthEnds(t *testing.T) {
	t.Parallel()
	one := func(id, minAge, complete, due string) ruleset.Series {
		return qSeries(t, id, "", "", complete, []ruleset.DoseRule{rule(t, minAge, "", "", ruleset.FromBirth, due)})
	}
	for _, c := range []struct {
		prefix, oldMin, oldMax, newMin, r1 string
		wide, narrow                       engine.RecalledDate
	}{
		{"g", "P9M", "P12M", "P2Y", "2025-07-15", engine.RecalledDate{Value: "2024", Precision: engine.Year}, dayDate("2024-09-01")},
		{"h", "P38W", "P40W", "P1Y", "2025-10-15", monthDate("2025-01"), dayDate("2025-01-15")},
	} {
		e := newEngine(t, oneDiseaseSnapshot(nil,
			ws(t, one(c.prefix+"-old", c.oldMin, c.oldMax, c.oldMin), "", "2026-01-01", "", ""),
			ws(t, one(c.prefix+"-new", c.newMin, "", c.newMin), "2026-01-01", "", "", "")))
		used := []string{c.prefix + "-new"}
		r1 := qDose("r1", c.r1)
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", c.wide, r1), unknownWith(used, true, possiblyInvalid("r1", 1, engine.CheckMinAge), complete))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-02-01", c.narrow, r1), completeWith(used, false))
	}
}

// TestTwinThroughCatalogChange (Q109): a series in effect, unchanged, in
// both segments is its own twin even when another product's series starts
// or ends inside the history and so changes the tiers of products other
// than the dose's (a product with no series in effect falls through to
// every series). illustrative-max-age with rota-rotateq starting, or
// ending, on 2002-03-12; birth year 2002, r0 rotavin 2002-03-05, today
// 2002-04-01, no rotateq dose. Both give the windowless output: r0 meets
// the 6-week minimum age at b_old 2002-01-01 (02-12) but not at the later
// end → ⚪ [BIRTH_DATE_IMPRECISE{birthMonth}, DOSE_INVALID{minAge, r0, 1}],
// est. As a version of itself, rota-rotavin would pass r0 at b_old and
// soften the later end's failure to DOSE_POSSIBLY_INVALID (Q107).
func TestTwinThroughCatalogChange(t *testing.T) {
	t.Parallel()
	base := loadSnapshot(t, "illustrative-max-age")
	rotateq := slices.IndexFunc(base.Series, func(sr ruleset.Series) bool { return sr.ID == "rota-rotateq" })
	want := engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotavin"}, Estimated: true, Reasons: []engine.Reason{
		{Code: engine.ReasonBirthDateImprecise, Params: engine.ReasonParams{Hint: engine.HintBirthMonth}}, invalidDose("r0", 1, engine.CheckMinAge)}}
	cut := mustDate(t, "2002-03-12")
	for _, c := range []struct {
		name string
		edit func(sr *ruleset.Series)
	}{
		{"windowless", func(*ruleset.Series) {}},
		{"rotateq starts", func(sr *ruleset.Series) { sr.ValidFrom = rdate(cut) }},
		{"rotateq ends", func(sr *ruleset.Series) { sr.ValidTo = rdate(cut) }},
	} {
		s := cloneSnapshot(base)
		c.edit(&s.Series[rotateq])
		got := assessmentOf(t, newEngine(t, s, warnCodes(s)...), "rota", "2002-04-01", engine.RecalledDate{Value: "2002", Precision: engine.Year},
			doseOf("rota", "r0", dayDate("2002-03-05"), "rotavin"))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %+v\nwant %+v", c.name, got, want)
		}
	}
}

// TestNearTwinIsAVersion (Q109): a series of another segment that differs
// from a series in effect today in its reach, its lone flag or the tier of
// the dose's product is not that series' twin, so it stays a version and
// counts the dose its rule counted then. Today 2026-02-01, two doses each:
//   - bracket: y-a [6w, 1y) with dose 2 at 8w and y-b [1y, —) at 4w until
//     2026-01-01; then y-a2 [6w, 2y) at 8w and y-b2 [2y, —) at 4w, the plans
//     unchanged and only the brackets moved. Birth 2024-01-01, r1
//     2025-03-01 (14 months), r2 2025-04-01. Today r1 lies in y-a2's
//     bracket, whose 8 weeks r2 fails, but y-b's reach then, [1y, —),
//     certainly held dose 1 and its 4 weeks (03-29) pass r2 → ✅ COMPLETE,
//     seriesUsed [y-a2]. Read as y-b2's twin, y-b would leave
//     DOSE_INVALID{minInterval, r2, 2};
//   - lone: l-b [1y, —) at 4w, in effect throughout, was alone in its group
//     until l-a [6w, 1y) at 8w started on 2026-01-01, so before that it held
//     every age (C8). Birth 2025-06-01, r1 2025-09-01 (3 months), r2
//     2025-10-01: today's l-a fails r2 (8 weeks = 10-27), l-b as it was
//     passes it (09-29) → ✅ COMPLETE, seriesUsed [l-a];
//   - tier: the generic g (dose 2 at 4w) is in effect throughout and held
//     product p's tier until p-2026 (8w) started on 2026-01-01. Birth
//     2025-09-01, p doses r1 2025-11-01 and r2 2025-12-01: today's c for a p
//     dose is p-2026, whose 8 weeks r2 fails, and g, in p's tier on the
//     dose dates but not today, passes it → ✅ COMPLETE, seriesUsed
//     [p-2026], as when g ends where p-2026 starts (TestVersionTiers).
func TestNearTwinIsAVersion(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		products []string
		series   []ruleset.Series
		birth    string
		doses    []engine.DoseRecord
		used     string
	}{
		{"bracket", nil, []ruleset.Series{
			ws(t, twoDose(t, "y-a", "", "P6W", "P8W"), "", "2026-01-01", "P6W", "P1Y"),
			ws(t, twoDose(t, "y-b", "", "P6W", "P4W"), "", "2026-01-01", "P1Y", ""),
			ws(t, twoDose(t, "y-a2", "", "P6W", "P8W"), "2026-01-01", "", "P6W", "P2Y"),
			ws(t, twoDose(t, "y-b2", "", "P6W", "P4W"), "2026-01-01", "", "P2Y", ""),
		}, "2024-01-01", []engine.DoseRecord{qDose("r1", "2025-03-01"), qDose("r2", "2025-04-01")}, "y-a2"},
		{"lone", nil, []ruleset.Series{
			ws(t, twoDose(t, "l-a", "", "P6W", "P8W"), "2026-01-01", "", "P6W", "P1Y"),
			ws(t, twoDose(t, "l-b", "", "P6W", "P4W"), "", "", "P1Y", ""),
		}, "2025-06-01", []engine.DoseRecord{qDose("r1", "2025-09-01"), qDose("r2", "2025-10-01")}, "l-a"},
		{"tier", []string{"p"}, []ruleset.Series{
			twoDose(t, "g", "", "P6W", "P4W"),
			ws(t, twoDose(t, "p-2026", "p", "P6W", "P8W"), "2026-01-01", "", "", ""),
		}, "2025-09-01", []engine.DoseRecord{pqDose("r1", "2025-11-01", "p"), pqDose("r2", "2025-12-01", "p")}, "p-2026"},
	} {
		s := oneDiseaseSnapshot(c.products, c.series...)
		got := assessmentOf(t, newEngine(t, s, warnCodes(s)...), "q", "2026-02-01", dayDate(c.birth), c.doses...)
		if want := completeWith([]string{c.used}, false); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %+v\nwant %+v", c.name, got, want)
		}
	}
}

// TestVersionNoDoseOneReadsWholeRange (Q106): with no dose 1 counted, dose
// 1's range for the bracket match ends at R.Hi of the whole dose being
// judged, not at the end of the part. k3-a [6w, 9m) with dose 2 at 4w and
// k3-b [9m, —) at 9m until 2025-02-10, k3-2026 (no bracket) at 8m from
// then. Birth 2024-05-15 (9 months = 2025-02-15), r1 2024-06-10 (below 6
// weeks: DOSE_INVALID{minAge, r1, 1}), r2 #2 in 2025-02, today 2026-02-01.
// r2's range [02-01, 02-28] is cut at 02-10: on [02-10, 02-28] k3-2026's 8
// months from r1 (02-10) pass. On [02-01, 02-09] k3-2026 fails, and dose 1
// lies in [birth, 02-28], which may pass 9 months, so k3-a only possibly
// holds it: its 4 weeks soften the part, and r2 is ⚪
// [DOSE_INVALID{minAge, r1, 1}, DOSE_POSSIBLY_INVALID{minInterval, r2, 2},
// COMPLETE], est. Ending dose 1's range at the part (02-09) would match
// k3-a certainly and count r2.
func TestVersionNoDoseOneReadsWholeRange(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "k3-a", "", "P6W", "P4W"), "", "2025-02-10", "P6W", "P9M"),
		ws(t, twoDose(t, "k3-b", "", "P6W", "P9M"), "", "2025-02-10", "P9M", ""),
		ws(t, twoDose(t, "k3-2026", "", "P6W", "P8M"), "2025-02-10", "", "", ""),
	))
	got := assessmentOf(t, e, "q", "2026-02-01", dayDate("2024-05-15"), qDose("r1", "2024-06-10"), numbered(qDose("r2", "2025-02"), 2))
	checkAssessment(t, got, unknownWith([]string{"k3-2026"}, true,
		invalidDose("r1", 1, engine.CheckMinAge), possiblyInvalid("r2", 2, engine.CheckMinInterval), complete))
}

// oneDose is a one-dose series of disease q with dose-1 minAge minAge;
// product "" is generic.
func oneDose(t testing.TB, id, product, minAge string) ruleset.Series {
	t.Helper()
	return qSeries(t, id, product, "", "", []ruleset.DoseRule{rule(t, minAge, "", "", ruleset.FromBirth, "P2Y")})
}

// threeDose is a series of disease q: dose 1 minAge minAge due birth + 2m,
// doses 2 and 3 minInterval interval due previous + interval; product ""
// is generic.
func threeDose(t testing.TB, id, product, minAge, interval string) ruleset.Series {
	t.Helper()
	next := rule(t, "", interval, "", ruleset.FromPreviousDose, interval)
	return qSeries(t, id, product, "", "", []ruleset.DoseRule{rule(t, minAge, "", "", ruleset.FromBirth, "P2M"), next, next})
}

// TestUnknownProductVersionsEveryTier (P1, SAD §5.7; issue #39, Q94, Q116,
// Q117): a dose with no product could be any catalog product, so a rule
// version of c's group counts it only when it would count a dose of every
// catalog product (the version is in each one's tier on the dose date),
// and a cross-group version never does. Each case but the last gives the
// same result with the first dose named and with its product removed.
// Disease q, birth 2016-06-26, today 2026-03-07 unless stated.
//   - Brackets, two doses: catalog px; generic gold [2y, 5y) (one dose,
//     minAge 6m) until 2020-01-01, generic gnew [6m, 2y) (6m, then 4w),
//     px's xa [6m, 3y) (6m, then 8w). r0 px 2018-07-20 (2y 24d), r2 with
//     no product 2022-09-25, so the candidates are gnew and xa. Under xa,
//     r0 is dose 1 and r2 dose 2: complete. Under gnew, r0 is past the
//     group's last max (2y); gold is not in px's tier in 2018 (px has xa),
//     so it does not count a px dose, nor a dose with no product (not in
//     every tier). Both doses are past the limit, gnew is out of age, and
//     the disagreement gives CHECK_PRODUCT_NAME (Q52) → ⚪
//     [SAFEST_SERIES{gnew, px xa}, CHECK_PRODUCT_NAME]. Main counted r0
//     with no product through gold → ✅. The same with gold ending on
//     2018-07-01, before r0 (a control, ⚪ on main too).
//   - One dose, no brackets: catalog q; generic gold (6m) and q's qold (12m)
//     until 2020-01-01, generic gnew (12m) from then. r0 2017-03-01 (8m
//     3d). Today q takes the generic tier, so c is gnew. For a q dose,
//     gold is not in q's tier in 2017 ({qold}) and qold (cross-group, gnew
//     in q's tier today) fails 12m; for a dose with no product, gold is not
//     in every tier and qold is cross-group → ⚪ [DOSE_INVALID{minAge, r0,
//     1}]. Main counted r0 with no product through gold → ✅.
//   - Product group: catalog p, q; p's p-old (6m, then 4w) until
//     2020-01-01 and p-new (12m, then 4w) from then; q's qa (6m, then 4w).
//     r0 q 2017-03-01, r2 with no product 2017-05-01 (10m 5d). Candidates
//     p-new and qa. Under qa r0 and r2 (61 days later) count. Under p-new,
//     p-old is in neither q's tier in 2017 nor every tier, so r0 and then
//     r2 fail 12m at position 1 → ⚪ [SAFEST_SERIES{p p-new, q qa},
//     DOSE_INVALID{minAge, r0, 1}, DOSE_INVALID{minAge, r2, 1}]. Main
//     counted r2 through p-old, and r0 too with no product → ✅.
//   - Envelope (Q105 (b)): TestEnvelopeInGapZero's w-old and w-new with px's
//     xa (6w, then 8w, no window) beside them; catalog px, birth
//     2025-06-01, today 2025-10-01; u px #2 undated, r1 with no product
//     2025-08-20. Under xa, r1 is dose 1 and u needs r1 + 8w = 10-15,
//     after today. Under w-new, r1 is past 2m and gap 0 needs 07-13 + 8w =
//     09-07, after r1. w-old is not in px's tier, nor in every tier, so the
//     envelope adds nothing for u; r1's DOSE_AFTER_MAX_AGE is dropped as a
//     disagreement (Q52) → ⚪ [SAFEST_SERIES{w-new, px xa},
//     DOSE_INVALID{minInterval, u, 2}, CHECK_PRODUCT_NAME], est. Main gave
//     u with no product DOSE_POSSIBLY_INVALID as well, from w-old's 4w.
//   - Q94 kept, ✅ [COMPLETE] each: the one-dose case without qold (q has
//     the generic tier in 2017, so gold is in every tier and its 6m count
//     r0), also with no catalog product at all (vacuously every tier); and
//     with no generic series, catalog p, q, p-old (6m) until 2020-01-01 and
//     p-new (12m) from then: q falls through to every series in both
//     segments, so p-old is in p's tier and q's and counts r0 for p, q and
//     no product alike.
//   - Stricter than a product: catalog q; q's qold (6m) until 2020-01-01,
//     generic gnew (12m) from then, no generic series before. For a q dose
//     qold is a cross-group version (gnew in q's tier today) → ✅
//     [COMPLETE]; a dose with no product never meets a cross-group version
//     → ⚪ [DOSE_INVALID{minAge, r0, 1}] (Q116 is stricter than any one
//     product here, by design).
func TestUnknownProductVersionsEveryTier(t *testing.T) {
	t.Parallel()
	birth := dayDate("2016-06-26")
	cpn := engine.Reason{Code: engine.ReasonCheckProductName}
	// both checks want with the first dose of doses as given and with its
	// product removed.
	both := func(t *testing.T, e *engine.Engine, today string, birth engine.RecalledDate, want engine.Assessment, doses ...engine.DoseRecord) {
		t.Helper()
		for _, named := range []bool{true, false} {
			ds := slices.Clone(doses)
			if !named {
				ds[0].Product = nil
			}
			checkAssessment(t, assessmentOf(t, e, "q", today, birth, ds...), want)
		}
	}
	t.Run("brackets, two doses", func(t *testing.T) {
		t.Parallel()
		gold := ws(t, oneDose(t, "gold", "", "P6M"), "", "2020-01-01", "P2Y", "P5Y")
		gnew := ws(t, twoDose(t, "gnew", "", "P6M", "P4W"), "", "", "P6M", "P2Y")
		xa := ws(t, twoDose(t, "xa", "px", "P6M", "P8W"), "", "", "P6M", "P3Y")
		want := unknownWith([]string{"gnew", "xa"}, false, safest("", "gnew", "px", "xa"), cpn)
		doses := []engine.DoseRecord{pqDose("r0", "2018-07-20", "px"), pqDose("r2", "2022-09-25", "")}
		both(t, newEngine(t, oneDiseaseSnapshot([]string{"px"}, gold, gnew, xa)), "2026-03-07", birth, want, doses...)
		gold.ValidTo = &ruleset.Date{Year: 2018, Month: 7, Day: 1}
		both(t, newEngine(t, oneDiseaseSnapshot([]string{"px"}, gold, gnew, xa)), "2026-03-07", birth, want, doses...)
	})
	t.Run("one dose, no brackets", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot([]string{"q"},
			ws(t, oneDose(t, "gold", "", "P6M"), "", "2020-01-01", "", ""),
			ws(t, oneDose(t, "gnew", "", "P12M"), "2020-01-01", "", "", ""),
			ws(t, oneDose(t, "qold", "q", "P12M"), "", "2020-01-01", "", "")))
		both(t, e, "2026-03-07", birth, unknownWith([]string{"gnew"}, false, invalidDose("r0", 1, engine.CheckMinAge)),
			pqDose("r0", "2017-03-01", "q"))
	})
	t.Run("product group", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot([]string{"p", "q"},
			ws(t, twoDose(t, "p-old", "p", "P6M", "P4W"), "", "2020-01-01", "", ""),
			ws(t, twoDose(t, "p-new", "p", "P12M", "P4W"), "2020-01-01", "", "", ""),
			twoDose(t, "qa", "q", "P6M", "P4W")))
		both(t, e, "2026-03-07", birth, unknownWith([]string{"p-new", "qa"}, false, safest("p", "p-new", "q", "qa"),
			invalidDose("r0", 1, engine.CheckMinAge), invalidDose("r2", 1, engine.CheckMinAge)),
			pqDose("r0", "2017-03-01", "q"), pqDose("r2", "2017-05-01", ""))
	})
	t.Run("envelope", func(t *testing.T) {
		t.Parallel()
		w := func(id, interval string) ruleset.Series {
			sr := twoDose(t, id, "", "P6W", interval)
			sr.MaxAgeStart = spanOf(t, "P2M")
			return sr
		}
		e := newEngine(t, oneDiseaseSnapshot([]string{"px"}, ws(t, w("w-old", "P4W"), "", "2025-09-01", "", ""),
			ws(t, w("w-new", "P8W"), "2025-09-01", "", "", ""), twoDose(t, "xa", "px", "P6W", "P8W")))
		both(t, e, "2025-10-01", dayDate("2025-06-01"), unknownWith([]string{"w-new", "xa"}, true, safest("", "w-new", "px", "xa"),
			invalidDose("u", 2, engine.CheckMinInterval), cpn),
			numbered(doseOf("q", "u", unknownDate, "px"), 2), qDose("r1", "2025-08-20"))
	})
	t.Run("Q94 kept", func(t *testing.T) {
		t.Parallel()
		gold, gnew := ws(t, oneDose(t, "gold", "", "P6M"), "", "2020-01-01", "", ""), ws(t, oneDose(t, "gnew", "", "P12M"), "2020-01-01", "", "", "")
		both(t, newEngine(t, oneDiseaseSnapshot([]string{"q"}, gold, gnew)), "2026-03-07", birth, completeWith([]string{"gnew"}, false),
			pqDose("r0", "2017-03-01", "q"))
		both(t, newEngine(t, oneDiseaseSnapshot(nil, gold, gnew)), "2026-03-07", birth, completeWith([]string{"gnew"}, false),
			pqDose("r0", "2017-03-01", ""))
		e := newEngine(t, oneDiseaseSnapshot([]string{"p", "q"},
			ws(t, oneDose(t, "p-old", "p", "P6M"), "", "2020-01-01", "", ""), ws(t, oneDose(t, "p-new", "p", "P12M"), "2020-01-01", "", "", "")))
		for _, p := range []string{"p", "q"} {
			both(t, e, "2026-03-07", birth, completeWith([]string{"p-new"}, false), pqDose("r0", "2017-03-01", p))
		}
	})
	t.Run("stricter than a product", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t, oneDiseaseSnapshot([]string{"q"},
			ws(t, oneDose(t, "qold", "q", "P6M"), "", "2020-01-01", "", ""), ws(t, oneDose(t, "gnew", "", "P12M"), "2020-01-01", "", "", "")))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-03-07", birth, pqDose("r0", "2017-03-01", "q")), completeWith([]string{"gnew"}, false))
		checkAssessment(t, assessmentOf(t, e, "q", "2026-03-07", birth, pqDose("r0", "2017-03-01", "")),
			unknownWith([]string{"gnew"}, false, invalidDose("r0", 1, engine.CheckMinAge)))
	})
}

// TestUnknownProductBracketChoice (P1, SAD §5.7; issue #39, Q106, Q116,
// Q119): which record the bracket-choice walk counts as dose 1 decides
// which brackets match, and it moves with the versions in either
// direction. For a dose with no product the walk runs with the versions
// every catalog product's dose meets (strict, Q116) and with every version
// some catalog product's dose meets (lenient). Counting at position 1 is
// monotone in the versions, so at each birth end every reading of the dose
// counts as dose 1 a record from the lenient walk's dose 1 up to the
// strict walk's, or up to the last record the strict walk tries at
// position 1 when it counts none there; a reading that counts none at
// every end reads the first record tried, the strict walk's fallback. When
// the two walks count different records at position 1, the dose-1
// candidate is the hull of both walks' candidates (the strict walk's
// fallback included) and every record with no product or another product
// that the strict walk tries at position 1 from the lenient walk's dose 1
// on; a named record meets the same versions in every reading, so one the
// strict walk tries and does not count is no reading's dose 1 (route F).
// Every reading's dose 1 lies in it, so the dose with no product is never
// ✅ where a named reading is not. Q119
// (a) words the candidate as the hull of the two walks' dose-1 ranges,
// which misses a record the strict walk tries but does not count (route
// D); the wider candidate is an amendment of Q119 (a) for the owner to
// confirm.
//
// Disease q, catalog q (no series of its own) and px (xa: two doses, 6w
// then 4w, no window), generic g-old (three doses, 6w then 4w) until
// 2020-01-01, today 2026-03-07. From 2020-01-01 a generic bracketed pair
// whose dose-1 minAge 13m lies above the younger bracket's floor:
//   - Route A: s1 [6w, 1y) (three doses, 13m then 4w) and s2 [1y, —) (two
//     doses, 13m with tolerance 8w, then 4w). Birth 2016-06 (b_old 06-01,
//     b_late 06-30), r0 2017-06-15 (12m 14d / 11m 16d), r1 q 2017-08-15.
//     s2 counts r0 through its tolerance (13m − 8w = 06-04 at b_late) and
//     its floor 1y (2017-06-01 at b_old) may hold it, so s2 always
//     matches. r0 q: g-old (q's generic tier in 2017, alone in its group)
//     counts r0 under s1, whose 1y may lie after 06-15 (b_late) → both
//     match, s1 has more doses, r0 and r1 count, dose 3 due → 🔴
//     [AGE_AT_START_UNCERTAIN{s1, s2}], est. r0 px: g-old is not in px's
//     tier in 2017, so s1's dose 1 is r1, past 1y, and only s2 matches; s2
//     and xa count both doses → ✅ [SAFEST_SERIES{s2, px xa}, COMPLETE],
//     est. No product: s1's strict walk counts r1, the lenient one r0, so
//     the candidate spans [06-15, 08-15] and s1 matches; step 5 is strict,
//     so r0 fails 13m at both ends → ⚪ [AGE_AT_START_UNCERTAIN{s1, s2},
//     SAFEST_SERIES{s1, px xa}, DOSE_INVALID{minAge, r0, 1}], est. The
//     strict walk alone gives ✅.
//   - Route B: s-young [6w, 1y) (two doses, 6w then 4w) and s-old [1y, —)
//     (three doses, 13m). Birth 2016-06-26, r0 2017-03-01 (8m 3d), r1 q
//     2017-08-15 (13m 20d). s-young counts r0 and matches. r0 px: s-old's
//     dose 1 is r1 (no g-old for px), past 1y → both match, s-old → ⚪
//     [AGE_AT_START_UNCERTAIN{s-young, s-old}, SAFEST_SERIES{s-old, px
//     xa}, DOSE_INVALID{minAge, r0, 1}]. r0 q: g-old counts r0 under s-old
//     at 8 months, below 1y, so only s-young matches → ✅ [COMPLETE]. No
//     product: strict r1, lenient r0, the span reaches 08-15 → as r0 px.
//     Main (Q94 in the walk) gives ✅.
//   - Route C: catalog px only, with its own p-old (three doses, 6w then
//     4w) until 2020-01-01 instead of g-old, then route A's pair (px falls
//     to the generic tier). Birth 2016-06, r0 2017-06-15, r1 px
//     2017-08-15. r0 px: p-old is a cross-group version of s1 (px's tier in
//     2017, s1 in px's tier today) and counts r0 → 🔴
//     [AGE_AT_START_UNCERTAIN{s1, s2}], est. No product: no cross-group
//     version in the strict walk (r1), p-old in the lenient one (r0) → both
//     match, s1, r0 fails 13m → ⚪ [AGE_AT_START_UNCERTAIN{s1, s2},
//     DOSE_INVALID{minAge, r0, 1}], est. Main gives ✅.
//   - Route D: route B's snapshot; r0 2017-03-01 and rm with no product
//     2017-07-10 (12m 14d, below 13m). r0 px: s-old's strict walk counts
//     neither r0 nor rm, the lenient one counts rm through g-old, past 1y
//     → both match, s-old; strict step 5 fails both doses at position 1,
//     xa counts them → ⚪ [AGE_AT_START_UNCERTAIN{s-young, s-old},
//     SAFEST_SERIES{s-old, px xa}, DOSE_INVALID{minAge, r0, 1},
//     DOSE_INVALID{minAge, rm, 1}]. r0 q: g-old counts r0 in both walks, so
//     only s-young matches; s-young and xa count both → ✅
//     [SAFEST_SERIES{s-young, px xa}, COMPLETE]. No product: strict counts
//     nothing (it tried r0, rm), lenient counts r0, so the candidate spans
//     [03-01, 07-10] → as r0 px. Q119 (a)'s hull of the two walks' own
//     ranges misses rm: the strict walk falls back to r0, the first record
//     it tried, the same range as the lenient walk's → ✅, a P1 failure.
//   - Route E: route A's snapshot, but g-old's dose-1 minAge is 55w and
//     s2's and xa's dose 2 come 2w after dose 1. Birth 2016-06, rf q
//     2017-06-15, rk 2017-06-30. g-old's 55w falls on 06-21 at b_old and
//     07-20 at b_late, 13m on 07-01 and 07-30. rf fails s1 and g-old at
//     both ends, and s2 counts it through its tolerance and always
//     matches, as in route A. rk px: no version, so s1 counts nothing and
//     reads rf, the first record tried, which may lie before 1y (06-30 at
//     b_late) → both match, s1; rf and rk fail 13m, xa counts both (15 days
//     apart) → ⚪ [AGE_AT_START_UNCERTAIN{s1, s2}, SAFEST_SERIES{s1, px
//     xa}, DOSE_INVALID{minAge, rf, 1}, DOSE_INVALID{minAge, rk, 1}], est.
//     rk q: g-old counts rk under s1 at b_old only, and 06-30 is 1y at
//     b_late, so only s2 matches; s2 counts rf and rk → ✅ [COMPLETE], est.
//     No product: the strict walk counts nothing (fallback rf), the
//     lenient one counts rk at b_old only, so the candidate spans [06-15,
//     06-30] → as rk px. A candidate that starts at the lenient walk's dose
//     1 drops rf → ✅, a P1 failure.
//   - Route F: route B's snapshot; r0 2017-05-26 (11m), r1 px 2017-06-30
//     (12m 4d, below 13m). s-young counts r0 and matches in every
//     reading. r0 px: xa alone counts both doses → ✅ [COMPLETE]. r0 q:
//     g-old counts r0 under s-old at 11 months, below 1y, so only s-young
//     matches; s-young and xa count both → ✅ [SAFEST_SERIES{s-young, px
//     xa}, COMPLETE]. r0 px: s-old counts nothing and reads r0, the first
//     record tried, again below 1y. No product: s-old's strict walk counts
//     nothing (it tried r0 and r1), the lenient one counts r0. r1 is named,
//     so it meets the same versions in every reading and no reading counts
//     it at position 1: every reading's dose 1 under s-old is r0, and only
//     s-young matches → as r0 q. A candidate widened through r1, at 12m 4d
//     inside s-old's reach, would match s-old and turn this ✅ into ⚪.
//   - Route G: route A's snapshot, birth 2016-06-26; ra with no product
//     2016-07-15 (19 days, below every dose-1 minAge), rb 2017-07-01 (12m
//     5d), rc q 2017-08-15 (13m 20d). ra fails position 1 in every walk and
//     under every candidate. s2 counts rb through its tolerance and matches
//     in every reading. rb q: g-old counts rb under s1, past 1y; rb px: s1
//     counts rc; so only s2 matches, and s2 and xa count rb and rc → ⚪
//     [SAFEST_SERIES{s2, px xa}, DOSE_INVALID{minAge, ra, 1}, COMPLETE].
//     No product: s1's strict walk counts rc (it tried ra and rb), the
//     lenient one counts rb, so the candidate spans [07-01, 08-15] → as rb
//     q. The lenient walk tried ra before rb and did not count it, so no
//     reading counts it at position 1: a candidate widened through every
//     record the strict walk tries, ra included, at 19 days inside s1's
//     reach, would match s1 too and pick it (three doses).
func TestUnknownProductBracketChoice(t *testing.T) {
	t.Parallel()
	from := func(sr ruleset.Series, min, max string) ruleset.Series { return ws(t, sr, "2020-01-01", "", min, max) }
	s2 := twoDose(t, "s2", "", "P13M", "P4W")
	s2.Plans[0].Doses[0] = tolerant(t, s2.Plans[0].Doses[0], "P8W")
	pairA := []ruleset.Series{from(threeDose(t, "s1", "", "P13M", "P4W"), "P6W", "P1Y"), from(s2, "P1Y", "")}
	pairB := []ruleset.Series{from(twoDose(t, "s-young", "", "P6W", "P4W"), "P6W", "P1Y"), from(threeDose(t, "s-old", "", "P13M", "P4W"), "P1Y", "")}
	gOld := ws(t, threeDose(t, "g-old", "", "P6W", "P4W"), "", "2020-01-01", "", "")
	xa := twoDose(t, "xa", "px", "P6W", "P4W")
	above, differs := ruleset.FirstDoseMinAgeAboveStartAge, ruleset.FirstDoseMinAgeDiffers
	engineA := newEngine(t, oneDiseaseSnapshot([]string{"q", "px"}, gOld, pairA[0], pairA[1], xa), above, above)
	engineB := newEngine(t, oneDiseaseSnapshot([]string{"q", "px"}, gOld, pairB[0], pairB[1], xa), above, differs)
	engineC := newEngine(t, oneDiseaseSnapshot([]string{"px"}, ws(t, threeDose(t, "p-old", "px", "P6W", "P4W"), "", "2020-01-01", "", ""),
		pairA[0], pairA[1]), above, above)
	monthBirth, dayBirth := monthDate("2016-06"), dayDate("2016-06-26")
	uncertain := func(a, b string) engine.Reason {
		return engine.Reason{Code: engine.ReasonAgeAtStartUncertain, Params: engine.ReasonParams{Candidates: []engine.SeriesRef{{Series: a}, {Series: b}}}}
	}
	// overdue is a 🔴 assessment with next dose n; its dates are not
	// compared.
	overdue := func(series []string, products []string, n int, reasons ...engine.Reason) engine.Assessment {
		return engine.Assessment{Disease: "q", Bucket: engine.BucketOverdue, SeriesUsed: series, Products: products, Estimated: true,
			NextDose: &engine.NextDose{Number: n}, Reasons: reasons}
	}
	type reading struct {
		product string // the product of the dose under test, "" for none
		want    engine.Assessment
	}
	// check runs each reading on doses, the dose with ID vary given the
	// reading's product, and checks the P1 shape: the reading with no
	// product is ✅ only when every named one is.
	check := func(t *testing.T, e *engine.Engine, birth engine.RecalledDate, vary string, readings []reading, doses ...engine.DoseRecord) {
		t.Helper()
		named, unknown := true, false
		for _, r := range readings {
			ds := slices.Clone(doses)
			for k, d := range ds {
				if d.ID == vary {
					ds[k] = pqDose(vary, d.Date.Value, r.product)
				}
			}
			got := assessmentOf(t, e, "q", "2026-03-07", birth, ds...)
			if r.want.Bucket == engine.BucketOverdue && got.NextDose != nil && got.NextDose.Number == r.want.NextDose.Number {
				got.NextDose = r.want.NextDose
			}
			checkAssessment(t, got, r.want)
			isComplete := got.Bucket == engine.BucketComplete
			if r.product == "" {
				unknown = isComplete
			} else {
				named = named && isComplete
			}
		}
		if unknown && !named {
			t.Errorf("%s with no product gives ✅, a named reading does not", vary)
		}
	}
	t.Run("route A", func(t *testing.T) {
		t.Parallel()
		check(t, engineA, monthBirth, "r0", []reading{
			{"q", overdue([]string{"s1"}, []string{"q"}, 3, uncertain("s1", "s2"))},
			{"px", completeWith([]string{"s2", "xa"}, true, safest("", "s2", "px", "xa"))},
			{"", unknownWith([]string{"s1", "xa"}, true, uncertain("s1", "s2"), safest("", "s1", "px", "xa"), invalidDose("r0", 1, engine.CheckMinAge))},
		}, pqDose("r0", "2017-06-15", ""), pqDose("r1", "2017-08-15", "q"))
	})
	t.Run("route B", func(t *testing.T) {
		t.Parallel()
		px := unknownWith([]string{"s-old", "xa"}, false, uncertain("s-young", "s-old"), safest("", "s-old", "px", "xa"), invalidDose("r0", 1, engine.CheckMinAge))
		check(t, engineB, dayBirth, "r0", []reading{
			{"px", px},
			{"q", completeWith([]string{"s-young"}, false)},
			{"", px},
		}, pqDose("r0", "2017-03-01", ""), pqDose("r1", "2017-08-15", "q"))
	})
	t.Run("route C", func(t *testing.T) {
		t.Parallel()
		check(t, engineC, monthBirth, "r0", []reading{
			{"px", overdue([]string{"s1"}, []string{"px"}, 3, uncertain("s1", "s2"))},
			{"", unknownWith([]string{"s1"}, true, uncertain("s1", "s2"), invalidDose("r0", 1, engine.CheckMinAge))},
		}, pqDose("r0", "2017-06-15", ""), pqDose("r1", "2017-08-15", "px"))
	})
	t.Run("route D", func(t *testing.T) {
		t.Parallel()
		px := unknownWith([]string{"s-old", "xa"}, false, uncertain("s-young", "s-old"), safest("", "s-old", "px", "xa"),
			invalidDose("r0", 1, engine.CheckMinAge), invalidDose("rm", 1, engine.CheckMinAge))
		check(t, engineB, dayBirth, "r0", []reading{
			{"px", px},
			{"q", completeWith([]string{"s-young", "xa"}, false, safest("", "s-young", "px", "xa"))},
			{"", px},
		}, pqDose("r0", "2017-03-01", ""), pqDose("rm", "2017-07-10", ""))
	})
	t.Run("route E", func(t *testing.T) {
		t.Parallel()
		s2E := twoDose(t, "s2", "", "P13M", "P2W")
		s2E.Plans[0].Doses[0] = tolerant(t, s2E.Plans[0].Doses[0], "P8W")
		e := newEngine(t, oneDiseaseSnapshot([]string{"q", "px"}, ws(t, threeDose(t, "g-old", "", "P55W", "P4W"), "", "2020-01-01", "", ""),
			pairA[0], from(s2E, "P1Y", ""), twoDose(t, "xa", "px", "P6W", "P2W")), above, above)
		px := unknownWith([]string{"s1", "xa"}, true, uncertain("s1", "s2"), safest("", "s1", "px", "xa"),
			invalidDose("rf", 1, engine.CheckMinAge), invalidDose("rk", 1, engine.CheckMinAge))
		check(t, e, monthBirth, "rk", []reading{
			{"px", px},
			{"q", completeWith([]string{"s2"}, true)},
			{"", px},
		}, pqDose("rf", "2017-06-15", "q"), pqDose("rk", "2017-06-30", ""))
	})
	t.Run("route F", func(t *testing.T) {
		t.Parallel()
		young := completeWith([]string{"s-young", "xa"}, false, safest("", "s-young", "px", "xa"))
		check(t, engineB, dayBirth, "r0", []reading{
			{"px", completeWith([]string{"xa"}, false)},
			{"q", young},
			{"", young},
		}, pqDose("r0", "2017-05-26", ""), pqDose("r1", "2017-06-30", "px"))
	})
	t.Run("route G", func(t *testing.T) {
		t.Parallel()
		want := unknownWith([]string{"s2", "xa"}, false, safest("", "s2", "px", "xa"), invalidDose("ra", 1, engine.CheckMinAge), complete)
		check(t, engineA, dayBirth, "rb", []reading{
			{"q", want},
			{"px", want},
			{"", want},
		}, pqDose("ra", "2016-07-15", ""), pqDose("rb", "2017-07-01", ""), pqDose("rc", "2017-08-15", "q"))
	})
}
