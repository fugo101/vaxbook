// Package contracts_test checks contracts/ruleset.schema.json against the
// engine's test data and Go structs. It lives outside engine/ so the engine
// keeps zero third-party dependencies.
package contracts_test

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

const schemaFile = "ruleset.schema.json"

func readJSON(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}

// compile returns the schema at #/$defs/<def>, or the root for def "". It
// leaves format assertion off, the draft 2020-12 default, so the tests see
// what a consumer with default settings sees: the schema must not depend on
// "format" to reject anything.
func compile(t *testing.T, def string) *jsonschema.Schema {
	t.Helper()
	abs, err := filepath.Abs(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource(abs, readJSON(t, schemaFile)); err != nil {
		t.Fatal(err)
	}
	loc := abs
	if def != "" {
		loc += "#/$defs/" + def
	}
	s, err := c.Compile(loc)
	if err != nil {
		t.Fatalf("compile %s: %v", loc, err)
	}
	return s
}

func glob(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.FromSlash(pattern))
	if err != nil || len(files) == 0 {
		t.Fatalf("no files match %s: %v", pattern, err)
	}
	return files
}

func TestFixtureRuleSetsMatchSchema(t *testing.T) {
	t.Parallel()
	root := compile(t, "")
	for _, f := range glob(t, "../engine/testdata/rulesets/*.json") {
		if err := root.Validate(readJSON(t, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// The rejected snapshots are shape-valid: only ruleset.Validate rejects them.
func TestRejectedRuleSetsMatchSchema(t *testing.T) {
	t.Parallel()
	snap := compile(t, "Snapshot")
	for _, f := range glob(t, "../ruleset/testdata/validate/*.json") {
		doc, _ := readJSON(t, f).(map[string]any)
		if err := snap.Validate(doc["snapshot"]); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestGoldenFilesMatchSchema(t *testing.T) {
	t.Parallel()
	input, output := compile(t, "Input"), compile(t, "Output")
	for _, f := range glob(t, "../engine/testdata/golden/*.json") {
		doc, _ := readJSON(t, f).(map[string]any)
		if err := input.Validate(doc["input"]); err != nil {
			t.Errorf("%s input: %v", f, err)
		}
		if err := output.Validate(doc["output"]); err != nil {
			t.Errorf("%s output: %v", f, err)
		}
	}
}

func TestSchemaRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ def, doc string }{
		"bucket and block":         {"Assessment", `{"disease": "a", "bucket": "due", "block": "notYetScheduled"}`},
		"neither bucket nor block": {"Assessment", `{"disease": "a"}`},
		"unknown member":           {"Disease", `{"id": "a", "name": "A", "extra": 1}`},
		"span with leading zero":   {"Span", `"06w"`},
		"span without unit":        {"Span", `"6"`},
		"impossible date":          {"Date", `"2026-02-30"`},
		"year zero":                {"Date", `"0000-01-01"`},
		"month thirteen":           {"Date", `"2026-13-01"`},
		"day 31 in a 30-day month": {"Date", `"2026-04-31"`},
		"leap day, common year":    {"Date", `"2025-02-29"`},
		"leap day, 1900":           {"Date", `"1900-02-29"`},
		"dose number zero":         {"NextDose", `{"number": 0, "earliest": "2026-01-01", "due": "2026-01-01", "overdue": "2026-01-08"}`},
		"unknown reason code":      {"Reason", `{"code": "NOPE"}`},
		"numeric param":            {"Reason", `{"code": "DOSE_INVALID", "params": {"number": 2}}`},
		"duplicate disease":        {"DoseRecord", `{"id": "r", "diseases": ["a", "a"], "date": {"value": "2026-01-01", "precision": "day"}}`},
		"bad precision":            {"RecalledDate", `{"value": "2026", "precision": "decade"}`},
		"null date value":          {"RecalledDate", `{"value": null, "precision": "day"}`},
		"numeric date value":       {"RecalledDate", `{"value": 20260210, "precision": "day"}`},
		"uppercase id":             {"Id", `"Ab"`},
	}
	for name, c := range cases {
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(c.doc))
		if err != nil {
			t.Fatal(err)
		}
		if err := compile(t, c.def).Validate(v); err == nil {
			t.Errorf("%s: %s accepted %s", name, c.def, c.doc)
		}
	}
}

// TestRecalledDateAcceptsAnyString: engine.Validate reports a recalled date
// value that does not parse as INVALID_DATE, and on a dose record Evaluate
// still assesses the profile (⚪, A9). So the schema accepts every string,
// or a client validating against it would reject the record instead.
func TestRecalledDateAcceptsAnyString(t *testing.T) {
	t.Parallel()
	recalled := compile(t, "RecalledDate")
	for _, value := range []string{
		"2026-02-30", "10/02/2026", "2026-1-5", "", " 2026-01-01", "2026-01-01T00:00:00Z",
		"２０２６-０１-０１", "2026-01", "2026", strings.Repeat("9", 200),
	} {
		doc := map[string]any{"value": value, "precision": "day"}
		if err := recalled.Validate(doc); err != nil {
			t.Errorf("RecalledDate rejected value %q: %v", value, err)
		}
	}
}

// TestDatePatternMatchesParseDate: $defs/Date's pattern alone accepts exactly
// the strings engine.ParseDate accepts, so a validator that treats "format" as
// an annotation agrees with the decoder. Every year is tried on the dates
// where the calendar depends on the year; every month and day pair, valid or
// not, is tried on years at the edges and at the leap-year rule's cases.
func TestDatePatternMatchesParseDate(t *testing.T) {
	t.Parallel()
	pattern, _ := schemaDefs(t)["Date"]["pattern"].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("$defs/Date pattern %q: %v", pattern, err)
	}
	check := func(s string) {
		_, perr := engine.ParseDate(s)
		if got, want := re.MatchString(s), perr == nil; got != want {
			t.Errorf("%q: pattern match %v, ParseDate ok %v", s, got, want)
		}
	}
	for y := range 10000 {
		for _, md := range []string{"01-01", "02-28", "02-29", "02-30", "12-31"} {
			check(fmt.Sprintf("%04d-%s", y, md))
		}
	}
	for _, y := range []int{0, 1, 4, 100, 400, 1900, 1999, 2000, 2023, 2024, 2100, 9999} {
		for m := range 14 {
			for d := range 33 {
				check(fmt.Sprintf("%04d-%02d-%02d", y, m, d))
			}
		}
	}
}

// TestSchemaMatchesStructs: for every object $def, the schema's members are
// the Go struct's JSON members, and its required list is the members the
// decoders require (contractjson.Members: no omitempty or omitzero).
func TestSchemaMatchesStructs(t *testing.T) {
	t.Parallel()
	types := map[string]reflect.Type{
		"Snapshot":     reflect.TypeFor[ruleset.Snapshot](),
		"Disease":      reflect.TypeFor[ruleset.Disease](),
		"Series":       reflect.TypeFor[ruleset.Series](),
		"Plan":         reflect.TypeFor[ruleset.Plan](),
		"DoseRule":     reflect.TypeFor[ruleset.DoseRule](),
		"Milestone":    reflect.TypeFor[ruleset.Milestone](),
		"Input":        reflect.TypeFor[engine.Input](),
		"Profile":      reflect.TypeFor[engine.Profile](),
		"RecalledDate": reflect.TypeFor[engine.RecalledDate](),
		"DoseRecord":   reflect.TypeFor[engine.DoseRecord](),
		"Output":       reflect.TypeFor[engine.Output](),
		"Assessment":   reflect.TypeFor[engine.Assessment](),
		"NextDose":     reflect.TypeFor[engine.NextDose](),
		"Reason":       reflect.TypeFor[engine.Reason](),
	}
	defs := schemaDefs(t)
	for name, def := range defs {
		if def["type"] == "object" {
			if _, ok := types[name]; !ok {
				t.Errorf("$defs/%s has no Go struct in this test", name)
			}
		}
	}
	for name, typ := range types {
		def, ok := defs[name]
		if !ok {
			t.Errorf("no $defs/%s for %s", name, typ)
			continue
		}
		props, _ := def["properties"].(map[string]any)
		var required []string
		for _, r := range asSlice(def["required"]) {
			required = append(required, r.(string))
		}
		var members, mandatory []string
		for _, m := range contractjson.Members(typ) {
			members = append(members, m.Name)
			if m.Required {
				mandatory = append(mandatory, m.Name)
			}
		}
		if got, want := slices.Sorted(maps.Keys(props)), slices.Sorted(slices.Values(members)); !slices.Equal(got, want) {
			t.Errorf("$defs/%s properties %v, Go %s members %v", name, got, typ, want)
		}
		slices.Sort(required)
		slices.Sort(mandatory)
		if !slices.Equal(required, mandatory) {
			t.Errorf("$defs/%s required %v, Go %s members without omitempty/omitzero %v", name, required, typ, mandatory)
		}
		if def["additionalProperties"] != false {
			t.Errorf("$defs/%s allows additional properties", name)
		}
	}
}

// TestSchemaEnumsMatchConstants: every enum lists exactly the Go constants.
func TestSchemaEnumsMatchConstants(t *testing.T) {
	t.Parallel()
	defs := schemaDefs(t)
	enum := func(def, member string) []string {
		props, _ := defs[def]["properties"].(map[string]any)
		m, _ := props[member].(map[string]any)
		var out []string
		for _, v := range asSlice(m["enum"]) {
			out = append(out, v.(string))
		}
		slices.Sort(out)
		return out
	}
	str := func(vs ...any) []string {
		var out []string
		for _, v := range vs {
			out = append(out, reflect.ValueOf(v).String())
		}
		slices.Sort(out)
		return out
	}
	cases := []struct {
		def, member string
		want        []string
	}{
		{"Series", "stage", str(ruleset.Childhood, ruleset.Adult)},
		{"Milestone", "from", str(ruleset.FromBirth, ruleset.FromFirstDose, ruleset.FromPreviousDose)},
		{"RecalledDate", "precision", str(engine.Day, engine.Month, engine.Year, engine.Unknown, engine.None)},
		{"Assessment", "bucket", str(engine.BucketOverdue, engine.BucketDue, engine.BucketUpcoming, engine.BucketUnknown, engine.BucketComplete)},
		{"Assessment", "block", str(engine.BlockNotYetScheduled)},
		{"Reason", "code", str(engine.ReasonInvalidDate, engine.ReasonDoseInvalid, engine.ReasonAlternativePlans, engine.ReasonComplete, engine.ReasonOnTrack)},
	}
	for _, c := range cases {
		if got := enum(c.def, c.member); !slices.Equal(got, c.want) {
			t.Errorf("$defs/%s.%s enum %v, Go constants %v", c.def, c.member, got, c.want)
		}
	}
}

func schemaDefs(t *testing.T) map[string]map[string]any {
	t.Helper()
	root, _ := readJSON(t, schemaFile).(map[string]any)
	raw, _ := root["$defs"].(map[string]any)
	defs := make(map[string]map[string]any, len(raw))
	for k, v := range raw {
		defs[k], _ = v.(map[string]any)
	}
	return defs
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
