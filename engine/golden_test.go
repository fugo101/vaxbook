package engine_test

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

var update = flag.Bool("update", false,
	"rewrite golden outputs whose only difference from the engine output is encoding; "+
		"any value difference fails and is never written")

// golden is one file of testdata/golden.
type golden struct {
	RuleSet string         `json:"ruleset"`
	Input   jsontext.Value `json:"input"`
	Output  jsontext.Value `json:"output"`
}

func goldenFiles(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	return files
}

func readGolden(t testing.TB, path string) golden {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := contractjson.Unmarshal(b, &g); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return g
}

func fixtureFiles(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "rulesets", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixture rule sets: %v", err)
	}
	return files
}

func fixtureName(path string) string { return strings.TrimSuffix(filepath.Base(path), ".json") }

func loadSnapshot(t testing.TB, name string) *ruleset.Snapshot {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "rulesets", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ruleset.Decode(b)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return s
}

func loadEngine(t testing.TB, name string) *engine.Engine {
	t.Helper()
	e, err := engine.New(loadSnapshot(t, name))
	if err != nil {
		t.Fatalf("engine.New(%s): %v", name, err)
	}
	return e
}

func indent(b []byte) string {
	v := jsontext.Value(bytes.Clone(b))
	if err := v.Indent(jsontext.WithIndent("  ")); err != nil {
		return string(b)
	}
	return string(v)
}

// TestGolden evaluates every golden input and compares the encoded output
// byte for byte with the hand-written expected output, so it also enforces
// the canonical member order (SAD §5.7).
func TestGolden(t *testing.T) {
	t.Parallel()
	for _, path := range goldenFiles(t) {
		t.Run(fixtureName(path), func(t *testing.T) {
			t.Parallel()
			g := readGolden(t, path)
			e := loadEngine(t, g.RuleSet)
			in, err := engine.DecodeInput(g.Input)
			if err != nil {
				t.Fatal(err)
			}
			out, err := e.Evaluate(in)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			got, err := engine.EncodeOutput(out)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range out.Assessments {
				checkReasonParams(t, a)
			}
			canonical, d, err := compareGolden(g.Output, got)
			if err != nil {
				t.Fatalf("%s: invalid expected output (-update never rewrites it): %v", path, err)
			}
			switch d {
			case goldenSame:
			case goldenFormat:
				if *update {
					writeGolden(t, path, g, canonical)
					return
				}
				t.Errorf("%s: expected output has the engine's values but not its encoding (member order or spacing); "+
					"run the golden test with -update to rewrite it\n--- got\n%s\n--- want\n%s", path, indent(got), indent(g.Output))
			case goldenValues:
				t.Errorf("%s: output values differ; re-derive the expectation by hand (testdata/golden/README.md), "+
					"-update never writes a value change\n--- got\n%s\n--- want\n%s", path, indent(got), indent(g.Output))
			}
		})
	}
}

// goldenDiff is how an engine output differs from a golden expectation.
type goldenDiff int

const (
	goldenSame   goldenDiff = iota // byte for byte, after compacting the expectation
	goldenFormat                   // same values, different encoding: -update may rewrite it
	goldenValues                   // different values: never rewritten
)

func (d goldenDiff) String() string {
	return [...]string{"same", "format", "values"}[d]
}

// compareGolden compares an expectation with the engine's encoded output.
// The expectation is decoded with the contract's shape rules into an Output
// and re-encoded with engine.EncodeOutput; that canonical form decides
// whether a difference is only encoding. An expectation that does not decode
// (an unknown, renamed, missing or null member, a wrong type, a bad date) is
// an error and is never rewritten (SAD §5.7, R6 of the S1 contract review).
// So is one with a member its canonical form drops: an optional member
// written with an empty or zero value ("series": "", "number": 0,
// "params": {}) is not an encoding of absence, and the schema rejects it.
func compareGolden(expected jsontext.Value, got []byte) (canonical []byte, d goldenDiff, err error) {
	var want engine.Output
	if err := contractjson.Unmarshal(expected, &want); err != nil {
		return nil, 0, fmt.Errorf("decode expected output: %w", err)
	}
	canonical, err = engine.EncodeOutput(want)
	if err != nil {
		return nil, 0, err
	}
	if dropped, err := droppedMembers(expected, canonical); err != nil {
		return nil, 0, err
	} else if len(dropped) > 0 {
		return nil, 0, fmt.Errorf("optional members written with an empty or zero value (omit them instead): %s",
			strings.Join(dropped, ", "))
	}
	compact := bytes.Clone(expected)
	if err := (*jsontext.Value)(&compact).Compact(); err != nil {
		return nil, 0, err
	}
	switch {
	case bytes.Equal(compact, got):
		return canonical, goldenSame, nil
	case bytes.Equal(canonical, got):
		return canonical, goldenFormat, nil
	}
	return canonical, goldenValues, nil
}

// droppedMembers lists, as JSON pointers, the members of expected that
// canonical does not have. canonical is expected decoded and re-encoded, so it
// can only lose members, never gain them.
func droppedMembers(expected, canonical []byte) ([]string, error) {
	have := map[string]bool{}
	var walk func(v any, ptr string, add func(string))
	walk = func(v any, ptr string, add func(string)) {
		switch v := v.(type) {
		case map[string]any:
			for k, m := range v {
				p := ptr + "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(k)
				add(p)
				walk(m, p, add)
			}
		case []any:
			for i, m := range v {
				p := fmt.Sprintf("%s/%d", ptr, i)
				add(p)
				walk(m, p, add)
			}
		}
	}
	var e, c any
	if err := json.Unmarshal(expected, &e); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(canonical, &c); err != nil {
		return nil, err
	}
	walk(c, "", func(p string) { have[p] = true })
	var dropped []string
	walk(e, "", func(p string) {
		if !have[p] {
			dropped = append(dropped, p)
		}
	})
	slices.Sort(dropped)
	return dropped, nil
}

// writeGolden rewrites the output member of a golden file with the canonical
// encoding of its own expectation, never with the engine's output.
func writeGolden(t *testing.T, path string, g golden, canonical []byte) {
	t.Helper()
	g.Output = canonical
	b, err := json.Marshal(g, jsontext.WithIndent("  "))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("rewrote the encoding of %s (values unchanged)", path)
}

// TestFixtureRuleSets: every fixture is marked illustrative, passes
// ruleset.Validate with no problems, loads with engine.New and is used by at
// least one golden file.
func TestFixtureRuleSets(t *testing.T) {
	t.Parallel()
	used := map[string]bool{}
	for _, path := range goldenFiles(t) {
		used[readGolden(t, path).RuleSet] = true
	}
	for _, path := range fixtureFiles(t) {
		name := fixtureName(path)
		s := loadSnapshot(t, name)
		if !s.Illustrative {
			t.Errorf("%s: illustrative is not set; every fixture is illustrative, not medically verified", name)
		}
		if p := ruleset.Validate(s); len(p) != 0 {
			t.Errorf("%s: ruleset.Validate = %v, want no problems", name, p)
		}
		if _, err := engine.New(s); err != nil {
			t.Errorf("%s: engine.New: %v", name, err)
		}
		if !used[name] {
			t.Errorf("%s: no golden file uses this fixture", name)
		}
		delete(used, name)
	}
	for name := range used {
		t.Errorf("a golden file names the missing fixture %q", name)
	}
}

func TestCompareGolden(t *testing.T) {
	t.Parallel()
	const got = `{"ruleSetVersion":"v","today":"2026-04-01","assessments":[{"disease":"a","bucket":"unknown",` +
		`"nextDose":{"number":1,"earliest":"2026-02-21","due":"2026-03-10","overdue":"2026-03-17"},` +
		`"reasons":[{"code":"DOSE_INVALID","params":{"checks":["minAge"],"dose":"r1","number":1}},{"code":"ALTERNATIVE_PLANS","params":{"series":"s"}}]}]}`
	edit := func(old, new string) string {
		s := strings.Replace(got, old, new, 1)
		if s == got {
			panic("case did not change the output: " + old)
		}
		return s
	}
	cases := []struct {
		name     string
		expected string
		want     goldenDiff
		invalid  bool
	}{
		{"identical", got, goldenSame, false},
		{"indented", indent([]byte(got)), goldenSame, false},
		{"reordered members", edit(`"ruleSetVersion":"v","today":"2026-04-01"`, `"today":"2026-04-01","ruleSetVersion":"v"`), goldenFormat, false},
		{"reordered params", edit(`{"checks":["minAge"],"dose":"r1","number":1}`, `{"number":1,"dose":"r1","checks":["minAge"]}`), goldenFormat, false},
		{"due date plus one day", edit(`"due":"2026-03-10"`, `"due":"2026-03-11"`), goldenValues, false},
		{"changed bucket", edit(`"bucket":"unknown"`, `"bucket":"due"`), goldenValues, false},
		{"swapped reasons", edit(`{"code":"DOSE_INVALID","params":{"checks":["minAge"],"dose":"r1","number":1}},{"code":"ALTERNATIVE_PLANS","params":{"series":"s"}}`,
			`{"code":"ALTERNATIVE_PLANS","params":{"series":"s"}},{"code":"DOSE_INVALID","params":{"checks":["minAge"],"dose":"r1","number":1}}`), goldenValues, false},
		{"renamed member", edit(`"due":`, `"dueDate":`), 0, true},
		{"extra member", edit(`"disease":"a",`, `"disease":"a","zz":1,`), 0, true},
		{"null member", edit(`"today":"2026-04-01"`, `"today":null`), 0, true},
		{"missing member", edit(`"disease":"a",`, ``), 0, true},
		{"wrong type", edit(`"number":1,"earliest"`, `"number":"1","earliest"`), 0, true},
		{"S1 string params", edit(`{"checks":["minAge"],"dose":"r1","number":1}`, `{"check":"minAge","dose":"r1","number":"1"}`), 0, true},
		{"bad date", edit(`"due":"2026-03-10"`, `"due":"2026-02-30"`), 0, true},
		{"explicit empty optional member", edit(`"dose":"r1","number":1}`, `"dose":"r1","number":1,"series":""}`), 0, true},
		{"explicit zero optional member", edit(`"dose":"r1","number":1}`, `"dose":"r1","number":0}`), 0, true},
		{"explicit empty params", edit(`"params":{"series":"s"}`, `"params":{"series":"s"}},{"code":"COMPLETE","params":{}`), 0, true},
	}
	for _, c := range cases {
		canonical, d, err := compareGolden(jsontext.Value(c.expected), []byte(got))
		if c.invalid {
			if err == nil {
				t.Errorf("%s: compareGolden = %v, want an error", c.name, d)
			}
			continue
		}
		if err != nil || d != c.want {
			t.Errorf("%s: compareGolden = %v, %v; want %v", c.name, d, err, c.want)
			continue
		}
		if d == goldenFormat && string(canonical) != got {
			t.Errorf("%s: canonical = %s, want the engine's bytes", c.name, canonical)
		}
	}
}

// reasonParams lists the params members each reason code carries, all of
// them required (SAD §5.4). The schema does not tie params to codes yet, so
// this table checks every reason of the goldens and the property tests.
var reasonParams = map[engine.ReasonCode][]string{
	engine.ReasonInvalidDate:               {"dose"},
	engine.ReasonOutOfRange:                {"dose"},
	engine.ReasonDateInFuture:              {"dose"},
	engine.ReasonDateBeforeBirth:           {"dose"},
	engine.ReasonUnexpectedValue:           {"dose"},
	engine.ReasonUnknownProduct:            {"dose"},
	engine.ReasonOverlappingEstimatedDates: {"doses"},
	engine.ReasonNumberingConflict:         {"dose", "number"},
	engine.ReasonImpliedEarlierDoses:       {"numbers"},
	engine.ReasonSafestSeries:              {"candidates"},
	engine.ReasonBirthDateImprecise:        {"hint"},
	engine.ReasonDoseInvalid:               {"checks", "dose", "number"},
	engine.ReasonDosePossiblyInvalid:       {"checks", "dose", "number"},
	engine.ReasonIntervalUnverifiable:      {"dose", "number"},
	engine.ReasonComplete:                  nil,
	engine.ReasonProductNotInCatalog:       {"doses"},
	engine.ReasonPreviousDoseDateUnknown:   {"dose"},
	engine.ReasonNoBirthDate:               nil,
	engine.ReasonFirstDoseDateAssumed:      {"dose"},
	engine.ReasonAlternativePlans:          {"series"},
	engine.ReasonOnTrack:                   nil,
	engine.ReasonAgeAtStartUncertain:       {"candidates"},
	engine.ReasonOutOfAge:                  nil,
	engine.ReasonDoseAfterMaxAge:           {"checks", "dose", "number"},
	engine.ReasonCheckProductName:          nil,
	engine.ReasonMixedProducts:             nil,
	engine.ReasonDueCappedByMaxAge:         {"lastDay"},
	engine.ReasonSeriesMayNotComplete:      {"lastDay"},
	engine.ReasonOutOfAgeUncertain:         {"numbers"},
}

// checkReasonParams fails when a reason of a carries params other than
// those its code lists.
func checkReasonParams(t testing.TB, a engine.Assessment) {
	t.Helper()
	typ := reflect.TypeFor[engine.ReasonParams]()
	members := contractjson.Members(typ)
	if len(members) != typ.NumField() {
		t.Fatalf("ReasonParams has fields that are not JSON members")
	}
	for _, r := range a.Reasons {
		want, ok := reasonParams[r.Code]
		if !ok {
			t.Errorf("%s: reason code %s has no row in reasonParams", a.Disease, r.Code)
			continue
		}
		v := reflect.ValueOf(r.Params)
		var got []string
		for i, m := range members {
			if !v.Field(i).IsZero() {
				got = append(got, m.Name)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %s params %v, want exactly %v", a.Disease, r.Code, got, want)
		}
		// Checks are in catalog order, without repeats. DOSE_INVALID lists
		// minimums only and DOSE_AFTER_MAX_AGE maximum ages only (issue #15,
		// Q62); DOSE_POSSIBLY_INVALID lists either.
		minimums := []engine.Check{engine.CheckMinAge, engine.CheckMinInterval, engine.CheckMinIntervalFromFirst}
		maximums := []engine.Check{engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete}
		allowed := slices.Concat(minimums, maximums)
		switch r.Code {
		case engine.ReasonDoseInvalid:
			allowed = minimums
		case engine.ReasonDoseAfterMaxAge:
			allowed = maximums
		}
		last := -1
		for _, c := range r.Params.Checks {
			i := slices.Index(allowed, c)
			if i < 0 {
				t.Errorf("%s: %s has unknown check %q", a.Disease, r.Code, c)
			} else if i <= last {
				t.Errorf("%s: %s checks %v are not in catalog order", a.Disease, r.Code, r.Params.Checks)
			}
			last = max(last, i)
		}
		if h := r.Params.Hint; h != "" && h != engine.HintBirthMonth && h != engine.HintFullBirthDate {
			t.Errorf("%s: %s has unknown hint %q", a.Disease, r.Code, h)
		}
		for _, c := range r.Params.Candidates {
			if c.Series == "" {
				t.Errorf("%s: %s has a candidate without a series: %+v", a.Disease, r.Code, r.Params.Candidates)
			}
		}
	}
}
