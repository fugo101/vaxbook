package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// TestSchemaAndGoAgree checks that contracts/ruleset.schema.json and the Go
// decoders and validators accept the same documents (SAD §5.7), on every
// seed document and on deterministic mutations of each of its nodes:
//
//   - I1: schema-valid ⇒ decodes, for an I-JSON text (RFC 7493: unique
//     member names, well-formed Unicode), which a schema cannot require;
//     the decoders reject the other texts (contractjson
//     TestUnmarshalRejectsNonIJSON);
//   - I2: decodes and Validate reports nothing (no Reject for a snapshot) ⇒
//     schema-valid;
//   - I3: an Input that decodes and that Evaluate assesses is schema-valid,
//     so ⚪ INVALID_DATE (A9) stays reachable for a client that validates
//     against the schema;
//   - I4: every Output validates against $defs/Output.
//
// Seeds are the fixture rule sets, the ruleset/testdata/validate snapshots
// and the golden inputs (with the engine of their fixture), so a member a
// later slice adds to them is covered without editing this test.
func TestSchemaAndGoAgree(t *testing.T) {
	t.Parallel()
	root, input, output := compile(t, ""), compile(t, "Input"), compile(t, "Output")

	snapshot := func(t *testing.T, name string, doc any) {
		forEachMutation(doc, func(mut string, d any) {
			b := marshal(t, d)
			schemaErr := root.Validate(d)
			s, decErr := ruleset.Decode(b)
			switch {
			case schemaErr == nil && decErr != nil:
				t.Errorf("I1 %s, %s: schema-valid, but Decode: %v\n%s", name, mut, decErr, b)
			case decErr == nil && len(ruleset.Rejects(ruleset.Validate(s))) == 0 && schemaErr != nil:
				t.Errorf("I2 %s, %s: Decode and Validate accept, but the schema rejects: %v\n%s", name, mut, schemaErr, b)
			}
		})
	}
	for _, f := range glob(t, "../engine/testdata/rulesets/*.json") {
		snapshot(t, f, readJSON(t, f))
	}
	for _, f := range glob(t, "../ruleset/testdata/validate/*.json") {
		doc, _ := readJSON(t, f).(map[string]any)
		snapshot(t, f, doc["snapshot"])
	}

	engines := map[string]*engine.Engine{}
	for _, f := range glob(t, "../engine/testdata/golden/*.json") {
		doc, _ := readJSON(t, f).(map[string]any)
		name, _ := doc["ruleset"].(string)
		e, ok := engines[name]
		if !ok {
			e = loadEngine(t, name)
			engines[name] = e
		}
		forEachMutation(doc["input"], func(mut string, d any) {
			b := marshal(t, d)
			schemaErr := input.Validate(d)
			in, decErr := engine.DecodeInput(b)
			if schemaErr == nil && decErr != nil {
				t.Errorf("I1 %s, %s: schema-valid, but DecodeInput: %v\n%s", f, mut, decErr, b)
			}
			if decErr != nil {
				return
			}
			if len(e.Validate(in)) == 0 && schemaErr != nil {
				t.Errorf("I2 %s, %s: DecodeInput and Validate accept, but the schema rejects: %v\n%s", f, mut, schemaErr, b)
			}
			out, err := e.Evaluate(in)
			if err != nil {
				return
			}
			if schemaErr != nil {
				t.Errorf("I3 %s, %s: Evaluate assesses an input the schema rejects: %v\n%s", f, mut, schemaErr, b)
			}
			enc, err := engine.EncodeOutput(out)
			if err != nil {
				t.Fatal(err)
			}
			v, err := jsonschema.UnmarshalJSON(bytes.NewReader(enc))
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Validate(v); err != nil {
				t.Errorf("I4 %s, %s: output does not match $defs/Output: %v\n%s", f, mut, err, enc)
			}
		})
	}
}

func loadEngine(t *testing.T, name string) *engine.Engine {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "engine", "testdata", "rulesets", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ruleset.Decode(b)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	e, err := engine.New(s)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return e
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// mutation changes one node. ok false removes the node from its parent.
type mutation struct {
	name    string
	applies func(v any) bool
	apply   func(v any) (out any, ok bool)
}

func always(any) bool { return true }

func replaceWith(name string, v any) mutation {
	return mutation{name, always, func(any) (any, bool) { return v, true }}
}

func isString(v any) bool { _, ok := v.(string); return ok }

func stringMutation(s string) mutation {
	return mutation{"replace with " + strconv.Quote(s), isString, func(any) (any, bool) { return s, true }}
}

var mutations = []mutation{
	{"delete", always, func(any) (any, bool) { return nil, false }},
	replaceWith("set null", nil),
	replaceWith(`replace with "x"`, "x"),
	replaceWith("replace with 0", json.Number("0")),
	replaceWith("replace with true", true),
	replaceWith("replace with []", []any{}),
	replaceWith("replace with {}", map[string]any{}),
	{"add member zz", func(v any) bool { _, ok := v.(map[string]any); return ok }, func(v any) (any, bool) {
		m := maps.Clone(v.(map[string]any))
		m["zz"] = json.Number("1")
		return m, true
	}},
	{"append null", func(v any) bool { _, ok := v.([]any); return ok }, func(v any) (any, bool) {
		return append(slices.Clone(v.([]any)), nil), true
	}},
	stringMutation(""),
	stringMutation("2026-02-30"),
	stringMutation("10/02/2026"),
	stringMutation("P06W"),
	stringMutation("P1000D"),
	stringMutation("2026-03"),
	stringMutation("2026-13"),
	stringMutation("2026"),
	// JSON Schema's "integer" is a mathematical integer, so 1.0 and 1e0
	// are schema-valid dose numbers that the decoder must accept (I1).
	numberMutation("1.0"),
	numberMutation("1e0"),
	numberMutation("2.5"),
	numberMutation("-1"),
	numberMutation("1000"),
	numberMutation("2147483648"),
	numberMutation("1e300"),
}

func isNumber(v any) bool { _, ok := v.(json.Number); return ok }

func numberMutation(n string) mutation {
	return mutation{"replace with " + n, isNumber, func(any) (any, bool) { return json.Number(n), true }}
}

// forEachMutation calls f with the unchanged document and then with every
// mutation of every node, each described with its JSON pointer.
func forEachMutation(doc any, f func(desc string, mutated any)) {
	f("unchanged", doc)
	var walk func(v any, tokens []string)
	walk = func(v any, tokens []string) {
		for _, m := range mutations {
			if !m.applies(v) || (len(tokens) == 0 && m.name == "delete") {
				continue
			}
			mutated, _ := edit(doc, tokens, m.apply)
			f(fmt.Sprintf("%s at %q", m.name, pointer(tokens)), mutated)
		}
		switch x := v.(type) {
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(x)) {
				walk(x[k], append(slices.Clip(tokens), k))
			}
		case []any:
			for i, e := range x {
				walk(e, append(slices.Clip(tokens), strconv.Itoa(i)))
			}
		}
	}
	walk(doc, nil)
}

// edit returns a copy of v with fn applied to the node at tokens. It copies
// only the containers on the path, so v and its other nodes stay shared and
// unchanged.
func edit(v any, tokens []string, fn func(any) (any, bool)) (any, bool) {
	if len(tokens) == 0 {
		return fn(v)
	}
	switch x := v.(type) {
	case map[string]any:
		m := maps.Clone(x)
		if nv, ok := edit(x[tokens[0]], tokens[1:], fn); ok {
			m[tokens[0]] = nv
		} else {
			delete(m, tokens[0])
		}
		return m, true
	case []any:
		i, err := strconv.Atoi(tokens[0])
		if err != nil {
			panic(err)
		}
		s := slices.Clone(x)
		if nv, ok := edit(x[i], tokens[1:], fn); ok {
			s[i] = nv
		} else {
			s = slices.Delete(s, i, i+1)
		}
		return s, true
	}
	panic(fmt.Sprintf("edit: no container at %v", tokens))
}

func pointer(tokens []string) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteString("/" + strings.ReplaceAll(strings.ReplaceAll(tok, "~", "~0"), "/", "~1"))
	}
	return b.String()
}
