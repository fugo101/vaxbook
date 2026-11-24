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

// TestImpliedDoseBelowDatedRecordBeyondSeries (issue #15 In scope 4): the
// dated counterpart of TestTrailingImpliedDoseBelowRecordBeyondSeries, on
// the same snapshot (series z-b removed). Birth 2006-06-01 (20y@b =
// 2026-06-01), r1 za on 2026-05-25 numbered 1 (< 06-01, counted), r2 za on
// 2026-06-25 numbered 3, today 2026-07-01. r2 is beyond the series length
// and not checked itself, but it closes the run of the implied dose 2 before
// it: that slot's chain gives r1 + 4w = 06-22 ≥ 06-01 → definite, so r2
// carries DOSE_AFTER_MAX_AGE{maxAgeComplete} with its own position, 3. The
// chain itself fits (06-25 ≥ 06-22). The implied dose stays counted, so z-a
// is complete and COMPLETE is masked: ⚪, not a false ✅.
func TestImpliedDoseBelowDatedRecordBeyondSeries(t *testing.T) {
	t.Parallel()
	r1 := doseOf("z", "r1", dayDate("2026-05-25"), "za")
	r1.DoseNumbers = map[string]engine.DoseNumber{"z": 1}
	r2 := doseOf("z", "r2", dayDate("2026-06-25"), "za")
	r2.DoseNumbers = map[string]engine.DoseNumber{"z": 3}
	s := loadSnapshot(t, "minimal-max-age-complete")
	s.Series = s.Series[:1]
	got := assessmentOf(t, newEngine(t, s), "z", "2026-07-01", dayDate("2006-06-01"), r1, r2)
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Bucket: engine.BucketUnknown, SeriesUsed: []string{"z-a"}, Estimated: true,
		Reasons: []engine.Reason{
			{Code: engine.ReasonImpliedEarlierDoses, Params: engine.ReasonParams{Numbers: []int{2}}},
			afterMaxAge("r2", 3, engine.CheckMaxAgeComplete),
			{Code: engine.ReasonComplete},
		},
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
//   - With g-old's maxAgeStart raised to 30y, the bracket max binds instead:
//     birth 2000-01-01, dose 1 on 2028-07-01 (28y6m), today 2028-08-01. No
//     bracket matches (r1 ≥ 27y@b), so the last one, g-old, is kept, and its
//     start limit is min(30y@b = 2030-01-01, 27y@b = 2027-01-01) ≤ r1 →
//     DOSE_AFTER_MAX_AGE{maxAgeStart}, although r1 is before its own
//     maxAgeStart.
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
	s.Series[1].MaxAgeStart = spanOf(t, "P30Y")
	got = assessmentOf(t, newEngine(t, s), "h", "2028-08-01", dayDate("2000-01-01"), doseOf("h", "r1", dayDate("2028-07-01"), "g"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "h", Bucket: engine.BucketUnknown, SeriesUsed: []string{"g-old"},
		Reasons: []engine.Reason{afterMaxAge("r1", 1, engine.CheckMaxAgeStart)},
	})
}

// TestBracketChoiceWalksWithoutMaxAges (issue #15 In scope 2): product g
// with g-y [9m, 1y), g-a [1y, 3y) (maxAgeStart 2y) and g-b [3y, 5y), each
// with dose-1 minAge 9m and dose 2 minInterval 4w. Birth 2024-01-01, r1 on
// 2024-07-01 (6m, < 9m@b = 2024-10-01), r2 on 2026-07-01 (2y6m), today
// 2026-08-01.
//   - Bracket choice, walking without maximum ages: in every bracket r1
//     fails the minimum age and r2 is the first record counted at position
//     1. r2 is in g-a's bracket only, so g-a is chosen. (Walking with g-a's
//     start limit would leave r2 uncounted too, make the first record tried
//     at position 1, r1, g-a's dose 1, match no bracket and fall back to
//     g-y.)
//   - Step 5 under g-a: r1 → DOSE_INVALID{minAge, 1}; r2 at position 1,
//     start limit min(2y@b = 2026-01-01, 5y@b) = 2026-01-01 ≤ r2 →
//     DOSE_AFTER_MAX_AGE{maxAgeStart, 1}. k = 0 → ⚪.
func TestBracketChoiceWalksWithoutMaxAges(t *testing.T) {
	t.Parallel()
	plan := []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{
		{MinAge: spanOf(t, "P9M"), Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, "P9M")}},
		{MinInterval: spanOf(t, "P4W"), Recommended: ruleset.Milestone{From: ruleset.FromPreviousDose, Offset: *spanOf(t, "P1M")}},
	}}}
	bracket := func(id, lo, hi string) ruleset.Series {
		return ruleset.Series{ID: id, Disease: "h", Product: new("g"), Stage: ruleset.Childhood,
			StartAge: &ruleset.StartAge{Min: *spanOf(t, lo), Max: spanOf(t, hi)}, Plans: plan}
	}
	a := bracket("g-a", "P1Y", "P3Y")
	a.MaxAgeStart = spanOf(t, "P2Y")
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "h", Name: "H"}},
		Products: []ruleset.Product{{ID: "g", Name: "G", Diseases: []string{"h"}}},
		Series:   []ruleset.Series{bracket("g-y", "P9M", "P1Y"), a, bracket("g-b", "P3Y", "P5Y")}}
	got := assessmentOf(t, newEngine(t, s), "h", "2026-08-01", dayDate("2024-01-01"),
		doseOf("h", "r1", dayDate("2024-07-01"), "g"), doseOf("h", "r2", dayDate("2026-07-01"), "g"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "h", Bucket: engine.BucketUnknown, SeriesUsed: []string{"g-a"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r1", Number: 1}},
			afterMaxAge("r2", 1, engine.CheckMaxAgeStart),
		},
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
//
// Since S4b (issue #28) dose 2 here can never be given in time (r1.Hi + 4w =
// 05-28 is past L = 04-22), so in the counted view the series is certainly
// out of age and layer 5 gives outOfAge, which keeps the reason as a masked
// reason before OUT_OF_AGE.
func TestMaxAgeCheckListsAreCatalogOrdered(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-max-age")
	s.Series[0].MaxAgeComplete = spanOf(t, "P16W")
	got := assessmentOf(t, newEngine(t, s), "rota", "2026-05-10", birthRota, rota("r1", monthDate("2026-04"), "rotarix"))
	if want := []engine.Reason{possiblyInvalid("r1", 1, engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete), {Code: engine.ReasonOutOfAge}}; !slices.EqualFunc(got.Reasons, want, func(a, b engine.Reason) bool {
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
//   - Another floating dose than the last can be the one that fails
//     (failedIn): r1 rotateq on 06-14 numbered 1 (≥ 15w@b → DOSE_AFTER_MAX_AGE
//     {maxAgeStart}, not counted), u0, u1 and u2 rotateq undated with no
//     number, today 08-02. Positions 2 and 3 are left, so the all-after
//     placement puts u0 at 2 (r1 + 4w = 07-12) and u1 at 3 (08-09, after
//     today → DOSE_INVALID{minInterval}), and u2 beyond the series, where it
//     is not checked. r1's number leaves no other point. u2, the last by ID,
//     is named with u1's failure and number: DOSE_INVALID{minInterval, u2,
//     3}; u0 and u1 stay counted floating doses, and step 7 stops on them.
//     Since S4b (issue #28 In scope 0b) a dose at position 1 past the start
//     limit behind an uncounted dated dose no longer reaches this branch:
//     the placement in front of that dose rescues it (golden
//     level4-dose-may-precede-dose-after-max-age-start).
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

	r1 = rota("r1", dayDate("2026-06-14"), "rotateq")
	r1.DoseNumbers = map[string]engine.DoseNumber{"rota": 1}
	got = assessmentOf(t, e, "rota", "2026-08-02", birthRota,
		r1, rota("u0", unknownDate, "rotateq"), rota("u1", unknownDate, "rotateq"), rota("u2", unknownDate, "rotateq"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{
			afterMaxAge("r1", 1, engine.CheckMaxAgeStart),
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinInterval}, Dose: "u2", Number: 3}},
			{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "u0"}},
			{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "u1"}},
		},
	})
}

// TestNarrowingNamedFloatingDoseShape pins the shape for which the narrowing
// property keys an unnumbered level-4 record by its ID alone
// (uncountedKeys): narrowing a dated dose changes the number that the named
// floating dose fails at. All rotateq, today 03-04: d0 on 01-18 (before
// 6w@b = 02-12 → DOSE_INVALID{minAge}, not counted), d1 in 2026-02, u0 and
// u1 undated with no number.
//   - d1 in 2026-02 is possibly before 02-12, so it counts at 1. The
//     all-after placement puts u0 at 2 and u1 at 3, whose chain (d1.Lo + 8w
//     = 03-29) is after today: DOSE_INVALID{minInterval, u1, 3}. In front of
//     d1 the doses push d1 past its interval (a new DOSE_INVALID), so no
//     placement fits, and u1 is named at 3.
//   - Narrowed to 02-01, d1 is DOSE_INVALID{minAge}, not counted, and the
//     guard keeps both floating doses after it (02-12 > d1): u0 at 1
//     (02-12) and u1 at 2 (03-12, after today) → DOSE_INVALID{minInterval,
//     u1, 2}.
//
// (Before S4b this test used r3 past the start limit, which the rescue of
// issue #28 In scope 0b now counts at both dates.)
func TestNarrowingNamedFloatingDoseShape(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	doses := func(d1 engine.RecalledDate) []engine.DoseRecord {
		return []engine.DoseRecord{rota("d0", dayDate("2026-01-18"), "rotateq"), rota("d1", d1, "rotateq"), rota("u0", unknownDate, "rotateq"), rota("u1", unknownDate, "rotateq")}
	}
	invalid := func(dose string, number int, c engine.Check) engine.Reason {
		return engine.Reason{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{c}, Dose: dose, Number: number}}
	}
	unknownU0 := engine.Reason{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "u0"}}
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-03-04", birthRota, doses(monthDate("2026-02"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{invalid("d0", 1, engine.CheckMinAge), possiblyInvalid("d1", 1, engine.CheckMinAge),
			invalid("u1", 3, engine.CheckMinInterval), unknownU0},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-03-04", birthRota, doses(dayDate("2026-02-01"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{invalid("d0", 1, engine.CheckMinAge), invalid("d1", 1, engine.CheckMinAge),
			invalid("u1", 2, engine.CheckMinInterval), unknownU0},
	})
}

// TestNarrowingDisagreeingRecordShape pins the known exception to the
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
func TestNarrowingDisagreeingRecordShape(t *testing.T) {
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
	// The property's exception covers exactly the disagreeing records.
	for _, x := range []engine.Input{wide, narrow} {
		q, err := engine.DisagreeingRecords(e, x)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"r0", "r1", "r3"}; !slices.Equal(q["rota"], want) {
			t.Errorf("disagreeing records %v, want %v", q["rota"], want)
		}
	}
}

// numbered gives a dose record a user-entered number for disease rota.
func numbered(r engine.DoseRecord, n int) engine.DoseRecord {
	for _, dz := range r.Diseases {
		r.DoseNumbers = map[string]engine.DoseNumber{dz: engine.DoseNumber(n)}
	}
	return r
}

func invalidDose(dose string, number int, checks ...engine.Check) engine.Reason {
	return engine.Reason{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: checks, Dose: dose, Number: number}}
}

// TestUndatedSlotUpperBound (issue #28 In scope 0a, Q84): an undated slot's
// maximum-age range is clamped at the record that closes its run.
//   - Closer beyond the series: rotateq r1 #1 on 04-01, r2 #2 on 07-25, r3 #3
//     undated, r4 #4 on 08-10, today 09-01. r3's chain (r2 + 4w = 08-22) is
//     after r4, so r3 lies in r4's range [08-10, 08-10], before 32w@b =
//     08-13: no maximum-age failure. r4, beyond the series, carries the
//     chain failure → ⚪ [DOSE_INVALID{minInterval, r4, 4}, COMPLETE] (main
//     gave r3 DOSE_AFTER_MAX_AGE).
//   - A chain that fits stays definite: r2 on 07-20, r4 in 2026-08, today
//     09-15. r3's chain is 08-17 ≤ r4.Hi = 08-31 and past 08-13 →
//     DOSE_AFTER_MAX_AGE{maxAgeComplete, r3, 3}; r4 is possibly before r3's
//     chain + 4w → ⚪ [DOSE_POSSIBLY_INVALID{minInterval, r4, 4},
//     DOSE_AFTER_MAX_AGE{maxAgeComplete, r3, 3}], as on main. The counted
//     view is complete, so the data ⚪ stays (masked dose 3 is 🟡, no reason).
func TestUndatedSlotUpperBound(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	doses := func(r2, r4 engine.RecalledDate) []engine.DoseRecord {
		return []engine.DoseRecord{
			numbered(rota("r1", dayDate("2026-04-01"), "rotateq"), 1), numbered(rota("r2", r2, "rotateq"), 2),
			numbered(rota("r3", unknownDate, "rotateq"), 3), numbered(rota("r4", r4, "rotateq"), 4),
		}
	}
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-09-01", birthRota, doses(dayDate("2026-07-25"), dayDate("2026-08-10"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{invalidDose("r4", 4, engine.CheckMinInterval), {Code: engine.ReasonComplete}},
	})
	checkAssessment(t, assessmentOf(t, e, "rota", "2026-09-15", birthRota, doses(dayDate("2026-07-20"), monthDate("2026-08"))...), engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
		Reasons: []engine.Reason{possiblyInvalid("r4", 4, engine.CheckMinInterval), afterMaxAge("r3", 3, engine.CheckMaxAgeComplete)},
	})
}

// TestFloatingDoseInFrontOfUncountedDose (issue #28 In scope 0b, Q83): a
// placement of the floating doses is also tried in front of an unnumbered
// dated dose that is not counted, and the dose it counts there is rescued
// (DOSE_POSSIBLY_INVALID).
func TestFloatingDoseInFrontOfUncountedDose(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	f := rota("f", unknownDate, "rotarix")
	for _, tc := range []struct {
		name, today string
		doses       []engine.DoseRecord
		want        engine.Assessment
	}{{
		// d on 02-05 (before 6w@b = 02-12): f in front of d would lie on
		// 02-12 or later, after d, so the guard keeps f after d, where it is
		// before 6w by today. Without the guard f would count.
		name: "time-order guard", today: "2026-02-06",
		doses: []engine.DoseRecord{rota("d", dayDate("2026-02-05"), "rotarix"), f},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
			Reasons: []engine.Reason{invalidDose("d", 1, engine.CheckMinAge), invalidDose("f", 1, engine.CheckMinAge)}},
	}, {
		// d1 on 02-05 fails the guard; d2 on 04-20 (≥ 15w@b = 04-16), in the
		// same run, gets the point: f at 1 and d2 at 2 → d2 is rescued.
		// A point only before each run's first member would keep
		// DOSE_AFTER_MAX_AGE on d2.
		name: "first member that passes the guard", today: "2026-06-01",
		doses: []engine.DoseRecord{rota("d1", dayDate("2026-02-05"), "rotarix"), rota("d2", dayDate("2026-04-20"), "rotarix"), f},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
			Reasons: []engine.Reason{invalidDose("d1", 1, engine.CheckMinAge), possiblyInvalid("d2", 1, engine.CheckMaxAgeStart), {Code: engine.ReasonComplete}}},
	}, {
		// rotateq r1 on 04-20 and r2 on 05-20: with f in front, r1 counts at
		// 2 and r2 at 3. The redone walk counts r1 at 1 and r2 at 2, and f
		// fits after them.
		name: "rescue re-walks later doses", today: "2026-06-01",
		doses: []engine.DoseRecord{rota("r1", dayDate("2026-04-20"), "rotateq"), rota("r2", dayDate("2026-05-20"), "rotateq"), rota("f", unknownDate, "rotateq")},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
			Reasons: []engine.Reason{possiblyInvalid("r1", 1, engine.CheckMaxAgeStart), {Code: engine.ReasonComplete}}},
	}, {
		// d on 04-20 and e on 05-01: with f in front, d counts at 2 and e is
		// beyond the series. So e, at 1 in the redone walk (past the start
		// limit), is forced counted with no reason.
		name: "record after a rescued dose", today: "2026-06-01",
		doses: []engine.DoseRecord{rota("d", dayDate("2026-04-20"), "rotarix"), rota("e", dayDate("2026-05-01"), "rotarix"), f},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"}, Estimated: true,
			Reasons: []engine.Reason{possiblyInvalid("d", 1, engine.CheckMaxAgeStart), {Code: engine.ReasonComplete}}},
	}, {
		// The same with f on 03-01: f at 1, d at 2 (within 24w@b) and e
		// beyond the series → ✅.
		name: "record after a rescued dose, f dated", today: "2026-06-01",
		doses: []engine.DoseRecord{rota("d", dayDate("2026-04-20"), "rotarix"), rota("e", dayDate("2026-05-01"), "rotarix"), rota("f", dayDate("2026-03-01"), "rotarix")},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketComplete, SeriesUsed: []string{"rota-rotarix"},
			Reasons: []engine.Reason{{Code: engine.ReasonComplete}}},
	}, {
		// As above with rotateq and two floating doses: with f1 and f2 in
		// front, d counts at 3 and e is beyond the series, so e (at 2 in
		// the redone walk, 11 days after d) is forced counted with no
		// reason. The fit of f2 after e must not bring e's reason back.
		name: "record after a rescued dose, two floating doses", today: "2026-06-01",
		doses: []engine.DoseRecord{
			rota("d", dayDate("2026-04-20"), "rotateq"), rota("e", dayDate("2026-05-01"), "rotateq"),
			rota("f1", unknownDate, "rotateq"), rota("f2", unknownDate, "rotateq"),
		},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
			Reasons: []engine.Reason{possiblyInvalid("d", 1, engine.CheckMaxAgeStart), {Code: engine.ReasonComplete}}},
	}, {
		// An undated record after a rescued dose is settled like a dated
		// one. rotateq d on 07-20 (past 15w@b = 04-16 at 1), u #2 undated
		// and f undated, today 09-01. With f at 1 and u at 2 in front, d
		// counts at 3 (chain 03-12 + 4w ≤ 07-20, before 32w@b = 08-13), so
		// d is rescued, and that walk gives u no reason. In the redone walk
		// d counts at 1 and u's chain is d + 4w = 08-17, past 08-13: u is
		// forced counted with DOSE_POSSIBLY_INVALID, not DOSE_AFTER_MAX_AGE.
		// f fits in front of d. k = 4 → masked COMPLETE.
		name: "undated record after a rescued dose", today: "2026-09-01",
		doses: []engine.DoseRecord{rota("d", dayDate("2026-07-20"), "rotateq"), numbered(rota("u", unknownDate, "rotateq"), 2), rota("f", unknownDate, "rotateq")},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotateq"}, Estimated: true,
			Reasons: []engine.Reason{possiblyInvalid("d", 1, engine.CheckMaxAgeStart), possiblyInvalid("u", 2, engine.CheckMaxAgeComplete), {Code: engine.ReasonComplete}}},
	}, {
		// The same with f on 02-12 and u on 03-12: f at 1, u at 2 and d at
		// 3 → ✅, so a definite failure on u would be false.
		name: "undated record after a rescued dose, f and u dated", today: "2026-09-01",
		doses: []engine.DoseRecord{rota("d", dayDate("2026-07-20"), "rotateq"), numbered(rota("u", dayDate("2026-03-12"), "rotateq"), 2), rota("f", dayDate("2026-02-12"), "rotateq")},
		want: engine.Assessment{Disease: "rota", Bucket: engine.BucketComplete, SeriesUsed: []string{"rota-rotateq"},
			Reasons: []engine.Reason{{Code: engine.ReasonComplete}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkAssessment(t, assessmentOf(t, e, "rota", tc.today, birthRota, tc.doses...), tc.want)
		})
	}
}

// TestSlotBehindCountedDatedDose (issue #28 In scope 0c): behind a counted
// dated dose the slot's chain is unchanged. minimal-max-age-complete, birth
// 2006-06-01 (20y@b = 2026-06-01): d0 za on 2026-05-20 (no number, counted
// at 1) and u za #2 undated, today 2026-07-01. u's chain (d0 + 4w = 06-17)
// is past 06-01 → DOSE_AFTER_MAX_AGE{maxAgeComplete, u, 2}, as on main (u on
// 2025-01-01 is valid only by moving d0 to 3). Masked dose 2 from d0:
// ON_TRACK; the counted view is complete, so the data ⚪ stays.
func TestSlotBehindCountedDatedDose(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "minimal-max-age-complete")
	got := assessmentOf(t, e, "z", "2026-07-01", dayDate("2006-06-01"),
		doseOf("z", "d0", dayDate("2026-05-20"), "za"), numbered(doseOf("z", "u", unknownDate, "za"), 2))
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Bucket: engine.BucketUnknown, SeriesUsed: []string{"z-a"}, Estimated: true,
		Reasons: []engine.Reason{afterMaxAge("u", 2, engine.CheckMaxAgeComplete), {Code: engine.ReasonOnTrack}},
	})
}

// TestFloatingFitAfterRescueJudgesMovedDose (issue #28 In scope 0b): after
// a rescue, the floating fit judges the rescued dose afresh at any position
// other than the one it is counted at, so a placement that moves it where
// it definitely fails does not fit. In-test series x-a (product x):
// maxAgeStart 15w (last day 04-15), maxAgeComplete 24w (exclusive 06-18);
// dose 1 minAge 6w, dose 2 minInterval 4w, dose 3 minInterval 10w. Birth
// 2026-01-01: D on 04-20 and f1, f2 undated, today 08-01.
//   - Rescue: f1 and f2 in front put D at 3, before 03-12 + 10w = 05-21
//     (DOSE_INVALID); f2 alone puts D at 2 (02-12 + 4w ≤ 04-20) → D is
//     rescued, counted at 1 with DOSE_POSSIBLY_INVALID{maxAgeStart}.
//   - Fit of f1 and f2: both in front of D put D at 3, DOSE_INVALID there;
//     f1 in front puts f2 at 3 on D + 10w = 06-29, past 06-18; both after D
//     put f2 at 3 on D + 4w + 10w = 07-27, past 06-18. No placement fits,
//     so f2, the last by ID, gets DOSE_AFTER_MAX_AGE{maxAgeComplete, f2, 3}
//     from the last one, and f1 stays counted.
//   - k = 3, whose previous dose f1 has no date: step 7 stops. The counted
//     view (every record counted) is complete, so the data ⚪ stays.
func TestFloatingFitAfterRescueJudgesMovedDose(t *testing.T) {
	t.Parallel()
	s := oneDiseaseSnapshot([]string{"x"}, qSeries(t, "x-a", "x", "P15W", "P24W", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P10W", "", ruleset.FromPreviousDose, "P3M"),
	}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-08-01", birthRota,
		doseOf("q", "d", dayDate("2026-04-20"), "x"), doseOf("q", "f1", unknownDate, "x"), doseOf("q", "f2", unknownDate, "x"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"x-a"}, Estimated: true,
		Reasons: []engine.Reason{
			possiblyInvalid("d", 1, engine.CheckMaxAgeStart), afterMaxAge("f2", 3, engine.CheckMaxAgeComplete),
			{Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "f1"}},
		},
	})
}
