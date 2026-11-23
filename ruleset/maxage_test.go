package ruleset

import (
	"reflect"
	"strconv"
	"testing"
)

// maxAgeDose is one dose rule of a maxAgeSeries plan: the minimum spans, ""
// for absent.
type maxAgeDose struct{ minAge, minInterval, fromFirst string }

// maxAgeSeries builds series "s" of disease "d" with the given maximum ages
// ("" for absent), start-age bracket min ("" for none, open-ended) and one
// plan per entry of plans, named p0, p1, ….
func maxAgeSeries(t *testing.T, maxStart, maxComplete, startMin string, plans ...[]maxAgeDose) Series {
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
	sr := Series{ID: "s", Disease: "d", Stage: Childhood, MaxAgeStart: span(maxStart), MaxAgeComplete: span(maxComplete)}
	if startMin != "" {
		sr.StartAge = &StartAge{Min: *span(startMin)}
	}
	for j, doses := range plans {
		pl := Plan{ID: "p" + strconv.Itoa(j)}
		for _, d := range doses {
			pl.Doses = append(pl.Doses, DoseRule{
				MinAge:               span(d.minAge),
				MinInterval:          span(d.minInterval),
				MinIntervalFromFirst: span(d.fromFirst),
				Recommended:          Milestone{From: FromBirth, Offset: Span{N: 1, Unit: Years}},
			})
		}
		sr.Plans = append(sr.Plans, pl)
	}
	return sr
}

// TestValidateMaxAges covers the S4a checks of maxAgeStart and
// maxAgeComplete (SAD §5.2, issue #15, Q60): MAX_AGE_NOT_AFTER_MIN_AGE only
// when certain, MAX_AGE_MAY_NOT_BE_AFTER_MIN_AGE in the uncertain band of
// the 31/28 comparison, MAX_AGE_START_AFTER_COMPLETE and
// SERIES_CANNOT_COMPLETE only when certain.
func TestValidateMaxAges(t *testing.T) {
	t.Parallel()
	const start, complete = "/series/0/maxAgeStart", "/series/0/maxAgeComplete"
	reject := func(code ProblemCode, path string) Problem {
		return Problem{Code: code, Severity: Reject, Path: path}
	}
	warn := func(code ProblemCode, path string, params ProblemParams) Problem {
		return Problem{Code: code, Severity: Warn, Path: path, Params: params}
	}
	one := func(minAge string) []maxAgeDose { return []maxAgeDose{{minAge: minAge}} }
	cases := []struct {
		name string
		sr   func(t *testing.T) Series
		want []Problem
	}{
		{"maxAgeStart equal to the dose-1 minAge", func(t *testing.T) Series {
			return maxAgeSeries(t, "P6W", "", "", one("P6W"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, start)}},
		{"maxAgeComplete below a second plan's dose-1 minAge", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P8W", "", one("P6W"), one("P10W"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, complete)}},
		{"maxAgeStart equal to startAge.min", func(t *testing.T) Series {
			return maxAgeSeries(t, "P9Y", "", "P9Y", one("P8Y"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, start)}},
		{"mixed units certainly not after: P5W minAge, P1M maxAgeStart", func(t *testing.T) Series {
			return maxAgeSeries(t, "P1M", "", "", one("P5W"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, start)}},
		{"a zero maxAgeComplete is certainly not after", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P0D", "", one("P0D"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, complete)}},
		{"uncertain band: P4W minAge, P1M maxAgeStart", func(t *testing.T) Series {
			return maxAgeSeries(t, "P1M", "", "", one("P4W"))
		}, []Problem{warn(MaxAgeMayNotBeAfterMinAge, start, ProblemParams{})}},
		{"uncertain band: P8W minAge, P2M maxAgeStart", func(t *testing.T) Series {
			return maxAgeSeries(t, "P2M", "", "", one("P8W"))
		}, []Problem{warn(MaxAgeMayNotBeAfterMinAge, start, ProblemParams{})}},
		{"uncertain band: P4W startAge.min, P1M maxAgeStart", func(t *testing.T) Series {
			return maxAgeSeries(t, "P1M", "", "P4W", one("P0D"))
		}, []Problem{warn(MaxAgeMayNotBeAfterMinAge, start, ProblemParams{})}},
		{"a certain pair wins over an uncertain one", func(t *testing.T) Series {
			return maxAgeSeries(t, "P1M", "", "", one("P4W"), one("P5W"))
		}, []Problem{reject(MaxAgeNotAfterMinAge, start)}},
		{"certainly after: P4W minAge, P5W maxAgeStart", func(t *testing.T) Series {
			return maxAgeSeries(t, "P5W", "", "", one("P4W"))
		}, nil},
		{"maxAgeStart after maxAgeComplete", func(t *testing.T) Series {
			return maxAgeSeries(t, "P20W", "P16W", "", []maxAgeDose{{minAge: "P6W"}, {minInterval: "P4W"}})
		}, []Problem{warn(MaxAgeStartAfterComplete, start, ProblemParams{})}},
		{"maxAgeStart may be after maxAgeComplete (mixed units)", func(t *testing.T) Series {
			return maxAgeSeries(t, "P4M", "P17W", "", one("P6W"))
		}, []Problem{warn(MaxAgeStartAfterComplete, start, ProblemParams{})}},
		{"start problems in document order", func(t *testing.T) Series {
			return maxAgeSeries(t, "P1M", "P3W", "", one("P4W"))
		}, []Problem{
			warn(MaxAgeMayNotBeAfterMinAge, start, ProblemParams{}),
			warn(MaxAgeStartAfterComplete, start, ProblemParams{}),
			reject(MaxAgeNotAfterMinAge, complete),
		}},
		// Dose 1 P1Y; dose 2 4w after; dose 3 4w after and 6m after dose 1.
		// The minAge bound 12m + 56d is not certainly at or after 18m, but
		// 12m + 6m = 18m is.
		{"cannot complete: minIntervalFromFirst bound", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P18M", "", []maxAgeDose{{minAge: "P1Y"}, {minInterval: "P4W"}, {minInterval: "P4W", fromFirst: "P6M"}})
		}, []Problem{warn(SeriesCannotComplete, complete, ProblemParams{ID: "p0"})}},
		{"cannot complete: a later dose's own minAge bound", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P20W", "", []maxAgeDose{{minAge: "P6W"}, {minAge: "P20W", minInterval: "P4W"}})
		}, []Problem{warn(SeriesCannotComplete, complete, ProblemParams{ID: "p0"})}},
		{"cannot complete: only the plan that cannot", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P14W", "",
				[]maxAgeDose{{minAge: "P6W"}, {minInterval: "P4W"}, {minInterval: "P3W"}},
				[]maxAgeDose{{minAge: "P6W"}, {minInterval: "P4W"}, {minInterval: "P4W"}})
		}, []Problem{warn(SeriesCannotComplete, complete, ProblemParams{ID: "p1"})}},
		// minimal-max-age-complete's z-a: 216m + 28d against 240m is in the
		// uncertain band of the 31/28 rule, so no Warn.
		{"no Warn in the uncertain band", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P20Y", "", []maxAgeDose{{minAge: "P18Y"}, {minInterval: "P4W"}})
		}, nil},
		{"a rejected maxAgeComplete is not checked for completion", func(t *testing.T) Series {
			return maxAgeSeries(t, "", "P6W", "", []maxAgeDose{{minAge: "P6W"}, {minInterval: "P4W"}})
		}, []Problem{reject(MaxAgeNotAfterMinAge, complete)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &Snapshot{Version: "v", Diseases: []Disease{{ID: "d", Name: "D"}}, Series: []Series{tc.sr(t)}}
			if got := Validate(s); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Validate =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}
