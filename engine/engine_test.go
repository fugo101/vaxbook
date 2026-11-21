package engine

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// testSnapshot: diseases a and b; a has alternative plans whose dose-2 due
// dates tie (both at birth + 4m) with different earliest dates.
const testSnapshot = `{
  "version": "test-1",
  "diseases": [{"id": "a", "name": "A"}, {"id": "b", "name": "B"}],
  "series": [
    {"id": "a-series", "disease": "a", "stage": "childhood", "plans": [
      {"id": "short", "doses": [
        {"minAge": "6w", "recommended": {"from": "birth", "offset": "2m"}},
        {"minInterval": "4w", "recommended": {"from": "birth", "offset": "4m"}}]},
      {"id": "long", "doses": [
        {"minAge": "6w", "recommended": {"from": "birth", "offset": "2m"}},
        {"minInterval": "6w", "recommended": {"from": "birth", "offset": "4m"}}]}
    ]},
    {"id": "b-series", "disease": "b", "stage": "childhood", "plans": [
      {"id": "main", "doses": [{"minAge": "6w", "recommended": {"from": "birth", "offset": "2m"}}]}
    ]}
  ]
}`

func testEngine(t *testing.T) *Engine {
	t.Helper()
	s, err := ruleset.Decode([]byte(testSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func day(v string) RecalledDate { return RecalledDate{Value: v, Precision: Day} }

func dose(id, date string, diseases ...string) DoseRecord {
	return DoseRecord{ID: id, Diseases: diseases, Date: day(date)}
}

func input(t *testing.T, today, birth string, doses ...DoseRecord) Input {
	t.Helper()
	return Input{Today: d(t, today), Profile: Profile{Birth: day(birth)}, Doses: doses}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	in := Input{
		Profile: Profile{Birth: RecalledDate{Value: "2026", Precision: Year}},
		Doses: []DoseRecord{
			{ID: "", Diseases: nil, Date: RecalledDate{Value: "2026-01-01"}},
			{ID: "x", Diseases: []string{"a", "zz", "a"}, Date: RecalledDate{Value: "2026", Precision: "decade"}},
			{ID: "x", Diseases: []string{"b"}, Date: day("2026-02-30")},
			{ID: "y", Diseases: []string{"b"}, Date: RecalledDate{Precision: None}},
			{ID: "z", Diseases: []string{"b"}, Date: RecalledDate{Value: "2026-03", Precision: Month}},
		},
	}
	want := []Problem{
		{Code: Required, Path: "/today"},
		{Code: UnsupportedPrecision, Path: "/profile/birth/precision", Params: map[string]string{"precision": "year"}},
		{Code: Required, Path: "/doses/0/id"},
		{Code: Required, Path: "/doses/0/diseases"},
		{Code: Required, Path: "/doses/0/date/precision"},
		{Code: UnknownDisease, Path: "/doses/1/diseases/1", Params: map[string]string{"disease": "zz"}},
		{Code: DuplicateValue, Path: "/doses/1/diseases/2", Params: map[string]string{"disease": "a"}},
		{Code: InvalidValue, Path: "/doses/1/date/precision", Params: map[string]string{"value": "decade"}},
		{Code: DuplicateID, Path: "/doses/2/id", Params: map[string]string{"id": "x"}},
		{Code: InvalidDate, Path: "/doses/2/date/value"},
		{Code: InvalidValue, Path: "/doses/3/date/precision", Params: map[string]string{"value": "none"}},
		{Code: UnsupportedPrecision, Path: "/doses/4/date/precision", Params: map[string]string{"precision": "month"}},
	}
	if got := e.Validate(in); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate =\n  %v\nwant\n  %v", got, want)
	}
}

func TestValidateDates(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	cases := []struct {
		name string
		in   Input
		want []Problem
	}{
		{"valid", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-03-10", "a")), nil},
		{"dose on birth day and on today", input(t, "2026-04-01", "2026-01-10",
			dose("r1", "2026-01-10", "a"), dose("r2", "2026-04-01", "a")), nil},
		{"invalid birth", input(t, "2026-04-01", "2026-02-30"),
			[]Problem{{Code: InvalidDate, Path: "/profile/birth/value"}}},
		{"birth in future", input(t, "2026-04-01", "2026-04-02"),
			[]Problem{{Code: DateInFuture, Path: "/profile/birth/value"}}},
		{"dose in future", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-04-02", "a")),
			[]Problem{{Code: DateInFuture, Path: "/doses/0/date/value"}}},
		{"dose before birth", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-01-09", "a")),
			[]Problem{{Code: DateBeforeBirth, Path: "/doses/0/date/value"}}},
		{"today out of range", input(t, "2200-01-01", "2026-01-10"),
			[]Problem{{Code: OutOfRange, Path: "/today"}}},
		{"birth out of range", input(t, "2026-04-01", "1899-12-31"),
			[]Problem{{Code: OutOfRange, Path: "/profile/birth/value"}}},
		{"garbage dose date", input(t, "2026-04-01", "2026-01-10", dose("r1", "abc", "a")),
			[]Problem{{Code: InvalidDate, Path: "/doses/0/date/value"}}},
		{"missing birth precision", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01-10"}}},
			[]Problem{{Code: Required, Path: "/profile/birth/precision"}}},
		// "unknown" is a dose-record level only (F03, GLOSSARY "Date
		// precision"): no engine version evaluates it for a birth date.
		{"birth precision unknown", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Precision: Unknown}}},
			[]Problem{{Code: InvalidValue, Path: "/profile/birth/precision", Params: map[string]string{"value": "unknown"}}}},
		// Each field's own level 4 is reserved for S2, so it is unsupported
		// rather than invalid.
		{"birth precision none", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Precision: None}}},
			[]Problem{{Code: UnsupportedPrecision, Path: "/profile/birth/precision", Params: map[string]string{"precision": "none"}}}},
		{"dose precision unknown", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Precision: Unknown}}),
			[]Problem{{Code: UnsupportedPrecision, Path: "/doses/0/date/precision", Params: map[string]string{"precision": "unknown"}}}},
		{"birth precision month", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01", Precision: Month}}},
			[]Problem{{Code: UnsupportedPrecision, Path: "/profile/birth/precision", Params: map[string]string{"precision": "month"}}}},
	}
	for _, c := range cases {
		if got := e.Validate(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Validate =\n  %v\nwant\n  %v", c.name, got, c.want)
		}
	}
}

// malformedDates are recalled date values that do not parse. Each is
// INVALID_DATE: on a dose record its diseases become ⚪, on the birth date
// Evaluate returns an input error (A9, EQ27).
var malformedDates = []string{
	"abc", "", "2026-02-30", "10/02/2026", "2026-1-5", " 2026-01-01", "2026-01-01 ",
	"2026-01-01T00:00:00Z", "２０２６-０１-０１", "2026-01", "2026", strings.Repeat("9", 200),
}

func TestEvaluateInputErrors(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	for _, v := range malformedDates {
		_, err := e.Evaluate(input(t, "2026-04-01", v))
		var ie *InputError
		want := []Problem{{Code: InvalidDate, Path: "/profile/birth/value"}}
		if !errors.As(err, &ie) || !reflect.DeepEqual(ie.Problems, want) {
			t.Errorf("birth %q: Evaluate error = %v, want INVALID_DATE at /profile/birth/value", v, err)
		}
	}
	for name, in := range map[string]Input{
		"invalid birth":         input(t, "2026-04-01", "2026-02-30"),
		"unsupported precision": {Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01", Precision: Month}}},
		"future dose":           input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-04-02", "a")),
		"dose before birth":     input(t, "2026-04-01", "2026-01-10", dose("r1", "2025-12-31", "a")),
		"unknown disease":       input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-03-01", "x")),
		"missing today":         {Profile: Profile{Birth: day("2026-01-10")}},
	} {
		_, err := e.Evaluate(in)
		var ie *InputError
		if !errors.As(err, &ie) || len(ie.Problems) == 0 {
			t.Errorf("%s: Evaluate error = %v, want *InputError", name, err)
		}
	}
}

func TestEvaluateGarbageDoseDateIsInvalidDate(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	for _, v := range append(malformedDates, "\x00") {
		out, err := e.Evaluate(input(t, "2026-04-01", "2026-01-10", dose("r1", v, "a")))
		if err != nil {
			t.Errorf("dose date %q: Evaluate error %v, want ⚪ INVALID_DATE", v, err)
			continue
		}
		want := []Reason{{Code: ReasonInvalidDate, Params: map[string]string{"dose": "r1"}}}
		if a := out.Assessments[0]; a.Bucket != BucketUnknown || a.NextDose != nil || !reflect.DeepEqual(a.Reasons, want) {
			t.Errorf("dose date %q: assessment = %+v", v, a)
		}
	}
	// A missing value with precision day is the empty string.
	out, err := e.Evaluate(input(t, "2026-04-01", "2026-01-10", DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Precision: Day}}))
	if err != nil || out.Assessments[0].Bucket != BucketUnknown {
		t.Errorf("absent dose date value: %+v, %v", out, err)
	}
	out, err = e.Evaluate(input(t, "2026-04-01", "2026-01-10", dose("r1", "abc", "a"), dose("r0", "", "a")))
	if err != nil {
		t.Fatal(err)
	}
	a := out.Assessments[0]
	want := []Reason{
		{Code: ReasonInvalidDate, Params: map[string]string{"dose": "r0"}},
		{Code: ReasonInvalidDate, Params: map[string]string{"dose": "r1"}},
	}
	if a.Bucket != BucketUnknown || a.NextDose != nil || !reflect.DeepEqual(a.Reasons, want) {
		t.Errorf("assessment = %+v", a)
	}
	if b := out.Assessments[1]; b.Bucket == BucketUnknown {
		t.Errorf("disease b has no invalid record but is %+v", b)
	}
}

// Alternative plans whose due dates tie: the merged earliest date is the
// latest of the tied plans' earliest dates (P1-conservative; SAD silent).
func TestAlternativePlansTieTakesLatestEarliest(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	out, err := e.Evaluate(input(t, "2026-03-20", "2026-01-10", dose("r1", "2026-03-10", "a")))
	if err != nil {
		t.Fatal(err)
	}
	// short: E = 03-10 + 4w = 04-07; long: E = 03-10 + 6w = 04-21; both Du = 05-10.
	want := NextDose{Number: 2, Earliest: d(t, "2026-04-21"), Due: d(t, "2026-05-10"), Overdue: d(t, "2026-05-17")}
	if nd := out.Assessments[0].NextDose; nd == nil || *nd != want {
		t.Errorf("nextDose = %+v, want %+v", nd, want)
	}
}

// Minimums are inclusive: a dose on birth + minAge or on previous + interval
// counts, one day earlier does not.
func TestMinimumsAreInclusive(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	cases := []struct {
		doses   []DoseRecord
		invalid string
	}{
		{[]DoseRecord{dose("r1", "2026-02-21", "b")}, ""},
		{[]DoseRecord{dose("r1", "2026-02-20", "b")}, "minAge"},
		{[]DoseRecord{dose("r1", "2026-02-21", "a"), dose("r2", "2026-03-21", "a")}, ""},
		{[]DoseRecord{dose("r1", "2026-02-21", "a"), dose("r2", "2026-03-20", "a")}, "minInterval"},
	}
	for i, c := range cases {
		out, err := e.Evaluate(input(t, "2026-06-01", "2026-01-10", c.doses...))
		if err != nil {
			t.Fatal(err)
		}
		var check string
		for _, a := range out.Assessments {
			for _, r := range a.Reasons {
				if r.Code == ReasonDoseInvalid {
					check = r.Params["check"]
				}
			}
		}
		if check != c.invalid {
			t.Errorf("case %d: DOSE_INVALID check = %q, want %q", i, check, c.invalid)
		}
	}
}

// Doses beyond the series length are ignored in S1 (boosters and extra
// doses arrive in S6).
func TestDosesBeyondSeriesLengthIgnored(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	out, err := e.Evaluate(input(t, "2026-06-01", "2026-01-10",
		dose("r1", "2026-03-01", "b"), dose("r2", "2026-03-02", "b")))
	if err != nil {
		t.Fatal(err)
	}
	b := out.Assessments[1]
	if b.Bucket != BucketComplete || len(b.Reasons) != 1 || b.Reasons[0].Code != ReasonComplete {
		t.Errorf("assessment = %+v, want complete with COMPLETE only", b)
	}
}

func TestNewRejectsInvalidSnapshot(t *testing.T) {
	t.Parallel()
	s, err := ruleset.Decode([]byte(strings.Replace(testSnapshot, `"6w"`, `"6w", "minInterval": "1w"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(s)
	var re *ruleset.RejectedError
	if !errors.As(err, &re) || len(re.Problems) == 0 {
		t.Fatalf("New error = %v, want *ruleset.RejectedError", err)
	}
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded")
	}
}

// A Snapshot built in Go skips ParseSpan's checks. New must still reject a
// span the date arithmetic cannot handle, instead of Evaluate panicking or
// emitting dates beyond four-digit years.
func TestNewRejectsGoBuiltSnapshotWithBadSpan(t *testing.T) {
	t.Parallel()
	for name, bad := range map[string]ruleset.Span{
		"zero span":     {},
		"too long":      {N: 100000, Unit: ruleset.Years},
		"negative span": {N: -5, Unit: ruleset.Days},
	} {
		s, err := ruleset.Decode([]byte(testSnapshot))
		if err != nil {
			t.Fatal(err)
		}
		s.Series[0].Plans[0].Doses[0].MinAge = &bad
		_, err = New(s)
		var re *ruleset.RejectedError
		if !errors.As(err, &re) {
			t.Errorf("%s: New error = %v, want *ruleset.RejectedError", name, err)
		}
	}
}

func TestNewRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()
	for name, snap := range map[string]string{
		"disease without series": `{"version": "v", "diseases": [{"id": "a", "name": "A"}], "series": []}`,
		"two series for a disease": `{"version": "v", "diseases": [{"id": "a", "name": "A"}], "series": [
		  {"id": "s1", "disease": "a", "stage": "childhood", "plans": [{"id": "p", "doses": [{"minAge": "0d", "recommended": {"from": "birth", "offset": "0d"}}]}]},
		  {"id": "s2", "disease": "a", "stage": "adult", "plans": [{"id": "p", "doses": [{"minAge": "0d", "recommended": {"from": "birth", "offset": "0d"}}]}]}]}`,
	} {
		s, err := ruleset.Decode([]byte(snap))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := New(s); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: New error = %v, want ErrUnsupported", name, err)
		}
	}
}

// New copies what it needs: changing the snapshot afterwards does not change
// the engine.
func TestNewCopiesSnapshot(t *testing.T) {
	t.Parallel()
	s, err := ruleset.Decode([]byte(testSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	in := input(t, "2026-02-01", "2026-01-10")
	before, _ := e.Evaluate(in)
	s.Series[1].Plans[0].Doses[0].Recommended.Offset = ruleset.Span{N: 9, Unit: ruleset.Months}
	s.Series[1].Plans[0].Doses[0].MinAge.N = 40
	s.Diseases[0].ID = "changed"
	after, _ := e.Evaluate(in)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("engine changed with its snapshot:\n%+v\n%+v", before, after)
	}
}
