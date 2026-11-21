package identity_test

import (
	"testing"

	"github.com/fugo101/vaxbook/apps/api/internal/identity"
)

func TestNormalizeEmail(t *testing.T) {
	valid := map[string]string{
		"a@example.com":             "a@example.com",
		"  Mixed.Case@Example.COM ": "mixed.case@example.com",
		"user+tag@sub.example.vn":   "user+tag@sub.example.vn",
	}
	for in, want := range valid {
		got, err := identity.NormalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	invalid := []string{
		"", "   ", "not-an-email", "a@", "@example.com",
		"Name <a@example.com>", "<a@example.com>", "a@example.com, b@example.com",
		"a b@example.com",
	}
	for _, in := range invalid {
		if got, err := identity.NormalizeEmail(in); err == nil {
			t.Errorf("NormalizeEmail(%q) = %q, want error", in, got)
		}
	}
}
