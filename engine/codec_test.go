package engine_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

const validInput = `{"today": "2026-04-01", "profile": {"birth": {"value": "2026-01-10", "precision": "day"}},
  "doses": [{"id": "r1", "diseases": ["a"], "date": {"value": "2026-03-10", "precision": "day"}}]}`

func TestDecodeInputAcceptsValid(t *testing.T) {
	t.Parallel()
	in, err := engine.DecodeInput([]byte(validInput))
	if err != nil {
		t.Fatal(err)
	}
	want := engine.Input{
		Today:   mustDate(t, "2026-04-01"),
		Profile: engine.Profile{Birth: engine.RecalledDate{Value: "2026-01-10", Precision: engine.Day}},
		Doses: []engine.DoseRecord{{ID: "r1", Diseases: []string{"a"},
			Date: engine.RecalledDate{Value: "2026-03-10", Precision: engine.Day}}},
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("DecodeInput = %+v, want %+v", in, want)
	}
	// A recalled date's value is optional and any string: Validate judges it.
	for _, v := range []string{`"date": {"precision": "day"}`, `"date": {"value": "", "precision": "day"}`, `"date": {"value": "10/02/2026", "precision": "day"}`} {
		doc := strings.Replace(validInput, `"date": {"value": "2026-03-10", "precision": "day"}`, v, 1)
		if _, err := engine.DecodeInput([]byte(doc)); err != nil {
			t.Errorf("%s: DecodeInput: %v", v, err)
		}
	}
}

// TestDecodeInputShape: DecodeInput agrees with $defs/Input on required
// members and null. Missing doses must never evaluate as "no doses given".
func TestDecodeInputShape(t *testing.T) {
	t.Parallel()
	replace := func(old, new string) string {
		s := strings.Replace(validInput, old, new, 1)
		if s == validInput {
			panic("case did not change the input: " + old)
		}
		return s
	}
	cases := []struct {
		name, doc string
		kind      contractjson.Kind
		pointer   string
	}{
		{"missing doses", replace(`,
  "doses": [{"id": "r1", "diseases": ["a"], "date": {"value": "2026-03-10", "precision": "day"}}]`, ``), contractjson.KindMissing, "/doses"},
		{"null doses", replace(`"doses": [{"id": "r1", "diseases": ["a"], "date": {"value": "2026-03-10", "precision": "day"}}]`, `"doses": null`),
			contractjson.KindNull, "/doses"},
		{"missing today", replace(`"today": "2026-04-01", `, ``), contractjson.KindMissing, "/today"},
		{"null today", replace(`"today": "2026-04-01"`, `"today": null`), contractjson.KindNull, "/today"},
		{"missing profile", replace(`"profile": {"birth": {"value": "2026-01-10", "precision": "day"}},`, ``), contractjson.KindMissing, "/profile"},
		{"missing birth", replace(`{"birth": {"value": "2026-01-10", "precision": "day"}}`, `{}`), contractjson.KindMissing, "/profile/birth"},
		{"missing precision", replace(`"value": "2026-01-10", "precision": "day"`, `"value": "2026-01-10"`), contractjson.KindMissing, "/profile/birth/precision"},
		{"missing record id", replace(`"id": "r1", `, ``), contractjson.KindMissing, "/doses/0/id"},
		{"missing record diseases", replace(`"diseases": ["a"], `, ``), contractjson.KindMissing, "/doses/0/diseases"},
		{"missing record date", replace(`, "date": {"value": "2026-03-10", "precision": "day"}`, ``), contractjson.KindMissing, "/doses/0/date"},
		{"null value", replace(`"value": "2026-03-10"`, `"value": null`), contractjson.KindNull, "/doses/0/date/value"},
		{"null disease", replace(`["a"]`, `["a", null]`), contractjson.KindNull, "/doses/0/diseases/1"},
		{"null record", replace(`"precision": "day"}}]`, `"precision": "day"}}, null]`), contractjson.KindNull, "/doses/1"},
	}
	for _, c := range cases {
		_, err := engine.DecodeInput([]byte(c.doc))
		var se *contractjson.Error
		if !errors.As(err, &se) || se.Kind != c.kind || se.Pointer != c.pointer {
			t.Errorf("%s: DecodeInput error = %v, want %s at %q", c.name, err, c.kind, c.pointer)
		}
	}
}

func TestDecodeInputSyntax(t *testing.T) {
	t.Parallel()
	for name, doc := range map[string]string{
		"unknown member":    strings.Replace(validInput, `"today"`, `"zz": 1, "today"`, 1),
		"wrong case":        strings.Replace(validInput, `"today"`, `"Today"`, 1),
		"bad today":         strings.Replace(validInput, `"2026-04-01"`, `"2026-02-30"`, 1),
		"numeric value":     strings.Replace(validInput, `"value": "2026-03-10"`, `"value": 20260310`, 1),
		"duplicate members": strings.Replace(validInput, `"today": "2026-04-01"`, `"today": "2026-04-01", "today": "2026-04-02"`, 1),
	} {
		if _, err := engine.DecodeInput([]byte(doc)); err == nil {
			t.Errorf("%s: DecodeInput succeeded", name)
		}
	}
}

// TestDecodeDoseNumbers: a dose number decodes from any JSON number with an
// integral value, which is what JSON Schema's "integer" accepts (I1), and
// from nothing else. Validate checks the range.
func TestDecodeDoseNumbers(t *testing.T) {
	t.Parallel()
	with := func(numbers string) string {
		return strings.Replace(validInput, `"precision": "day"}}]`, `"precision": "day"}, "doseNumbers": `+numbers+`}]`, 1)
	}
	for numbers, want := range map[string]engine.DoseNumber{
		`{"a": 2}`: 2, `{"a": 2.0}`: 2, `{"a": 2e0}`: 2, `{"a": 0.2e1}`: 2, `{"a": 0}`: 0, `{"a": -3}`: -3, `{"a": 1000}`: 1000,
	} {
		in, err := engine.DecodeInput([]byte(with(numbers)))
		if err != nil {
			t.Errorf("%s: DecodeInput: %v", numbers, err)
			continue
		}
		if got := in.Doses[0].DoseNumbers["a"]; got != want {
			t.Errorf("%s: dose number %d, want %d", numbers, got, want)
		}
	}
	for _, numbers := range []string{
		`{"a": 1.5}`, `{"a": "2"}`, `{"a": true}`, `{"a": [2]}`, `{"a": {}}`, `{"a": 1e300}`, `[2]`, `{"a": 2, "a": 3}`,
	} {
		if _, err := engine.DecodeInput([]byte(with(numbers))); err == nil {
			t.Errorf("%s: DecodeInput succeeded", numbers)
		}
	}
	var se *contractjson.Error
	if _, err := engine.DecodeInput([]byte(with(`{"a": null}`))); !errors.As(err, &se) || se.Kind != contractjson.KindNull || se.Pointer != "/doses/0/doseNumbers/a" {
		t.Errorf("null dose number: DecodeInput error = %v", err)
	}
	if _, err := engine.DecodeInput([]byte(with(`null`))); !errors.As(err, &se) || se.Kind != contractjson.KindNull || se.Pointer != "/doses/0/doseNumbers" {
		t.Errorf("null doseNumbers: DecodeInput error = %v", err)
	}
}
