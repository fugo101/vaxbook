package engine

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// placement is the result of SAD §5.3 step 2 for one disease: where each
// dose record sits in a series. It does not depend on the birth date, so
// both ends of a birth-date range (step 4) walk the same placement.
//
// arrange computes the part that does not depend on the series: the walk
// order, the step-2 reasons and the positions the records take. forLength
// then fills the slots of a series of a given length, so the candidate
// series of a disease (step 3) share one arrangement.
type placement struct {
	// dated are the dated doses in walk order: time order (Lo, Hi, ID),
	// except that the members of an overlap resolved by user-entered
	// numbers are ordered by number, and a run of exact doses on the same
	// day with numbers by position (orderSameDay, F04.2).
	dated []givenDose
	// fixed holds the undated slots, by position: a numbered level-4 dose
	// at its number, an unnumbered level-4 dose filling a gap, or an
	// implied dose (id ""). Positions are ≤ the series length.
	fixed map[int]givenDose
	// floating are the unnumbered level-4 doses that fill no gap: counted,
	// with no position, and possibly the latest dose (F04.2). By ID.
	floating []givenDose
	// implied are the positions of the implied doses, ascending (F04.1).
	implied []int
	// beyond are the numbered level-4 doses whose number is above the
	// series length, by number and then ID. They are not walked, but the
	// lowest one carries the reason when the implied doses below it cannot
	// fit by today (walk).
	beyond []givenDose
	// reasons are the blocking step-2 reasons: OVERLAPPING_ESTIMATED_DATES,
	// then NUMBERING_CONFLICT. Any of them skips steps 5–9 (blocked).
	reasons []Reason
	// force and forceAll are the walk's per-record override (issue #28 In
	// scope 0e): the dated doses and level-4 slots in force, or every
	// record with forceAll (the counted view), count whatever their checks
	// (placement.forced). force maps each record to the position it is
	// forced at, or 0 for any position. Set only on walk's own copy.
	force    map[string]int
	forceAll bool
	// steps are the step-2 positions of the dated doses, in walk order:
	// the next free one for an unnumbered dose, the number for a numbered
	// one (item 2's time-order check, which assumes every dated dose takes
	// a position). The S2 chain gap search reads them (issue #35, q_k).
	steps []int
	// noGapSearch turns the gap search off, so that a test can compare the
	// walks with and without it (issue #35, Q103). Only tests set it.
	noGapSearch bool

	// Set by arrange, read by forLength: the numbered and the unnumbered
	// level-4 doses, each by ID, and the positions the records take.
	numbered, unnumbered []givenDose
	occupied             map[int]bool
}

func (pl placement) blocked() bool { return len(pl.reasons) > 0 }

// place assigns positions in series s: arrange, then forLength.
func (s seriesModel) place(doses []givenDose) placement {
	return arrange(doses).forLength(s.length)
}

// arrange assigns positions (SAD §5.3 step 2, F04.1, F04.2, A4, EQ20a);
// forLength fills the gaps of item 3 for one series length.
//
//  1. Dated doses are put in time order. Two dated ranges overlap when they
//     intersect and are not both single days. Overlapping ranges form
//     connected groups. A group is resolved only when every member has a
//     user-entered number, and is then ordered by number; otherwise it is
//     OVERLAPPING_ESTIMATED_DATES{doses}, with the IDs in time order. Two
//     exact doses on the same day do not overlap and have no time order
//     between them: when some of them have numbers, the numbers place
//     those and the others fill the free positions around them by ID
//     (orderSameDay); otherwise they keep their S1 order by ID.
//  2. A user-entered number that repeats an earlier record's number, or
//     that is not above the position reached by the dated doses before it
//     in time, is NUMBERING_CONFLICT{dose, number}, at most one per record,
//     in walk order and then for undated records by ID. The time-order
//     check assumes every earlier dated dose takes a position, so it never
//     depends on dose validity or on the birth date; it runs only when no
//     overlap is unresolved.
//  3. Positions below the highest position taken that no dose takes are
//     gaps. They are filled, ascending, by the unnumbered level-4 doses in
//     ID order; the rest are implied doses (IMPLIED_EARLIER_DOSES). The
//     unnumbered level-4 doses left over float.
//
// A dose numbered above the series length sits beyond the series, like
// S1's doses beyond the series: it is not checked or counted itself, but
// its number implies the positions below it, and the doses there must fit
// before it (walk).
func arrange(doses []givenDose) placement {
	var dated, numbered, unnumbered []givenDose
	for _, g := range doses {
		switch {
		case g.dated:
			dated = append(dated, g)
		case g.number > 0:
			numbered = append(numbered, g)
		default:
			unnumbered = append(unnumbered, g)
		}
	}
	slices.SortFunc(dated, func(x, y givenDose) int {
		return cmp.Or(x.date.Lo.Compare(y.date.Lo), x.date.Hi.Compare(y.date.Hi), strings.Compare(x.id, y.id))
	})
	byID := func(x, y givenDose) int { return strings.Compare(x.id, y.id) }
	slices.SortFunc(numbered, byID)
	slices.SortFunc(unnumbered, byID)

	occupied := map[int]bool{}
	reserved := map[int]bool{}
	for _, g := range numbered {
		occupied[g.number] = true
		reserved[g.number] = true
	}
	pl := placement{dated: dated}
	unresolved := pl.resolveOverlaps()
	if !unresolved {
		orderSameDay(pl.dated, reserved)
	}

	// Numbering conflicts, collected per record and reported in walk order.
	conflict := map[string]int{}
	seen := map[int]bool{}
	for _, g := range slices.Concat(pl.dated, numbered) {
		if g.number == 0 {
			continue
		}
		if seen[g.number] {
			conflict[g.id] = g.number
		}
		seen[g.number] = true
	}
	if !unresolved {
		c := 0
		for _, g := range pl.dated {
			if g.number == 0 {
				c = nextFree(c, reserved)
				occupied[c] = true
				pl.steps = append(pl.steps, c)
				continue
			}
			pl.steps = append(pl.steps, g.number)
			if _, dup := conflict[g.id]; !dup && g.number <= c {
				conflict[g.id] = g.number
			}
			c = max(c, g.number)
			occupied[g.number] = true
		}
	}
	for _, g := range slices.Concat(pl.dated, numbered) {
		if n, ok := conflict[g.id]; ok {
			pl.reasons = append(pl.reasons, Reason{Code: ReasonNumberingConflict, Params: ReasonParams{Dose: g.id, Number: n}})
		}
	}
	pl.numbered, pl.unnumbered, pl.occupied = numbered, unnumbered, occupied
	return pl
}

// forLength returns the placement in a series of n doses: the undated
// slots up to n (fixed, implied), the numbered level-4 doses beyond n and
// the unnumbered level-4 doses that fill no gap. A blocked placement is
// returned unchanged. pl is not modified.
func (pl placement) forLength(n int) placement {
	if pl.blocked() {
		return pl
	}
	numbered, unnumbered, occupied := pl.numbered, pl.unnumbered, pl.occupied
	pl.fixed, pl.beyond, pl.implied = map[int]givenDose{}, nil, nil
	for _, g := range numbered {
		if g.number <= n {
			pl.fixed[g.number] = g
		} else {
			pl.beyond = append(pl.beyond, g)
		}
	}
	slices.SortStableFunc(pl.beyond, func(x, y givenDose) int { return cmp.Compare(x.number, y.number) })
	top := 0
	for q := range occupied {
		top = max(top, q)
	}
	for q := 1; q < top && q <= n; q++ {
		if occupied[q] {
			continue
		}
		if len(unnumbered) > 0 {
			pl.fixed[q], unnumbered = unnumbered[0], unnumbered[1:]
			continue
		}
		pl.fixed[q] = givenDose{}
		pl.implied = append(pl.implied, q)
	}
	pl.floating = unnumbered
	return pl
}

// resolveOverlaps groups the overlapping dated ranges, reorders each group
// whose members all have numbers by number, and adds
// OVERLAPPING_ESTIMATED_DATES for each other group. It reports whether any
// group is unresolved.
func (pl *placement) resolveOverlaps() bool {
	d := pl.dated
	parent := make([]int, len(d))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	// One sweep over d, sorted by (Lo, Hi): an earlier range intersects
	// d[j] when its Hi is on or after d[j].Lo. The earlier ranges that do
	// are already connected through one another, except exact doses on the
	// day d[j] starts, which do not overlap one another (orderSameDay). So
	// d[j] joins the earlier range with the latest Hi when that range
	// reaches it and is not an exact dose on the same day as an exact d[j],
	// and a range that is not exact also joins each exact dose on its first
	// day. Exact doses on a day sort before the longer ranges starting on
	// it, so that list is used once. On a tie the first range stays the
	// widest: a range ending on a day sorts before an exact dose on it, so
	// the widest is an exact dose only when no longer range reaches its day.
	widest := -1      // the earlier range with the latest Hi
	var sameDay []int // the exact doses on the last exact day not yet joined
	for j := range d {
		r := d[j].date
		if widest >= 0 && !d[widest].date.Hi.Before(r.Lo) && (!r.IsExact() || !d[widest].date.IsExact()) {
			parent[find(j)] = find(widest)
		}
		if r.IsExact() {
			if len(sameDay) > 0 && d[sameDay[0]].date.Lo != r.Lo {
				sameDay = sameDay[:0]
			}
			sameDay = append(sameDay, j)
		} else {
			if len(sameDay) > 0 && d[sameDay[0]].date.Lo == r.Lo {
				for _, i := range sameDay {
					parent[find(i)] = find(j)
				}
			}
			sameDay = sameDay[:0]
		}
		if widest < 0 || d[widest].date.Hi.Before(r.Hi) {
			widest = j
		}
	}
	groups := map[int][]int{}
	var roots []int
	for i := range d {
		r := find(i)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], i)
	}
	unresolved := false
	for _, r := range roots {
		idx := groups[r]
		if len(idx) < 2 {
			continue
		}
		members := make([]givenDose, len(idx))
		numberedAll := true
		for t, i := range idx {
			members[t] = d[i]
			numberedAll = numberedAll && d[i].number > 0
		}
		if !numberedAll {
			ids := make([]string, len(members))
			for t, g := range members {
				ids[t] = g.id
			}
			pl.reasons = append(pl.reasons, Reason{Code: ReasonOverlappingEstimatedDates, Params: ReasonParams{Doses: ids}})
			unresolved = true
			continue
		}
		slices.SortStableFunc(members, func(x, y givenDose) int { return cmp.Compare(x.number, y.number) })
		for t, i := range idx {
			d[i] = members[t]
		}
	}
	return unresolved
}

// orderSameDay orders each run of two or more exact doses on the same day,
// which have no time order between them (F04.2). When some of them have
// numbers, each numbered dose sits at its number and the unnumbered ones
// take, in ID order, the free positions after the position reached before
// the run (skipping reserved positions and the run's numbers); the run is
// then walked in position order. So a number never contradicts the time
// order inside the run, and the result does not depend on the record IDs.
// A run with no numbers keeps its order by ID. reserved are the numbers of
// the undated records; the position reached advances as in place.
func orderSameDay(d []givenDose, reserved map[int]bool) {
	c := 0
	for i := 0; i < len(d); {
		j := i + 1
		for j < len(d) && d[i].date.IsExact() && d[j].date.IsExact() && d[j].date.Lo == d[i].date.Lo {
			j++
		}
		run := d[i:j]
		if len(run) > 1 && slices.ContainsFunc(run, func(g givenDose) bool { return g.number > 0 }) {
			taken := maps.Clone(reserved)
			for _, g := range run {
				if g.number > 0 {
					taken[g.number] = true
				}
			}
			pos := make(map[string]int, len(run))
			cc := c
			for _, g := range run {
				if g.number > 0 {
					pos[g.id] = g.number
				} else {
					cc = nextFree(cc, taken)
					pos[g.id] = cc
				}
			}
			slices.SortStableFunc(run, func(x, y givenDose) int { return cmp.Compare(pos[x.id], pos[y.id]) })
		}
		for _, g := range run {
			if g.number > 0 {
				c = max(c, g.number)
			} else {
				c = nextFree(c, reserved)
			}
		}
		i = j
	}
}

// nextFree returns the first position after c that taken does not hold.
func nextFree(c int, taken map[int]bool) int {
	q := c + 1
	for taken[q] {
		q++
	}
	return q
}
