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

// The rejected snapshots are shape-valid, so only ruleset.Validate rejects
// them, except those named reject-schema-*: they break a schema keyword that
// Validate mirrors (minLength, uniqueItems, the Id pattern), and the schema
// must reject them too (R1).
func TestRejectedRuleSetsMatchSchema(t *testing.T) {
	t.Parallel()
	snap := compile(t, "Snapshot")
	for _, f := range glob(t, "../ruleset/testdata/validate/*.json") {
		doc, _ := readJSON(t, f).(map[string]any)
		err := snap.Validate(doc["snapshot"])
		switch schemaOnly := strings.HasPrefix(filepath.Base(f), "reject-schema-"); {
		case schemaOnly && err == nil:
			t.Errorf("%s: the schema accepts a snapshot named reject-schema-*", f)
		case !schemaOnly && err != nil:
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
		"bucket and block":          {"Assessment", `{"disease": "a", "bucket": "due", "block": "notYetScheduled"}`},
		"neither bucket nor block":  {"Assessment", `{"disease": "a"}`},
		"unknown with next dose":    {"Assessment", `{"disease": "a", "bucket": "unknown", "nextDose": {"number": 1, "earliest": "2026-01-01", "due": "2026-01-01", "overdue": "2026-01-08"}}`},
		"unknown member":            {"Disease", `{"id": "a", "name": "A", "extra": 1}`},
		"span with leading zero":    {"Span", `"P06W"`},
		"span without unit":         {"Span", `"P6"`},
		"compact S1 span":           {"Span", `"6w"`},
		"compound span":             {"Span", `"P1Y6M"`},
		"impossible date":           {"Date", `"2026-02-30"`},
		"year zero":                 {"Date", `"0000-01-01"`},
		"month thirteen":            {"Date", `"2026-13-01"`},
		"day 31 in a 30-day month":  {"Date", `"2026-04-31"`},
		"leap day, common year":     {"Date", `"2025-02-29"`},
		"leap day, 1900":            {"Date", `"1900-02-29"`},
		"dose number zero":          {"NextDose", `{"number": 0, "earliest": "2026-01-01", "due": "2026-01-01", "overdue": "2026-01-08"}`},
		"unknown reason code":       {"Reason", `{"code": "NOPE"}`},
		"string count param":        {"Reason", `{"code": "DOSE_INVALID", "params": {"checks": ["minAge"], "dose": "r1", "number": "2"}}`},
		"comma-joined checks":       {"Reason", `{"code": "DOSE_INVALID", "params": {"checks": "minAge,minInterval", "dose": "r1", "number": 2}}`},
		"unknown check":             {"ReasonParams", `{"checks": ["maxAge"]}`},
		"repeated check":            {"ReasonParams", `{"checks": ["minAge", "minAge"]}`},
		"no checks":                 {"ReasonParams", `{"checks": []}`},
		"empty params":              {"Reason", `{"code": "COMPLETE", "params": {}}`},
		"unknown param":             {"ReasonParams", `{"disease": "a"}`},
		"param number zero":         {"ReasonParams", `{"number": 0}`},
		"duplicate disease":         {"DoseRecord", `{"id": "r", "diseases": ["a", "a"], "date": {"value": "2026-01-01", "precision": "day"}}`},
		"bad precision":             {"RecalledDate", `{"value": "2026", "precision": "decade"}`},
		"null date value":           {"RecalledDate", `{"value": null, "precision": "day"}`},
		"numeric date value":        {"RecalledDate", `{"value": 20260210, "precision": "day"}`},
		"uppercase id":              {"Id", `"Ab"`},
		"user dose number zero":     {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": {"a": 0}}`},
		"dose number fraction":      {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": {"a": 1.5}}`},
		"dose number too large":     {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": {"a": 1000}}`},
		"dose number string":        {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": {"a": "2"}}`},
		"dose numbers bad key":      {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": {"Bad Key": 1}}`},
		"dose numbers array":        {"DoseRecord", `{"id": "r", "diseases": ["a"], "date": {"precision": "unknown"}, "doseNumbers": [1]}`},
		"estimated false":           {"Assessment", `{"disease": "a", "bucket": "due", "estimated": false}`},
		"unknown hint":              {"ReasonParams", `{"hint": "fullName"}`},
		"no doses":                  {"ReasonParams", `{"doses": []}`},
		"repeated overlapping dose": {"ReasonParams", `{"doses": ["r1", "r1"]}`},
		"no implied numbers":        {"ReasonParams", `{"numbers": []}`},
		"implied number zero":       {"ReasonParams", `{"numbers": [0]}`},
		"one candidate":             {"ReasonParams", `{"candidates": [{"series": "s"}]}`},
		"candidate without series":  {"ReasonParams", `{"candidates": [{"product": "p"}, {"series": "s"}]}`},
		"repeated candidate":        {"ReasonParams", `{"candidates": [{"series": "s"}, {"series": "s"}]}`},
		"candidate empty product":   {"SeriesRef", `{"product": "", "series": "s"}`},
		"candidates as strings":     {"ReasonParams", `{"candidates": ["p/s", "generic/t"]}`},
		"empty seriesUsed":          {"Assessment", `{"disease": "a", "bucket": "due", "seriesUsed": []}`},
		"repeated seriesUsed":       {"Assessment", `{"disease": "a", "bucket": "due", "seriesUsed": ["s", "s"]}`},
		"empty products":            {"Assessment", `{"disease": "a", "bucket": "due", "products": []}`},
		"empty dose product":        {"DoseRecord", `{"id": "r", "diseases": ["a"], "product": "", "date": {"precision": "unknown"}}`},
		"empty free product":        {"DoseRecord", `{"id": "r", "diseases": ["a"], "freeProduct": "", "date": {"precision": "unknown"}}`},
		"numeric dose product":      {"DoseRecord", `{"id": "r", "diseases": ["a"], "product": 1, "date": {"precision": "unknown"}}`},
		"product without live":      {"Product", `{"id": "p", "name": "P", "diseases": ["a"]}`},
		"product live as a string":  {"Product", `{"id": "p", "name": "P", "diseases": ["a"], "live": "true"}`},
		"product without diseases":  {"Product", `{"id": "p", "name": "P", "diseases": [], "live": false}`},
		"product repeated disease":  {"Product", `{"id": "p", "name": "P", "diseases": ["a", "a"], "live": false}`},
		"product empty name":        {"Product", `{"id": "p", "name": "", "diseases": ["a"], "live": false}`},
		"out of age with next dose": {"Assessment", `{"disease": "a", "block": "outOfAge", "nextDose": {"number": 1, "earliest": "2026-01-01", "due": "2026-01-01", "overdue": "2026-01-08"}}`},
		"start age without min":     {"StartAge", `{"max": "P15Y"}`},
		"start age bad span":        {"StartAge", `{"min": "9y"}`},
		"start age unknown member":  {"StartAge", `{"min": "P9Y", "upTo": "P15Y"}`},
		"series product not an id":  {"Series", `{"id": "s", "disease": "a", "product": "Bad", "stage": "childhood", "plans": [{"id": "p", "doses": [{"minAge": "P6W", "recommended": {"from": "birth", "offset": "P2M"}}]}]}`},
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

// TestDoseProductAcceptsAnyString: engine.Validate reports a product ID the
// rule set does not have as UNKNOWN_PRODUCT, record-scoped, and Evaluate
// still assesses the profile (⚪, issue #14 Q35). So the schema gives the
// member no Id pattern, or a client validating against it would reject the
// record instead.
func TestDoseProductAcceptsAnyString(t *testing.T) {
	t.Parallel()
	record := compile(t, "DoseRecord")
	for _, value := range []string{"rotavin", "Rotavin", "vaccine x", "-", "２"} {
		doc := map[string]any{"id": "r", "diseases": []any{"a"}, "product": value, "date": map[string]any{"precision": "unknown"}}
		if err := record.Validate(doc); err != nil {
			t.Errorf("DoseRecord rejected product %q: %v", value, err)
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

// TestSpanPatternMatchesParseSpan: $defs/Span's pattern accepts exactly the
// strings ruleset.ParseSpan accepts, over every count up to past the cap,
// every designator letter in both cases, and leading zeros.
func TestSpanPatternMatchesParseSpan(t *testing.T) {
	t.Parallel()
	pattern, _ := schemaDefs(t)["Span"]["pattern"].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("$defs/Span pattern %q: %v", pattern, err)
	}
	check := func(s string) {
		_, perr := ruleset.ParseSpan(s)
		if got, want := re.MatchString(s), perr == nil; got != want {
			t.Errorf("%q: pattern match %v, ParseSpan ok %v", s, got, want)
		}
	}
	for n := range 1201 {
		for _, u := range "DWMYdwmyHST" {
			for _, form := range []string{"P%d%c", "P0%d%c", "P%03d%c", "p%d%c", "%d%c", "PT%d%c", "P%d%cP"} {
				check(fmt.Sprintf(form, n, u))
			}
		}
	}
	for _, s := range []string{"", "P", "PD", "P-1D", "P1Y6M", "P14W6D", " P1D", "P1D "} {
		check(s)
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
		"Product":      reflect.TypeFor[ruleset.Product](),
		"Series":       reflect.TypeFor[ruleset.Series](),
		"StartAge":     reflect.TypeFor[ruleset.StartAge](),
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
		"ReasonParams": reflect.TypeFor[engine.ReasonParams](),
		"SeriesRef":    reflect.TypeFor[engine.SeriesRef](),
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
// Problem codes are not here: the schema has no Problem definition (engine
// problems reach clients only through the WASM bridge, and openapi.yaml has
// only the RFC 9457 wrapper). TestProblemCodeStrings in engine and ruleset
// pins their strings instead (issue #14, In scope 12).
func TestSchemaEnumsMatchConstants(t *testing.T) {
	t.Parallel()
	defs := schemaDefs(t)
	enum := func(def, member string) []string {
		props, _ := defs[def]["properties"].(map[string]any)
		m, _ := props[member].(map[string]any)
		if m["type"] == "array" {
			m, _ = m["items"].(map[string]any)
		}
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
		{"Assessment", "block", str(engine.BlockNotYetScheduled, engine.BlockOutOfAge)},
		{"Reason", "code", str(engine.ReasonInvalidDate, engine.ReasonOutOfRange, engine.ReasonDateInFuture, engine.ReasonDateBeforeBirth,
			engine.ReasonUnexpectedValue, engine.ReasonUnknownProduct, engine.ReasonOverlappingEstimatedDates, engine.ReasonNumberingConflict,
			engine.ReasonImpliedEarlierDoses, engine.ReasonSafestSeries, engine.ReasonBirthDateImprecise, engine.ReasonDoseInvalid,
			engine.ReasonDosePossiblyInvalid, engine.ReasonIntervalUnverifiable, engine.ReasonComplete, engine.ReasonProductNotInCatalog,
			engine.ReasonPreviousDoseDateUnknown, engine.ReasonNoBirthDate, engine.ReasonFirstDoseDateAssumed,
			engine.ReasonAlternativePlans, engine.ReasonOnTrack, engine.ReasonAgeAtStartUncertain, engine.ReasonOutOfAge,
			engine.ReasonDoseAfterMaxAge, engine.ReasonCheckProductName, engine.ReasonMixedProducts,
			engine.ReasonDueCappedByMaxAge, engine.ReasonSeriesMayNotComplete)},
		{"ReasonParams", "checks", str(engine.CheckMinAge, engine.CheckMinInterval, engine.CheckMinIntervalFromFirst,
			engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete)},
		{"ReasonParams", "hint", str(engine.HintBirthMonth, engine.HintFullBirthDate)},
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
