package engine

import (
	"cmp"
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
	rank := map[string]int{}
	for i, sr := range s.Series {
		rank[sr.ID] = i
	}
	tl.compileVersions(rank)
	return tl, nil
}

// version is a series d of other validity segments that may count a dose
// of today's series c as the rule in effect on its date (SAD §5.3 step 5,
// F05.10 "Rule versions", EQ26; issue #29 In scope 3, Q77, Q93–Q96).
type version struct {
	// s is d as compiled in its segments: its plans, its maximum ages and its
	// bracket's reach there (floor, max, groupMax). It has no versions, and
	// is shared by every version of d in the same segments.
	s *seriesModel
	// seg is the run of adjacent segments, [from, to), in which d is compiled
	// alike (sameSegmentRules) and applies alike; a missing bound is
	// unbounded. It never holds today's segment.
	seg window
	// lone: d is alone in its product group in seg, so its bracket holds
	// every age there (C8, Q95).
	lone bool
	// sameGroup: d is in c's product group. d applies to a dose of catalog
	// product P under c (Q94) when d is in P's tier in seg (inTier), and d is
	// in c's group, or c is in P's tier today (cTier), neither tier being the
	// every-series fall-through (fallsThrough in seg; cTier leaves out
	// today's). A dose with no product or another product has every series
	// as its tier, so d applies to it iff sameGroup. The three tables are
	// shared by every version that reads them.
	sameGroup                   bool
	inTier, fallsThrough, cTier map[string]bool
}

// applies reports whether the version applies to a dose of product
// (Q94).
func (v version) applies(product string) bool {
	in, catalog := v.inTier[product]
	switch {
	case !catalog:
		return v.sameGroup
	case v.sameGroup:
		return in
	}
	return in && !v.fallsThrough[product] && v.cTier[product]
}

// lastDose reports whether the version may count a dose at position pos of
// a series of length n (Q96): it has at least pos doses, and when pos is
// the series' last dose it is the version's last too.
func (v version) lastDose(pos, n int) bool {
	return v.s.length >= pos && (pos != n || v.s.length == pos)
}

// tierSet is an interned tier table of compileVersions: a map from each
// catalog product to whether a series is in its tier, and an id equal for
// equal contents.
type tierSet struct {
	id  int
	set map[string]bool
}

// sameSegmentRules reports whether a and b, one series compiled in two
// segments, have the same rules. compileSeries derives every field from
// the series alone except the bracket's floor and groupMax, which follow
// from the product group in effect (compileDisease).
func sameSegmentRules(a, b *seriesModel) bool {
	return a.id == b.id && a.start == b.start && a.groupMax == b.groupMax
}

// compileVersions attaches to each series c of each segment i its
// versions: every series d of every other segment j that applies to some
// dose under it (version.applies), and its cuts, the bounds of those
// segments, ascending. A series in effect in several segments is a version
// of itself there, since its floor, groupMax or tiers may differ.
//
// The versions of one d in adjacent segments other than i that compile and
// apply alike are one version over their run, and the versions are in
// rule-set order of d (rank), then segment order. Both keep datedVerdict's
// output: the cuts still split a dose range at every segment bound, a part
// lies in one segment, where at most one version of each d is in effect,
// so its versions come in rule-set order as they would one segment at a
// time; and the envelope's checks do not depend on the order or repeats of
// its plans.
//
// Which versions apply depends on c only through its product group: its
// product, and the tiers that hold it, which are whole groups. So the
// versions are compiled once per group and segment, and its series share
// them (a group of many bracketed series would otherwise hold a copy per
// series and segment, a fourth power). The work is the series d of each
// other segment that apply, found through the tiers, with tier tables
// compared by interned id, so a snapshot with many series in effect at
// once compiles in about segments² × series time, not as the product of
// all pairs and the catalog. Nothing in a walk reads another segment's
// model, so a version carries what it needs.
func (tl diseaseTimeline) compileVersions(rank map[string]int) {
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
	// The shared tables of each segment j: inTier[j][d] for each series d,
	// cTier[j][c] (with its products in catalog order, cProducts[j][c]) for
	// each series c, fallsID[j] for its fall-through products, and
	// byProduct[j], the series of each product ("" for generic) in rule-set
	// order. A tier table is interned by its contents (the catalog is the
	// same in every segment), so equal tables are one map with one id and
	// two versions compare theirs in constant time. orig holds every
	// segment's series before any versions are attached, and bare maps each
	// of them to a copy with no maximum age.
	n := len(tl.models)
	orig := make([][]seriesModel, n)
	bare := map[*seriesModel]*seriesModel{}
	inTier := make([][]tierSet, n)
	fallsID := make([]int, n)
	cTier := make([][]map[string]bool, n)
	cProducts := make([][][]string, n)
	byProduct := make([]map[string][]int, n)
	sets := map[string]tierSet{}
	intern := func(catalog []string, key []byte) tierSet {
		if t, ok := sets[string(key)]; ok {
			return t
		}
		t := tierSet{id: len(sets), set: make(map[string]bool, len(catalog))}
		for k, p := range catalog {
			t.set[p] = key[k] == 1
		}
		sets[string(key)] = t
		return t
	}
	for j, m := range tl.models {
		orig[j] = slices.Clone(m.series)
		inTier[j] = make([]tierSet, len(m.series))
		cTier[j] = make([]map[string]bool, len(m.series))
		cProducts[j] = make([][]string, len(m.series))
		byProduct[j] = map[string][]int{}
		keys := make([][]byte, len(m.series))
		for d, sm := range m.series {
			b := sm.withoutMaxAges()
			bare[&orig[j][d]] = &b
			keys[d] = make([]byte, len(m.catalog))
			cTier[j][d] = map[string]bool{}
			byProduct[j][sm.product] = append(byProduct[j][sm.product], d)
		}
		falls := make([]byte, len(m.catalog))
		for k, p := range m.catalog {
			if m.fallsThrough[p] {
				falls[k] = 1
			}
			for _, d := range m.tier[p] {
				keys[d][k] = 1
				if !m.fallsThrough[p] {
					cTier[j][d][p] = true
					cProducts[j][d] = append(cProducts[j][d], p)
				}
			}
		}
		for d := range m.series {
			inTier[j][d] = intern(m.catalog, keys[d])
		}
		fallsID[j] = intern(m.catalog, falls).id
	}
	var found []int
	for i, mi := range tl.models {
		for _, g := range mi.groups {
			// Every series of group g gets the same versions: those of c, its
			// first, since the product and the tiers that hold a series are
			// its group's (a tier other than the fall-through is a whole
			// group).
			ci := g.series[0]
			c := mi.series[ci]
			var versions, bareVersions []version
			var ids [][2]int // the tier ids of versions[k]: inTier, fallsThrough
			var cuts []Date
			last := map[string]int{} // d's id → its latest version in versions
			for j, mj := range tl.models {
				if j == i {
					continue
				}
				// The series of segment j that apply to some dose under c:
				// c's group, and the tier in j of each product whose tier
				// today holds c, neither falling through.
				found = append(found[:0], byProduct[j][c.product]...)
				for _, p := range cProducts[i][ci] {
					if !mj.fallsThrough[p] {
						found = append(found, mj.tier[p]...)
					}
				}
				slices.Sort(found)
				for _, di := range slices.Compact(found) {
					d := &orig[j][di]
					v := version{s: d, seg: segs[j], lone: len(mj.groups[mj.groupOf[di]].series) == 1, sameGroup: d.product == c.product,
						inTier: inTier[j][di].set, fallsThrough: mj.fallsThrough, cTier: cTier[i][ci]}
					id := [2]int{inTier[j][di].id, fallsID[j]}
					if v.seg.hasFrom {
						cuts = append(cuts, v.seg.from)
					}
					if v.seg.hasTo {
						cuts = append(cuts, v.seg.to)
					}
					if k, ok := last[d.id]; ok {
						if p := &versions[k]; p.seg.hasTo && p.seg.to == v.seg.from && sameSegmentRules(p.s, d) && p.lone == v.lone && ids[k] == id {
							p.seg.to, p.seg.hasTo = v.seg.to, v.seg.hasTo
							continue
						}
					}
					last[d.id] = len(versions)
					versions, ids = append(versions, v), append(ids, id)
				}
			}
			slices.SortStableFunc(versions, func(a, b version) int { return cmp.Compare(rank[a.s.id], rank[b.s.id]) })
			for _, v := range versions {
				v.s = bare[v.s]
				bareVersions = append(bareVersions, v)
			}
			slices.SortFunc(cuts, Date.Compare)
			versions, bareVersions, cuts = slices.Clip(versions), slices.Clip(bareVersions), slices.Clip(slices.Compact(cuts))
			for _, si := range g.series {
				s := &mi.series[si]
				s.versions, s.bare, s.cuts = versions, bareVersions, cuts
			}
		}
	}
}
