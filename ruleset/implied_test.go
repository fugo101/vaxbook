package ruleset

import "testing"

func span(t *testing.T, s string) *Span {
	t.Helper()
	sp, err := ParseSpan(s)
	if err != nil {
		t.Fatal(err)
	}
	return &sp
}

// TestMinAgeImplied: SAD §5.2, minAge_k ≤ minAge_1 + Σ minInterval_j for
// j = 2…k, a missing interval counting 0. Within one unit family (months
// and years, or days and weeks) the comparison is exact; across families it
// is conservative (a month of minAge_k counts 31 days, a month of the sum
// 28 days), so a mixed case is implied only when it holds for every month
// length.
func TestMinAgeImplied(t *testing.T) {
	t.Parallel()
	type rule struct{ minAge, minInterval string }
	plan := func(rules ...rule) Plan {
		var p Plan
		for _, r := range rules {
			d := DoseRule{Recommended: Milestone{From: FromBirth, Offset: Span{N: 1, Unit: Months}}}
			if r.minAge != "" {
				d.MinAge = span(t, r.minAge)
			}
			if r.minInterval != "" {
				d.MinInterval = span(t, r.minInterval)
			}
			p.Doses = append(p.Doses, d)
		}
		return p
	}
	cases := []struct {
		name string
		plan Plan
		k    int
		want bool
	}{
		{"dose 1", plan(rule{"P6W", ""}), 1, true},
		{"no minimum age", plan(rule{"P6W", ""}, rule{"", "P4W"}), 2, true},
		{"months, equal", plan(rule{"P9M", ""}, rule{"P10M", "P1M"}), 2, true},
		{"months, one more", plan(rule{"P9M", ""}, rule{"P11M", "P1M"}), 2, false},
		{"years count 12 months", plan(rule{"P1Y", ""}, rule{"P13M", "P1M"}), 2, true},
		{"weeks count 7 days", plan(rule{"P6W", ""}, rule{"P70D", "P4W"}), 2, true},
		{"days, one more", plan(rule{"P6W", ""}, rule{"P71D", "P4W"}), 2, false},
		{"missing interval counts 0", plan(rule{"P6W", ""}, rule{"P6W", ""}), 2, true},
		{"missing interval, larger age", plan(rule{"P6W", ""}, rule{"P4M", ""}), 2, false},
		{"sum over doses 2..k", plan(rule{"P6W", ""}, rule{"", "P4W"}, rule{"P14W", "P4W"}), 3, true},
		{"sum over doses 2..k, one more", plan(rule{"P6W", ""}, rule{"", "P4W"}, rule{"P99D", "P4W"}), 3, false},
		// Mixed families: 2m + 4w is at least 56 + 28 = 84 days, and 12w = 84
		// days, so 12w is implied; 3m is up to 93 days, so it is not.
		{"mixed, implied for every month length", plan(rule{"P2M", ""}, rule{"P12W", "P4W"}), 2, true},
		{"mixed, not implied for some month length", plan(rule{"P2M", ""}, rule{"P3M", "P4W"}), 2, false},
		{"mixed, minimum age in months, implied", plan(rule{"P6W", ""}, rule{"P2M", "P4W"}), 2, true},
		{"mixed, minimum age in months, not implied", plan(rule{"P6W", ""}, rule{"P3M", "P4W"}), 2, false},
	}
	for _, c := range cases {
		if got := MinAgeImplied(c.plan, c.k); got != c.want {
			t.Errorf("%s: MinAgeImplied(k=%d) = %v, want %v", c.name, c.k, got, c.want)
		}
	}
}
