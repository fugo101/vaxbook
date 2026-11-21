//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"syscall/js"
	"testing"
)

const testdata = "../testdata"

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// call invokes a global the way a browser does and requires a JS string back.
func call(t *testing.T, name string, args ...any) string {
	t.Helper()
	f := js.Global().Get(name)
	if f.Type() != js.TypeFunction {
		t.Fatalf("global %s is %s, want function", name, f.Type())
	}
	res := f.Invoke(args...)
	if res.Type() != js.TypeString {
		t.Fatalf("%s returned %s, want string", name, res.Type())
	}
	return res.String()
}

// TestJSBoundary runs one golden case through the registered globals, so the
// global names, argument handling and string results are covered, not only
// package bridge.
func TestJSBoundary(t *testing.T) {
	register()
	f := filepath.Join(testdata, "golden", "f13-on-track.json")
	var g struct {
		RuleSet string         `json:"ruleset"`
		Input   jsontext.Value `json:"input"`
		Output  jsontext.Value `json:"output"`
	}
	if err := json.Unmarshal(read(t, f), &g, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("%s: %v", f, err)
	}

	// Before a rule set loads, and with a non-string argument, the globals
	// still answer with a JSON error string.
	if got := call(t, "vaxbookEvaluate", string(g.Input)); !bytes.Contains([]byte(got), []byte(`"kind":"notLoaded"`)) {
		t.Errorf("vaxbookEvaluate before load = %s", got)
	}
	if got := call(t, "vaxbookLoadRuleSet", 42); !bytes.Contains([]byte(got), []byte(`"kind":"decode"`)) {
		t.Errorf("vaxbookLoadRuleSet(42) = %s", got)
	}

	snapshot := read(t, filepath.Join(testdata, "rulesets", g.RuleSet+".json"))
	if got := call(t, "vaxbookLoadRuleSet", string(snapshot)); got != `{"ok":true}` {
		t.Fatalf("vaxbookLoadRuleSet = %s", got)
	}
	want := bytes.Clone(g.Output)
	if err := (*jsontext.Value)(&want).Compact(); err != nil {
		t.Fatal(err)
	}
	want = append(append([]byte(`{"output":`), want...), '}')
	if got := call(t, "vaxbookEvaluate", string(g.Input)); got != string(want) {
		t.Errorf("vaxbookEvaluate:\ngot  %s\nwant %s", got, want)
	}
}
