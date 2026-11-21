package engine

import (
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// d is a test helper that builds a Date or fails the test.
func d(t testing.TB, s string) Date {
	t.Helper()
	v, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return v
}

func TestParseDateRejects(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		"", "2026-02-30", "2026-13-01", "2026-00-10", "2026-01-00", "2025-02-29",
		"2026-2-03", "2026-02-3", "+202-01-01", "-202-01-01", "2026/02/03",
		"2026-02-03T00:00:00Z", " 2026-02-03", "2026-02-03 ", "20260203", "2026-02-0a",
		"0000-01-01", "２０２６-01-01",
	} {
		if got, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) = %v, want error", in, got)
		}
	}
}

func TestParseDateAccepts(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"2024-02-29", "2000-02-29", "1900-01-01", "2199-12-31", "0001-01-01", "9999-12-31"} {
		if got := d(t, in).String(); got != in {
			t.Errorf("round trip %q = %q", in, got)
		}
	}
}

func TestDateZeroValue(t *testing.T) {
	t.Parallel()
	var z Date
	if !z.IsZero() {
		t.Fatal("zero Date is not IsZero")
	}
	if _, err := z.MarshalText(); err == nil {
		t.Fatal("zero Date marshalled without error")
	}
	if d(t, "0001-01-01").IsZero() {
		t.Fatal("0001-01-01 is IsZero")
	}
}

func TestAddMonthsClamps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-01-31", 1, "2026-02-28"},
		{"2024-01-31", 1, "2024-02-29"},
		{"2026-03-31", -1, "2026-02-28"},
		{"2026-08-31", 6, "2027-02-28"},
		{"2026-05-31", 9, "2027-02-28"},
		{"2016-02-29", 108, "2025-02-28"},
		{"2024-02-29", 12, "2025-02-28"},
		{"2026-01-10", 0, "2026-01-10"},
		{"2026-12-15", 1, "2027-01-15"},
		{"2026-01-15", -13, "2024-12-15"},
	}
	for _, c := range cases {
		if got := d(t, c.from).AddMonths(c.n).String(); got != c.want {
			t.Errorf("%s + %dm = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestAddSpan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from string
		span string
		want string
	}{
		{"2026-01-10", "P6W", "2026-02-21"},
		{"2026-01-10", "P42D", "2026-02-21"},
		{"2026-01-10", "P2M", "2026-03-10"},
		{"2024-02-29", "P1Y", "2025-02-28"},
		{"2015-03-31", "P9Y", "2024-03-31"},
		{"2026-03-10", "P0D", "2026-03-10"},
	}
	for _, c := range cases {
		s, err := ruleset.ParseSpan(c.span)
		if err != nil {
			t.Fatal(err)
		}
		if got := d(t, c.from).AddSpan(s).String(); got != c.want {
			t.Errorf("%s + %s = %s, want %s", c.from, c.span, got, c.want)
		}
	}
}

func TestDateCompare(t *testing.T) {
	t.Parallel()
	a, b := d(t, "2026-01-10"), d(t, "2026-01-11")
	if !a.Before(b) || a.After(b) || a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
		t.Fatal("ordering is wrong")
	}
	if maxDate(a, b) != b || minDate(a, b) != a {
		t.Fatal("maxDate/minDate wrong")
	}
}

func TestRecalledDateRange(t *testing.T) {
	t.Parallel()
	r, err := RecalledDate{Value: "2026-03-15", Precision: Day}.Range()
	if err != nil || r != Exact(d(t, "2026-03-15")) || r.Lo != r.Hi {
		t.Fatalf("Range = %v, %v", r, err)
	}
	if _, err := (RecalledDate{Value: "2026-02-30", Precision: Day}).Range(); err == nil {
		t.Error("an impossible day gave a range")
	}
	for _, p := range []Precision{Month, Year, Unknown, None} {
		if _, err := (RecalledDate{Value: "2026", Precision: p}).Range(); err == nil {
			t.Errorf("precision %s is not supported in S1 but gave a range", p)
		}
	}
}
