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

// TestVersionBirthRange (In scope 3): a version is judged at both ends of
// the birth range. g-2025 (until 2026-01-01): one dose, minAge 9m,
// maxAgeComplete 2y. g-2026 (from then): minAge 4w, maxAgeComplete 8m. r1
// 2025-08-15, today 2026-02-01.
//   - Birth in 2024: at 2024-01-01 g-2026's 8m fails r1 and g-2025 fails its
//     9m at b_late; at 2024-12-31 g-2026 counts it → the ends differ → ⚪
//     [BIRTH_DATE_IMPRECISE{fullBirthDate}, COMPLETE, DOSE_AFTER_MAX_AGE],
//     est. Judged at b alone, g-2025 would count it at 2024-01-01 although
//     a birth date between fails both.
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
		unknownWith(used, true, fullBirthDate, complete, after))
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

// TestIdenticalCopyInBracketedGroup (Q106): an identical re-issue of
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
// gap-b, in effect in the older segment, is a version of gap-a-copy, but it
// cannot count r4: with no dose 1 counted, dose 1 lies in [birth, r4] =
// [2023-08-06, 2023-10-29] (Q106), under 3 months old, which gap-b's reach
// there, [2y, 5y), cannot hold.
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

// TestVersionBirthRangeDefinite pins a consequence of #29 In scope 3's
// birth-range rule (a version is judged by the worse of its verdicts at
// b_old and b_late), open for the owner: a definite verdict that narrowing
// the birth date turns into a counted dose. x-old (until 2026-01-01) has
// dose 1 minAge P4W, x-new (from then) P6W, each dose 2 at 4w. Birth
// 2025-09, r1 2025-10-06, today 2026-02-01:
//   - month birth date: x-old passes r1 at b_old 09-01 (09-29) and fails it
//     at b_late 09-30 (10-28), so its worse verdict is definite; x-new fails
//     it at both (10-13, 11-11). Both step-4 ends give DOSE_INVALID{minAge,
//     r1, 1}: ⚪ with that reason alone, estimated;
//   - birth 2025-09-01: x-old counts r1, the rule in effect on its date, so
//     dose 2 is due 4 weeks later and the result is 🔴 for dose 2 with no
//     DOSE_INVALID.
//
// The month result is P1-safe (⚪), but its definite reason does not hold
// for every birth date in the range, as S4a's meaning of definite asks.
func TestVersionBirthRangeDefinite(t *testing.T) {
	t.Parallel()
	e := newEngine(t, oneDiseaseSnapshot(nil,
		ws(t, twoDose(t, "x-old", "", "P4W", "P4W"), "", "2026-01-01", "", ""),
		ws(t, twoDose(t, "x-new", "", "P6W", "P4W"), "2026-01-01", "", "", "")))
	r1 := qDose("r1", "2025-10-06")
	wide := assessmentOf(t, e, "q", "2026-02-01", monthDate("2025-09"), r1)
	if want := []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge)}; wide.Bucket != engine.BucketUnknown || !reflect.DeepEqual(wide.Reasons, want) {
		t.Errorf("birth 2025-09: %+v\nwant ⚪ %v", wide, want)
	}
	narrow := assessmentOf(t, e, "q", "2026-02-01", dayDate("2025-09-01"), r1)
	if narrow.Bucket != engine.BucketOverdue || narrow.NextDose == nil || narrow.NextDose.Number != 2 || hasCode(narrow, engine.ReasonDoseInvalid) {
		t.Errorf("birth 2025-09-01: %+v\nwant 🔴 for dose 2, r1 counted", narrow)
	}
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
