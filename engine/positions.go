package engine

import (
	"cmp"
	"slices"
	"strings"
)

// placement is the result of SAD §5.3 step 2 for one disease: where each
// dose record sits in the series. It does not depend on the birth date, so
// both ends of a birth-date range (step 4) walk the same placement.
type placement struct {
	// dated are the dated doses in walk order: time order (Lo, Hi, ID),
	// except that the members of an overlap resolved by user-entered
	// numbers are ordered by number (F04.2).
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
}

func (pl placement) blocked() bool { return len(pl.reasons) > 0 }

// place assigns positions (SAD §5.3 step 2, F04.1, F04.2, A4, EQ20a).
//
//  1. Dated doses are put in time order. Two dated ranges overlap when they
//     intersect and are not both single days. Overlapping ranges form
//     connected groups. A group is resolved only when every member has a
//     user-entered number, and is then ordered by number; otherwise it is
//     OVERLAPPING_ESTIMATED_DATES{doses}, with the IDs in time order. Two
//     exact doses on the same day keep their S1 order by ID, unless both
//     have numbers, which then order them (a same-day pair never
//     contradicts the time order).
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
func (s seriesModel) place(doses []givenDose) placement {
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

	pl := placement{dated: dated}
	unresolved := pl.resolveOverlaps()

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
	occupied := map[int]bool{}
	reserved := map[int]bool{}
	for _, g := range numbered {
		occupied[g.number] = true
		reserved[g.number] = true
	}
	if !unresolved {
		c := 0
		for _, g := range pl.dated {
			if g.number == 0 {
				c = nextFree(c, reserved)
				occupied[c] = true
				continue
			}
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
	if pl.blocked() {
		return pl
	}

	pl.fixed = map[int]givenDose{}
	for _, g := range numbered {
		if g.number <= s.length {
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
	for q := 1; q < top && q <= s.length; q++ {
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
	for i := range d {
		for j := i + 1; j < len(d); j++ {
			// Two exact doses on the same day are ordered by ID, unless both
			// have numbers: then the numbers order them.
			bothExact := d[i].date.IsExact() && d[j].date.IsExact()
			if d[i].date.Intersects(d[j].date) && (!bothExact || d[i].number > 0 && d[j].number > 0) {
				parent[find(j)] = find(i)
			}
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

// nextFree returns the first position after c that taken does not hold.
func nextFree(c int, taken map[int]bool) int {
	q := c + 1
	for taken[q] {
		q++
	}
	return q
}
