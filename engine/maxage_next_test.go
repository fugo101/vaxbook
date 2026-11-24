package engine_test

import (
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of the maximum ages of the next dose outside the goldens
// (issue #28, S4b). Every in-test snapshot passes ruleset.Validate with no
// Reject. Limits are last allowed days, one day before S4a's exclusive
// limit dates.

// rule is a dose rule of an in-test series; "" leaves a span out.
func rule(t testing.TB, minAge, minInterval, fromFirst string, from ruleset.Anchor, offset string) ruleset.DoseRule {
	t.Helper()
	opt := func(s string) *ruleset.Span {
		if s == "" {
			return nil
		}
		return spanOf(t, s)
	}
	return ruleset.DoseRule{MinAge: opt(minAge), MinInterval: opt(minInterval), MinIntervalFromFirst: opt(fromFirst),
		Recommended: ruleset.Milestone{From: from, Offset: *spanOf(t, offset)}}
}

// oneDiseaseSnapshot is an in-test snapshot of disease q with the given
// products (each listing q) and series.
func oneDiseaseSnapshot(products []string, series ...ruleset.Series) *ruleset.Snapshot {
	s := &ruleset.Snapshot{Version: "in-test", Illustrative: true, Diseases: []ruleset.Disease{{ID: "q", Name: "Q"}}, Series: series}
	for _, p := range products {
		s.Products = append(s.Products, ruleset.Product{ID: p, Name: p, Diseases: []string{"q"}})
	}
	return s
}

// qSeries is a series of disease q; product "" is generic.
func qSeries(t testing.TB, id, product, maxStart, maxComplete string, plans ...[]ruleset.DoseRule) ruleset.Series {
	t.Helper()
	sr := ruleset.Series{ID: id, Disease: "q", Stage: ruleset.Childhood}
	if product != "" {
		sr.Product = new(product)
	}
	if maxStart != "" {
		sr.MaxAgeStart = spanOf(t, maxStart)
	}
	if maxComplete != "" {
		sr.MaxAgeComplete = spanOf(t, maxComplete)
	}
	for i, p := range plans {
		sr.Plans = append(sr.Plans, ruleset.Plan{ID: string(rune('a' + i)), Doses: p})
	}
	return sr
}

func capped(day string) engine.Reason {
	d, _ := engine.ParseDate(day)
	return engine.Reason{Code: engine.ReasonDueCappedByMaxAge, Params: engine.ReasonParams{LastDay: d}}
}

func mayNotComplete(day string) engine.Reason {
	d, _ := engine.ParseDate(day)
	return engine.Reason{Code: engine.ReasonSeriesMayNotComplete, Params: engine.ReasonParams{LastDay: d}}
}

var fullBirthDate = engine.Reason{Code: engine.ReasonBirthDateImprecise, Params: engine.ReasonParams{Hint: engine.HintFullBirthDate}}

// TestLastAllowedDayFromFirstAnchor (issue #28 Q70) end to end: a 4-dose
// generic series (dose 1 minAge 9y; doses 2–4 minInterval 4w; dose 4 also
// minIntervalFromFirst 6m; maxAgeComplete 15y), birth 2010-01-01, so L =
// 2024-12-31. r1 numbered 1 with an unknown date, r2 numbered 2 on
// 2024-08-01, today 2024-09-01. k = 3, and the first-dose anchor is r2,
// the EQ20c stand-in: dose 4 is at least 2024-08-01 + 6m = 2025-02-01 > L,
// so no day for dose 3 leaves room (C_3 impossible), while dose 3 itself
// is in age: E = r2 + 4w = 08-29, Du = r2 + 1m = 09-01, O = 09-08, F =
// 09-01 ≤ Lim = L → SERIES_MAY_NOT_COMPLETE{2024-12-31}, nothing lowered,
// 🟠. Without the stand-in C_3 would be L − 4w; the stand-in can only move
// it earlier. r2 is behind an undated slot: INTERVAL_UNVERIFIABLE.
func TestLastAllowedDayFromFirstAnchor(t *testing.T) {
	t.Parallel()
	prev := func(fromFirst string) ruleset.DoseRule {
		return rule(t, "", "P4W", fromFirst, ruleset.FromPreviousDose, "P1M")
	}
	s := oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P15Y",
		[]ruleset.DoseRule{rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"), prev(""), prev(""), prev("P6M")}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2024-09-01", dayDate("2010-01-01"),
		numbered(doseOf("q", "r1", unknownDate, ""), 1), numbered(doseOf("q", "r2", dayDate("2024-08-01"), ""), 2))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketDue, NextDose: nextDose(t, 3, "2024-08-29", "2024-09-01", "2024-09-08"),
		SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{{Code: engine.ReasonIntervalUnverifiable, Params: engine.ReasonParams{Dose: "r2", Number: 2}}, mayNotComplete("2024-12-31")},
	})
}

// TestFromFirstOvershootsLastDay (Q70): an HPV-like generic series (dose 1
// minAge 9y; dose 2 minInterval 4w; dose 3 minInterval 12w and
// minIntervalFromFirst 5m; maxAgeComplete 15y), birth 2010-01-01, L =
// 2024-12-31. r1 on 2024-09-01, today 2024-10-01: dose 3 is at least r1 +
// 5m = 2025-02-01 > L, so C_2 is impossible. Dose 2: E = r1 + 4w = 09-29,
// Du = r1 + 1m = 10-01, O = 10-08, F = 10-01 ≤ Lim = L → case 2, 🟠.
func TestFromFirstOvershootsLastDay(t *testing.T) {
	t.Parallel()
	s := oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P15Y", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
		rule(t, "", "P12W", "P5M", ruleset.FromPreviousDose, "P5M"),
	}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2024-10-01", dayDate("2010-01-01"), doseOf("q", "r1", dayDate("2024-09-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketDue, NextDose: nextDose(t, 2, "2024-09-29", "2024-10-01", "2024-10-08"),
		SeriesUsed: []string{"q-1"}, Reasons: []engine.Reason{mayNotComplete("2024-12-31")},
	})
}

// TestCappedAtForwardChainDay (Q70): the cap day of case 1 is L_k from the
// forward chain, not Lim, when a later dose still needs room. A generic
// 3-dose series (dose 1 minAge 6w; doses 2 and 3 minInterval 4w; dose 2 due
// previous + 10m; maxAgeComplete 1y), birth 2026-01-01, so L = 2026-12-31.
// r1 on 03-01, today 07-01. Dose 2: E = 03-29, Du = 2027-01-01, O =
// 2027-01-08. C_2 = L − 4w = 12-03, the last day from which dose 3 still
// fits by L, so L_2 = 12-03 < Lim = L. F = 07-01 ≤ L_2 → Du = O = 12-03,
// lowered, with DUE_CAPPED_BY_MAX_AGE{12-03}; due within 6 months → 🟡.
// Capping at Lim would show 12-31, from which dose 3 cannot be completed.
func TestCappedAtForwardChainDay(t *testing.T) {
	t.Parallel()
	s := oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P1Y", []ruleset.DoseRule{
		rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P10M"),
		rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P1M"),
	}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-07-01", birthRota, doseOf("q", "r1", dayDate("2026-03-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2026-03-29", "2026-12-03", "2026-12-03"),
		SeriesUsed: []string{"q-1"}, Reasons: []engine.Reason{capped("2026-12-03")},
	})
}

// TestCappedEndsDisagree (Q68): a generic 2-dose series whose dose 2 counts
// from birth (offset 179m, minInterval 4w; maxAgeComplete 15y), birth
// 2010-01 (month), so L = 2024-12-31 from b_old at both ends. r1 on
// 2024-06-01, today 2024-07-01. Older end: Du = 2024-12-01, O = 12-08,
// both ≤ L → 🟡, uncapped. Later end (01-31): Du = 12-31, O = 2025-01-07 →
// O lowered to 12-31 → 🟡 with DUE_CAPPED_BY_MAX_AGE. A capped and an
// uncapped result disagree → ⚪ BIRTH_DATE_IMPRECISE{fullBirthDate} with the
// later end's cap reason.
func TestCappedEndsDisagree(t *testing.T) {
	t.Parallel()
	s := oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P15Y", []ruleset.DoseRule{
		rule(t, "P9Y", "", "", ruleset.FromBirth, "P9Y"),
		rule(t, "", "P4W", "", ruleset.FromBirth, "P179M"),
	}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2024-07-01", monthDate("2010-01"), doseOf("q", "r1", dayDate("2024-06-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"q-1"}, Estimated: true,
		Reasons: []engine.Reason{fullBirthDate, capped("2024-12-31")},
	})
}

// TestCappedOnTrack: minimal-max-age-complete, birth 2006-06-01 (L z-a =
// 2026-05-31), r1 za on 2025-06-01, today 2025-07-01. Dose 2: E = 06-29,
// Du = r1 + 12m = 2026-06-01, O = 06-08; F = 07-01 ≤ L → Du = O = 05-31,
// lowered. Due is more than 6 months away and a dose is counted → ✅
// ON_TRACK, after the cap reason.
func TestCappedOnTrack(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "minimal-max-age-complete")
	got := assessmentOf(t, e, "z", "2025-07-01", dayDate("2006-06-01"), doseOf("z", "r1", dayDate("2025-06-01"), "za"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Bucket: engine.BucketComplete, NextDose: nextDose(t, 2, "2025-06-29", "2026-05-31", "2026-05-31"),
		SeriesUsed: []string{"z-a"}, Products: []string{"za"},
		Reasons: []engine.Reason{capped("2026-05-31"), {Code: engine.ReasonOnTrack}},
	})
}

// TestTwoOutOfAgeEndsWithDoses: minimal-max-age-complete, birth 2006-06
// (month), r1 za on 2026-01-01, today 2026-07-15: past L at both ends
// (05-31 from b_old; the later end's own L^b is 06-29), certain at both →
// outOfAge once, with seriesUsed (Q74).
func TestTwoOutOfAgeEndsWithDoses(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "minimal-max-age-complete")
	got := assessmentOf(t, e, "z", "2026-07-15", monthDate("2006-06"), doseOf("z", "r1", dayDate("2026-01-01"), "za"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "z", Block: engine.BlockOutOfAge, SeriesUsed: []string{"z-a"}, Estimated: true,
		Reasons: []engine.Reason{{Code: engine.ReasonOutOfAge}},
	})
}

// TestPartialRetirementWithMaxAges (Q72): illustrative-max-age with rotarix
// and rotateq retired, today 2026-04-10, no dose. The only non-retired
// series, rota-rotavin, is past its start limit (03-25), so the choice is
// redone over every group, where rotavin is removed again: 🔴 over
// rota-rotarix and rota-rotateq as in golden
// a3-no-dose-one-product-past-start-limit, with products omitted (both
// retired).
func TestPartialRetirementWithMaxAges(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-max-age")
	s.Products[0].Retired, s.Products[1].Retired = true, true
	e := newEngine(t, s)
	got := assessmentOf(t, e, "rota", "2026-04-10", birthRota)
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketOverdue, NextDose: nextDose(t, 1, "2026-02-12", "2026-03-12", "2026-03-19"),
		SeriesUsed: []string{"rota-rotarix", "rota-rotateq"},
		Reasons:    []engine.Reason{safest("rotarix", "rota-rotarix", "rotateq", "rota-rotateq")},
	})
}

// TestMergedFRemovesCandidate (issue #28 In scope 1): products x (dose-1
// minAge 6w, maxAgeStart 15w: S = 04-15) and y (minAge 16w, maxAgeStart
// 20w: S = 05-20), birth 2026-01-01, no dose, today 02-26 (8w). Each alone
// is in (F_x = 02-26, F_y = 04-23), but the merged F 04-23 is past x's
// limit, so x is removed and y alone remains: no SAFEST_SERIES and no
// CHECK_PRODUCT_NAME. Dose 1 of y: E = 04-23, Du = b + 4m = 05-01, O =
// 05-08 → 🟡; L_1 = 05-20 lowers nothing.
func TestMergedFRemovesCandidate(t *testing.T) {
	t.Parallel()
	one := func(minAge string) []ruleset.DoseRule {
		return []ruleset.DoseRule{rule(t, minAge, "", "", ruleset.FromBirth, "P4M")}
	}
	s := oneDiseaseSnapshot([]string{"x", "y"}, qSeries(t, "x-1", "x", "P15W", "", one("P6W")), qSeries(t, "y-1", "y", "P20W", "", one("P16W")))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-02-26", birthRota)
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 1, "2026-04-23", "2026-05-01", "2026-05-08"),
		SeriesUsed: []string{"y-1"}, Products: []string{"y"},
	})
}

// TestLevel4DoseCandidateWithoutMaxAgeComplete (EQ11, Q75):
// illustrative-max-age, birth 2023-05-01, r1 with no product and an unknown
// date, today 2026-06-01. Every series is a candidate; r1 floats and fits
// in each, and step 7 stops on it. rota-rotarix and rota-rotateq are past
// their maxAgeComplete, but rota-rotavin has none, so not every candidate
// is certainly out: the data ⚪ stays.
func TestLevel4DoseCandidateWithoutMaxAgeComplete(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	got := assessmentOf(t, e, "rota", "2026-06-01", dayDate("2023-05-01"), rota("r1", unknownDate, ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: rotaAll, Estimated: true,
		Reasons: []engine.Reason{rotaRefs, {Code: engine.ReasonPreviousDoseDateUnknown, Params: engine.ReasonParams{Dose: "r1"}}},
	})
}

// TestOwnFNotMergedF (Q86): p (dose-2 minInterval 4w, maxAgeComplete 17w:
// L = 04-29) and q (8w, 18w: L = 05-06), both dose-1 minAge 6w; birth
// 2026-01-01, r1 with no product on 03-12 (10w), today 04-16 (15w). p's own
// F is today (E = 04-09), in; q's own F is E = 05-07, out. The merged F
// would put both out, but p can still be given today → ⚪
// CHECK_PRODUCT_NAME, not outOfAge.
func TestOwnFNotMergedF(t *testing.T) {
	t.Parallel()
	two := func(interval string) []ruleset.DoseRule {
		return []ruleset.DoseRule{rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"), rule(t, "", interval, "", ruleset.FromBirth, "P4M")}
	}
	s := oneDiseaseSnapshot([]string{"p", "q"}, qSeries(t, "p-1", "p", "", "P17W", two("P4W")), qSeries(t, "q-1", "q", "", "P18W", two("P8W")))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-04-16", birthRota, doseOf("q", "r1", dayDate("2026-03-12"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"p-1", "q-1"},
		Reasons: []engine.Reason{safest("p", "p-1", "q", "q-1"), {Code: engine.ReasonCheckProductName}},
	})
}

// TestMergedOutCandidate (issue #28 In scope 2c, Q86): a candidate that is
// in on its own F but past its limit at the merged F gives ⚪, not a capped
// date. p (dose-2 minInterval 4w, maxAgeComplete 18w: L = 05-06) and q (8w,
// 30w: L = 07-29), both dose-1 minAge 6w; birth 2026-01-01, r1 with no
// product on 03-12, today 04-16. p's own F is today (E = 04-09) and q's is
// E = 05-07, each by its own limit, so neither is out. The merged F 05-07 is
// past p's L: p could be given only before the date q needs → ⚪
// CHECK_PRODUCT_NAME. Merging anyway would show dose 2 from 05-07, past p's
// last day (P1).
func TestMergedOutCandidate(t *testing.T) {
	t.Parallel()
	two := func(interval string) []ruleset.DoseRule {
		return []ruleset.DoseRule{rule(t, "P6W", "", "", ruleset.FromBirth, "P2M"), rule(t, "", interval, "", ruleset.FromBirth, "P4M")}
	}
	s := oneDiseaseSnapshot([]string{"p", "q"}, qSeries(t, "p-1", "p", "", "P18W", two("P4W")), qSeries(t, "q-1", "q", "", "P30W", two("P8W")))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-04-16", birthRota, doseOf("q", "r1", dayDate("2026-03-12"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUnknown, SeriesUsed: []string{"p-1", "q-1"},
		Reasons: []engine.Reason{safest("p", "p-1", "q", "q-1"), {Code: engine.ReasonCheckProductName}},
	})
}

// TestMostLenientPlanForOut (Q88): a generic 2-dose series with two plans
// (dose 1 minAge 2m), maxAgeComplete 4m, birth 2026-01-21 (L = 05-20). Dose 2: plan a minInterval
// 4w and due previous + 4w; plan b minInterval 2w and due previous + 8w. r1
// on 05-01, today 05-10. Plan a: E = Du = 05-29, past L → dropped; plan b:
// E = 05-15, Du = 06-26, O = 07-03, kept. So the series is not out; A7's
// choice over plan b, capped at L_2 = L: {E 05-15, Du 05-20, O 05-20} →
// 🟡 with DUE_CAPPED_BY_MAX_AGE. With one plan kept there is no
// ALTERNATIVE_PLANS.
func TestMostLenientPlanForOut(t *testing.T) {
	t.Parallel()
	first := rule(t, "P2M", "", "", ruleset.FromBirth, "P3M")
	s := oneDiseaseSnapshot(nil, qSeries(t, "q-1", "", "", "P4M",
		[]ruleset.DoseRule{first, rule(t, "", "P4W", "", ruleset.FromPreviousDose, "P4W")},
		[]ruleset.DoseRule{first, rule(t, "", "P2W", "", ruleset.FromPreviousDose, "P8W")}))
	e := newEngine(t, s)
	got := assessmentOf(t, e, "q", "2026-05-10", dayDate("2026-01-21"), doseOf("q", "r1", dayDate("2026-05-01"), ""))
	checkAssessment(t, got, engine.Assessment{
		Disease: "q", Bucket: engine.BucketUpcoming, NextDose: nextDose(t, 2, "2026-05-15", "2026-05-20", "2026-05-20"),
		SeriesUsed: []string{"q-1"}, Reasons: []engine.Reason{capped("2026-05-20")},
	})
}

// TestCountedViewCountsEveryRecord (Q85): rotarix doses on 02-05 (before
// 6w@b = 02-12 → DOSE_INVALID{minAge}) and 03-01, today 07-01. As walked,
// r2 is dose 1 and dose 2 is past L = 06-17. In the counted view r1 counts
// at 1 and r2 at 2 (although only 24 days later), so the series is
// complete: the data ⚪ stays (its masked dose 2 is 🔴, with no reason), not
// outOfAge.
func TestCountedViewCountsEveryRecord(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-max-age")
	got := assessmentOf(t, e, "rota", "2026-07-01", birthRota, rota("r1", dayDate("2026-02-05"), "rotarix"), rota("r2", dayDate("2026-03-01"), "rotarix"))
	checkAssessment(t, got, engine.Assessment{
		Disease: "rota", Bucket: engine.BucketUnknown, SeriesUsed: []string{"rota-rotarix"},
		Reasons: []engine.Reason{invalidDose("r1", 1, engine.CheckMinAge)},
	})
}
