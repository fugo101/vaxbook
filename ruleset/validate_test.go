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
      {"minAge": "6w", "recommended": {"from": "birth", "offset": "2m"}},
      {"minInterval": "4w", "recommended": {"from": "previousDose", "offset": "2m"}, "buffer": "14d"}
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
		"bad span":         strings.Replace(validCase, `"6w"`, `"6 weeks"`, 1),
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
		{"missing offset", strings.Replace(validCase, `"recommended": {"from": "birth", "offset": "2m"}`, `"recommended": {"from": "birth"}`, 1),
			contractjson.KindMissing, "/series/0/plans/0/doses/0/recommended/offset"},
		{"missing recommended", strings.Replace(validCase, `, "recommended": {"from": "birth", "offset": "2m"}`, ``, 1),
			contractjson.KindMissing, "/series/0/plans/0/doses/0/recommended"},
		{"missing plan doses", `{"version": "v1", "diseases": [], "series": [{"id": "s", "disease": "d", "stage": "adult", "plans": [{"id": "p"}]}]}`,
			contractjson.KindMissing, "/series/0/plans/0/doses"},
		{"null version", strings.Replace(validCase, `"version": "v1"`, `"version": null`, 1), contractjson.KindNull, "/version"},
		{"null min interval", strings.Replace(validCase, `"minInterval": "4w"`, `"minInterval": null`, 1),
			contractjson.KindNull, "/series/0/plans/0/doses/1/minInterval"},
		{"null buffer", strings.Replace(validCase, `"buffer": "14d"`, `"buffer": null`, 1),
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
				{Code: InvalidValue, Severity: Reject, Path: "/diseases/0/id", Params: map[string]string{"value": "D!"}},
				{Code: UnknownReference, Severity: Reject, Path: "/series/0/disease", Params: map[string]string{"disease": "d"}},
			}},
		{"missing disease name", `"name": "D"`, `"name": ""`,
			[]Problem{{Code: Required, Severity: Reject, Path: "/diseases/0/name"}}},
		{"duplicate disease id", `{"id": "d", "name": "D"}`, `{"id": "d", "name": "D"}, {"id": "d", "name": "E"}`,
			[]Problem{{Code: DuplicateID, Severity: Reject, Path: "/diseases/1/id", Params: map[string]string{"id": "d"}}}},
		{"unknown disease", `"disease": "d"`, `"disease": "x"`,
			[]Problem{{Code: UnknownReference, Severity: Reject, Path: "/series/0/disease", Params: map[string]string{"disease": "x"}}}},
		{"bad stage", `"stage": "childhood"`, `"stage": "teen"`,
			[]Problem{{Code: InvalidValue, Severity: Reject, Path: "/series/0/stage", Params: map[string]string{"value": "teen"}}}},
		{"bad anchor", `"from": "previousDose"`, `"from": "lastDose"`,
			[]Problem{{Code: InvalidValue, Severity: Reject, Path: "/series/0/plans/0/doses/1/recommended/from", Params: map[string]string{"value": "lastDose"}}}},
		{"first dose interval", `{"minAge": "6w", "recommended"`, `{"minAge": "6w", "minInterval": "1w", "recommended"`,
			[]Problem{{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minInterval"}}},
		{"first dose without min age", `{"minAge": "6w", "recommended"`, `{"minInterval": "1w", "recommended"`,
			[]Problem{
				{Code: DoseRuleNoMinimum, Severity: Reject, Path: "/series/0/plans/0/doses/0"},
				{Code: FirstDoseHasInterval, Severity: Reject, Path: "/series/0/plans/0/doses/0/minInterval"},
			}},
		{"empty anchor", `"from": "previousDose"`, `"from": ""`,
			[]Problem{{Code: Required, Severity: Reject, Path: "/series/0/plans/0/doses/1/recommended/from"}}},
		{"first dose anchor", `"from": "birth"`, `"from": "firstDose"`,
			[]Problem{{Code: FirstDoseAnchorNotBirth, Severity: Reject, Path: "/series/0/plans/0/doses/0/recommended/from"}}},
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
	      {"id": "p", "doses": [{"minAge": "0d", "recommended": {"from": "birth", "offset": "0d"}}]},
	      {"id": "p", "doses": []}
	    ]},
	    {"id": "s", "disease": "d", "stage": "adult", "plans": [
	      {"id": "p", "doses": [{"minAge": "0d", "recommended": {"from": "birth", "offset": "0d"}}]}
	    ]}
	  ]}`
	want := []Problem{
		{Code: PlanDoseCountMismatch, Severity: Reject, Path: "/series/0/plans/1"},
		{Code: DuplicateID, Severity: Reject, Path: "/series/0/plans/1/id", Params: map[string]string{"id": "p"}},
		{Code: Empty, Severity: Reject, Path: "/series/0/plans/1/doses"},
		{Code: DuplicateID, Severity: Reject, Path: "/series/1/id", Params: map[string]string{"id": "s"}},
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
	invalid := func(path string, s Span) Problem {
		return Problem{Code: InvalidValue, Severity: Reject, Path: path, Params: map[string]string{"value": s.String()}}
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
}

// validateCase is one file of testdata/validate: a rejected snapshot and the
// complete expected result of Validate, written by hand.
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
			if snap.Note != "Illustrative, not medically verified" {
				t.Errorf("note = %q, want the illustrative marker", snap.Note)
			}
			got := Validate(snap)
			if !reflect.DeepEqual(got, c.Problems) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, c.Problems)
			}
		})
	}
}
