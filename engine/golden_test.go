package engine_test

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

var update = flag.Bool("update", false,
	"rewrite the output member of golden files with the engine's output. "+
		"Only for pure output format changes; expectations are written by hand "+
		"(SAD §5.7) and the diff is reviewed in the PR")

const illustrativeNote = "Illustrative, not medically verified"

// golden is one file of testdata/golden.
type golden struct {
	RuleSet string         `json:"ruleset"`
	Input   jsontext.Value `json:"input"`
	Output  jsontext.Value `json:"output"`
}

var strict = json.RejectUnknownMembers(true)

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
	if err := json.Unmarshal(b, &g, strict); err != nil {
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
// byte for byte with the hand-written expected output.
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
			// Decoding the expectation strictly turns a typo in a member name
			// into a clear failure instead of a byte mismatch.
			var want engine.Output
			if err := json.Unmarshal(g.Output, &want, strict); err != nil {
				t.Fatalf("decode expected output: %v", err)
			}
			wantBytes := bytes.Clone(g.Output)
			if err := (*jsontext.Value)(&wantBytes).Compact(); err != nil {
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
			if *update {
				if !bytes.Equal(got, wantBytes) {
					writeGolden(t, path, g, got)
				}
				return
			}
			if !bytes.Equal(got, wantBytes) {
				t.Errorf("output differs from %s\n--- got\n%s\n--- want\n%s", path, indent(got), indent(wantBytes))
			}
		})
	}
}

func writeGolden(t *testing.T, path string, g golden, output []byte) {
	t.Helper()
	g.Output = output
	b, err := json.Marshal(g, jsontext.WithIndent("  "))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("rewrote the output of %s: review the diff (format changes only)", path)
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
		if s.Note != illustrativeNote {
			t.Errorf("%s: note = %q, want %q", name, s.Note, illustrativeNote)
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
