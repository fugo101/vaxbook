// Package repocheck holds checks on the repository itself that no single
// package owns.
package repocheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// root is the repository root; go test runs a package in its own directory.
const root = "../.."

// pins returns the first submatch of every match of pattern in file (relative
// to the repository root), and fails the test when there is none.
func pins(t *testing.T, file, pattern string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllSubmatch(b, -1) {
		out = append(out, string(m[1]))
	}
	if len(out) == 0 {
		t.Fatalf("%s: nothing matches %q", file, pattern)
	}
	return out
}

// TestGoVersionAgrees: Dependabot bumps the golang image in the Dockerfile but
// never the go directive, and the Dockerfile builds with GOTOOLCHAIN=local, so
// a Dependabot golang PR stays red here until go.mod and tools/go.mod are
// bumped in the same PR. CI (setup-go) reads go.mod, so this also keeps the
// shipped binary on the Go version CI tested.
func TestGoVersionAgrees(t *testing.T) {
	want := pins(t, "go.mod", `(?m)^go (\S+)$`)[0]
	for _, c := range []struct{ file, pattern string }{
		{"tools/go.mod", `(?m)^go (\S+)$`},
		{"Dockerfile", `(?m)^FROM golang:([^\s@]+)`},
	} {
		for _, got := range pins(t, c.file, c.pattern) {
			if got != want {
				t.Errorf("%s names Go %s but go.mod says %s; set both to the same version", c.file, got, want)
			}
		}
	}
}

// TestNodeVersionAgrees: no bot tracks the Node major; CI and the Taskfile
// precondition message must name the same one.
func TestNodeVersionAgrees(t *testing.T) {
	want := pins(t, ".github/workflows/ci.yml", `node-version: '(\d+)'`)[0]
	if got := pins(t, "Taskfile.yml", `needs Node (\d+)`)[0]; got != want {
		t.Errorf("Taskfile.yml names Node %s but ci.yml uses %s", got, want)
	}
}

// TestGolangciLintVersionIsSingleSource: CI (version-file) and the Taskfile
// (sh: cat) both read .golangci-lint-version; a pinned version in either would
// bring the two sources back.
func TestGolangciLintVersionIsSingleSource(t *testing.T) {
	version := strings.TrimSpace(pins(t, ".golangci-lint-version", `(?m)^(v\d+\.\d+\.\d+)$`)[0])
	if got := len(pins(t, ".github/workflows/ci.yml", `version-file: (\.golangci-lint-version)`)); got != 2 {
		t.Errorf("ci.yml reads .golangci-lint-version in %d golangci-lint steps, want 2", got)
	}
	pins(t, "Taskfile.yml", `sh: cat (\.golangci-lint-version)`)
	for _, file := range []string{".github/workflows/ci.yml", "Taskfile.yml"} {
		b, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), version) {
			t.Errorf("%s names %s; read .golangci-lint-version instead", file, version)
		}
	}
}
