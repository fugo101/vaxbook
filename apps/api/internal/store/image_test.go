package store_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/fugo101/vaxbook/apps/api/internal/store"
)

// TestPostgresImageMatchesCompose: Dependabot bumps compose.dev.yaml but
// cannot edit a Go constant, so its Postgres PR stays red here until
// store.PostgresImage is bumped in the same PR.
func TestPostgresImageMatchesCompose(t *testing.T) {
	b, err := os.ReadFile("../../../../compose.dev.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s+image:\s*"?(postgres:[^\s"@]+)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("compose.dev.yaml: no postgres image line")
	}
	if got := string(m[1]); got != store.PostgresImage {
		t.Errorf("compose.dev.yaml uses %s but store.PostgresImage is %s; set both to the same tag", got, store.PostgresImage)
	}
}
