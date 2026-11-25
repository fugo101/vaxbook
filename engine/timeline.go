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
	tl.compileVersions()
	return tl, nil
}

// version is a series d of another validity segment j that may count a
// dose of today's series c as the rule in effect on its date (SAD §5.3 step
// 5, F05.10 "Rule versions", EQ26; issue #29 In scope 3, Q77, Q93–Q96).
type version struct {
	// s is d as compiled in segment j: its plans, its maximum ages and its
	// bracket's reach there (floor, max, groupMax). It has no versions.
	s seriesModel
	// seg is segment j, [from, to); a missing bound is unbounded.
	seg window
	// lone: d is alone in its product group in segment j, so its bracket
	// holds every age there (C8, Q95).
	lone bool
	// sameGroup: d is in c's product group. tier holds, for each catalog
	// product P of the disease, whether d applies to a P dose under c
	// (Q94): d is in P's tier in segment j, and d is in c's group or c is
	// in P's tier today, neither tier being the every-series fall-through.
	// A dose with no product or another product has every series as its
	// tier, so d applies to it iff sameGroup.
	sameGroup bool
	tier      map[string]bool
}

// applies reports whether the version applies to a dose of product
// (Q94).
func (v version) applies(product string) bool {
	if a, ok := v.tier[product]; ok {
		return a
	}
	return v.sameGroup
}

// lastDose reports whether the version may count a dose at position pos of
// a series of length n (Q96): it has at least pos doses, and when pos is
// the series' last dose it is the version's last too.
func (v version) lastDose(pos, n int) bool {
	return v.s.length >= pos && (pos != n || v.s.length == pos)
}

// compileVersions attaches to each series of each segment its versions:
// every series d of every other segment that applies to some dose under it
// (version.tier, version.sameGroup), in segment order, then rule-set order.
// A series in effect in several segments is a version of itself there,
// since its floor, groupMax or tiers may differ. Nothing in a walk reads
// another segment's model, so a version carries what it needs.
func (tl diseaseTimeline) compileVersions() {
	if len(tl.models) < 2 {
		return
	}
	segs := make([]window, len(tl.models))
	for j := range segs {
		segs[j] = window{from: tl.starts[j], hasFrom: j > 0}
		if j+1 < len(tl.starts) {
			segs[j].to, segs[j].hasTo = tl.starts[j+1], true
		}
	}
	// orig holds every segment's series before any versions are attached.
	orig := make([][]seriesModel, len(tl.models))
	for j, m := range tl.models {
		orig[j] = slices.Clone(m.series)
	}
	for i, mi := range tl.models {
		for ci := range mi.series {
			c := &mi.series[ci]
			for j, mj := range tl.models {
				if j == i {
					continue
				}
				for di, d := range orig[j] {
					v := version{s: d, seg: segs[j], lone: len(mj.groups[mj.groupOf[di]].series) == 1, sameGroup: d.product == c.product}
					useful := v.sameGroup
					for _, p := range mi.catalog {
						a := slices.Contains(mj.tier[p], di) &&
							(v.sameGroup || !mj.fallsThrough[p] && !mi.fallsThrough[p] && slices.Contains(mi.tier[p], ci))
						if v.tier == nil {
							v.tier = map[string]bool{}
						}
						v.tier[p] = a
						useful = useful || a
					}
					if !useful {
						continue
					}
					c.versions = append(c.versions, v)
					v.s = v.s.withoutMaxAges()
					c.bare = append(c.bare, v)
					if v.seg.hasFrom {
						c.cuts = append(c.cuts, v.seg.from)
					}
					if v.seg.hasTo {
						c.cuts = append(c.cuts, v.seg.to)
					}
				}
			}
			slices.SortFunc(c.cuts, Date.Compare)
			c.cuts = slices.Compact(c.cuts)
		}
	}
}
