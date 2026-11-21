package engine_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestImportsStdlibOnly: every Go file under engine/ and ruleset/, test files
// and files behind any build constraint included, imports only the standard
// library and the packages the depguard rules in .golangci.yml allow (SAD §3,
// ADR-0002). golangci-lint loads only the files of the platform it runs for,
// so this test guards files such as the js/wasm entry point whatever the
// platform.
func TestImportsStdlibOnly(t *testing.T) {
	t.Parallel()
	const module = "github.com/fugo101/vaxbook/"
	roots := map[string][]string{
		".":          {module + "engine", module + "ruleset"},
		"../ruleset": {module + "ruleset"},
	}
	for root, allowed := range roots {
		files := 0
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir() && d.Name() == "testdata":
				return filepath.SkipDir
			case d.IsDir() || !strings.HasSuffix(path, ".go"):
				return nil
			}
			files++
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if !isStdlib(p) && !slices.ContainsFunc(allowed, func(a string) bool {
					return p == a || strings.HasPrefix(p, a+"/")
				}) {
					t.Errorf("%s imports %s: only the standard library and %v are allowed", path, p, allowed)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
		if files == 0 {
			t.Fatalf("no Go files under %s", root)
		}
	}
}

// isStdlib reports whether an import path belongs to the standard library,
// whose first path element has no dot (the rule the go command uses).
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
