package ruleset

import (
	"reflect"
	"testing"
)

// windowed returns sr with the validity window [from, to) ("" for an absent
// bound).
func windowed(t *testing.T, sr Series, from, to string) Series {
	t.Helper()
	date := func(s string) *Date {
		if s == "" {
			return nil
		}
		d, err := ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	sr.ValidFrom, sr.ValidTo = date(from), date(to)
	return sr
}

// TestValidateStartAgeByWindow covers the S4c scoping of the EQ26 checks by
// validity window (issue #29 In scope 1): START_AGE_REQUIRED,
// START_AGE_OVERLAP and FIRST_DOSE_MIN_AGE_DIFFERS only between series of
// one product group whose windows overlap, with an absent bound unbounded.
func TestValidateStartAgeByWindow(t *testing.T) {
	t.Parallel()
	reject := func(code ProblemCode, path string, params ProblemParams) Problem {
		return Problem{Code: code, Severity: Reject, Path: path, Params: params}
	}
	warn := func(code ProblemCode, path string, params ProblemParams) Problem {
		return Problem{Code: code, Severity: Warn, Path: path, Params: params}
	}
	type s = []Series
	cases := []struct {
		name   string
		series func(t *testing.T) s
		want   []Problem
	}{
		{"bracketless pair, overlapping windows", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "a", "", "", "", "P6W"), "", "2026-01-02"),
				windowed(t, bracketSeries(t, "b", "", "", "", "P6W"), "2026-01-01", ""),
			}
		}, []Problem{
			reject(StartAgeRequired, "/series/0", ProblemParams{Disease: "d"}),
			reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d"}),
		}},
		{"bracketless pair, disjoint windows (validTo exclusive)", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "a", "", "", "", "P6W"), "", "2026-01-01"),
				windowed(t, bracketSeries(t, "b", "", "", "", "P6W"), "2026-01-01", ""),
			}
		}, nil},
		{"bracketless series overlapping a windowless one", func(t *testing.T) s {
			return s{
				bracketSeries(t, "a", "p", "", "", "P6W"),
				windowed(t, bracketSeries(t, "b", "p", "", "", "P6W"), "2030-01-01", ""),
			}
		}, []Problem{
			reject(StartAgeRequired, "/series/0", ProblemParams{Disease: "d", Product: "p"}),
			reject(StartAgeRequired, "/series/1", ProblemParams{Disease: "d", Product: "p"}),
		}},
		{"overlapping brackets, disjoint windows", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "a", "", "P6W", "P2Y", "P6W"), "", "2025-01-01"),
				windowed(t, bracketSeries(t, "b", "", "P6W", "P3Y", "P6W"), "2025-01-01", ""),
			}
		}, nil},
		{"overlapping brackets, overlapping windows", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "a", "", "P6W", "P2Y", "P6W"), "", "2025-01-02"),
				windowed(t, bracketSeries(t, "b", "", "P6W", "P3Y", "P6W"), "2025-01-01", ""),
			}
		}, []Problem{reject(StartAgeOverlap, "/series/1/startAge", ProblemParams{ID: "a"})}},
		// A third series in effect alone is a group of one there.
		{"a lone successor needs no bracket", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "young", "", "P6W", "P1Y", "P6W"), "", "2026-01-01"),
				windowed(t, bracketSeries(t, "old", "", "P1Y", "", "P6W"), "", "2026-01-01"),
				windowed(t, bracketSeries(t, "new", "", "", "", "P8W"), "2026-01-01", ""),
			}
		}, nil},
		// The Z/X/Y case of the contract table: comparing with the first
		// overlapping series only would leave {X, Y} from 2022 unwarned.
		{"min age differs, pairwise", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "z", "", "P8W", "P1Y", "P8W"), "2020-01-01", "2022-01-01"),
				windowed(t, bracketSeries(t, "x", "", "P6W", "P1Y", "P6W"), "2022-01-01", ""),
				windowed(t, bracketSeries(t, "y", "", "P1Y", "", "P8W"), "2021-01-01", ""),
			}
		}, []Problem{warn(FirstDoseMinAgeDiffers, "/series/2/plans/0/doses/0/minAge", ProblemParams{ID: "x"})}},
		// Without windows: series 2 names series 1 when 0 and 2 agree.
		{"min age differs, windowless, middle series differs", func(t *testing.T) s {
			return s{
				bracketSeries(t, "a", "", "P6W", "P1Y", "P6W"),
				bracketSeries(t, "b", "", "P1Y", "P2Y", "P1Y"),
				bracketSeries(t, "c", "", "P2Y", "", "P6W"),
			}
		}, []Problem{
			warn(FirstDoseMinAgeDiffers, "/series/1/plans/0/doses/0/minAge", ProblemParams{ID: "a"}),
			warn(FirstDoseMinAgeDiffers, "/series/2/plans/0/doses/0/minAge", ProblemParams{ID: "b"}),
		}},
		// A series' own plans are compared with its plan 0 last.
		{"min age differs within a series", func(t *testing.T) s {
			return s{
				bracketSeries(t, "a", "", "P8W", "P1Y", "P6W", "P8W"),
				bracketSeries(t, "b", "", "P1Y", "", "P6W"),
			}
		}, []Problem{warn(FirstDoseMinAgeDiffers, "/series/0/plans/1/doses/0/minAge", ProblemParams{ID: "a"})}},
		// No other series in effect with it: no comparison at all.
		{"plans differ in a series alone in its window", func(t *testing.T) s {
			return s{
				windowed(t, bracketSeries(t, "a", "", "", "", "P6W", "P8W"), "", "2026-01-01"),
				windowed(t, bracketSeries(t, "b", "", "", "", "P6W"), "2026-01-01", ""),
			}
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			snap := &Snapshot{
				Version:  "v1",
				Diseases: []Disease{{ID: "d", Name: "D"}},
				Products: []Product{{ID: "p", Name: "P", Diseases: []string{"d"}}},
				Series:   tc.series(t),
			}
			if got := Validate(snap); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

// TestValidateToleranceAndCatchUp covers TOLERANCE_UNIT,
// TOLERANCE_NOT_BELOW_MINIMUM and CATCH_UP_AFTER_MAX_AGE (issue #29), with
// the conservative span comparison.
func TestValidateToleranceAndCatchUp(t *testing.T) {
	t.Parallel()
	const tol = "/series/0/plans/0/doses/1/tolerance"
	span := func(s string) *Span {
		v, err := ParseSpan(s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	notBelow := []Problem{{Code: ToleranceNotBelowMinimum, Severity: Warn, Path: tol}}
	tolCases := []struct {
		tolerance, minInterval string
		want                   []Problem
	}{
		{"P4D", "P3D", notBelow},
		{"P4D", "P4D", notBelow},
		{"P4D", "P5D", nil},
		{"P4D", "P1W", nil},
		{"P4D", "P1M", nil},
		{"P1W", "P7D", notBelow},
		{"P5W", "P1M", notBelow}, // a 31-day month is below 35 days
		{"P4W", "P1M", notBelow}, // a 28-day month equals 4 weeks
		{"P27D", "P1M", nil},
		{"P1M", "P2M", []Problem{{Code: ToleranceUnit, Severity: Reject, Path: tol, Params: ProblemParams{Value: "P1M"}}}},
		{"P1Y", "P2Y", []Problem{{Code: ToleranceUnit, Severity: Reject, Path: tol, Params: ProblemParams{Value: "P1Y"}}}},
	}
	for _, tc := range tolCases {
		s := mustDecode(t, validCase)
		d1 := &s.Series[0].Plans[0].Doses[1]
		d1.Tolerance, d1.MinInterval = span(tc.tolerance), span(tc.minInterval)
		if got := Validate(s); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("tolerance %s, minInterval %s: Validate =\n  %v\nwant\n  %v", tc.tolerance, tc.minInterval, got, tc.want)
		}
	}
	// The minimum age and minIntervalFromFirst are compared too, one Warn
	// for the rule.
	s := mustDecode(t, validCase)
	d1 := &s.Series[0].Plans[0].Doses[1]
	d1.Tolerance, d1.MinAge, d1.MinIntervalFromFirst = span("P2W"), span("P1W"), span("P1W")
	if got := Validate(s); !reflect.DeepEqual(got, notBelow) {
		t.Errorf("tolerance at or above minAge and minIntervalFromFirst: Validate = %v, want %v", got, notBelow)
	}

	const cu = "/series/0/catchUpAfterAge"
	after := []Problem{{Code: CatchUpAfterMaxAge, Severity: Warn, Path: cu}}
	cuCases := []struct {
		catchUp, maxComplete string
		want                 []Problem
	}{
		{"P2Y", "", nil},
		{"P2Y", "P3Y", nil},
		{"P2Y", "P2Y", after},
		{"P24M", "P2Y", after},
		{"P3Y", "P2Y", after},
		{"P104W", "P2Y", after}, // 728 days is not certainly below 24 months of 28 days
		{"P90W", "P2Y", nil},
	}
	for _, tc := range cuCases {
		s := mustDecode(t, validCase)
		s.Series[0].CatchUpAfterAge = span(tc.catchUp)
		if tc.maxComplete != "" {
			s.Series[0].MaxAgeComplete = span(tc.maxComplete)
		}
		if got := Validate(s); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("catchUpAfterAge %s, maxAgeComplete %s: Validate =\n  %v\nwant\n  %v", tc.catchUp, tc.maxComplete, got, tc.want)
		}
	}
}

// TestDate covers ruleset.Date's text form: exactly the schema's
// $defs/Date pattern (years 0001–9999, real days, Gregorian leap years).
func TestDate(t *testing.T) {
	t.Parallel()
	good := map[string]Date{
		"2026-01-01": {2026, 1, 1},
		"0001-01-01": {1, 1, 1},
		"9999-12-31": {9999, 12, 31},
		"2024-02-29": {2024, 2, 29},
		"2000-02-29": {2000, 2, 29},
	}
	for in, want := range good {
		got, err := ParseDate(in)
		if err != nil || got != want {
			t.Errorf("ParseDate(%q) = %v, %v; want %v", in, got, err, want)
		}
		if b, err := got.MarshalText(); err != nil || string(b) != in {
			t.Errorf("MarshalText(%v) = %q, %v; want %q", got, b, err, in)
		}
	}
	for _, in := range []string{"", "0000-01-01", "2025-02-29", "1900-02-29", "2026-13-01", "2026-00-10", "2026-04-31",
		"2026-1-01", "+2026-01-01", "2026-01-01 ", "2026/01/01", "20260101", "2026-01-0a", "10000-01-01"} {
		if d, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) = %v, want an error", in, d)
		}
	}
	for _, d := range []Date{{}, {2025, 2, 30}, {2026, 13, 1}} {
		if _, err := d.MarshalText(); err == nil {
			t.Errorf("MarshalText(%s) succeeded, want an error", d)
		}
	}
	var d Date
	if err := d.UnmarshalText([]byte("2026-01-31")); err != nil || d != (Date{2026, 1, 31}) {
		t.Errorf("UnmarshalText = %v, %v", d, err)
	}
	if err := d.UnmarshalText([]byte("2026-02-31")); err == nil {
		t.Error("UnmarshalText(2026-02-31) succeeded")
	}
}
