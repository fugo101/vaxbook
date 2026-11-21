// Package contracts_test checks contracts/ruleset.schema.json against the
// engine's test data and Go structs. It lives outside engine/ so the engine
// keeps zero third-party dependencies.
package contracts_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
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

// compile returns the schema at #/$defs/<def>, or the root for def "".
func compile(t *testing.T, def string) *jsonschema.Schema {
	t.Helper()
	abs, err := filepath.Abs(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
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
		"dose number zero":         {"NextDose", `{"number": 0, "earliest": "2026-01-01", "due": "2026-01-01", "overdue": "2026-01-08"}`},
		"unknown reason code":      {"Reason", `{"code": "NOPE"}`},
		"numeric param":            {"Reason", `{"code": "DOSE_INVALID", "params": {"number": 2}}`},
		"duplicate disease":        {"DoseRecord", `{"id": "r", "diseases": ["a", "a"], "date": {"value": "2026-01-01", "precision": "day"}}`},
		"bad precision":            {"RecalledDate", `{"value": "2026", "precision": "decade"}`},
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
	// A calendar-invalid recalled date is shape-valid: the engine reports it (A9).
	v, _ := jsonschema.UnmarshalJSON(strings.NewReader(`{"value": "2026-02-30", "precision": "day"}`))
	if err := compile(t, "RecalledDate").Validate(v); err != nil {
		t.Errorf("RecalledDate rejected a shape-valid value: %v", err)
	}
}

// TestSchemaMatchesStructs: for every object $def, the schema's members are
// the Go struct's JSON members, and its required list is the members without
// omitempty or omitzero.
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
		members, mandatory := jsonMembers(typ)
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

// jsonMembers returns the JSON member names of a struct and those that are
// always present (no omitempty or omitzero).
func jsonMembers(typ reflect.Type) (all, mandatory []string) {
	for f := range typ.Fields() {
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		all = append(all, name)
		optional := false
		for o := range strings.SplitSeq(opts, ",") {
			if o == "omitempty" || o == "omitzero" {
				optional = true
			}
		}
		if !optional {
			mandatory = append(mandatory, name)
		}
	}
	return all, mandatory
}
