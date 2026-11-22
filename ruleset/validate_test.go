package ruleset

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

// validCase is a well-formed one-series snapshot that individual cases break.
const validCase = `{
  "version": "v1",
  "diseases": [{"id": "d", "name": "D"}],
  "series": [{
    "id": "s", "disease": "d", "stage": "childhood",
    "plans": [{"id": "p", "doses": [
      {"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}},
      {"minInterval": "P4W", "recommended": {"from": "previousDose", "offset": "P2M"}, "buffer": "P14D"}
    ]}]
  }]
}`

func mustDecode(t *testing.T, s string) *Snapshot {
	t.Helper()
	snap, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return snap
}

func TestValidateAcceptsWellFormedSnapshot(t *testing.T) {
	t.Parallel()
	if got := Validate(mustDecode(t, validCase)); len(got) != 0 {
		t.Fatalf("Validate = %v, want no problems", got)
	}
}

func TestDecodeIsStrict(t *testing.T) {
	t.Parallel()
	bad := map[string]string{
		"unknown member":   strings.Replace(validCase, `"version"`, `"extra": 1, "version"`, 1),
		"wrong case":       strings.Replace(validCase, `"version"`, `"Version"`, 1),
		"duplicate member": strings.Replace(validCase, `"version": "v1"`, `"version": "v1", "version": "v2"`, 1),
		"bad span":         strings.Replace(validCase, `"P6W"`, `"6 weeks"`, 1),
		"S1 compact span":  strings.Replace(validCase, `"P6W"`, `"6w"`, 1),
		"compound span":    strings.Replace(validCase, `"P6W"`, `"P1M2W"`, 1),
		"trailing data":    validCase + `{}`,
	}
	for name, in := range bad {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("%s: Decode succeeded, want error", name)
		}
	}
}

// TestDecodeShape: Decode agrees with contracts/ruleset.schema.json on
// required members and null, so a truncated snapshot never loads as one with
// no assessments.
func TestDecodeShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, doc string
		kind      contractjson.Kind
		pointer   string
	}{
		{"truncated snapshot", `{"version": "x"}`, contractjson.KindMissing, "/diseases"},
		{"missing series", `{"version": "v1", "diseases": [{"id": "d", "name": "D"}]}`, contractjson.KindMissing, "/series"},
		{"missing offset", strings.Replace(validCase, `"recommended": {"from": "birth", "offset": "P2M"}`, `"recommended": {"from": "birth"}`, 1),
			contractjson.KindMissing, "/series/0/plans/0/doses/0/recommended/offset"},
		{"missing recommended", strings.Replace(validCase, `, "recommended": {"from": "birth", "offset": "P2M"}`, ``, 1),
			contractjson.KindMissing, "/series/0/plans/0/doses/0/recommended"},
		{"missing plan doses", `{"version": "v1", "diseases": [], "series": [{"id": "s", "disease": "d", "stage": "adult", "plans": [{"id": "p"}]}]}`,
			contractjson.KindMissing, "/series/0/plans/0/doses"},
		{"null version", strings.Replace(validCase, `"version": "v1"`, `"version": null`, 1), contractjson.KindNull, "/version"},
		{"null min interval", strings.Replace(validCase, `"minInterval": "P4W"`, `"minInterval": null`, 1),
			contractjson.KindNull, "/series/0/plans/0/doses/1/minInterval"},
		{"null min interval from first", strings.Replace(validCase, `"minInterval": "P4W"`, `"minInterval": "P4W", "minIntervalFromFirst": null`, 1),
			contractjson.KindNull, "/series/0/plans/0/doses/1/minIntervalFromFirst"},
		{"null buffer", strings.Replace(validCase, `"buffer": "P14D"`, `"buffer": null`, 1),
			contractjson.KindNull, "/series/0/plans/0/doses/1/buffer"},
		{"null disease", strings.Replace(validCase, `[{"id": "d", "name": "D"}]`, `[{"id": "d", "name": "D"}, null]`, 1),
			contractjson.KindNull, "/diseases/1"},
		{"null root", `null`, contractjson.KindNull, ""},
	}
	for _, c := range cases {
		_, err := Decode([]byte(c.doc))
		var se *contractjson.Error
		if !errors.As(err, &se) || se.Kind != c.kind || se.Pointer != c.pointer {
			t.Errorf("%s: Decode error = %v, want %s at %q", c.name, err, c.kind, c.pointer)
		}
	}
}

func TestValidateProblems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		old  string
		new  string
		want []Problem
	}{
		{"missing version", `"version": "v1"`, `"version": ""`,
			[]Problem{{Code: Required, Severity: Reject, Path: "/version"}}},
		{"disease id shape", `{"id": "d", "name": "D"}`, `{"id": "D!", "name": "D"}`,
			[]Problem{
				{Code: InvalidValue, Severity: Reject, Path: "/diseases/0/id", Params: ProblemParams{Value: "D!"}},
				{Code: UnknownReference, Severity: Reject, Path: "/series/0/disease", Params: ProblemParams{Disease: "d"}},
			}},
		{"missing disease name", `"name": "D"`, `"name": ""`,
			[]Problem{{Code: Required, Severity: Reject, Path: "/diseases/0/name"}}},
		{"duplicate disease id", `{"id": "d", "name": "D"}`, `{"id": "d", "name": "D"}, {"id": "d", "name": "E"}`,
			[]Problem{{Code: DuplicateID, Severity: Reject, Path: "/diseases/1/id", Params: ProblemParams{ID: "d"}}}},
		{"unknown disease", `"disease": "d"`, `"disease": "x"`,
			[]Problem{{Code: UnknownReference, Severity: Reject, Path: "/series/0/disease", Params: ProblemParams{Disease: "x"}}}},
		{"bad stage", `"stage": "childhood"`, `"stage": "teen"`,
			[]Problem{{Code: InvalidValue, Severity: Reject, Path: "/series/0/stage", Params: ProblemParams{Value: "teen"}}}},
		{"bad anchor", `"from": "previousDose"`, `"from": "lastDose"`,
			[]Problem{{Code: InvalidValue, Severity: Reject, Path: "/series/0/plans/0/doses/1/recommended/from", Params: ProblemParams{Value: "lastDose"}}}},
		{"first dose interval", `{"minAge": "P6W", "recommended"`, `{"minAge": "P6W", "minInterval": "P1W", "recommended"`,
			[]Problem{{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minInterval"}}},
		{"first dose without min age", `{"minAge": "P6W", "recommended"`, `{"minInterval": "P1W", "recommended"`,
			[]Problem{
				{Code: DoseRuleNoMinimum, Severity: Reject, Path: "/series/0/plans/0/doses/0"},
				{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minInterval"},
			}},
		{"empty anchor", `"from": "previousDose"`, `"from": ""`,
			[]Problem{{Code: Required, Severity: Reject, Path: "/series/0/plans/0/doses/1/recommended/from"}}},
		{"first dose anchor", `"from": "birth"`, `"from": "firstDose"`,
			[]Problem{{Code: FirstDoseAnchorNotBirth, Severity: Reject, Path: "/series/0/plans/0/doses/0/recommended/from"}}},
		// minIntervalFromFirst satisfies A7 for a dose after dose 1 (SAD §5.2).
		{"min interval from first only", `{"minInterval": "P4W", "recommended": {"from": "previousDose"`,
			`{"minIntervalFromFirst": "P5M", "recommended": {"from": "previousDose"`, nil},
		{"first dose interval from first", `{"minAge": "P6W", "recommended"`, `{"minAge": "P6W", "minIntervalFromFirst": "P1M", "recommended"`,
			[]Problem{{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minIntervalFromFirst"}}},
		{"first dose with only interval from first", `{"minAge": "P6W", "recommended"`, `{"minIntervalFromFirst": "P1M", "recommended"`,
			[]Problem{
				{Code: DoseRuleNoMinimum, Severity: Reject, Path: "/series/0/plans/0/doses/0"},
				{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minIntervalFromFirst"},
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := strings.Replace(validCase, tc.old, tc.new, 1)
			if in == validCase {
				t.Fatalf("case did not change the snapshot")
			}
			got := Validate(mustDecode(t, in))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

// productCase has two diseases, a product catalog, a product series and a
// generic series; individual cases break it (issue #14).
const productCase = `{
  "version": "v1",
  "diseases": [{"id": "d", "name": "D"}, {"id": "e", "name": "E"}],
  "products": [
    {"id": "p", "name": "P", "diseases": ["d"], "live": true},
    {"id": "q", "name": "Q", "diseases": ["d", "e"], "live": false, "retired": true}
  ],
  "series": [
    {"id": "s", "disease": "d", "product": "p", "stage": "childhood",
     "plans": [{"id": "p1", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}]},
    {"id": "t", "disease": "e", "stage": "childhood",
     "plans": [{"id": "p1", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}]}
  ]
}`

func TestValidateProducts(t *testing.T) {
	t.Parallel()
	reject := func(code ProblemCode, path string, params ProblemParams) Problem {
		return Problem{Code: code, Severity: Reject, Path: path, Params: params}
	}
	cases := []struct {
		name, old, new string
		want           []Problem
	}{
		{"valid", "", "", nil},
		// A catalog product needs no series of its own (rotasiil, A2). A
		// product group may hold several series, each with a start-age
		// bracket (S3b, EQ26; TestValidateStartAge).
		{"product without series", `"id": "q", "name": "Q", "diseases": ["d", "e"]`, `"id": "q", "name": "Q", "diseases": ["e"]`, nil},
		{"product group with two series", `{"id": "t", "disease": "e", "stage"`, `{"id": "t", "disease": "d", "product": "p", "stage"`,
			[]Problem{
				reject(StartAgeRequired, "/series/0", ProblemParams{Disease: "d", Product: "p"}),
				reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d", Product: "p"}),
			}},
		{"missing product id", `"id": "p", "name": "P"`, `"id": "", "name": "P"`,
			[]Problem{
				reject(Required, "/products/0/id", ProblemParams{}),
				reject(UnknownReference, "/series/0/product", ProblemParams{Product: "p"}),
			}},
		{"invalid product id", `"id": "q", "name": "Q"`, `"id": "Q!", "name": "Q"`,
			[]Problem{reject(InvalidValue, "/products/1/id", ProblemParams{Value: "Q!"})}},
		{"repeated product id", `"id": "q", "name": "Q"`, `"id": "p", "name": "Q"`,
			[]Problem{reject(DuplicateID, "/products/1/id", ProblemParams{ID: "p"})}},
		{"empty product name", `"name": "P"`, `"name": ""`,
			[]Problem{reject(Required, "/products/0/name", ProblemParams{})}},
		{"product without diseases", `"diseases": ["d", "e"]`, `"diseases": []`,
			[]Problem{reject(Empty, "/products/1/diseases", ProblemParams{})}},
		{"product with an unknown disease", `"diseases": ["d", "e"]`, `"diseases": ["d", "x"]`,
			[]Problem{reject(UnknownReference, "/products/1/diseases/1", ProblemParams{Disease: "x"})}},
		{"product with an empty disease", `"diseases": ["d", "e"]`, `"diseases": ["d", ""]`,
			[]Problem{reject(Required, "/products/1/diseases/1", ProblemParams{})}},
		{"product repeats a disease", `"diseases": ["d", "e"]`, `"diseases": ["d", "e", "d"]`,
			[]Problem{reject(DuplicateValue, "/products/1/diseases/2", ProblemParams{Disease: "d"})}},
		{"empty series product", `"product": "p"`, `"product": ""`,
			[]Problem{reject(Required, "/series/0/product", ProblemParams{})}},
		{"invalid series product", `"product": "p"`, `"product": "P!"`,
			[]Problem{reject(InvalidValue, "/series/0/product", ProblemParams{Value: "P!"})}},
		{"unknown series product", `"product": "p"`, `"product": "z"`,
			[]Problem{reject(UnknownReference, "/series/0/product", ProblemParams{Product: "z"})}},
		{"series product does not list the disease", `{"id": "t", "disease": "e", "stage"`, `{"id": "t", "disease": "e", "product": "p", "stage"`,
			[]Problem{reject(ProductDiseaseMismatch, "/series/1/product", ProblemParams{Disease: "e", Product: "p"})}},
		// A retired product still names its series' product.
		{"series of a retired product", `{"id": "t", "disease": "e", "stage"`, `{"id": "t", "disease": "e", "product": "q", "stage"`, nil},
		// An unknown series disease is reported once; the product is not
		// also a mismatch.
		{"series with an unknown disease", `"disease": "d", "product": "p"`, `"disease": "x", "product": "p"`,
			[]Problem{reject(UnknownReference, "/series/0/disease", ProblemParams{Disease: "x"})}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := strings.Replace(productCase, tc.old, tc.new, 1)
			if tc.old != "" && in == productCase {
				t.Fatalf("case did not change the snapshot")
			}
			if got := Validate(mustDecode(t, in)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

// Product and Series.product decode a present empty string as such, so
// Validate sees it (a plain omitzero string could not).
// TestProblemCodeStrings pins the wire string of every rule set problem
// code. The schema has no Problem definition, so TestSchemaEnumsMatchConstants
// cannot cover these codes (issue #14, In scope 12), and not every code has
// a reject snapshot in testdata/validate.
func TestProblemCodeStrings(t *testing.T) {
	t.Parallel()
	want := map[ProblemCode]string{
		PlanDoseCountMismatch:   "PLAN_DOSE_COUNT_MISMATCH",
		DoseRuleNoMinimum:       "DOSE_RULE_NO_MINIMUM",
		FirstDoseHasInterval:    "FIRST_DOSE_HAS_INTERVAL",
		FirstDoseAnchorNotBirth: "FIRST_DOSE_ANCHOR_NOT_BIRTH",
		Required:                "REQUIRED",
		InvalidValue:            "INVALID_VALUE",
		DuplicateID:             "DUPLICATE_ID",
		UnknownReference:        "UNKNOWN_REFERENCE",
		Empty:                   "EMPTY",
		DuplicateValue:          "DUPLICATE_VALUE",
		ProductDiseaseMismatch:  "PRODUCT_DISEASE_MISMATCH",
		StartAgeEmpty:           "START_AGE_EMPTY",
		StartAgeRequired:        "START_AGE_REQUIRED",
		StartAgeOverlap:         "START_AGE_OVERLAP",
		// The two Warns of S3b.
		FirstDoseMinAgeAboveStartAge: "FIRST_DOSE_MIN_AGE_ABOVE_START_AGE",
		FirstDoseMinAgeDiffers:       "FIRST_DOSE_MIN_AGE_DIFFERS",
	}
	for c, s := range want {
		if string(c) != s {
			t.Errorf("problem code %q, want %q", c, s)
		}
	}
}

func TestDecodeProductMembers(t *testing.T) {
	t.Parallel()
	s := mustDecode(t, strings.Replace(productCase, `"product": "p"`, `"product": ""`, 1))
	if s.Series[0].Product == nil || *s.Series[0].Product != "" {
		t.Errorf("series product = %v, want a present empty string", s.Series[0].Product)
	}
	if s.Series[1].Product != nil {
		t.Errorf("generic series product = %q, want absent", *s.Series[1].Product)
	}
	if _, err := Decode([]byte(strings.Replace(productCase, `"live": true`, `"live": "true"`, 1))); err == nil {
		t.Error("Decode accepted a string live flag")
	}
	var se *contractjson.Error
	if _, err := Decode([]byte(strings.Replace(productCase, `, "live": true`, ``, 1))); !errors.As(err, &se) ||
		se.Kind != contractjson.KindMissing || se.Pointer != "/products/0/live" {
		t.Errorf("Decode without live: %v, want missing /products/0/live", err)
	}
}

func TestValidateSeriesWithoutPlans(t *testing.T) {
	t.Parallel()
	in := `{"version": "v1", "diseases": [{"id": "d", "name": "D"}],
	  "series": [{"id": "s", "disease": "d", "stage": "adult", "plans": []}]}`
	want := []Problem{{Code: Empty, Severity: Reject, Path: "/series/0/plans"}}
	if got := Validate(mustDecode(t, in)); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate =\n  %v\nwant\n  %v", got, want)
	}
}

// A snapshot without diseases would load and silently assess nothing.
// testdata/validate holds shape-valid snapshots only, so this case is inline.
func TestValidateSnapshotWithoutDiseases(t *testing.T) {
	t.Parallel()
	want := []Problem{{Code: Empty, Severity: Reject, Path: "/diseases"}}
	if got := Validate(mustDecode(t, `{"version": "v1", "diseases": [], "series": []}`)); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate =\n  %v\nwant\n  %v", got, want)
	}
	if got := Validate(&Snapshot{Version: "v1"}); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate(Go-built) =\n  %v\nwant\n  %v", got, want)
	}
}

func TestValidateDuplicateSeriesAndPlanIDs(t *testing.T) {
	t.Parallel()
	in := `{"version": "v1", "diseases": [{"id": "d", "name": "D"}],
	  "series": [
	    {"id": "s", "disease": "d", "stage": "adult", "plans": [
	      {"id": "p", "doses": [{"minAge": "P0D", "recommended": {"from": "birth", "offset": "P0D"}}]},
	      {"id": "p", "doses": []}
	    ]},
	    {"id": "s", "disease": "d", "stage": "adult", "plans": [
	      {"id": "p", "doses": [{"minAge": "P0D", "recommended": {"from": "birth", "offset": "P0D"}}]}
	    ]}
	  ]}`
	// Two generic series of one disease also need start-age brackets (S3b).
	want := []Problem{
		{Code: StartAgeRequired, Severity: Reject, Path: "/series/0", Params: ProblemParams{Disease: "d"}},
		{Code: PlanDoseCountMismatch, Severity: Reject, Path: "/series/0/plans/1"},
		{Code: DuplicateID, Severity: Reject, Path: "/series/0/plans/1/id", Params: ProblemParams{ID: "p"}},
		{Code: Empty, Severity: Reject, Path: "/series/0/plans/1/doses"},
		{Code: StartAgeRequired, Severity: Reject, Path: "/series/1", Params: ProblemParams{Disease: "d"}},
		{Code: DuplicateID, Severity: Reject, Path: "/series/1/id", Params: ProblemParams{ID: "s"}},
	}
	if got := Validate(mustDecode(t, in)); !reflect.DeepEqual(got, want) {
		t.Errorf("Validate =\n  %v\nwant\n  %v", got, want)
	}
}

// A Snapshot built in Go skips ParseSpan, so Validate itself must hold every
// span to a real unit and 0..MaxSpanN: the engine relies on that to compute
// four-digit-year dates without panicking.
func TestValidateSpansOfGoBuiltSnapshot(t *testing.T) {
	t.Parallel()
	const dose = "/series/0/plans/0/doses/"
	const startAge = "/series/0/startAge/"
	invalid := func(path string, s Span) Problem {
		return Problem{Code: InvalidValue, Severity: Reject, Path: path, Params: ProblemParams{Value: s.String()}}
	}
	cases := []struct {
		name   string
		mutate func(d0, d1 *DoseRule)
		want   []Problem
	}{
		{"zero min age", func(d0, _ *DoseRule) { d0.MinAge = &Span{} },
			[]Problem{invalid(dose+"0/minAge", Span{})}},
		{"min age too long", func(d0, _ *DoseRule) { d0.MinAge = &Span{N: 100000, Unit: Years} },
			[]Problem{invalid(dose+"0/minAge", Span{N: 100000, Unit: Years})}},
		{"negative min interval", func(_, d1 *DoseRule) { d1.MinInterval = &Span{N: -1, Unit: Weeks} },
			[]Problem{invalid(dose+"1/minInterval", Span{N: -1, Unit: Weeks})}},
		{"negative min interval from first", func(_, d1 *DoseRule) { d1.MinIntervalFromFirst = &Span{N: -1, Unit: Months} },
			[]Problem{invalid(dose+"1/minIntervalFromFirst", Span{N: -1, Unit: Months})}},
		{"negative buffer", func(_, d1 *DoseRule) { d1.Buffer = &Span{N: -5, Unit: Days} },
			[]Problem{invalid(dose+"1/buffer", Span{N: -5, Unit: Days})}},
		{"buffer bad unit", func(_, d1 *DoseRule) { d1.Buffer = &Span{N: 14, Unit: 'x'} },
			[]Problem{invalid(dose+"1/buffer", Span{N: 14, Unit: 'x'})}},
		{"offset too long", func(d0, _ *DoseRule) { d0.Recommended.Offset = Span{N: 1000, Unit: Days} },
			[]Problem{invalid(dose+"0/recommended/offset", Span{N: 1000, Unit: Days})}},
		// Decode rejects a missing offset or milestone; a Go-built snapshot
		// can still leave them zero.
		{"zero offset", func(d0, _ *DoseRule) { d0.Recommended.Offset = Span{} },
			[]Problem{{Code: Required, Severity: Reject, Path: dose + "0/recommended/offset"}}},
		{"zero milestone", func(d0, _ *DoseRule) { d0.Recommended = Milestone{} },
			[]Problem{
				{Code: Required, Severity: Reject, Path: dose + "0/recommended/from"},
				{Code: Required, Severity: Reject, Path: dose + "0/recommended/offset"},
			}},
		{"spans at the cap", func(d0, d1 *DoseRule) {
			d0.MinAge = &Span{N: MaxSpanN, Unit: Years}
			d1.MinInterval = &Span{N: 0, Unit: Days}
			d1.Buffer = &Span{N: MaxSpanN, Unit: Days}
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := mustDecode(t, validCase)
			doses := s.Series[0].Plans[0].Doses
			tc.mutate(&doses[0], &doses[1])
			if got := Validate(s); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
	// A bracket with an invalid span gets only the span problem (issue
	// #23): dose 1's minimum age P6W would otherwise be above a zero or
	// one-day min (FIRST_DOSE_MIN_AGE_ABOVE_START_AGE), and the spans below
	// would otherwise make the bracket START_AGE_EMPTY.
	bracketCases := []struct {
		name     string
		want     []Problem
		startAge *StartAge
	}{
		{"zero start age min", []Problem{{Code: Required, Severity: Reject, Path: "/series/0/startAge/min"}},
			&StartAge{}},
		{"start age min bad unit", []Problem{invalid(startAge+"min", Span{N: 1, Unit: 'x'})},
			&StartAge{Min: Span{N: 1, Unit: 'x'}}},
		{"start age min too long", []Problem{invalid(startAge+"min", Span{N: 1000, Unit: Years})},
			&StartAge{Min: Span{N: 1000, Unit: Years}, Max: &Span{N: 1, Unit: Years}}},
		{"start age max bad unit", []Problem{invalid(startAge+"max", Span{N: 5, Unit: 'x'})},
			&StartAge{Min: Span{N: 1, Unit: Days}, Max: &Span{N: 5, Unit: 'x'}}},
		{"start age max negative", []Problem{invalid(startAge+"max", Span{N: -1, Unit: Days})},
			&StartAge{Min: Span{N: 1, Unit: Years}, Max: &Span{N: -1, Unit: Days}}},
		{"start age max zero", []Problem{invalid(startAge+"max", Span{})},
			&StartAge{Min: Span{N: 1, Unit: Years}, Max: &Span{}}},
		{"start age min and max invalid", []Problem{
			{Code: Required, Severity: Reject, Path: "/series/0/startAge/min"},
			invalid(startAge+"max", Span{N: 1000, Unit: Months}),
		}, &StartAge{Max: &Span{N: 1000, Unit: Months}}},
		{"start age at the cap", nil, &StartAge{Min: Span{N: 6, Unit: Weeks}, Max: &Span{N: MaxSpanN, Unit: Years}}},
	}
	for _, tc := range bracketCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := mustDecode(t, validCase)
			s.Series[0].StartAge = tc.startAge
			if got := Validate(s); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

// validateCase is one file of testdata/validate: a rejected snapshot
// (reject-*) or an accepted snapshot with Warns (warn-*), and the complete
// expected result of Validate, written by hand.
type validateCase struct {
	Snapshot jsontext.Value `json:"snapshot"`
	Problems []Problem      `json:"problems"`
}

func TestValidateTestdata(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "validate", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no validate cases: %v", err)
	}
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".json"), func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var c validateCase
			if err := json.Unmarshal(b, &c, json.RejectUnknownMembers(true)); err != nil {
				t.Fatalf("decode case: %v", err)
			}
			snap, err := Decode(c.Snapshot)
			if err != nil {
				t.Fatalf("decode snapshot: %v", err)
			}
			if !snap.Illustrative {
				t.Error("illustrative is not set; every validate case is illustrative, not medically verified")
			}
			got := Validate(snap)
			if !reflect.DeepEqual(got, c.Problems) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, c.Problems)
			}
			switch name := filepath.Base(f); {
			case strings.HasPrefix(name, "reject-"):
				if len(Rejects(c.Problems)) == 0 {
					t.Error("a reject-* case expects no Reject")
				}
			case strings.HasPrefix(name, "warn-"):
				if len(c.Problems) == 0 || len(Rejects(c.Problems)) > 0 {
					t.Errorf("a warn-* case is accepted with at least one Warn, got %v", c.Problems)
				}
			default:
				t.Error("a validate case is named reject-* or warn-*")
			}
		})
	}
}
