package engine_test

import (
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of maximum ages on given doses outside the goldens (issue
// #15, S4a). Every in-test snapshot passes ruleset.Validate with no problem.
// On illustrative-max-age with birth 2026-01-01: 6w@b = 02-12, 15w@b =
// 04-16 (rota-rotarix and rota-rotateq start limit), 12w@b = 03-26
// (rota-rotavin), 24w@b = 06-18 (rota-rotarix complete limit), 32w@b =
// 08-13 (rota-rotateq).

// assessmentOf evaluates one profile and returns the assessment of disease.
func assessmentOf(t testing.TB, e *engine.Engine, disease, today string, birth engine.RecalledDate, doses ...engine.DoseRecord) engine.Assessment {
	t.Helper()
	in := engine.Input{Today: mustDate(t, today), Profile: engine.Profile{Birth: birth}, Doses: doses}
	if doses == nil {
		in.Doses = []engine.DoseRecord{}
	}
	out, b := evaluate(t, e, in)
	for _, a := range out.Assessments {
		checkAssessmentShape(t, "in-test", a)
		checkReasonParams(t, a)
		if a.Disease == disease {
			return a
		}
	}
	t.Fatalf("no %s assessment in %s", disease, b)
	return engine.Assessment{}
}

func dayDate(v string) engine.RecalledDate {
	return engine.RecalledDate{Value: v, Precision: engine.Day}
}

func monthDate(v string) engine.RecalledDate {
	return engine.RecalledDate{Value: v, Precision: engine.Month}
}

var unknownDate = engine.RecalledDate{Precision: engine.Unknown}

// doseOf is a dose record of one disease; product "" is none.
func doseOf(disease, id string, date engine.RecalledDate, product string) engine.DoseRecord {
	r := engine.DoseRecord{ID: id, Diseases: []string{disease}, Date: date}
	if product != "" {
		r.Product = &product
	}
	return r
}

func rota(id string, date engine.RecalledDate, product string) engine.DoseRecord {
	return doseOf("rota", id, date, product)
}

func afterMaxAge(dose string, number int, checks ...engine.Check) engine.Reason {
	return engine.Reason{Code: engine.ReasonDoseAfterMaxAge, Params: engine.ReasonParams{Checks: checks, Dose: dose, Number: number}}
}

func possiblyInvalid(dose string, number int, checks ...engine.Check) engine.Reason {
	return engine.Reason{Code: engine.ReasonDosePossiblyInvalid, Params: engine.ReasonParams{Checks: checks, Dose: dose, Number: number}}
}

func safest(refs ...string) engine.Reason {
	var cs []engine.SeriesRef
	for i := 0; i < len(refs); i += 2 {
		cs = append(cs, engine.SeriesRef{Product: refs[i], Series: refs[i+1]})
	}
	return engine.Reason{Code: engine.ReasonSafestSeries, Params: engine.ReasonParams{Candidates: cs}}
}

var (
	rotaAll   = []string{"rota-rotarix", "rota-rotateq", "rota-rotavin"}
	rotaRefs  = safest("rotarix", "rota-rotarix", "rotateq", "rota-rotateq", "rotavin", "rota-rotavin")
	birthRota = dayDate("2026-01-01")
)

// TestMaxAgesAreExclusive (SAD §5.2, issue #15 In scope 3): a dose on the
// day before b + max meets it, a dose on b + max does not.
//   - rota-rotarix dose 1 on 04-15 (< 04-16) is counted; dose 2: E = r1 +
//     4w = 05-13, Du = max(E, b + 4m = 05-01) = 05-13, O = 05-20; today
//     05-10 < E, Du ≤ T30 → 🟡. On 04-16 it is DOSE_AFTER_MAX_AGE.
//   - dose 2 on 06-17 (< 06-18) completes the series; on 06-18 it is
//     DOSE_AFTER_MAX_AGE.
func TestMaxAgesAreExclusive(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	rotarix := []string{"rota-rotarix"}
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-05-10", birthRota, rota("r1", dayDate("2026-04-15"), "rotarix")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2026-05-13", "2026-05-13", "2026-05-20"),
		SeriesUsed: rotarix, Products: []string{"rotarix"},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-05-10", birthRota, rota("r1", dayDate("2026-04-16"), "rotarix")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotarix,
		Reasons: []engine.Reason{afterMaxAge("r1", 1, engine.CheckMaxAgeStart)},
	})
	r1 := rota("r1", dayDate("2026-03-01"), "rotarix")
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-07-10", birthRota, r1, rota("r2", dayDate("2026-06-17"), "rotarix")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketComplete, SeriesUsed: rotarix,
		Reasons: []engine.Reason{{Code: engine.ReasonComplete}},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-07-10", birthRota, r1, rota("r2", dayDate("2026-06-18"), "rotarix")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotarix,
		Reasons: []engine.Reason{afterMaxAge("r2", 2, engine.CheckMaxAgeComplete)},
	})
}

// TestProductNameForMaxAgeDisagreement (issue #15 In scope 5, 6; Q53, Q54):
// r1 on 03-01 and r2 on 06-25, today 07-10. Every rota series is a candidate
// in each case, so rota-rotarix gives r2 DOSE_AFTER_MAX_AGE (06-25 ≥ 06-18)
// while rota-rotateq counts it (< 08-13, k 3) and rota-rotavin is complete:
// the candidates disagree on r2, its DOSE_AFTER_MAX_AGE is dropped, and the
// reason depends only on whether a dose has an unknown product. k = 2 from
// rota-rotarix (masked 🟡), no COMPLETE.
//   - Q36 sub-case: both records name othervax, a catalog product that does
//     not list rota → unknown → CHECK_PRODUCT_NAME.
//   - rotarix retired, both records rotarix → named → MIXED_PRODUCTS.
//   - both records with freeProduct "Rotavac" → named → MIXED_PRODUCTS, and
//     no PRODUCT_NOT_IN_CATALOG (the disease is not complete).
//   - r1 rotarix, r2 with no product → CHECK_PRODUCT_NAME.
func TestProductNameForMaxAgeDisagreement(t *testing.T) {
	t.Parallel()
	free := func(r engine.DoseRecord) engine.DoseRecord {
		r.FreeProduct = new("Rotavac")
		return r
	}
	cases := []struct {
		name   string
		mutate func(s *ruleset.Snapshot)
		r1, r2 engine.DoseRecord
		want   engine.ReasonCode
	}{
		{"catalog product not listing the disease", func(s *ruleset.Snapshot) {
			s.Diseases = append(s.Diseases, ruleset.Disease{ID: "other", Name: "Other"})
			s.Products = append(s.Products, ruleset.Product{ID: "othervax", Name: "Othervax", Diseases: []string{"other"}})
			other := s.Series[2]
			other.ID, other.Disease, other.Product, other.MaxAgeStart = "other-1", "other", new("othervax"), nil
			s.Series = append(s.Series, other)
		}, rota("r1", dayDate("2026-03-01"), "othervax"), rota("r2", dayDate("2026-06-25"), "othervax"), engine.ReasonCheckProductName},
		{"single retired product", func(s *ruleset.Snapshot) { s.Products[0].Retired = true },
			rota("r1", dayDate("2026-03-01"), "rotarix"), rota("r2", dayDate("2026-06-25"), "rotarix"), engine.ReasonMixedProducts},
		{"named free text", func(*ruleset.Snapshot) {},
			free(rota("r1", dayDate("2026-03-01"), "")), free(rota("r2", dayDate("2026-06-25"), "")), engine.ReasonMixedProducts},
		{"named plus no product", func(*ruleset.Snapshot) {},
			rota("r1", dayDate("2026-03-01"), "rotarix"), rota("r2", dayDate("2026-06-25"), ""), engine.ReasonCheckProductName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := loadSnapshot(t, "illustrative-max-age")
			tc.mutate(s)
			e := newEngine(t, s)
			checkAssessment(t, assessmentOf(t, e, "rota", "2026-07-10", birthRota, tc.r1, tc.r2), engine.Assessment{
				Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll,
				Reasons: []engine.Reason{rotaRefs, {Code: tc.want}},
			})
		})
	}
}

// TestMaxAgeNoCandidateCounts (Q52): products x (series x-1: dose-1 minAge
// 8w, maxAgeStart 10w) and y (y-1: minAge 12w, maxAgeStart 20w), one dose 1
// with no product at 11w (birth 2026-01-01, 03-19), today 04-01. x-1:
// 03-19 ≥ 10w@b = 03-12 → DOSE_AFTER_MAX_AGE; y-1: 03-19 < 12w@b = 03-26 →
// DOSE_INVALID{minAge} only. No candidate counts r1, so both reasons are
// kept, by candidate in rule-set order, with no CHECK_PRODUCT_NAME.
func TestMaxAgeNoCandidateCounts(t *testing.T) {
	t.Parallel()
	series := func(id, product, minAge, maxStart string) ruleset.Series {
		return ruleset.Series{ID: id, Disease: "q", Product: new(product), Stage: ruleset.Childhood, MaxAgeStart: spanOf(t, maxStart),
			Plans: []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{{
				MinAge: spanOf(t, minAge), Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, minAge)},
			}}}}}
	}
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "q", Name: "Q"}},
		Products: []ruleset.Product{{ID: "x", Name: "X", Diseases: []string{"q"}}, {ID: "y", Name: "Y", Diseases: []string{"q"}}},
		Series:   []ruleset.Series{series("x-1", "x", "P8W", "P10W"), series("y-1", "y", "P12W", "P20W")}}
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-04-01", birthRota, doseOf("q", "r1", dayDate("2026-03-19"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"x-1", "y-1"},
		Reasons: []engine.Reason{
			safest("x", "x-1", "y", "y-1"),
			afterMaxAge("r1", 1, engine.CheckMaxAgeStart),
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r1", Number: 1}},
		},
	})
}

// TestUndatedDoseOneBehindUncountedDatedDose (issue #15 In scope 4): r1
// rotarix on 05-01 (no number; ≥ 04-16 → DOSE_AFTER_MAX_AGE, not counted)
// and r2 rotarix with an unknown date and no number, today 06-01. r2 floats;
// r1 is not counted, so no placement goes in front of it, and the only
// placement puts r2 after r1 at position 1. Its chain starts at r1: best =
// worst = 05-01 ≥ 04-16 → definite, so no placement fits and r2 gets
// DOSE_AFTER_MAX_AGE{maxAgeStart, number 1}, not counted (k = 1, masked).
func TestUndatedDoseOneBehindUncountedDatedDose(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	got := assessmentOf(t, e, "rota", "2026-06-01", birthRota,
		rota("r1", dayDate("2026-05-01"), "rotarix"), rota("r2", unknownDate, "rotarix"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
		Reasons: []engine.Reason{afterMaxAge("r1", 1, engine.CheckMaxAgeStart), afterMaxAge("r2", 1, engine.CheckMaxAgeStart)},
	})
}

// TestTrailingImpliedDoseBelowRecordBeyondSeries (issue #15 In scope 4):
// minimal-max-age-complete, birth 2006-06-01 (20y@b = 2026-06-01), r1 za on
// 2026-05-25 numbered 1, r2 za with an unknown date numbered 3, today
// 2026-07-01. Dose 2 is implied after r1, and its chain gives 05-25 + 4w =
// 06-22 ≥ 06-01 under z-a → definite. r2, the lowest record beyond the
// series, carries it with its number. As for an S2 chain failure, the
// implied dose stays counted, so z-a is complete (k 3) and COMPLETE is
// masked: the result is ⚪, not a false ✅.
//
// This deviates from issue #15's engine-test list, reported to the owner:
// #15 expects ⚪ [IMPLIED_EARLIER_DOSES{[2]}, DOSE_AFTER_MAX_AGE{maxAgeComplete,
// r2, 3}] on the unmodified fixture, which the decided rules cannot give.
// An implied dose makes every series a candidate (Q30), and under z-b
// (21y@b = 2027-06-01) the implied dose is within its limit and z-b is
// complete, leaving r2 unchecked beyond its length: the candidates
// disagree on r2 (Q52), and the implied dose is an unknown product →
// CHECK_PRODUCT_NAME (the second case, on the unmodified fixture). #15's
// list also omits the masked COMPLETE, which a blocking reason keeps (SAD
// §5.3 step 10). To pin the carrier itself, the first case removes series
// z-b (product zb stays in the catalog).
func TestTrailingImpliedDoseBelowRecordBeyondSeries(t *testing.T) {
	t.Parallel()
	birth := dayDate("2006-06-01")
	r1 := doseOf("z", "r1", dayDate("2026-05-25"), "za")
	r1.DoseNumbers = map[string]engine.DoseNumber{"z": 1}
	r2 := doseOf("z", "r2", unknownDate, "za")
	r2.DoseNumbers = map[string]engine.DoseNumber{"z": 3}
	implied := engine.Reason{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{2}}}
	complete := engine.Reason{Code: engine.ReasonComplete}

	s := loadSnapshot(t, "minimal-max-age-complete")
	s.Series = s.Series[:1]
	got := assessmentOf(t, newEngine(t, s), "z", "2026-07-01", birth, r1, r2)
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Bucket: engine.BucketUnknown, SeriesUsed: []string{"z-a"}, Estimated: true,
		Reasons: []engine.Reason{implied, afterMaxAge("r2", 3, engine.CheckMaxAgeComplete), complete},
	})

	got = assessmentOf(t, loadEngine(t, "minimal-max-age-complete"), "z", "2026-07-01", birth, r1, r2)
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Bucket: engine.BucketUnknown, SeriesUsed: []string{"z-a", "z-b"}, Estimated: true,
		Reasons: []engine.Reason{implied, safest("za", "z-a", "zb", "z-b"), {Code: engine.ReasonCheckProductName}, complete},
	})
}

// TestMaxAgeDefiniteAtOneCandidatePossibleAtAnother (Q52): products p (series
// p-s, maxAgeComplete 24w) and q (q-s, 25w), each dose 1 minAge 6w and dose
// 2 minInterval 4w. Birth 2026-01-14: 24w@b = 07-01, 25w@b = 07-08. r1 p on
// 03-01, r2 q in 2026-07 ([07-01, 07-31]), today 08-10. Under p-s r2 is
// DOSE_AFTER_MAX_AGE (Lo 07-01 ≥ 07-01); under q-s it is possible (Lo <
// 07-08 ≤ Hi) and counted. q-s counts r2, so they disagree: the definite
// reason is dropped, q-s's DOSE_POSSIBLY_INVALID is kept, and both products
// are named → MIXED_PRODUCTS.
func TestMaxAgeDefiniteAtOneCandidatePossibleAtAnother(t *testing.T) {
	t.Parallel()
	series := func(id, product, maxComplete string) ruleset.Series {
		return ruleset.Series{ID: id, Disease: "rota", Product: new(product), Stage: ruleset.Childhood, MaxAgeComplete: spanOf(t, maxComplete),
			Plans: []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{
				{MinAge: spanOf(t, "P6W"), Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, "P2M")}},
				{MinInterval: spanOf(t, "P4W"), Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, "P4M")}},
			}}}}
	}
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "rota", Name: "Rota"}},
		Products: []ruleset.Product{{ID: "p", Name: "P", Diseases: []string{"rota"}}, {ID: "q", Name: "Q", Diseases: []string{"rota"}}},
		Series:   []ruleset.Series{series("p-s", "p", "P24W"), series("q-s", "q", "P25W")}}
	got := assessmentOf(t, newEngine(t, s), "rota", "2026-08-10", dayDate("2026-01-14"),
		rota("r1", dayDate("2026-03-01"), "p"), rota("r2", monthDate("2026-07"), "q"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"p-s", "q-s"}, Estimated: true,
		Reasons: []engine.Reason{safest("p", "p-s", "q", "q-s"), possiblyInvalid("r2", 2, engine.CheckMaxAgeComplete), {Code: engine.ReasonMixedProducts}},
	})
}

// TestStartLimitIsEarlierOfMaxAgeStartAndLastBracket (Q55): product g with
// g-young [9y, 15y) (3 doses) and g-old [15y, 27y) (1 dose, maxAgeStart
// 26y), every dose-1 minAge 9y.
//   - Birth 2000-01-01, dose 1 on 2026-07-01 (26y6m), today 2026-08-01: only
//     g-old matches, and its start limit is min(26y@b = 2026-01-01, 27y@b =
//     2027-01-01) → DOSE_AFTER_MAX_AGE{maxAgeStart}, although the dose is
//     inside the bracket.
//   - Birth 2000-01-15 (15y@b = 2015-01-15), dose 1 in 2015-01 ([01-01,
//     01-31]), today 2015-02-10: both brackets match and the most-doses rule
//     picks g-young. Its start limit is the group's last max (27y), not its
//     own 15y, so the straddle gives no max-age reason. Dose 2: E = r1.Hi +
//     4w = 02-28, Du = max(E, r1.Hi + 1m = 02-28) = 02-28, O = 03-07; today
//     < E, Du ≤ T30 → 🟡.
func TestStartLimitIsEarlierOfMaxAgeStartAndLastBracket(t *testing.T) {
	t.Parallel()
	dose := func(first bool, offset string) ruleset.DoseRule {
		if first {
			return ruleset.DoseRule{MinAge: spanOf(t, "P9Y"), Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, offset)}}
		}
		return ruleset.DoseRule{MinInterval: spanOf(t, "P4W"), Recommended: ruleset.Milestone{From: ruleset.FromPreviousDose, Offset: *spanOf(t, offset)}}
	}
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "h", Name: "H"}},
		Products: []ruleset.Product{{ID: "g", Name: "G", Diseases: []string{"h"}}},
		Series: []ruleset.Series{
			{ID: "g-young", Disease: "h", Product: new("g"), Stage: ruleset.Childhood,
				StartAge: &ruleset.StartAge{Min: *spanOf(t, "P9Y"), Max: spanOf(t, "P15Y")},
				Plans:    []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{dose(true, "P9Y"), dose(false, "P1M"), dose(false, "P1M")}}}},
			{ID: "g-old", Disease: "h", Product: new("g"), Stage: ruleset.Adult,
				StartAge:    &ruleset.StartAge{Min: *spanOf(t, "P15Y"), Max: spanOf(t, "P27Y")},
				MaxAgeStart: spanOf(t, "P26Y"),
				Plans:       []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{dose(true, "P15Y")}}}},
		}}
	e := newEngine(t, s)
	got := assessmentOf(t, e, "h", "2026-08-01", dayDate("2000-01-01"), doseOf("h", "r1", dayDate("2026-07-01"), "g"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "h", Bucket: engine.BucketUnknown, SeriesUsed: []string{"g-old"},
		Reasons: []engine.Reason{afterMaxAge("r1", 1, engine.CheckMaxAgeStart)},
	})
	got = assessmentOf(t, e, "h", "2015-02-10", dayDate("2000-01-15"), doseOf("h", "r1", monthDate("2015-01"), "g"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "h", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2015-02-28", "2015-02-28", "2015-03-07"),
		SeriesUsed: []string{"g-young"}, Products: []string{"g"}, Estimated: true,
		Reasons: []engine.Reason{{Code: engine.ReasonAgeAtStartUncertain, Params: engine.ReasonParams{Candidates: []engine.SeriesRef{
			{Product: "g", Series: "g-young"}, {Product: "g", Series: "g-old"}}}}},
	})
}

// TestMinimumAndMaximumBothDefinite (Q62): rota-rotarix with dose 2's
// minimum interval raised to 12w. r1 on 04-10 (counted), r2 on 06-20, today
// 07-10: r2 is before r1 + 12w = 07-03 and on or after 24w@b = 06-18, so it
// gets DOSE_INVALID{minInterval} then DOSE_AFTER_MAX_AGE{maxAgeComplete}.
func TestMinimumAndMaximumBothDefinite(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-max-age")
	s.Series[0].Plans[0].Doses[1].MinInterval = spanOf(t, "P12W")
	got := assessmentOf(t, newEngine(t, s), "rota", "2026-07-10", birthRota,
		rota("r1", dayDate("2026-04-10"), "rotarix"), rota("r2", dayDate("2026-06-20"), "rotarix"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinInterval}, Dose: "r2", Number: 2}},
			afterMaxAge("r2", 2, engine.CheckMaxAgeComplete),
		},
	})
}

// TestNoBirthDateNeverChecksMaxAges (EQ19, A6, Q59): with no birth date a
// dose 1 dated far past any 15 weeks after the earliest possible birth is
// counted.
//   - rota-rotarix: dose 2's series has maxAgeComplete → exactly
//     [NO_BIRTH_DATE], with no DOSE_AFTER_MAX_AGE.
//   - rota-rotavin (maxAgeStart only): dose 2 is computed, which shows r1
//     was counted: E = r1 + 4w = 09-29, Du = r1 + (3m − 2m) = 10-01, O =
//     10-08; today 09-10 < E, Du ≤ T30 → 🟡.
func TestNoBirthDateNeverChecksMaxAges(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	none := engine.RecalledDate{Precision: engine.None}
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-09-10", none, rota("r1", dayDate("2026-09-01"), "rotarix")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
		Reasons: []engine.Reason{{Code: engine.ReasonNoBirthDate}},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-09-10", none, rota("r1", dayDate("2026-09-01"), "rotavin")), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2026-09-29", "2026-10-01", "2026-10-08"),
		SeriesUsed: []string{"rota-rotavin"}, Products: []string{"rotavin"}, Estimated: true,
	})
}

// TestMaxAgeCheckListsAreCatalogOrdered guards the reason lists: a dose 1
// that possibly fails both maximum ages lists them in catalog order. An
// in-test series has maxAgeStart 15w and maxAgeComplete 16w (16w@b =
// 04-23): r1 in 2026-04 ([04-01, 04-30]) is possibly past both → one
// DOSE_POSSIBLY_INVALID{[maxAgeStart, maxAgeComplete]}, counted.
func TestMaxAgeCheckListsAreCatalogOrdered(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-max-age")
	s.Series[0].MaxAgeComplete = spanOf(t, "P16W")
	got := assessmentOf(t, newEngine(t, s), "rota", "2026-05-10", birthRota, rota("r1", monthDate("2026-04"), "rotarix"))
	if want := []engine.Reason{possiblyInvalid("r1", 1, engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete)}; !slices.EqualFunc(got.Reasons, want, func(a, b engine.Reason) bool {
		return a.Code == b.Code && slices.Equal(a.Params.Checks, b.Params.Checks) && a.Params.Dose == b.Params.Dose && a.Params.Number == b.Params.Number
	}) {
		t.Errorf("reasons %+v, want %+v", got.Reasons, want)
	}
}

// TestImpliedDoseAboveEverySeriesIsUnknown (issue #15 In scope 6, Q30): an
// implied dose numbered above every candidate's length still widens the
// candidates to every series (S3a: whether there are implied doses is
// decided from the records alone), so it is a dose of the disease with an
// unknown product for the reason too. r1 rotarix on 03-01, r2 on 06-25, r3
// on 07-01, r4 rotarix with an unknown date numbered 5 (dose 4 implied),
// today 07-10.
//   - rota-rotarix: r1 counted; r2 ≥ 24w@b = 06-18 → DOSE_AFTER_MAX_AGE, not
//     counted, so r3 is checked at 2: before r2 + 4w = 07-23 →
//     DOSE_INVALID{minInterval} and ≥ 06-18 → DOSE_AFTER_MAX_AGE. k 2.
//   - rota-rotateq: r1, r2 counted (< 32w@b = 08-13); r3 at 3 → DOSE_INVALID
//     {minInterval}. k 3.
//   - rota-rotavin: r1 (< 12w@b = 03-26), r2 counted, complete; r3 beyond.
//
// rota-rotateq counts r2 and the complete rota-rotavin leaves r3 unchecked,
// so both DOSE_AFTER_MAX_AGE are dropped. The implied dose 4 is above the
// longest candidate (3), so IMPLIED_EARLIER_DOSES does not list it (Q48),
// but it is unknown → CHECK_PRODUCT_NAME, not MIXED_PRODUCTS. k 2 from
// rota-rotarix: overdue, masked, with no reason.
func TestImpliedDoseAboveEverySeriesIsUnknown(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	r4 := rota("r4", unknownDate, "rotarix")
	r4.DoseNumbers = map[string]engine.DoseNumber{"rota": 5}
	got := assessmentOf(t, e, "rota", "2026-07-10", birthRota, rota("r1", dayDate("2026-03-01"), "rotarix"),
		rota("r2", dayDate("2026-06-25"), "rotarix"), rota("r3", dayDate("2026-07-01"), "rotarix"), r4)
	invalid := func(number int) engine.Reason {
		return engine.Reason{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinInterval}, Dose: "r3", Number: number}}
	}
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{rotaRefs, invalid(2), invalid(3), {Code: engine.ReasonCheckProductName}},
	})
}

// TestNoFloatingPlacementNamesLastByID (issue #15 In scope 4, "Floating
// doses"): when no placement fits, the existing "last floating dose by ID"
// rule names the record, with DOSE_AFTER_MAX_AGE when the maximum age is
// what fails in the all-after placement. The other floating doses stay
// counted.
//   - r1 rotateq on 08-20 numbered 1 (≥ 15w@b and ≥ 32w@b = 08-13 →
//     DOSE_AFTER_MAX_AGE{maxAgeStart, maxAgeComplete}, not counted), r2 and
//     r3 rotateq with unknown dates and no number, today 12-01. r1's number
//     leaves only the all-after placement: r2 at 2 (r1 + 4w = 09-17) and r3
//     at 3 (10-15), both ≥ 08-13, so it does not fit. Only r3 is named:
//     DOSE_AFTER_MAX_AGE{maxAgeComplete, r3, 3}. r2 stays a counted floating
//     dose, so k = 0 + 1 + 1 = 2, and step 7 stops: r2 may be the latest dose
//     (PREVIOUS_DOSE_DATE_UNKNOWN{r2}).
//   - r1 rotarix on 05-01 with no number (≥ 15w@b → not counted), f2 and f3
//     rotarix undated with no number, today 06-01. The all-after placement
//     puts f2 at 1, chained from r1 (05-01 ≥ 04-16 → maxAgeStart definite),
//     and f3 at 2 (05-29 < 24w@b = 06-18, by today). Only the dose at 1
//     fails, but the last by ID, f3, is named, with what fails:
//     DOSE_AFTER_MAX_AGE{maxAgeStart, f3, 1}. f2 stays counted: k = 2, and
//     step 7 stops on f2.
func TestNoFloatingPlacementNamesLastByID(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	r1 := rota("r1", dayDate("2026-08-20"), "rotateq")
	r1.DoseNumbers = map[string]engine.DoseNumber{"rota": 1}
	got := assessmentOf(t, e, "rota", "2026-12-01", birthRota, r1, rota("r2", unknownDate, "rotateq"), rota("r3", unknownDate, "rotateq"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{
			afterMaxAge("r1", 1, engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete),
			afterMaxAge("r3", 3, engine.CheckMaxAgeComplete),
			{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "r2"}},
		},
	})

	got = assessmentOf(t, e, "rota", "2026-06-01", birthRota,
		rota("r1", dayDate("2026-05-01"), "rotarix"), rota("f2", unknownDate, "rotarix"), rota("f3", unknownDate, "rotarix"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
		Reasons: []engine.Reason{
			afterMaxAge("r1", 1, engine.CheckMaxAgeStart),
			afterMaxAge("f3", 1, engine.CheckMaxAgeStart),
			{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "f2"}},
		},
	})
}

// TestNarrowingNamedFloatingDoseShape pins the shape for which the narrowing
// property keys an unnumbered level-4 record by its ID alone
// (uncountedKeys). r0 with no product on 02-05 (before 6w@b = 02-12 →
// DOSE_INVALID{minAge} under every candidate, not counted), r3 rotarix in
// 2026-05 (at position 1, ≥ 04-16 → DOSE_AFTER_MAX_AGE{maxAgeStart}), r1
// rotarix and r2 with no product undated with no number, today 06-20. r2
// makes every series a candidate. In each, the only placement puts r1 at 1
// (chained from r3: ≥ every start limit, definite) and r2 at 2 (05-29 to
// 06-28: possibly past rota-rotarix's 06-18, possibly not by today). No
// placement fits, and r2, the last by ID, is named with r1's failure
// (number 1); its own possible checks are dropped (Q62). Narrowed to 05-25,
// r2's own slot (06-22) is past 06-18 and after today, so r2 is named with
// its own failures (number 2). The same records are not counted.
func TestNarrowingNamedFloatingDoseShape(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	doses := func(r3 engine.RecalledDate) []engine.DoseRecord {
		return []engine.DoseRecord{rota("r0", dayDate("2026-02-05"), ""), rota("r1", unknownDate, "rotarix"), rota("r2", unknownDate, ""), rota("r3", r3, "rotarix")}
	}
	invalid := func(dose string, number int, c engine.Check) engine.Reason {
		return engine.Reason{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{c}, Dose: dose, Number: number}}
	}
	unknownR1 := engine.Reason{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "r1"}}
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-06-20", birthRota, doses(monthDate("2026-05"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{rotaRefs, invalid("r0", 1, engine.CheckMinAge), afterMaxAge("r3", 1, engine.CheckMaxAgeStart),
			afterMaxAge("r2", 1, engine.CheckMaxAgeStart), unknownR1},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-06-20", birthRota, doses(dayDate("2026-05-25"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{rotaRefs, invalid("r0", 1, engine.CheckMinAge), afterMaxAge("r3", 1, engine.CheckMaxAgeStart),
			invalid("r2", 2, engine.CheckMinInterval), afterMaxAge("r2", 2, engine.CheckMaxAgeComplete), unknownR1},
	})
}

// TestNarrowingQuestionedRecordShape pins the known exception to the
// narrowing property (SAD §5.7) that Q52 and Q62 together give, reported to
// the owner: a record whose DOSE_AFTER_MAX_AGE a disagreement drops shows no
// reason from the candidate that does not count it, so a possible minimum
// check hidden there (Q62) can narrow to a definite DOSE_INVALID that the
// wider result gave no sign of. Issue #15 says this cannot happen ("a wide
// result with no DOSE_POSSIBLY_INVALID has no possible max-age failure").
// No candidate's count changes: rota-rotateq does not count r1 in either
// result, and the others leave it beyond their length.
//
// Birth 2009-11-29 (6w@b = 2010-01-10, 12w@b = 02-21, 15w@b = 03-14, 24w@b =
// 05-16, 32w@b = 07-11), today 2011-03-30. r4 rotarix 2010-01-22, r2 with
// no product 2010-05-10 numbered 2, r3 rotarix 2010-08-11, r0 with no
// product 2011-01-10, r1 rotarix in 2011-02. Every series is a candidate.
//   - rota-rotarix and rota-rotavin: r4 and r2 counted (within every limit),
//     complete; r3, r0 and r1 beyond the length.
//   - rota-rotateq: r4, r2 counted; r3, r0 and r1 at 3, each ≥ 07-11 →
//     DOSE_AFTER_MAX_AGE, not counted. r1 is also possibly before r0 + 4w =
//     02-07 (Lo 02-01, Hi 02-28), a check it drops as it is not counted.
//
// The complete candidates leave r3, r0 and r1 unchecked, so all three
// disagree and lose their DOSE_AFTER_MAX_AGE: [SAFEST_SERIES,
// CHECK_PRODUCT_NAME]. Narrowed to 2011-02-04, r1 is definitely before
// 02-07 under rota-rotateq: DOSE_INVALID{minInterval, r1, 3} is kept (Q31).
func TestNarrowingQuestionedRecordShape(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	in := func(r1 engine.RecalledDate) engine.Input {
		r2 := rota("r2", dayDate("2010-05-10"), "")
		r2.DoseNumbers = map[string]engine.DoseNumber{"rota": 2}
		return engine.Input{Today: mustDate(t, "2011-03-30"), Profile: engine.Profile{Birth: dayDate("2009-11-29")}, Doses: []engine.DoseRecord{
			rota("r0", dayDate("2011-01-10"), ""), rota("r1", r1, "rotarix"), r2, rota("r3", dayDate("2010-08-11"), "rotarix"), rota("r4", dayDate("2010-01-22"), "rotarix"),
		}}
	}
	cpn := engine.Reason{Code: engine.ReasonCheckProductName}
	wide, narrow := in(monthDate("2011-02")), in(dayDate("2011-02-04"))
	checkAssessment(t, assessmentOf(t, e, "rota", "2011-03-30", wide.Profile.Birth, wide.Doses...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{rotaRefs, cpn},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2011-03-30", narrow.Profile.Birth, narrow.Doses...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll,
		Reasons: []engine.Reason{rotaRefs,
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinInterval}, Dose: "r1", Number: 3}}, cpn},
	})
	// The property's exception covers exactly the questioned records.
	for _, x := range []engine.Input{wide, narrow} {
		q, err := engine.QuestionedRecords(e, x)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"r0", "r1", "r3"}; !slices.Equal(q["rota"], want) {
			t.Errorf("questioned records %v, want %v", q["rota"], want)
		}
	}
}
