package engine

import (
	"fmt"
	"slices"

	"github.com/fugo101/vaxbook/ruleset"
)

// Effective dating (SAD §5.3 step 3, F05.1, EQ26; issue #29 In scope 2,
// Q77): a series is in effect on day d iff (validFrom absent or d ≥
// validFrom) and (validTo absent or d < validTo). The validity bounds of a
// disease's series cut time into segments, and the same series are in
// effect throughout a segment, so the disease is compiled once per segment:
// tiers, product groups, the no-dose list, the catalog and generic
// products, bracket order, floors and groupMax all follow from which series
// are in effect. Steps 3–9 run on the segment that contains today.

// diseaseTimeline is one disease's models per validity segment. Segment i
// runs from starts[i] up to the day before starts[i+1]; starts[0] is the
// zero Date and stands for no lower bound. A snapshot with no windows has
// one segment, compiled exactly as before S4c.
type diseaseTimeline struct {
	id     string
	starts []Date
	models []diseaseModel
}

// at returns the model of the segment that contains day.
func (tl diseaseTimeline) at(day Date) diseaseModel {
	i, found := slices.BinarySearchFunc(tl.starts[1:], day, Date.Compare)
	if found {
		i++
	}
	return tl.models[i]
}

// window is a series' validity window, converted once: [from, to), each
// bound absent when not set.
type window struct {
	from, to       Date
	hasFrom, hasTo bool
}

// contains reports whether day lies in the window.
func (w window) contains(day Date) bool {
	return (!w.hasFrom || !day.Before(w.from)) && (!w.hasTo || day.Before(w.to))
}

// windowOf converts a series' validity bounds. Validate has accepted them,
// so a conversion error is an engine bug.
func windowOf(sr ruleset.Series) (window, error) {
	var w window
	conv := func(d *ruleset.Date) (Date, bool, error) {
		if d == nil {
			return Date{}, false, nil
		}
		v, err := NewDate(d.Year, d.Month, d.Day)
		if err != nil {
			return Date{}, false, fmt.Errorf("engine: series %q: validity bound passed validation but %w", sr.ID, err)
		}
		return v, true, nil
	}
	var err error
	if w.from, w.hasFrom, err = conv(sr.ValidFrom); err != nil {
		return w, err
	}
	w.to, w.hasTo, err = conv(sr.ValidTo)
	return w, err
}

// compileTimeline compiles one disease per validity segment: the sorted,
// unique validFrom and validTo dates of its series are the segment starts,
// and each segment's model is compileDisease over the series in effect on
// its first day (the first segment: those with no validFrom). Then it
// attaches each series' versions (compileVersions).
func compileTimeline(s *ruleset.Snapshot, disease string) (diseaseTimeline, error) {
	windows := make([]window, len(s.Series))
	var bounds []Date
	for i, sr := range s.Series {
		if sr.Disease != disease {
			continue
		}
		w, err := windowOf(sr)
		if err != nil {
			return diseaseTimeline{}, err
		}
		windows[i] = w
		if w.hasFrom {
			bounds = append(bounds, w.from)
		}
		if w.hasTo {
			bounds = append(bounds, w.to)
		}
	}
	slices.SortFunc(bounds, Date.Compare)
	tl := diseaseTimeline{id: disease, starts: append([]Date{{}}, slices.Compact(bounds)...)}
	for i, start := range tl.starts {
		inEffect := func(j int) bool {
			if i == 0 {
				return !windows[j].hasFrom
			}
			return windows[j].contains(start)
		}
		tl.models = append(tl.models, compileDisease(s, disease, inEffect))
	}
	return tl, nil
}
