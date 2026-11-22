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
