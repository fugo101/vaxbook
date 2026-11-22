package engine

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
        {"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}},
        {"minInterval": "P4W", "recommended": {"from": "birth", "offset": "P4M"}}]},
      {"id": "long", "doses": [
        {"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}},
        {"minInterval": "P6W", "recommended": {"from": "birth", "offset": "P4M"}}]}
    ]},
    {"id": "b-series", "disease": "b", "stage": "childhood", "plans": [
      {"id": "main", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}
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
	in := everyProblemInput()
	want := []Problem{
		{Code: Required, Path: "/today", Scope: ScopeInput},
		{Code: UnexpectedValue, Path: "/profile/birth/value", Scope: ScopeInput},
		{Code: Required, Path: "/doses/0/id", Scope: ScopeInput},
		{Code: Required, Path: "/doses/0/diseases", Scope: ScopeInput},
		{Code: Required, Path: "/doses/0/date/precision", Scope: ScopeInput},
		{Code: UnknownDisease, Path: "/doses/1/diseases/1", Params: ProblemParams{Disease: "zz"}, Scope: ScopeInput},
		{Code: DuplicateValue, Path: "/doses/1/diseases/2", Params: ProblemParams{Disease: "a"}, Scope: ScopeInput},
		{Code: InvalidValue, Path: "/doses/1/date/precision", Params: ProblemParams{Value: "decade"}, Scope: ScopeInput},
		{Code: DuplicateID, Path: "/doses/2/id", Params: ProblemParams{ID: "x"}, Scope: ScopeInput},
		{Code: InvalidDate, Path: "/doses/2/date/value", Scope: ScopeRecord},
		{Code: InvalidValue, Path: "/doses/3/date/precision", Params: ProblemParams{Value: "none"}, Scope: ScopeInput},
		{Code: InvalidDate, Path: "/doses/4/date/value", Scope: ScopeRecord},
		{Code: UnknownReference, Path: "/doses/4/doseNumbers/a~1b", Params: ProblemParams{Disease: "a/b"}, Scope: ScopeInput},
		{Code: OutOfRange, Path: "/doses/4/doseNumbers/b", Scope: ScopeInput},
		{Code: UnexpectedValue, Path: "/doses/5/date/value", Scope: ScopeRecord},
	}
	if got := e.Validate(in); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate =\n  %v\nwant\n  %v", got, want)
	}
}

// productSnapshot: disease a with a series of product pa; disease b with a
// generic series; product pb lists b and is retired.
const productSnapshot = `{
  "version": "test-products",
  "diseases": [{"id": "a", "name": "A"}, {"id": "b", "name": "B"}],
  "products": [
    {"id": "pa", "name": "PA", "diseases": ["a"], "live": false},
    {"id": "pb", "name": "PB", "diseases": ["b"], "live": false, "retired": true}
  ],
  "series": [
    {"id": "a-series", "disease": "a", "product": "pa", "stage": "childhood", "plans": [
      {"id": "main", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}]},
    {"id": "b-series", "disease": "b", "stage": "childhood", "plans": [
      {"id": "main", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}]}
  ]
}`

// TestValidateProducts: the product members of a dose record (issue #14).
// An empty member is REQUIRED, both members are MUTUALLY_EXCLUSIVE, and an
// ID the rule set does not have is UNKNOWN_PRODUCT, record-scoped. A retired
// product and a product that does not list the record's disease are valid
// (Q36).
func TestValidateProducts(t *testing.T) {
	t.Parallel()
	s, err := ruleset.Decode([]byte(productSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	str := func(v string) *string { return &v }
	rec := func(product, free *string, date string, diseases ...string) Input {
		r := dose("r1", date, diseases...)
		r.Product, r.FreeProduct = product, free
		return input(t, "2026-04-01", "2026-01-10", r)
	}
	cases := []struct {
		name string
		in   Input
		want []Problem
	}{
		{"catalog product", rec(str("pa"), nil, "2026-03-01", "a"), nil},
		{"retired product of another disease", rec(str("pb"), nil, "2026-03-01", "a", "b"), nil},
		{"free product", rec(nil, str("Vắc xin khác"), "2026-03-01", "a"), nil},
		{"empty product", rec(str(""), nil, "2026-03-01", "a"),
			[]Problem{{Code: Required, Path: "/doses/0/product", Scope: ScopeInput}}},
		{"empty free product", rec(nil, str(""), "2026-03-01", "a"),
			[]Problem{{Code: Required, Path: "/doses/0/freeProduct", Scope: ScopeInput}}},
		{"both members", rec(str("pa"), str("x"), "2026-03-01", "a"),
			[]Problem{{Code: MutuallyExclusive, Path: "/doses/0/freeProduct", Scope: ScopeInput}}},
		{"both members, both empty", rec(str(""), str(""), "2026-03-01", "a"),
			[]Problem{
				{Code: Required, Path: "/doses/0/product", Scope: ScopeInput},
				{Code: Required, Path: "/doses/0/freeProduct", Scope: ScopeInput},
				{Code: MutuallyExclusive, Path: "/doses/0/freeProduct", Scope: ScopeInput},
			}},
		{"unknown product", rec(str("rotavin"), nil, "2026-03-01", "a"),
			[]Problem{{Code: UnknownProduct, Path: "/doses/0/product", Params: ProblemParams{Product: "rotavin"}, Scope: ScopeRecord}}},
		{"unknown product and a date in the future", rec(str("rotavin"), nil, "2026-04-02", "a"),
			[]Problem{
				{Code: UnknownProduct, Path: "/doses/0/product", Params: ProblemParams{Product: "rotavin"}, Scope: ScopeRecord},
				{Code: DateInFuture, Path: "/doses/0/date/value", Scope: ScopeRecord},
			}},
	}
	for _, c := range cases {
		if got := e.Validate(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Validate =\n  %v\nwant\n  %v", c.name, got, c.want)
		}
	}
	// A record with two record-scoped problems gives both reasons, in path
	// order, on every disease of the record.
	out, err := e.Evaluate(rec(str("rotavin"), nil, "2026-04-02", "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Reason{{Code: ReasonUnknownProduct, Params: ReasonParams{Dose: "r1"}}, {Code: ReasonDateInFuture, Params: ReasonParams{Dose: "r1"}}}
	for _, a := range out.Assessments {
		if a.Bucket != BucketUnknown || a.SeriesUsed != nil || !reflect.DeepEqual(a.Reasons, want) {
			t.Errorf("%s: %+v, want ⚪ with %v and no seriesUsed", a.Disease, a, want)
		}
	}
}

type validateCase struct {
	name string
	in   Input
	want []Problem
}

// everyProblemInput has a problem of nearly every code.
func everyProblemInput() Input {
	return Input{
		Profile: Profile{Birth: RecalledDate{Value: "2026", Precision: None}},
		Doses: []DoseRecord{
			{ID: "", Diseases: nil, Date: RecalledDate{Value: "2026-01-01"}},
			{ID: "x", Diseases: []string{"a", "zz", "a"}, Date: RecalledDate{Value: "2026", Precision: "decade"}},
			{ID: "x", Diseases: []string{"b"}, Date: day("2026-02-30")},
			{ID: "y", Diseases: []string{"b"}, Date: RecalledDate{Precision: None}},
			{ID: "z", Diseases: []string{"b"}, Date: RecalledDate{Value: "2026-13", Precision: Month},
				DoseNumbers: map[string]DoseNumber{"b": 0, "a/b": 1}},
			{ID: "w", Diseases: []string{"a"}, Date: RecalledDate{Value: "2026-03", Precision: Unknown}},
		},
	}
}

func validateDateCases(t *testing.T) []validateCase {
	t.Helper()
	return []validateCase{
		{"valid", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-03-10", "a")), nil},
		{"dose on birth day and on today", input(t, "2026-04-01", "2026-01-10",
			dose("r1", "2026-01-10", "a"), dose("r2", "2026-04-01", "a")), nil},
		{"invalid birth", input(t, "2026-04-01", "2026-02-30"),
			[]Problem{{Code: InvalidDate, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"birth in future", input(t, "2026-04-01", "2026-04-02"),
			[]Problem{{Code: DateInFuture, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"dose in future", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-04-02", "a")),
			[]Problem{{Code: DateInFuture, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"dose before birth", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-01-09", "a")),
			[]Problem{{Code: DateBeforeBirth, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"today out of range", input(t, "2200-01-01", "2026-01-10"),
			[]Problem{{Code: OutOfRange, Path: "/today", Scope: ScopeInput}}},
		{"birth out of range", input(t, "2026-04-01", "1899-12-31"),
			[]Problem{{Code: OutOfRange, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"garbage dose date", input(t, "2026-04-01", "2026-01-10", dose("r1", "abc", "a")),
			[]Problem{{Code: InvalidDate, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"missing birth precision", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01-10"}}},
			[]Problem{{Code: Required, Path: "/profile/birth/precision", Scope: ScopeInput}}},
		// "unknown" is a dose-record level only (F03, GLOSSARY "Date
		// precision"): no engine version evaluates it for a birth date.
		{"birth precision unknown", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Precision: Unknown}}},
			[]Problem{{Code: InvalidValue, Path: "/profile/birth/precision", Params: ProblemParams{Value: "unknown"}, Scope: ScopeInput}}},
		// Each field's own level 4 is evaluable since S2, with no value.
		{"birth precision none", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Precision: None}}}, nil},
		{"birth precision none with a value", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01-10", Precision: None}}},
			[]Problem{{Code: UnexpectedValue, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"dose precision unknown", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Precision: Unknown}}), nil},
		{"dose precision unknown with a value", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2026-03", Precision: Unknown}}),
			[]Problem{{Code: UnexpectedValue, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"birth precision month", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01", Precision: Month}}}, nil},
		{"birth precision year", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026", Precision: Year}}}, nil},
		{"birth month value with precision day", input(t, "2026-04-01", "2026-01"),
			[]Problem{{Code: InvalidDate, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"birth month 13", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-13", Precision: Month}}},
			[]Problem{{Code: InvalidDate, Path: "/profile/birth/value", Scope: ScopeInput}}},
		// The current month or year is not in the future: only a period that
		// starts after today is (issue #13).
		{"birth in the current month", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-04", Precision: Month}}}, nil},
		{"birth month after today", Input{Today: d(t, "2026-04-30"), Profile: Profile{Birth: RecalledDate{Value: "2026-05", Precision: Month}}},
			[]Problem{{Code: DateInFuture, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"birth year after today", Input{Today: d(t, "2026-12-31"), Profile: Profile{Birth: RecalledDate{Value: "2027", Precision: Year}}},
			[]Problem{{Code: DateInFuture, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"birth year before the input range", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "1899", Precision: Year}}},
			[]Problem{{Code: OutOfRange, Path: "/profile/birth/value", Scope: ScopeInput}}},
		{"dose year after the input range", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2200", Precision: Year}}),
			[]Problem{{Code: OutOfRange, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"dose in the current month", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2026-04", Precision: Month}}), nil},
		{"dose month after today", input(t, "2026-04-30", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2026-05", Precision: Month}}),
			[]Problem{{Code: DateInFuture, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		// A dose is before birth only when its whole period is before the
		// whole birth range; a partial overlap is not a problem.
		{"dose month overlapping the birth day", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2026-01", Precision: Month}}), nil},
		{"dose month before the birth day", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: RecalledDate{Value: "2025-12", Precision: Month}}),
			[]Problem{{Code: DateBeforeBirth, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"dose day inside the birth month", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01", Precision: Month}},
			Doses: []DoseRecord{dose("r1", "2026-01-01", "a")}}, nil},
		{"dose day before the birth year", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026", Precision: Year}},
			Doses: []DoseRecord{dose("r1", "2025-12-31", "a")}},
			[]Problem{{Code: DateBeforeBirth, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"dose with no birth date", Input{Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Precision: None}},
			Doses: []DoseRecord{dose("r1", "1950-01-01", "a")}}, nil},
		{"dose numbers", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a", "b"}, Date: day("2026-03-01"), DoseNumbers: map[string]DoseNumber{"b": 999, "a": 1}}), nil},
		{"dose number of another disease", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: day("2026-03-01"), DoseNumbers: map[string]DoseNumber{"b": 1}}),
			[]Problem{{Code: UnknownReference, Path: "/doses/0/doseNumbers/b", Params: ProblemParams{Disease: "b"}, Scope: ScopeInput}}},
		{"dose number out of range", input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a", "b"}, Date: day("2026-03-01"), DoseNumbers: map[string]DoseNumber{"b": 1000, "a": -1}}),
			[]Problem{
				{Code: OutOfRange, Path: "/doses/0/doseNumbers/a", Scope: ScopeInput},
				{Code: OutOfRange, Path: "/doses/0/doseNumbers/b", Scope: ScopeInput},
			}},
		{"dose after the input range", input(t, "2026-04-01", "2026-01-10", dose("r1", "2200-01-01", "a")),
			[]Problem{{Code: OutOfRange, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"dose before the input range", input(t, "2026-04-01", "2026-01-10", dose("r1", "1899-12-31", "a")),
			[]Problem{{Code: OutOfRange, Path: "/doses/0/date/value", Scope: ScopeRecord}}},
		{"record and input problems", input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-04-02", "a"), dose("r2", "2026-03-01", "x")),
			[]Problem{
				{Code: DateInFuture, Path: "/doses/0/date/value", Scope: ScopeRecord},
				{Code: UnknownDisease, Path: "/doses/1/diseases/0", Params: ProblemParams{Disease: "x"}, Scope: ScopeInput},
			}},
	}
}

func TestValidateDates(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	for _, c := range validateDateCases(t) {
		if got := e.Validate(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Validate =\n  %v\nwant\n  %v", c.name, got, c.want)
		}
	}
}

// TestValidateScopeMatchesEvaluate: Evaluate returns an *InputError holding
// exactly the ScopeInput problems iff there is one; otherwise every
// ScopeRecord problem becomes a reason with the same code on every disease
// of its record, and that disease is ⚪ with no next dose.
func TestValidateScopeMatchesEvaluate(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	withProduct := func(r DoseRecord, product string) DoseRecord {
		r.Product = &product
		return r
	}
	inputs := []Input{everyProblemInput(),
		input(t, "2026-04-01", "2026-01-10", dose("r2", "2026-04-02", "a", "b"), dose("r1", "2026-01-09", "a"), dose("r3", "abc", "b"), dose("r4", "2026-03-01", "a")),
		// testSnapshot has no products, so every product is unknown.
		input(t, "2026-04-01", "2026-01-10", withProduct(dose("r1", "2026-03-01", "a", "b"), "zz"),
			withProduct(dose("r2", "2026-04-02", "a"), "zz"), dose("r3", "2026-03-01", "b")),
	}
	for _, c := range validateDateCases(t) {
		inputs = append(inputs, c.in)
	}
	for _, in := range inputs {
		problems := e.Validate(in)
		var inputScoped []Problem
		for _, p := range problems {
			if p.Scope == ScopeInput {
				inputScoped = append(inputScoped, p)
			}
		}
		out, err := e.Evaluate(in)
		if len(inputScoped) > 0 {
			var ie *InputError
			if !errors.As(err, &ie) || !reflect.DeepEqual(ie.Problems, inputScoped) {
				t.Errorf("%+v: Evaluate error = %v, want the ScopeInput problems %v", in, err, inputScoped)
			}
			continue
		}
		if err != nil {
			t.Errorf("%+v: Evaluate error = %v with only record problems %v", in, err, problems)
			continue
		}
		for _, p := range problems {
			if p.Scope != ScopeRecord {
				t.Fatalf("problem %v has scope %q", p, p.Scope)
			}
			var i int
			var member string
			if _, err := fmt.Sscanf(p.Path, "/doses/%d/%s", &i, &member); err != nil || member != "date/value" && member != "product" {
				t.Fatalf("record problem at %s: %v", p.Path, err)
			}
			r := in.Doses[i]
			for _, a := range out.Assessments {
				if !slices.Contains(r.Diseases, a.Disease) {
					continue
				}
				want := Reason{Code: ReasonCode(p.Code), Params: ReasonParams{Dose: r.ID}}
				if a.Bucket != BucketUnknown || a.NextDose != nil || !slices.ContainsFunc(a.Reasons, func(x Reason) bool { return reflect.DeepEqual(x, want) }) {
					t.Errorf("%s of record %s: assessment %+v, want ⚪ with %v and no nextDose", p.Code, r.ID, a, want)
				}
			}
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
		want := []Problem{{Code: InvalidDate, Path: "/profile/birth/value", Scope: ScopeInput}}
		if !errors.As(err, &ie) || !reflect.DeepEqual(ie.Problems, want) {
			t.Errorf("birth %q: Evaluate error = %v, want INVALID_DATE at /profile/birth/value", v, err)
		}
	}
	for name, in := range map[string]Input{
		"invalid birth":           input(t, "2026-04-01", "2026-02-30"),
		"birth none with a value": {Today: d(t, "2026-04-01"), Profile: Profile{Birth: RecalledDate{Value: "2026-01", Precision: None}}},
		"dose number of another disease": input(t, "2026-04-01", "2026-01-10",
			DoseRecord{ID: "r1", Diseases: []string{"a"}, Date: day("2026-03-01"), DoseNumbers: map[string]DoseNumber{"b": 1}}),
		"unknown disease": input(t, "2026-04-01", "2026-01-10", dose("r1", "2026-03-01", "x")),
		"missing today":   {Profile: Profile{Birth: day("2026-01-10")}},
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
		want := []Reason{{Code: ReasonInvalidDate, Params: ReasonParams{Dose: "r1"}}}
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
		{Code: ReasonInvalidDate, Params: ReasonParams{Dose: "r0"}},
		{Code: ReasonInvalidDate, Params: ReasonParams{Dose: "r1"}},
	}
	if a.Bucket != BucketUnknown || a.NextDose != nil || !reflect.DeepEqual(a.Reasons, want) {
		t.Errorf("assessment = %+v", a)
	}
	if b := out.Assessments[1]; b.Bucket == BucketUnknown {
		t.Errorf("disease b has no invalid record but is %+v", b)
	}
}

// Alternative plans whose due dates tie: the merged earliest date is the
// latest of the tied plans' earliest dates (P1, SAD §5.3 step 7 merge 1).
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

// A ⚪ assessment emits no next dose (SAD §5.1), so no golden shows the dates
// computed after an uncounted dose. The earliest date is measured from the
// latest dose in time even when it is not counted, so a dose given on that
// date always passes the interval check.
func TestNextAfterUncountedDose(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	s := e.diseases[0].series[0]
	birth := birthEnd{date: d(t, "2026-01-10"), known: true}
	doses := []givenDose{{id: "r1", date: Exact(d(t, "2026-03-10")), dated: true}, {id: "r2", date: Exact(d(t, "2026-03-31")), dated: true}}
	w := s.walk(birth, d(t, "2026-04-01"), s.place(doses))
	if w.k != 2 || len(w.reasons) != 1 || w.reasons[0].Code != ReasonDoseInvalid {
		t.Fatalf("walk = %+v; want r1 counted and r2 invalid", w)
	}
	// short: E = r2 03-31 + 4w = 04-28, Du = max(E, 05-10) = 05-10, O = 05-17;
	// long: E = 03-31 + 6w = 05-12, Du = 05-12, O = 05-19. Earliest-due plan
	// short gives E and Du; O from the latest plan.
	want := NextDose{Number: 2, Earliest: d(t, "2026-04-28"), Due: d(t, "2026-05-10"), Overdue: d(t, "2026-05-19")}
	if got := s.next(birth, w); got.stop || got.next != want {
		t.Errorf("next = %+v, want %+v", got, want)
	}
}

// Minimums are inclusive: a dose on birth + minAge or on previous + interval
// counts, one day earlier does not.
func TestMinimumsAreInclusive(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	cases := []struct {
		doses   []DoseRecord
		invalid []Check
	}{
		{[]DoseRecord{dose("r1", "2026-02-21", "b")}, nil},
		{[]DoseRecord{dose("r1", "2026-02-20", "b")}, []Check{CheckMinAge}},
		{[]DoseRecord{dose("r1", "2026-02-21", "a"), dose("r2", "2026-03-21", "a")}, nil},
		{[]DoseRecord{dose("r1", "2026-02-21", "a"), dose("r2", "2026-03-20", "a")}, []Check{CheckMinInterval}},
	}
	for i, c := range cases {
		out, err := e.Evaluate(input(t, "2026-06-01", "2026-01-10", c.doses...))
		if err != nil {
			t.Fatal(err)
		}
		var checks []Check
		for _, a := range out.Assessments {
			for _, r := range a.Reasons {
				if r.Code == ReasonDoseInvalid {
					checks = r.Params.Checks
				}
			}
		}
		if !slices.Equal(checks, c.invalid) {
			t.Errorf("case %d: DOSE_INVALID checks = %v, want %v", i, checks, c.invalid)
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
	s, err := ruleset.Decode([]byte(strings.Replace(testSnapshot, `"P6W"`, `"P6W", "minInterval": "P1W"`, 1)))
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

// TestNewRejectsUnsupportedShapes: S3a accepts several series per disease,
// at most one per product group (issue #14); S3b tells the series of a
// group apart by start age.
func TestNewRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()
	const products = `"products": [{"id": "generic", "name": "G", "diseases": ["a"], "live": false}],`
	series := func(id, product string) string {
		p := ""
		if product != "" {
			p = `"product": "` + product + `", `
		}
		return `{"id": "` + id + `", "disease": "a", ` + p + `"stage": "childhood", "plans": [{"id": "p", "doses": [{"minAge": "P0D", "recommended": {"from": "birth", "offset": "P0D"}}]}]}`
	}
	for name, c := range map[string]struct{ snap, msg string }{
		"disease without series": {`{"version": "v", "diseases": [{"id": "a", "name": "A"}], "series": []}`,
			`disease "a" has no series`},
		"two generic series": {`{"version": "v", "diseases": [{"id": "a", "name": "A"}], "series": [` + series("s1", "") + `, ` + series("s2", "") + `]}`,
			`disease "a": more than one generic series`},
		// "generic" is a valid product ID, so the generic group is never
		// named by it.
		"product group with two series": {`{"version": "v", "diseases": [{"id": "a", "name": "A"}], ` + products + ` "series": [` +
			series("s1", "generic") + `, ` + series("s0", "") + `, ` + series("s2", "generic") + `]}`,
			`disease "a": product "generic" has more than one series`},
	} {
		s, err := ruleset.Decode([]byte(c.snap))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := New(s); !errors.Is(err, ErrUnsupported) || !strings.HasSuffix(err.Error(), c.msg) {
			t.Errorf("%s: New error = %v, want ErrUnsupported ending in %s", name, err, c.msg)
		}
	}
	// A product series and a generic series of one disease are two groups.
	s, err := ruleset.Decode([]byte(`{"version": "v", "diseases": [{"id": "a", "name": "A"}], ` + products + ` "series": [` +
		series("s1", "generic") + `, ` + series("s0", "") + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(s); err != nil {
		t.Errorf("product and generic series: New error = %v", err)
	}
}

// TestMergeMaskedByDoseInvalid pins the masked result of a ⚪ merge, which
// no output shows (golden candidates-disagree-on-minimum): only the
// candidates at the smallest k are merged, and the dates are measured from
// the latest dose in time, counted or not. pertussis-primary counts r1
// (03-01) and r2 (04-01) and needs dose 3; pertussis-pertu2 rejects r2 (r1
// + 8w = 04-26) and needs dose 2: E = r2 + 8w = 05-27, Du = max(E, b + 4m
// = 05-01) = 05-27, O = 06-03, upcoming on 04-20, with its product only
// and no ALTERNATIVE_PLANS.
func TestMergeMaskedByDoseInvalid(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("testdata", "rulesets", "illustrative-products.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ruleset.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	dz := e.diseases[1]
	doses := []givenDose{{id: "r1", date: Exact(d(t, "2026-03-01")), dated: true}, {id: "r2", date: Exact(d(t, "2026-04-01")), dated: true}}
	cands := dz.candidates(doses, false)
	ev := evaluation{dz: dz, cands: cands, today: d(t, "2026-04-20")}
	for _, c := range cands {
		ev.pls = append(ev.pls, dz.series[c].place(doses))
	}
	res := ev.at(birthEnd{date: d(t, "2026-01-01"), known: true})
	want := NextDose{Number: 2, Earliest: d(t, "2026-05-27"), Due: d(t, "2026-05-27"), Overdue: d(t, "2026-06-03")}
	if len(cands) != 2 || res.bucket != BucketUnknown || !res.hasNext || res.next != want || !slices.Equal(res.products, []string{"pertu2"}) {
		t.Errorf("candidates %v, result %+v; want ⚪ masking %+v with products [pertu2]", cands, res, want)
	}
	wantReasons := []Reason{{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: []Check{CheckMinInterval}, Dose: "r2", Number: 2}}}
	if !reflect.DeepEqual(res.reasons, wantReasons) {
		t.Errorf("reasons %v, want %v", res.reasons, wantReasons)
	}
	if bucket, _, _ := classify(ev.today, res.next, true); bucket != BucketUpcoming {
		t.Errorf("masked bucket %s, want upcoming", bucket)
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

// TestReasonCodesClassified: every reason code is either blocking or
// informational (issue #13); blocks panics on an unclassified code.
func TestReasonCodesClassified(t *testing.T) {
	t.Parallel()
	informational := []ReasonCode{ReasonImpliedEarlierDoses, ReasonSafestSeries, ReasonIntervalUnverifiable, ReasonFirstDoseDateAssumed,
		ReasonAlternativePlans, ReasonComplete, ReasonProductNotInCatalog, ReasonOnTrack}
	blockingCodes := []ReasonCode{ReasonInvalidDate, ReasonOutOfRange, ReasonDateInFuture, ReasonDateBeforeBirth,
		ReasonUnexpectedValue, ReasonUnknownProduct, ReasonOverlappingEstimatedDates, ReasonNumberingConflict, ReasonBirthDateImprecise,
		ReasonDoseInvalid, ReasonDosePossiblyInvalid, ReasonPreviousDoseDateUnknown, ReasonNoBirthDate}
	for _, c := range informational {
		if c.blocks() {
			t.Errorf("%s blocks, want informational", c)
		}
	}
	for _, c := range blockingCodes {
		if !c.blocks() {
			t.Errorf("%s is informational, want blocking", c)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("blocks of an unknown code did not panic")
		}
	}()
	ReasonCode("NOPE").blocks()
}

// TestProblemCodeStrings pins the wire string of every input problem code.
// The schema has no Problem definition, so TestSchemaEnumsMatchConstants
// cannot cover these codes (issue #14, In scope 12), and the tests that use
// them compare with the Go constants. The WASM bridge returns the strings to
// clients, so a renamed or mistyped value must fail here.
func TestProblemCodeStrings(t *testing.T) {
	t.Parallel()
	want := map[ProblemCode]string{
		Required:             "REQUIRED",
		InvalidValue:         "INVALID_VALUE",
		UnsupportedPrecision: "UNSUPPORTED_PRECISION",
		InvalidDate:          "INVALID_DATE",
		UnexpectedValue:      "UNEXPECTED_VALUE",
		OutOfRange:           "OUT_OF_RANGE",
		DateInFuture:         "DATE_IN_FUTURE",
		DateBeforeBirth:      "DATE_BEFORE_BIRTH",
		UnknownReference:     "UNKNOWN_REFERENCE",
		UnknownDisease:       "UNKNOWN_DISEASE",
		DuplicateID:          "DUPLICATE_ID",
		DuplicateValue:       "DUPLICATE_VALUE",
		UnknownProduct:       "UNKNOWN_PRODUCT",
		MutuallyExclusive:    "MUTUALLY_EXCLUSIVE",
	}
	for c, s := range want {
		if string(c) != s {
			t.Errorf("problem code %q, want %q", c, s)
		}
	}
}

// TestPositionsAndUndatedDoses pins step-2 and undated-dose behaviour that
// no golden file covers (SAD §5.3 steps 2 and 5).
func TestPositionsAndUndatedDoses(t *testing.T) {
	t.Parallel()
	e := testEngine(t)
	undated := func(id string, number int, diseases ...string) DoseRecord {
		r := DoseRecord{ID: id, Diseases: diseases, Date: RecalledDate{Precision: Unknown}}
		if number > 0 {
			r.DoseNumbers = map[string]DoseNumber{diseases[0]: DoseNumber(number)}
		}
		return r
	}
	numbered := func(r DoseRecord, n int) DoseRecord {
		r.DoseNumbers = map[string]DoseNumber{r.Diseases[0]: DoseNumber(n)}
		return r
	}
	month := func(id, v string, diseases ...string) DoseRecord {
		return DoseRecord{ID: id, Diseases: diseases, Date: RecalledDate{Value: v, Precision: Month}}
	}
	cases := []struct {
		name    string
		in      Input
		disease int
		want    []Reason
	}{
		// A level-4 dose 1 whose earliest placement (birth + 6w = 02-21) is
		// after today cannot have been given yet: DOSE_INVALID, not counted.
		{"trailing level-4 dose not feasible by today", input(t, "2026-02-01", "2026-01-10", undated("r1", 1, "b")), 1,
			[]Reason{{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: []Check{CheckMinAge}, Dose: "r1", Number: 1}}}},
		// Two exact doses on the same day are not an overlap, and their
		// numbers order them: b1 (#1) then a1 (#2), which fails the interval.
		{"same-day doses ordered by number", input(t, "2026-04-01", "2026-01-10",
			numbered(dose("a1", "2026-03-01", "a"), 2), numbered(dose("b1", "2026-03-01", "a"), 1)), 0,
			[]Reason{
				{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: []Check{CheckMinInterval}, Dose: "a1", Number: 2}},
				{Code: ReasonAlternativePlans, Params: ReasonParams{Series: "a-series"}},
			}},
		{"duplicate level-4 numbers", input(t, "2026-04-01", "2026-01-10", undated("r1", 1, "a"), undated("r2", 1, "a")), 0,
			[]Reason{{Code: ReasonNumberingConflict, Params: ReasonParams{Dose: "r2", Number: 1}}}},
		// r2 and r3 do not overlap each other (two exact days), but each
		// overlaps r1, so the three form one group.
		{"overlap group of three", input(t, "2026-04-01", "2026-01-10",
			dose("r3", "2026-03-20", "b"), dose("r2", "2026-03-10", "b"), month("r1", "2026-03", "b")), 1,
			[]Reason{{Code: ReasonOverlappingEstimatedDates, Params: ReasonParams{Doses: []string{"r1", "r2", "r3"}}}}},
		// A number beyond the series sits beyond it; the positions below are
		// implied, so the series is complete.
		{"number beyond the series", input(t, "2026-06-01", "2026-01-10", numbered(dose("r1", "2026-03-01", "b"), 3)), 1,
			[]Reason{{Code: ReasonImpliedEarlierDoses, Params: ReasonParams{Numbers: []int{1}}}, {Code: ReasonComplete}}},
	}
	for _, c := range cases {
		out, err := e.Evaluate(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if a := out.Assessments[c.disease]; !reflect.DeepEqual(a.Reasons, c.want) {
			t.Errorf("%s: reasons\n  %v\nwant\n  %v", c.name, a.Reasons, c.want)
		}
	}
}

// TestResolveOverlapsMatchesPairwise: the linear sweep of resolveOverlaps
// gives the groups of the pairwise definition (two dated ranges overlap
// when they intersect and are not both single days) on random short ranges
// over a few days, where ties, same-day exact doses and nested ranges are
// common.
func TestResolveOverlapsMatchesPairwise(t *testing.T) {
	t.Parallel()
	start, err := ParseDate("2025-01-01")
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(1)
	rnd := func(n int) int { // xorshift, so the cases are reproducible
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return int(seed % uint64(n)) //nolint:gosec // n is small and positive
	}
	for range 20000 {
		d := make([]givenDose, 1+rnd(7))
		for i := range d {
			lo := start.AddDays(rnd(6))
			d[i] = givenDose{id: fmt.Sprintf("r%d", i), dated: true, number: rnd(2), date: Range{Lo: lo, Hi: lo.AddDays(rnd(3) * rnd(2))}}
		}
		slices.SortFunc(d, func(x, y givenDose) int {
			return cmp.Or(x.date.Lo.Compare(y.date.Lo), x.date.Hi.Compare(y.date.Hi), strings.Compare(x.id, y.id))
		})
		// The pairwise groups, as the sets of IDs of the groups of two or
		// more members whose members do not all have numbers.
		parent := make([]int, len(d))
		for i := range parent {
			parent[i] = i
		}
		var find func(int) int
		find = func(i int) int {
			if parent[i] != i {
				parent[i] = find(parent[i])
			}
			return parent[i]
		}
		for i := range d {
			for j := range d {
				x, y := d[i].date, d[j].date
				if i != j && !x.Hi.Before(y.Lo) && !y.Hi.Before(x.Lo) && (!x.IsExact() || !y.IsExact()) {
					parent[find(j)] = find(i)
				}
			}
		}
		groups := map[int][]string{}
		numbered := map[int]bool{}
		for i := range d {
			r := find(i)
			if _, ok := numbered[r]; !ok {
				numbered[r] = true
			}
			groups[r] = append(groups[r], d[i].id)
			numbered[r] = numbered[r] && d[i].number > 0
		}
		var want []string
		for r, ids := range groups {
			if len(ids) > 1 && !numbered[r] {
				want = append(want, strings.Join(ids, ","))
			}
		}
		pl := placement{dated: slices.Clone(d)}
		pl.resolveOverlaps()
		var got []string
		for _, r := range pl.reasons {
			got = append(got, strings.Join(r.Params.Doses, ","))
		}
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("%v: groups %q, want %q", d, got, want)
		}
	}
}
