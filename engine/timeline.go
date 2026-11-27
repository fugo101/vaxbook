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
	// today's). The three tables are shared by every version that reads
	// them.
	//
	// A dose with no product or another product has every series as its
	// tier, so it meets c's group only; and since it could be any catalog
	// product, d applies to it only when d is also in the tier in seg of
	// every catalog product (everyTier, Q116). It is then never counted by a
	// rule that a dose of one of them would not meet (P1, SAD §5.7). A
	// fall-through product's tier holds every series, so it adds no
	// restriction, and with no catalog product everyTier holds.
	sameGroup, everyTier        bool
	inTier, fallsThrough, cTier map[string]bool
	// twin, when not nil, is the tier table of d's twins in today's segment
	// (compileVersions, Q109): d does not apply to a dose of a catalog
	// product in a twin's tier today, nor to a dose with no product or
	// another product, since today's candidate choice already decides those
	// doses with the same rules.
	twin map[string]bool
	// lenient marks a copy for the lenient walk of the bracket choice
	// (seriesModel.lenient, Q119): a dose with no product or another
	// product meets it, whatever the rules above say.
	lenient bool
}

// applies reports whether the version applies to a dose of product (Q94,
// Q109, Q116; Q119 for a lenient copy). A product the catalog tier tables
// do not hold (none, another product, a retired one or free text) is
// tested before the twin, so that a lenient copy reaches it.
func (v version) applies(product string) bool {
	in, catalog := v.inTier[product]
	switch {
	case !catalog:
		return v.lenient || v.sameGroup && v.everyTier && v.twin == nil
	case v.twin != nil && v.twin[product]:
		return false
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
// equal contents. all: every catalog product's tier holds the series
// (version.everyTier, Q116), true for an empty catalog; it follows from
// the contents, so equal tables share it.
type tierSet struct {
	id  int
	set map[string]bool
	all bool
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
// dose under it (version.applies), other than through a twin in segment i
// (twinKey, Q109), and its cuts, the bounds of those segments, ascending. A
// series in effect in several segments is a version of itself there when
// its floor, groupMax or lone flag differ, or for the products whose tier
// held it then but not today.
//
// A twin of d is a series of segment i with d's product, compiled rules,
// reach and lone flag. For a dose of a product in a twin's tier today, or
// with no product or another product, d's rule is then still in effect
// today, in the same place, and today's candidate choice (A2, C3) already
// decides whether it applies, as on the windowless snapshot. Judging such a
// dose by d again would count it by a series that choice did not pick, so
// an identical re-issue of one series, which leaves the others of its group
// in effect in both segments, would change the output. The tiers are
// compared per product (version.twin), not as whole tables: another
// product's series starting or ending changes the tables of every series,
// the fall-through ones included, but not the tier of the dose's product.
// A d whose every product tier in j holds a twin applies to nothing and is
// left out.
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
// all pairs and the catalog. The twin test reads one key per series and
// segment, built once. Nothing in a walk reads another segment's model, so
// a version carries what it needs.
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
	// twinKeys[j][d] is series d of segment j's twin key, and twins[j] maps
	// each key of segment j to its series' tier table there (twins share a
	// product, so they share a tier table). kinds[j] is the tier kind of
	// each catalog product in j, and spans caches tierSpan per pair of
	// segments.
	twinKeys := make([][]string, n)
	twins := make([]map[string]tierSet, n)
	kinds := make([][]tierKind, n)
	spans := map[[2]int]tierSpan{}
	intern := func(catalog []string, key []byte) tierSet {
		if t, ok := sets[string(key)]; ok {
			return t
		}
		t := tierSet{id: len(sets), set: make(map[string]bool, len(catalog)), all: true}
		for k, p := range catalog {
			t.set[p] = key[k] == 1
			t.all = t.all && key[k] == 1
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
		twins[j] = make(map[string]tierSet, len(m.series))
		twinKeys[j] = make([]string, len(m.series))
		for d := range m.series {
			k := twinKey(&orig[j][d], len(m.groups[m.groupOf[d]].series) == 1)
			twinKeys[j][d], twins[j][k] = k, inTier[j][d]
		}
		kinds[j] = make([]tierKind, len(m.catalog))
		generic := make(map[string]bool, len(m.genericProducts))
		for _, p := range m.genericProducts {
			generic[p] = true
		}
		for k, p := range m.catalog {
			switch {
			case m.fallsThrough[p]:
				kinds[j][k] = tierFalls
			case generic[p]:
				kinds[j][k] = tierGeneric
			}
		}
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
			var versions, bareVersions, lenient []version
			var ids [][3]int // the tier ids of versions[k]: inTier, fallsThrough, twin (-1 for none)
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
						everyTier: inTier[j][di].all, inTier: inTier[j][di].set, fallsThrough: mj.fallsThrough, cTier: cTier[i][ci]}
					id := [3]int{inTier[j][di].id, fallsID[j], -1}
					if tw, ok := twins[i][twinKeys[j][di]]; ok {
						sp, seen := spans[[2]int{j, i}]
						if !seen {
							sp = compareTiers(mj.catalog, kinds[j], kinds[i])
							spans[[2]int{j, i}] = sp
						}
						if sp.covers(d.product) {
							continue
						}
						v.twin, id[2] = tw.set, tw.id
					}
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
			strict := true // every version applies to a dose outside the catalog
			for _, v := range versions {
				v.s = bare[v.s]
				bareVersions = append(bareVersions, v)
				strict = strict && v.applies("")
			}
			if !strict {
				lenient = make([]version, len(bareVersions))
				for k, v := range bareVersions {
					v.lenient = true
					lenient[k] = v
				}
			}
			slices.SortFunc(cuts, Date.Compare)
			versions, bareVersions, cuts = slices.Clip(versions), slices.Clip(bareVersions), slices.Clip(slices.Compact(cuts))
			for _, si := range g.series {
				s := &mi.series[si]
				s.versions, s.bare, s.lenient, s.cuts = versions, bareVersions, lenient, cuts
			}
		}
	}
}

// tierKind is how a catalog product's tier is chosen in a segment
// (compileDisease): its own series, else the generic ones, else every
// series (the fall-through).
type tierKind uint8

const (
	tierOwn tierKind = iota
	tierGeneric
	tierFalls
)

// tierSpan compares the tier kinds of segment j with those of today's
// segment i, for compileVersions' twin test. The series of product X lie
// in the tiers of X (when X has series, as a twin's product does) and of
// every fall-through product; generic series in the tiers of the generic
// and fall-through products. A twin in i therefore holds every product
// tier that holds d in j when the products that fall through in j but not
// in i are at most d's own (product series), or when no product with a
// generic or fall-through tier in j has its own series in i (generic
// series). Each pair of segments is compared once, in catalog time.
type tierSpan struct {
	falls   int    // products falling through in j but not in i
	one     string // one of them, when falls is 1
	generic bool   // no generic or fall-through product of j is own in i
}

// compareTiers compares two segments' tier kinds over their catalog.
func compareTiers(catalog []string, j, i []tierKind) tierSpan {
	sp := tierSpan{generic: true}
	for k, p := range catalog {
		if j[k] == tierFalls && i[k] != tierFalls {
			sp.falls++
			sp.one = p
		}
		if j[k] != tierOwn && i[k] == tierOwn {
			sp.generic = false
		}
	}
	return sp
}

// covers reports whether a twin of product in today's segment holds every
// product tier that holds the series in j, so the version applies to no
// dose.
func (sp tierSpan) covers(product string) bool {
	if product == "" {
		return sp.generic
	}
	return sp.falls == 0 || sp.falls == 1 && sp.one == product
}

// twinKey encodes what makes series s of a segment a twin of a series of
// another segment (compileVersions, Q109): its product, its compiled rules
// (length, plans with their tolerances, maximum ages, groupMax, catch-up
// age, retired product and its bracket with the floor, so its reach) and
// its lone flag (C8). The tiers are compared per product (version.twin).
// The series ID is left out: a re-issue under a new ID is a twin. Every
// field is a value, so equal keys mean equal rules.
func twinKey(s *seriesModel, lone bool) string {
	return fmt.Sprintf("%q %v %d %+v %+v %+v %+v %+v %+v %v",
		s.product, s.retired, s.length, s.start, s.plans, s.maxStart, s.maxComplete, s.groupMax, s.catchUp, lone)
}
