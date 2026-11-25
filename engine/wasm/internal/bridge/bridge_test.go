package bridge

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testdata = "../../../testdata"

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type decodedError struct {
	Error struct {
		Kind     string         `json:"kind"`
		Message  string         `json:"message"`
		Problems jsontext.Value `json:"problems"`
	} `json:"error"`
}

func errorKind(t *testing.T, res []byte) decodedError {
	t.Helper()
	var r decodedError
	if err := json.Unmarshal(res, &r, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("result %s: %v", res, err)
	}
	return r
}

// TestGoldenThroughBridge feeds every golden input through the JSON bridge
// the browser calls and checks the output bytes equal the golden output.
func TestGoldenThroughBridge(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(testdata, "golden", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	for _, f := range files {
		var g struct {
			RuleSet string         `json:"ruleset"`
			Input   jsontext.Value `json:"input"`
			Output  jsontext.Value `json:"output"`
		}
		if err := json.Unmarshal(read(t, f), &g, json.RejectUnknownMembers(true)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var b Bridge
		if res := b.LoadRuleSet(read(t, filepath.Join(testdata, "rulesets", g.RuleSet+".json"))); string(res) != `{"ok":true}` {
			t.Fatalf("%s: LoadRuleSet = %s", f, res)
		}
		want := bytes.Clone(g.Output)
		if err := (*jsontext.Value)(&want).Compact(); err != nil {
			t.Fatal(err)
		}
		want = append(append([]byte(`{"output":`), want...), '}')
		if got := b.Evaluate(g.Input); !bytes.Equal(got, want) {
			t.Errorf("%s:\ngot  %s\nwant %s", f, got, want)
		}
	}
}

func TestBridgeErrors(t *testing.T) {
	t.Parallel()
	var b Bridge
	if r := errorKind(t, b.Evaluate([]byte(`{}`))); r.Error.Kind != "notLoaded" {
		t.Errorf("Evaluate before load: %+v", r)
	}
	if r := errorKind(t, b.LoadRuleSet([]byte(`{"version": 1}`))); r.Error.Kind != "decode" || r.Error.Message == "" {
		t.Errorf("bad snapshot: %+v", r)
	}
	rejected := read(t, filepath.Join("..", "..", "..", "..", "ruleset", "testdata", "validate", "reject-reports-every-problem.json"))
	var c struct {
		Snapshot jsontext.Value `json:"snapshot"`
		Problems jsontext.Value `json:"problems"`
	}
	if err := json.Unmarshal(rejected, &c); err != nil {
		t.Fatal(err)
	}
	if r := errorKind(t, b.LoadRuleSet(c.Snapshot)); r.Error.Kind != "ruleset" || !jsonEqual(t, r.Error.Problems, c.Problems) {
		t.Errorf("rejected snapshot: kind %q problems %s, want %s", r.Error.Kind, r.Error.Problems, c.Problems)
	}
	// A disease with no series loads since S4c (issue #29, Q81) and is
	// assessed ⚪ NO_SERIES_IN_EFFECT.
	seriesless := `{"version": "v", "diseases": [{"id": "a", "name": "A"}], "series": []}`
	if res := b.LoadRuleSet([]byte(seriesless)); string(res) != `{"ok":true}` {
		t.Errorf("series-less snapshot: LoadRuleSet = %s", res)
	}
	noSeries := `{"today": "2026-04-01", "profile": {"birth": {"value": "2026-02-01", "precision": "day"}}, "doses": []}`
	if res := b.Evaluate([]byte(noSeries)); !strings.Contains(string(res), `"reasons":[{"code":"NO_SERIES_IN_EFFECT"}]`) {
		t.Errorf("series-less snapshot: Evaluate = %s", res)
	}
	if res := b.LoadRuleSet(read(t, filepath.Join(testdata, "rulesets", "minimal-single.json"))); string(res) != `{"ok":true}` {
		t.Fatalf("LoadRuleSet = %s", res)
	}
	if r := errorKind(t, b.Evaluate([]byte(`{"today": "2026-13-01"}`))); r.Error.Kind != "decode" {
		t.Errorf("bad input: %+v", r)
	}
	in := `{"today": "2026-04-01", "profile": {"birth": {"value": "2026-02-30", "precision": "day"}}, "doses": []}`
	r := errorKind(t, b.Evaluate([]byte(in)))
	if r.Error.Kind != "input" || !strings.Contains(string(r.Error.Problems), `"path":"/profile/birth/value"`) {
		t.Errorf("invalid birth: kind %q problems %s", r.Error.Kind, r.Error.Problems)
	}
}

func jsonEqual(t *testing.T, a, b jsontext.Value) bool {
	t.Helper()
	a, b = bytes.Clone(a), bytes.Clone(b)
	if err := a.Compact(); err != nil {
		return false
	}
	if err := b.Compact(); err != nil {
		return false
	}
	return bytes.Equal(a, b)
}
