package ruleset

import (
	"reflect"
	"strconv"
	"testing"
)

// bracketSeries builds a series of disease d with one dose per plan: plan j
// has dose-1 minimum age minAges[j] ("" for none). product "" is a generic
// series, and min "" leaves out startAge (max "" is open-ended).
func bracketSeries(t *testing.T, id, product, min, max string, minAges ...string) Series {
	t.Helper()
	span := func(s string) *Span {
		if s == "" {
			return nil
		}
		v, err := ParseSpan(s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	sr := Series{ID: id, Disease: "d", Stage: Childhood}
	if product != "" {
		sr.Product = &product
	}
	if min != "" {
		sr.StartAge = &StartAge{Min: *span(min), Max: span(max)}
	}
	for j, a := range minAges {
		sr.Plans = append(sr.Plans, Plan{ID: "p" + strconv.Itoa(j), Doses: []DoseRule{{
			MinAge:      span(a),
			Recommended: Milestone{From: FromBirth, Offset: Span{N: 1, Unit: Years}},
		}}})
	}
	return sr
}

// TestValidateStartAge covers the S3b checks of start-age brackets (SAD
// §5.2, EQ26, issue #23): START_AGE_EMPTY, START_AGE_REQUIRED,
// START_AGE_OVERLAP and the two dose-1 Warns, with the conservative span
// comparison (a month counts 31 days on the side that must be smaller and
// 28 days on the other, unless both sides are in one unit family).
func TestValidateStartAge(t *testing.T) {
	t.Parallel()
	problem := func(sev Severity, code ProblemCode, path string, params ProblemParams) Problem {
		return Problem{Code: code, Severity: sev, Path: path, Params: params}
	}
	reject := func(code ProblemCode, path string, params ProblemParams) Problem {
		return problem(Reject, code, path, params)
	}
	warn := func(code ProblemCode, path string, params ProblemParams) Problem {
		return problem(Warn, code, path, params)
	}
	type s = []Series
	cases := []struct {
		name   string
		series func(t *testing.T) s
		want   []Problem
	}{
		{"valid product group", func(t *testing.T) s {
			return s{bracketSeries(t, "young", "p", "P9Y", "P15Y", "P9Y"), bracketSeries(t, "old", "p", "P15Y", "", "P9Y")}
		}, nil},
		{"one series without a bracket (C8)", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "", "", "P0D")} }, nil},
		{"product and generic series are two groups", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "p", "", "", "P0D"), bracketSeries(t, "b", "", "", "", "P0D")}
		}, nil},

		{"empty: equal bounds", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P1Y", "P1Y", "P0D")} },
			[]Problem{reject(StartAgeEmpty, "/series/0/startAge", ProblemParams{})}},
		{"empty: reversed bounds", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P2Y", "P1Y", "P0D")} },
			[]Problem{reject(StartAgeEmpty, "/series/0/startAge", ProblemParams{})}},
		{"empty: 4 weeks to 1 month (a 28-day month)", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P4W", "P1M", "P0D")} },
			[]Problem{reject(StartAgeEmpty, "/series/0/startAge", ProblemParams{})}},
		{"not empty: 27 days to 1 month", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P27D", "P1M", "P0D")} }, nil},
		// An empty bracket with valid spans still takes part in the overlap
		// check.
		{"empty and overlapping", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P2Y", "P0D"), bracketSeries(t, "b", "", "P1Y", "P12M", "P0D")}
		}, []Problem{
			reject(StartAgeEmpty, "/series/1/startAge", ProblemParams{}),
			reject(StartAgeOverlap, "/series/1/startAge", ProblemParams{ID: "a"}),
		}},
		{"empty, touching the earlier bracket", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P12M", "P13M", "P0D"), bracketSeries(t, "b", "", "P1Y", "P12M", "P0D")}
		}, []Problem{reject(StartAgeEmpty, "/series/1/startAge", ProblemParams{})}},

		{"required: generic group", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P1Y", "P0D"), bracketSeries(t, "b", "", "", "", "P0D")}
		}, []Problem{reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d"})}},
		{"required: product group, every series", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "p", "", "", "P0D"), bracketSeries(t, "b", "", "", "", "P0D"), bracketSeries(t, "c", "p", "", "", "P0D")}
		}, []Problem{
			reject(StartAgeRequired, "/series/0", ProblemParams{Disease: "d", Product: "p"}),
			reject(StartAgeRequired, "/series/2", ProblemParams{Disease: "d", Product: "p"}),
		}},
		{"required: before the series' own problems", func(t *testing.T) s {
			b := bracketSeries(t, "", "", "", "", "P0D")
			b.Stage = ""
			return s{bracketSeries(t, "a", "", "P0D", "P1Y", "P0D"), b}
		}, []Problem{
			reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d"}),
			reject(Required, "/series/1/id", ProblemParams{}),
			reject(Required, "/series/1/stage", ProblemParams{}),
		}},

		{"overlap", func(t *testing.T) s {
			return s{bracketSeries(t, "young", "", "P9Y", "P15Y", "P9Y"), bracketSeries(t, "old", "", "P14Y", "", "P9Y")}
		}, []Problem{reject(StartAgeOverlap, "/series/1/startAge", ProblemParams{ID: "young"})}},
		{"overlap: 30 days against 1 month", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P30D", "P0D"), bracketSeries(t, "b", "", "P1M", "P2M", "P0D")}
		}, []Problem{reject(StartAgeOverlap, "/series/1/startAge", ProblemParams{ID: "a"})}},
		{"disjoint: 28 days against 1 month", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P28D", "P0D"), bracketSeries(t, "b", "", "P1M", "P2M", "P0D")}
		}, nil},
		{"disjoint: later series first", func(t *testing.T) s {
			return s{bracketSeries(t, "old", "", "P5Y", "", "P0D"), bracketSeries(t, "young", "", "P0D", "P5Y", "P0D")}
		}, nil},
		{"overlap: two open-ended brackets", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "", "P0D"), bracketSeries(t, "b", "", "P2Y", "", "P0D")}
		}, []Problem{reject(StartAgeOverlap, "/series/1/startAge", ProblemParams{ID: "a"})}},
		{"overlap: two earlier series", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P1Y", "P0D"), bracketSeries(t, "b", "", "P1Y", "P2Y", "P0D"),
				bracketSeries(t, "c", "", "P6M", "P18M", "P0D")}
		}, []Problem{
			reject(StartAgeOverlap, "/series/2/startAge", ProblemParams{ID: "a"}),
			reject(StartAgeOverlap, "/series/2/startAge", ProblemParams{ID: "b"}),
		}},
		{"overlap: other groups do not count", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P0D", "P1Y", "P0D"), bracketSeries(t, "b", "p", "P0D", "P1Y", "P0D"),
				bracketSeries(t, "c", "q", "P0D", "P1Y", "P0D")}
		}, nil},

		{"above: 10 years against 9", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P9Y", "", "P10Y")} },
			[]Problem{warn(FirstDoseMinAgeAboveStartAge, "/series/0/plans/0/doses/0/minAge", ProblemParams{})}},
		{"not above: 4 weeks against 1 month", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P1M", "", "P4W")} }, nil},
		{"above: 6 weeks against 1 month", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P1M", "", "P6W")} },
			[]Problem{warn(FirstDoseMinAgeAboveStartAge, "/series/0/plans/0/doses/0/minAge", ProblemParams{})}},
		{"above: in a second plan only", func(t *testing.T) s { return s{bracketSeries(t, "a", "", "P9Y", "", "P9Y", "P108M", "P109M")} },
			[]Problem{warn(FirstDoseMinAgeAboveStartAge, "/series/0/plans/2/doses/0/minAge", ProblemParams{})}},

		{"same: 1 year against 12 months", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P1Y"), bracketSeries(t, "b", "", "P2Y", "", "P12M")}
		}, nil},
		{"differs: 52 weeks against 12 months", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P12M"), bracketSeries(t, "b", "", "P2Y", "", "P52W")}
		}, []Problem{warn(FirstDoseMinAgeDiffers, "/series/1/plans/0/doses/0/minAge", ProblemParams{ID: "a"})}},
		{"same: zero in either family", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P0D"), bracketSeries(t, "b", "", "P2Y", "", "P0M")}
		}, nil},
		{"differs: a second plan of the first series", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "p", "P1Y", "P2Y", "P1Y", "P6M"), bracketSeries(t, "b", "p", "P2Y", "", "P12M")}
		}, []Problem{warn(FirstDoseMinAgeDiffers, "/series/0/plans/1/doses/0/minAge", ProblemParams{ID: "a"})}},
		{"differs: only within a group", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P1Y"), bracketSeries(t, "b", "p", "P1Y", "", "P6M")}
		}, nil},
		{"differs: above too, each once", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P1Y"), bracketSeries(t, "b", "", "P2Y", "", "P3Y")}
		}, []Problem{
			warn(FirstDoseMinAgeAboveStartAge, "/series/1/plans/0/doses/0/minAge", ProblemParams{}),
			warn(FirstDoseMinAgeDiffers, "/series/1/plans/0/doses/0/minAge", ProblemParams{ID: "a"}),
		}},
		{"differs: series without a bracket still compared", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", "P1Y"), bracketSeries(t, "b", "", "", "", "P6M")}
		}, []Problem{
			reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d"}),
			warn(FirstDoseMinAgeDiffers, "/series/1/plans/0/doses/0/minAge", ProblemParams{ID: "a"}),
		}},
		{"differs: no reference without a dose-1 minimum age", func(t *testing.T) s {
			return s{bracketSeries(t, "a", "", "P1Y", "P2Y", ""), bracketSeries(t, "b", "", "P2Y", "", "P6M")}
		}, []Problem{reject(DoseRuleNoMinimum, "/series/0/plans/0/doses/0", ProblemParams{})}},
		{"warns after the earlier series' problems", func(t *testing.T) s {
			a := bracketSeries(t, "a", "", "P1Y", "P2Y", "P1Y")
			a.Stage = "teen"
			return s{a, bracketSeries(t, "b", "", "P2Y", "", "P6M")}
		}, []Problem{
			reject(InvalidValue, "/series/0/stage", ProblemParams{Value: "teen"}),
			warn(FirstDoseMinAgeDiffers, "/series/1/plans/0/doses/0/minAge", ProblemParams{ID: "a"}),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snap := &Snapshot{
				Version:  "v1",
				Diseases: []Disease{{ID: "d", Name: "D"}},
				Products: []Product{{ID: "p", Name: "P", Diseases: []string{"d"}}, {ID: "q", Name: "Q", Diseases: []string{"d"}}},
				Series:   tc.series(t),
			}
			if got := Validate(snap); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

// TestSpanComparison pins the conservative comparison of two spans that
// Validate uses for brackets (SAD §5.2): exact within one unit family, and
// across families a month counts 31 days on the smaller side and 28 on the
// larger.
func TestSpanComparison(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b         string
		atMost, less bool
	}{
		{"P1Y", "P12M", true, false},
		{"P12M", "P1Y", true, false},
		{"P11M", "P1Y", true, true},
		{"P1W", "P7D", true, false},
		{"P6D", "P1W", true, true},
		{"P4W", "P1M", true, false},
		{"P27D", "P1M", true, true},
		{"P29D", "P1M", false, false},
		{"P1M", "P31D", true, false},
		{"P1M", "P32D", true, true},
		{"P1M", "P30D", false, false},
		{"P0D", "P0M", true, false},
		{"P0M", "P1D", true, true},
		{"P0W", "P0Y", true, false},
		{"P47W", "P1Y", true, true},
		{"P48W", "P1Y", true, false},
		{"P52W", "P1Y", false, false},
	}
	for _, c := range cases {
		a, err := ParseSpan(c.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseSpan(c.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := SpanAtMost(a, b); got != c.atMost {
			t.Errorf("SpanAtMost(%s, %s) = %v, want %v", c.a, c.b, got, c.atMost)
		}
		if got := spanLess(a, b); got != c.less {
			t.Errorf("spanLess(%s, %s) = %v, want %v", c.a, c.b, got, c.less)
		}
	}
}
