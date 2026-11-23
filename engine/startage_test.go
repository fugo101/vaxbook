package engine_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Engine tests of start-age brackets outside the goldens (issue #23). Every
// in-test snapshot passes ruleset.Validate with no Reject.

func spanOf(t testing.TB, s string) *ruleset.Span {
	t.Helper()
	v, err := ruleset.ParseSpan(s)
	if err != nil {
		t.Fatal(err)
	}
	return &v
}

// newEngine builds an Engine from s and fails unless Validate gives exactly
// the Warn codes in warns (and no Reject).
func newEngine(t testing.TB, s *ruleset.Snapshot, warns ...ruleset.ProblemCode) *engine.Engine {
	t.Helper()
	var got []ruleset.ProblemCode
	for _, p := range ruleset.Validate(s) {
		got = append(got, p.Code)
		if p.Severity == ruleset.Reject {
			t.Fatalf("in-test snapshot rejected: %v", p)
		}
	}
	if !slices.Equal(got, warns) {
		t.Fatalf("Validate gives %v, want %v", got, warns)
	}
	e, err := engine.New(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// hpvAssessment evaluates one profile and returns its hpv assessment.
func hpvAssessment(t testing.TB, e *engine.Engine, today, birth string, doses ...engine.DoseRecord) engine.Assessment {
	t.Helper()
	in := engine.Input{Today: mustDate(t, today), Profile: engine.Profile{Birth: engine.RecalledDate{Value: birth, Precision: engine.Day}}, Doses: doses}
	if doses == nil {
		in.Doses = []engine.DoseRecord{}
	}
	out, b := evaluate(t, e, in)
	for _, a := range out.Assessments {
		checkAssessmentShape(t, "in-test", a)
		if a.Disease == "hpv" {
			return a
		}
	}
	t.Fatalf("no hpv assessment in %s", b)
	return engine.Assessment{}
}

func hpvDose(id, date, product string) engine.DoseRecord {
	r := engine.DoseRecord{ID: id, Diseases: []string{"hpv"}, Date: engine.RecalledDate{Value: date, Precision: engine.Day}}
	if product != "" {
		r.Product = &product
	}
	return r
}

func nextDose(t testing.TB, n int, earliest, due, overdue string) *engine.NextDose {
	t.Helper()
	return &engine.NextDose{Number: n, Earliest: mustDate(t, earliest), Due: mustDate(t, due), Overdue: mustDate(t, overdue)}
}

func checkAssessment(t *testing.T, got, want engine.Assessment) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assessment\n  %+v\nwant\n  %+v", got, want)
	}
}

// TestStartAgeDiffersExactDates (Q43): when the dose-1 minimum age differs
// between the brackets of a group (the FIRST_DOSE_MIN_AGE_DIFFERS Warn), two
// brackets can match with exact dates. 9–14 has dose-1 min age 9y and 15–45
// has 12y. Birth 2000-01-01; doses at 10y (2010-01-01) and 15y1m
// (2015-02-01); today 2015-03-01.
//   - Under 9–14, dose 1 is the 10y dose, inside [9y, 15y).
//   - Under 15–45, the 10y dose fails min age 12y, and dose 1 is the 15y1m
//     dose, inside [15y, 46y).
//
// Both match → 15–45 (most doses), AGE_AT_START_UNCERTAIN, not estimated,
// and DOSE_INVALID{minAge} on the 10y dose (⚪).
func TestStartAgeDiffersExactDates(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-start-age")
	s.Series[1].Plans[0].Doses[0].MinAge = spanOf(t, "P12Y")
	e := newEngine(t, s, ruleset.FirstDoseMinAgeDiffers)
	got := hpvAssessment(t, e, "2015-03-01", "2000-01-01", hpvDose("r1", "2010-01-01", "gardasil9"), hpvDose("r2", "2015-02-01", "gardasil9"))
	checkAssessment(t, got, engine.Assessment{
		Disease:    "hpv",
		Bucket:     engine.BucketUnknown,
		SeriesUsed: []string{"hpv-g9-15to45"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonAgeAtStartUncertain, Params: engine.ReasonParams{Candidates: []engine.SeriesRef{
				{Product: "gardasil9", Series: "hpv-g9-9to14"}, {Product: "gardasil9", Series: "hpv-g9-15to45"}}}},
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r1", Number: 1}},
		},
	})
}

// TestStartAgeNoDoseCountedFirstAtPositionOne (issue #23 In scope 4, Q32):
// when no bracket counts a dose as dose 1, the fallback is one dose, the
// one at position 1, so every bracket of a group with the same dose-1
// minimum age reads the same record and exact dates match at most one
// bracket. Both brackets get dose-1 min age 16y (only the two
// FIRST_DOSE_MIN_AGE_ABOVE_START_AGE Warns). Birth 2000-01-01; doses at 12y
// (2012-01-01) and 15y6m (2015-07-01); today 2016-01-01.
//   - Both doses fail min age 16y under either bracket, so none is counted,
//     and each is tried at position 1 in turn.
//   - The dose at position 1 is the first one tried, r1 at 12y: only 9–14
//     ([9y, 15y)) contains it. The later r2 at 15y6m does not make 15–45 a
//     second match.
//
// → 9–14, no AGE_AT_START_UNCERTAIN, not estimated, DOSE_INVALID{minAge}
// on both doses (⚪).
func TestStartAgeNoDoseCountedFirstAtPositionOne(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-start-age")
	for i := range s.Series {
		s.Series[i].Plans[0].Doses[0].MinAge = spanOf(t, "P16Y")
	}
	e := newEngine(t, s, ruleset.FirstDoseMinAgeAboveStartAge, ruleset.FirstDoseMinAgeAboveStartAge)
	got := hpvAssessment(t, e, "2016-01-01", "2000-01-01", hpvDose("r1", "2012-01-01", "gardasil9"), hpvDose("r2", "2015-07-01", "gardasil9"))
	checkAssessment(t, got, engine.Assessment{
		Disease:    "hpv",
		Bucket:     engine.BucketUnknown,
		SeriesUsed: []string{"hpv-g9-9to14"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r1", Number: 1}},
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r2", Number: 1}},
		},
	})
}

// TestStartAgeDiffersNoMatchUsesEarliestCandidate (Q43, A5): when the
// dose-1 minimum age differs between the brackets of a group (the
// FIRST_DOSE_MIN_AGE_DIFFERS Warn), each bracket walks its own dose 1, so
// nothing can match while a candidate is younger than every max. The
// no-match rule then uses a, the earliest dose-1 candidate across the
// group: the first bracket whose max is after a (else the latest max).
// Disease w, one generic group:
//
//	series  startAge      doses  dose-1 minAge
//	w-a     [18m, 2y)     3      18m
//	w-b     [2y, 5y)      2      1y
//
// Birth 2020-01-01; doses x 2021-04-01 (15m) and y 2023-01-01 (3y); today
// 2026-11-23; every date exact.
//   - Under w-a, x fails min age 18m, so dose 1 is y at 3y, at or past w-a's
//     max 2y: no match.
//   - Under w-b, dose 1 is x at 15m, below w-b's reach [2y, 5y): no match.
//
// a is x at 15m, before w-a's max 2y → w-a, the bracket whose reach holds
// 15m: x is DOSE_INVALID{minAge, x, 1} (⚪), not dose 1 of w-b (which would
// be a false ✅ COMPLETE).
func TestStartAgeDiffersNoMatchUsesEarliestCandidate(t *testing.T) {
	t.Parallel()
	dose := func(minAge, minInterval, offset string, from ruleset.Anchor) ruleset.DoseRule {
		d := ruleset.DoseRule{Recommended: ruleset.Milestone{From: from, Offset: *spanOf(t, offset)}}
		if minAge != "" {
			d.MinAge = spanOf(t, minAge)
		}
		if minInterval != "" {
			d.MinInterval = spanOf(t, minInterval)
		}
		return d
	}
	s := &ruleset.Snapshot{
		Version:  "w-1",
		Diseases: []ruleset.Disease{{ID: "w", Name: "Bệnh minh hoạ W"}},
		Series: []ruleset.Series{
			{
				ID: "w-a", Disease: "w", Stage: ruleset.Childhood,
				StartAge: &ruleset.StartAge{Min: *spanOf(t, "P18M"), Max: spanOf(t, "P2Y")},
				Plans: []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{
					dose("P18M", "", "P18M", ruleset.FromBirth),
					dose("", "P4W", "P2M", ruleset.FromPreviousDose),
					dose("", "P4W", "P2M", ruleset.FromPreviousDose),
				}}},
			},
			{
				ID: "w-b", Disease: "w", Stage: ruleset.Childhood,
				StartAge: &ruleset.StartAge{Min: *spanOf(t, "P2Y"), Max: spanOf(t, "P5Y")},
				Plans: []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{
					dose("P1Y", "", "P2Y", ruleset.FromBirth),
					dose("", "P4W", "P2M", ruleset.FromPreviousDose),
				}}},
			},
		},
	}
	e := newEngine(t, s, ruleset.FirstDoseMinAgeDiffers)
	w := func(id, date string) engine.DoseRecord {
		return engine.DoseRecord{ID: id, Diseases: []string{"w"}, Date: engine.RecalledDate{Value: date, Precision: engine.Day}}
	}
	out, _ := evaluate(t, e, engine.Input{
		Today:   mustDate(t, "2026-11-23"),
		Profile: engine.Profile{Birth: engine.RecalledDate{Value: "2020-01-01", Precision: engine.Day}},
		Doses:   []engine.DoseRecord{w("x", "2021-04-01"), w("y", "2023-01-01")},
	})
	checkAssessment(t, out.Assessments[0], engine.Assessment{
		Disease:    "w",
		Bucket:     engine.BucketUnknown,
		SeriesUsed: []string{"w-a"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "x", Number: 1}},
		},
	})
}

// TestStartAgeDiffersNoMatchBelowGap (Q43, A5): the no-match rule with a
// gap, on minimal-start-age-gap with gap-b's dose-1 min age lowered to 6m
// (the FIRST_DOSE_MIN_AGE_DIFFERS Warn). Birth 2020-01-01; doses r1
// 2020-09-01 (8m) and r2 2022-07-01 (2y6m); today 2026-11-01; every date
// exact.
//   - Under gap-a and gap-c, r1 fails min age 1y, so dose 1 is r2 at 2y6m:
//     outside gap-a's reach [birth, 2y) and gap-c's [5y, —).
//   - Under gap-b, dose 1 is r1 at 8m, below gap-b's reach [2y, 5y).
//
// Nothing matches; a is r1 at 8m, before gap-a's max 2y → gap-a, not the
// last bracket gap-c. Under gap-a r1 is DOSE_INVALID{minAge, r1, 1} (⚪).
func TestStartAgeDiffersNoMatchBelowGap(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "minimal-start-age-gap")
	s.Series[1].Plans[0].Doses[0].MinAge = spanOf(t, "P6M")
	e := newEngine(t, s, ruleset.FirstDoseMinAgeDiffers)
	gap := func(id, date string) engine.DoseRecord {
		return engine.DoseRecord{ID: id, Diseases: []string{"gap"}, Date: engine.RecalledDate{Value: date, Precision: engine.Day}}
	}
	out, _ := evaluate(t, e, engine.Input{
		Today:   mustDate(t, "2026-11-01"),
		Profile: engine.Profile{Birth: engine.RecalledDate{Value: "2020-01-01", Precision: engine.Day}},
		Doses:   []engine.DoseRecord{gap("r1", "2020-09-01"), gap("r2", "2022-07-01")},
	})
	checkAssessment(t, out.Assessments[0], engine.Assessment{
		Disease:    "gap",
		Bucket:     engine.BucketUnknown,
		SeriesUsed: []string{"gap-a"},
		Reasons: []engine.Reason{
			{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinAge}, Dose: "r1", Number: 1}},
		},
	})
}

// withHPVOther adds product hpv-other [hpv] to the illustrative-start-age
// snapshot.
func withHPVOther(s *ruleset.Snapshot) {
	s.Products = append(s.Products, ruleset.Product{ID: "hpv-other", Name: "HPV khác", Diseases: []string{"hpv"}})
}

// TestTierBeforeStartAge (Q39): the product's own series come first, and its
// brackets are matched with no fallback to the generic series. The gardasil9
// group plus a generic hpv-generic (no startAge) used by product hpv-other.
// One exact gardasil9 dose at 50y (birth 1970-01-01, dose 2020-01-01, today
// 2020-06-01): no bracket matches and the dose is past every max, so the
// latest-max bracket 15–45 is kept and the dose is counted normally (A5,
// C3). Dose 2: E = r1 + 4w = 01-29, Du = r1 + 2m = 03-01, O = 03-08 → 🔴.
//
// S4 changes this case: the dose is after 15–45's startAge.max, so it gets
// DOSE_AFTER_MAX_AGE (issue #23 Q45, #15).
func TestTierBeforeStartAge(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-start-age")
	withHPVOther(s)
	generic := s.Series[0]
	generic.ID, generic.Product, generic.StartAge = "hpv-generic", nil, nil
	s.Series = append(s.Series, generic)
	e := newEngine(t, s)
	got := hpvAssessment(t, e, "2020-06-01", "1970-01-01", hpvDose("r1", "2020-01-01", "gardasil9"))
	checkAssessment(t, got, engine.Assessment{
		Disease:    "hpv",
		Bucket:     engine.BucketOverdue,
		NextDose:   nextDose(t, 2, "2020-01-29", "2020-03-01", "2020-03-08"),
		SeriesUsed: []string{"hpv-g9-15to45"},
		Products:   []string{"gardasil9"},
	})
}

// TestEveryBracketedSeriesRetired (Q38 with brackets): gardasil9 retired,
// birth 2014-01-01, no dose, today 2026-11-10 (12y10m). Every series is
// retired, so the Q38 fallback keeps both, and 9–14 is the bracket open at
// that age: 🔴 dose 1 {E = Du = b + 9y = 2023-01-01, O 2023-01-08}, with no
// products (a retired product is never offered), not outOfAge.
func TestEveryBracketedSeriesRetired(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-start-age")
	s.Products[0].Retired = true
	e := newEngine(t, s)
	got := hpvAssessment(t, e, "2026-11-10", "2014-01-01")
	checkAssessment(t, got, engine.Assessment{
		Disease:    "hpv",
		Bucket:     engine.BucketOverdue,
		NextDose:   nextDose(t, 1, "2023-01-01", "2023-01-01", "2023-01-08"),
		SeriesUsed: []string{"hpv-g9-9to14"},
	})
}

// TestPartialRetirementFallsBackAfterBrackets (Q50, option (b)): gardasil9
// retired, and a non-retired hpv-other with one series hpv-other-9to14
// ([9y, 15y), the 9–14 doses). Birth 2006-11-01, no dose, today 2026-11-10
// (20y): the only non-retired group is past its bracket, so the bracket
// choice is redone over every group, retired ones included → 🔴 dose 1 on
// hpv-g9-15to45 {E = b + 9y = 2015-11-01, Du = b + 15y = 2021-11-01, O
// 2021-11-08}, no products. Under option (a) the result would be outOfAge
// with OUT_OF_AGE.
func TestPartialRetirementFallsBackAfterBrackets(t *testing.T) {
	t.Parallel()
	s := loadSnapshot(t, "illustrative-start-age")
	s.Products[0].Retired = true
	withHPVOther(s)
	other := s.Series[0]
	other.ID, other.Product = "hpv-other-9to14", new("hpv-other")
	s.Series = append(s.Series, other)
	e := newEngine(t, s)
	got := hpvAssessment(t, e, "2026-11-10", "2006-11-01")
	checkAssessment(t, got, engine.Assessment{
		Disease:    "hpv",
		Bucket:     engine.BucketOverdue,
		NextDose:   nextDose(t, 1, "2015-11-01", "2021-11-01", "2021-11-08"),
		SeriesUsed: []string{"hpv-g9-15to45"},
	})
}

// TestNarrowingChangesBracket pins the two examples of issue #23 (Q44 and
// its extension) for which narrowing a dose date changes the bracket, and
// with it seriesUsed: the narrowing properties are checked only when
// seriesUsed is unchanged. Birth 2010-06-15 (exact), gardasil9 dose r1 in
// 2025-06 (month).
//   - Today 2025-07-15, r1 alone. Wide: r1 may be before or after 15y
//     (2025-06-15), so both brackets match and 15–45 is used: dose 2 E =
//     06-30 + 4w = 07-28, Du = 06-30 + 2m = 08-30. Narrowed to 2025-06-10,
//     only 9–14 matches: E = 06-10 + 5m = 11-10, Du = 06-10 + 6m = 12-10,
//     later with the same k and no ⚪.
//   - Today 2025-09-01, r2 2025-08-15. Wide: 15–45, and r2 ≥ 06-30 + 4w =
//     07-28 is counted. Narrowed to 2025-06-10: 9–14, and r2 < 06-10 + 5m =
//     11-10 → DOSE_INVALID{minInterval, r2, 2}.
func TestNarrowingChangesBracket(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "illustrative-start-age")
	month := hpvDose("r1", "2025-06", "gardasil9")
	month.Date.Precision = engine.Month
	narrow := hpvDose("r1", "2025-06-10", "gardasil9")

	wide := hpvAssessment(t, e, "2025-07-15", "2010-06-15", month)
	got := hpvAssessment(t, e, "2025-07-15", "2010-06-15", narrow)
	if !slices.Equal(wide.SeriesUsed, []string{"hpv-g9-15to45"}) || !slices.Equal(got.SeriesUsed, []string{"hpv-g9-9to14"}) ||
		!reflect.DeepEqual(wide.NextDose, nextDose(t, 2, "2025-07-28", "2025-08-30", "2025-09-06")) ||
		!reflect.DeepEqual(got.NextDose, nextDose(t, 2, "2025-11-10", "2025-12-10", "2025-12-17")) {
		t.Errorf("dates example:\nwide   %+v\nnarrow %+v", wide, got)
	}

	r2 := hpvDose("r2", "2025-08-15", "gardasil9")
	wide = hpvAssessment(t, e, "2025-09-01", "2010-06-15", month, r2)
	got = hpvAssessment(t, e, "2025-09-01", "2010-06-15", narrow, r2)
	wantInvalid := []engine.Reason{{Code: engine.ReasonDoseInvalid, Params: engine.ReasonParams{Checks: []engine.Check{engine.CheckMinInterval}, Dose: "r2", Number: 2}}}
	if !slices.Equal(wide.SeriesUsed, []string{"hpv-g9-15to45"}) || len(reasonsWith(wide, engine.ReasonDoseInvalid)) > 0 ||
		!slices.Equal(got.SeriesUsed, []string{"hpv-g9-9to14"}) || !reflect.DeepEqual(reasonsWith(got, engine.ReasonDoseInvalid), wantInvalid) {
		t.Errorf("validity example:\nwide   %+v\nnarrow %+v", wide, got)
	}
}

// TestNoDoseThreeSubRanges (SAD §5.3 step 4; PR #27, issue #23): a no-dose
// birth range split into three sub-ranges, each with its own start-age
// choice. Disease tri, every dose 1 at min age 6m, one dose per series:
//
//	series  product  startAge      dose 1 recommended at
//	g-a     generic  [6m, 15m)     6m
//	g-b     generic  [15m, 5y)     15m
//	x-a     tri-x    [6m, 18m)     6m
//	x-b     tri-x    [18m, 6y)     18m
//
// Birth year 2024, today 2026-02-15. x-a closes for births up to 2024-08-15
// (2024-08-15 + 18m = today) and g-a for births up to 2024-11-15, so the
// sub-ranges are [01-01, 08-15] → [g-b, x-b], [08-16, 11-15] → [g-b, x-a]
// and [11-16, 12-31] → [g-a, x-a]. 2024-01-01 is 🔴 (x-b Du = b + 18m =
// 2025-07-01, O 07-08), and 2024-08-15 is 🟠 (x-b Du = 2026-02-15 =
// today), so the result is ⚪ BIRTH_DATE_IMPRECISE{birthMonth} with every
// sub-range's SAFEST_SERIES, latest first, and the latest end's seriesUsed
// [g-a, x-a]. Comparing the two birth ends alone gives 🔴 {1, 2025-06-30,
// 2025-06-30, 2025-07-07}: both are overdue.
func TestNoDoseThreeSubRanges(t *testing.T) {
	t.Parallel()
	series := func(id, product, minAge, maxAge string) ruleset.Series {
		sr := ruleset.Series{
			ID: id, Disease: "tri", Stage: ruleset.Childhood,
			StartAge: &ruleset.StartAge{Min: *spanOf(t, minAge), Max: spanOf(t, maxAge)},
			Plans: []ruleset.Plan{{ID: "main", Doses: []ruleset.DoseRule{{
				MinAge:      spanOf(t, "P6M"),
				Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: *spanOf(t, minAge)},
			}}}},
		}
		if product != "" {
			sr.Product = &product
		}
		return sr
	}
	s := &ruleset.Snapshot{
		Version:  "tri-1",
		Diseases: []ruleset.Disease{{ID: "tri", Name: "Bệnh minh hoạ ba khoảng"}},
		Products: []ruleset.Product{{ID: "tri-x", Name: "Vắc xin minh hoạ X", Diseases: []string{"tri"}}},
		Series: []ruleset.Series{
			series("g-a", "", "P6M", "P15M"),
			series("g-b", "", "P15M", "P5Y"),
			series("x-a", "tri-x", "P6M", "P18M"),
			series("x-b", "tri-x", "P18M", "P6Y"),
		},
	}
	e := newEngine(t, s)
	out, _ := evaluate(t, e, engine.Input{
		Today:   mustDate(t, "2026-02-15"),
		Profile: engine.Profile{Birth: engine.RecalledDate{Value: "2024", Precision: engine.Year}},
		Doses:   []engine.DoseRecord{},
	})
	safest := func(generic, own string) engine.Reason {
		return engine.Reason{Code: engine.ReasonSafestSeries, Params: engine.ReasonParams{Candidates: []engine.SeriesRef{{Series: generic}, {Product: "tri-x", Series: own}}}}
	}
	checkAssessment(t, out.Assessments[0], engine.Assessment{
		Disease:    "tri",
		Bucket:     engine.BucketUnknown,
		SeriesUsed: []string{"g-a", "x-a"},
		Estimated:  true,
		Reasons: []engine.Reason{
			{Code: engine.ReasonBirthDateImprecise, Params: engine.ReasonParams{Hint: engine.HintBirthMonth}},
			safest("g-a", "x-a"),
			safest("g-b", "x-a"),
			safest("g-b", "x-b"),
		},
	})
}
