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

func TestRecalledDatePeriod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		r      RecalledDate
		lo, hi string // "" when not dated
	}{
		{RecalledDate{Value: "2026-03-15", Precision: Day}, "2026-03-15", "2026-03-15"},
		{RecalledDate{Value: "2026-02", Precision: Month}, "2026-02-01", "2026-02-28"},
		{RecalledDate{Value: "2024-02", Precision: Month}, "2024-02-01", "2024-02-29"},
		{RecalledDate{Value: "2026-12", Precision: Month}, "2026-12-01", "2026-12-31"},
		{RecalledDate{Value: "2026", Precision: Year}, "2026-01-01", "2026-12-31"},
		{RecalledDate{Value: "0001", Precision: Year}, "0001-01-01", "0001-12-31"},
		{RecalledDate{Precision: Unknown}, "", ""},
		{RecalledDate{Precision: None}, "", ""},
	}
	for _, c := range cases {
		p, dated, err := c.r.Period()
		switch {
		case err != nil:
			t.Errorf("%+v: Period error %v", c.r, err)
		case dated != (c.lo != ""):
			t.Errorf("%+v: dated = %v", c.r, dated)
		case dated && (p.Lo != d(t, c.lo) || p.Hi != d(t, c.hi)):
			t.Errorf("%+v: Period = %v, want [%s, %s]", c.r, p, c.lo, c.hi)
		}
	}
	for _, bad := range []RecalledDate{
		{Value: "2026-02-30", Precision: Day},
		{Value: "2026-03", Precision: Day},
		{Value: "2026-13", Precision: Month},
		{Value: "2026-00", Precision: Month},
		{Value: "2026-3", Precision: Month},
		{Value: "2026-03-01", Precision: Month},
		{Value: "0000-01", Precision: Month},
		{Value: "2026", Precision: Month},
		{Value: "26", Precision: Year},
		{Value: "0000", Precision: Year},
		{Value: "20260", Precision: Year},
		{Value: "2026-01", Precision: Year},
		{Value: "+202", Precision: Year},
		{Value: "", Precision: Year},
		{Value: "2026", Precision: "decade"},
	} {
		if _, _, err := bad.Period(); err == nil {
			t.Errorf("%+v: Period succeeded, want an error", bad)
		}
	}
}

func TestRangeCapAndIntersects(t *testing.T) {
	t.Parallel()
	r := Range{Lo: d(t, "2026-04-01"), Hi: d(t, "2026-04-30")}
	if got := r.CapAt(d(t, "2026-04-15")); got.Lo != r.Lo || got.Hi != d(t, "2026-04-15") {
		t.Errorf("CapAt inside = %v", got)
	}
	if got := r.CapAt(d(t, "2026-05-15")); got != r {
		t.Errorf("CapAt after = %v", got)
	}
	cases := []struct {
		a, b Range
		want bool
	}{
		{r, Exact(d(t, "2026-04-30")), true},
		{r, Exact(d(t, "2026-05-01")), false},
		{r, Range{Lo: d(t, "2026-03-01"), Hi: d(t, "2026-04-01")}, true},
		{r, Range{Lo: d(t, "2026-03-01"), Hi: d(t, "2026-03-31")}, false},
		{Exact(d(t, "2026-04-02")), Exact(d(t, "2026-04-02")), true},
	}
	for _, c := range cases {
		if c.a.Intersects(c.b) != c.want || c.b.Intersects(c.a) != c.want {
			t.Errorf("%v intersects %v: want %v", c.a, c.b, c.want)
		}
	}
}
