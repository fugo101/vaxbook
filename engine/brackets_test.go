package engine

import (
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// splitModel compiles disease d of a snapshot with one series per entry of
// maxes, each a group of its own with startAge [P0D, max) (product "p<i>"),
// plus, when retiredFirst, the first group's product retired and a generic
// series with no startAge.
func splitModel(t *testing.T, retiredFirst bool, maxes ...string) diseaseModel {
	t.Helper()
	s := &ruleset.Snapshot{Version: "split-1", Diseases: []ruleset.Disease{{ID: "d", Name: "D"}}}
	dose := []ruleset.DoseRule{{MinAge: &ruleset.Span{Unit: ruleset.Days}, Recommended: ruleset.Milestone{From: ruleset.FromBirth, Offset: ruleset.Span{Unit: ruleset.Days}}}}
	for i, m := range maxes {
		maxSpan, err := ruleset.ParseSpan(m)
		if err != nil {
			t.Fatal(err)
		}
		product := string(rune('a' + i))
		s.Products = append(s.Products, ruleset.Product{ID: product, Name: product, Diseases: []string{"d"}, Retired: retiredFirst && i == 0})
		s.Series = append(s.Series, ruleset.Series{
			ID: "s-" + product, Disease: "d", Product: &product, Stage: ruleset.Childhood,
			StartAge: &ruleset.StartAge{Min: ruleset.Span{Unit: ruleset.Days}, Max: &maxSpan},
			Plans:    []ruleset.Plan{{ID: "p", Doses: dose}},
		})
	}
	if retiredFirst {
		s.Series = append(s.Series, ruleset.Series{ID: "s-generic", Disease: "d", Stage: ruleset.Childhood, Plans: []ruleset.Plan{{ID: "p", Doses: dose}}})
	}
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return onlySegment(t, e, 0)
}

// TestNoDoseSplitPoints checks the birth dates noDosePoints evaluates (SAD
// §5.3 step 4; PR #27): a split c is the first birth date with c + max after
// today, found with the clamping of AddSpan, and counts only when the
// no-dose choice changes there.
func TestNoDoseSplitPoints(t *testing.T) {
	t.Parallel()
	ds := func(vs ...string) []Date {
		var out []Date
		for _, v := range vs {
			out = append(out, d(t, v))
		}
		return out
	}
	cases := []struct {
		name         string
		retiredFirst bool
		maxes        []string
		lo, hi       string
		today        string
		want         []Date
	}{
		// 2024-02-27 + 2y = 2026-02-27 is not after today; 2024-02-28 +
		// 2y = 2026-02-28 is, and so is 2024-02-29 + 2y (clamped to
		// 2026-02-28).
		{"years, clamped open", false, []string{"P2Y"}, "2024-02-01", "2024-02-29", "2026-02-27", ds("2024-02-01", "2024-02-27", "2024-02-28", "2024-02-29")},
		// 2024-02-29 + 2y clamps to 2026-02-28, not after today: closed at
		// the later end, so no split.
		{"years, clamped closed", false, []string{"P2Y"}, "2024-02-01", "2024-02-29", "2026-02-28", ds("2024-02-01", "2024-02-29")},
		// c = today − 400 days + 1 day = 2025-05-27.
		{"days", false, []string{"P400D"}, "2025-01-01", "2025-12-31", "2026-06-30", ds("2025-01-01", "2025-05-26", "2025-05-27", "2025-12-31")},
		// c = today − 392 days + 1 day = 2025-06-04.
		{"weeks", false, []string{"P56W"}, "2025-01-01", "2025-12-31", "2026-06-30", ds("2025-01-01", "2025-06-03", "2025-06-04", "2025-12-31")},
		{"open at the older end", false, []string{"P5Y"}, "2025-01-01", "2025-12-31", "2026-06-30", ds("2025-01-01", "2025-12-31")},
		// 2025-12-31 + 6m = 2026-06-30 is not after today.
		{"closed at the later end", false, []string{"P6M"}, "2025-01-01", "2025-12-31", "2026-06-30", ds("2025-01-01", "2025-12-31")},
		// Two groups closing on different days: 2025-03-31 + 15m and
		// 2025-06-30 + 1y are both 2026-06-30, not after today.
		{"two splits", false, []string{"P15M", "P1Y"}, "2025-01-01", "2025-12-31", "2026-06-30", ds("2025-01-01", "2025-03-31", "2025-04-01", "2025-06-30", "2025-07-01", "2025-12-31")},
		// The retired group closes inside the range, but the generic series
		// is chosen on every day, so the choice does not change.
		{"retired group unused", true, []string{"P2Y"}, "2024-02-01", "2024-02-29", "2026-02-27", ds("2024-02-01", "2024-02-29")},
		{"one day", false, []string{"P2Y"}, "2024-02-28", "2024-02-28", "2026-02-27", ds("2024-02-28")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dz := splitModel(t, c.retiredFirst, c.maxes...)
			got := dz.noDosePoints(Range{Lo: d(t, c.lo), Hi: d(t, c.hi)}, d(t, c.today))
			if !slices.Equal(got, c.want) {
				t.Errorf("points %v, want %v", got, c.want)
			}
		})
	}
}
