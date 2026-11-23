package engine

import "slices"

// Start-age brackets: the start-age part of SAD §5.3 step 3 (F05.2, F05.5,
// EQ10, EQ26, A5, C1–C3, C8; issue #23). Brackets apply inside each product
// group of the candidates S3a's tiers give (Q39): the tier union is a union
// of whole groups, and each group keeps the series its brackets choose.

// groupsIn returns the groups that have a series in cands, in group order.
func (dz diseaseModel) groupsIn(cands []int) []int {
	in := make([]bool, len(dz.groups))
	for _, c := range cands {
		in[dz.groupOf[c]] = true
	}
	var out []int
	for g, ok := range in {
		if ok {
			out = append(out, g)
		}
	}
	return out
}

// members returns the series of group g that are in cands, in bracket order.
func (dz diseaseModel) members(g int, cands []int) []int {
	var out []int
	for _, i := range dz.groups[g].series {
		if slices.Contains(cands, i) {
			out = append(out, i)
		}
	}
	return out
}

// noDoseAt returns the candidates at birth end b when no dose is given (SAD
// §5.3 step 3, EQ10, A5, Q33): from S3a's no-dose candidates (Q38), each
// group keeps its first bracket open at b, the bracket whose reach contains
// the age today (the bracket the age falls in, the next one above a gap or
// the earliest one); a group with none is dropped.
// When every group is dropped, the choice is redone over every group,
// retired ones included (issue #23, Q50 (b)), so the result is empty only
// when no series of the disease can be started at b: the outOfAge block
// (Q24). In rule-set order.
func (dz diseaseModel) noDoseAt(b, today Date) []int {
	if c := dz.openAt(dz.noDose, b, today); len(c) > 0 {
		return c
	}
	return dz.openAt(dz.all, b, today)
}

// noDosePoints returns the birth dates at which step 4 evaluates the
// birth-date range r when no dose is given (SAD §5.3 step 4, EQ10; PR #27,
// issue #23), ascending and without repeats: r.Lo, r.Hi and, for each date
// c in (r.Lo, r.Hi] where noDoseAt changes, c − 1 and c.
//
// The choice at b depends only on which brackets are open at b, and a
// bracket with a max is open from the first b with b + max after today on
// (AddSpan never decreases as b grows, month-end clamping included). So the
// changes are among those dates, one per bracket that is closed at r.Lo and
// open at r.Hi, found by bisection on openAt itself so that clamping is
// exact; every series of the disease counts, retired ones included, since
// the Q50 (b) fallback reads them. A date where the choice does not change
// is no split. The choice is then fixed on each sub-range [r.Lo, c₁ − 1],
// [c₁, c₂ − 1], …, [c_k, r.Hi], and for fixed candidates the dates move one
// way with the birth date, so the ends of a sub-range bound every day in
// it.
func (dz diseaseModel) noDosePoints(r Range, today Date) []Date {
	var cs []Date
	for _, s := range dz.series {
		br := s.start
		if !br.ok || !br.max.ok || br.openAt(r.Lo, today) || !br.openAt(r.Hi, today) {
			continue
		}
		closed, open := r.Lo, r.Hi
		for closed.AddDays(1).Before(open) {
			mid := closed.AddDays(int(open.days-closed.days) / 2)
			if br.openAt(mid, today) {
				open = mid
			} else {
				closed = mid
			}
		}
		cs = append(cs, open)
	}
	slices.SortFunc(cs, Date.Compare)
	pts := []Date{r.Lo}
	for _, c := range slices.Compact(cs) {
		if !slices.Equal(dz.noDoseAt(c.AddDays(-1), today), dz.noDoseAt(c, today)) {
			pts = append(pts, c.AddDays(-1), c)
		}
	}
	return slices.Compact(append(pts, r.Hi))
}

func (dz diseaseModel) openAt(cands []int, b, today Date) []int {
	var out []int
	for _, g := range dz.groupsIn(cands) {
		for _, i := range dz.members(g, cands) {
			if dz.series[i].start.openAt(b, today) {
				out = append(out, i)
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

// byAgeAtDoseOne chooses, in each product group of union, the series by the
// age at dose 1, when doses are given (SAD §5.3 step 3, F05.2, F05.5; issue
// #23 In scope 4). It runs once, before step 4, so both birth ends use the
// same candidates.
//
//   - A group of one series keeps it (C8; a single bracket is always chosen
//     by the rules below).
//   - With no birth date every bracket possibly matches (Q37).
//   - Otherwise a bracket matches when its reach (from the previous
//     bracket's max in the group's order, or from birth for the first, up
//     to its own max; see bracket) possibly contains the age at one of its
//     series' dose-1 candidates (doseOneRanges) at either birth end. A gap
//     between brackets belongs to the bracket above it (Q33), so a range
//     that may fall in a bracket or in the gap above it matches both.
//   - One match: it. Several: the matching series with the most doses, all
//     of them on a tie (Q34), and AGE_AT_START_UNCERTAIN listing every
//     match in rule-set order.
//   - No match: with a the earliest dose-1 candidate date of the group, the
//     first bracket whose max is absent or after a (A5), else the one with
//     the latest max (C3); the doses are counted normally. When every
//     bracket reads the same dose-1 record (the same dose-1 minAge across
//     the group), no match means every candidate is at or past the last
//     max, so this is the latest-max bracket. When the dose-1 minAge
//     differs (the FIRST_DOSE_MIN_AGE_DIFFERS Warn, Q43), each bracket
//     walks its own dose 1, and nothing can match while a candidate is
//     younger than every max: a then picks the bracket whose reach holds
//     the earliest candidate.
//
// The candidates are in rule-set order; the reasons are one per group with
// several matches, in group order.
func (dz diseaseModel) byAgeAtDoseOne(arr placement, union []int, birth birthRange, today Date) (cands []int, uncertain []Reason) {
	for _, g := range dz.groupsIn(union) {
		members := dz.members(g, union)
		if len(members) == 1 {
			cands = append(cands, members[0])
			continue
		}
		var matched []int
		if !birth.known {
			matched = members
		} else {
			ends := []birthEnd{{date: birth.r.Lo, known: true}}
			if !birth.r.IsExact() {
				ends = append(ends, birthEnd{date: birth.r.Hi, known: true})
			}
			var a Date // the earliest dose-1 candidate of the group
			seen := false
			for _, i := range members {
				s := dz.series[i]
				hit := false
				for _, d := range s.doseOneRanges(arr.forLength(s.length), ends, birth.r.Lo, today) {
					hit = hit || s.start.mayContain(d, birth.r)
					if !seen || d.Lo.Before(a) {
						a, seen = d.Lo, true
					}
				}
				if hit {
					matched = append(matched, i)
				}
			}
			if len(matched) == 0 {
				pick := members[len(members)-1]
				for _, i := range members {
					if br := dz.series[i].start; !br.max.ok || a.Before(birth.r.Hi.AddSpan(br.max.span)) {
						pick = i
						break
					}
				}
				cands = append(cands, pick)
				continue
			}
		}
		if len(matched) == 1 {
			cands = append(cands, matched[0])
			continue
		}
		most := 0
		for _, i := range matched {
			most = max(most, dz.series[i].length)
		}
		for _, i := range matched {
			if dz.series[i].length == most {
				cands = append(cands, i)
			}
		}
		matched = slices.Sorted(slices.Values(matched))
		uncertain = append(uncertain, Reason{Code: ReasonAgeAtStartUncertain, Params: ReasonParams{Candidates: dz.refs(matched)}})
	}
	slices.Sort(cands)
	return cands, uncertain
}

// doseOneRanges returns the dose-1 candidates of series s for bracket
// matching, as date ranges, walking pl at each birth end (issue #23, Q32,
// Q49):
//   - the item counted at position 1 at each end: a dated dose's range, or
//     for an undated dose 1 at a fixed position (a numbered or gap-filling
//     level-4 dose, or an implied dose) [birthLo, R.Hi of the earliest dated
//     dose, or today];
//   - a floating level-4 dose when one may be dose 1: the series has
//     floating doses, and no dated dose is counted or the first counted
//     dated dose is an insertion point of floatingFit (no number, and no
//     numbered dated dose after it). Its range is [birthLo, R.Hi of that
//     dose, or today when there is none];
//   - when no end gives one, the dose at position 1: the first item in walk
//     order tried at position 1 and not counted. Each later dose that fails
//     there is tried at position 1 again, but only the first is dose 1's
//     record, so every bracket of a group with the same dose-1 minimum age
//     reads the same record. Else the earliest dated dose (else [birthLo,
//     today], which no placement reaches).
//
// A wider or extra candidate can only add matching brackets (P1): each
// test is monotone in the candidate's range, and the reaches of a group
// tile every age below the last max (Q33), so a range that may fall in a
// bracket or in the gap above it matches both, and the most-doses rule
// then picks the longer series. When every bracket of the group reads the
// same dose-1 record (one dose-1 minAge), nothing matches only when every
// candidate is at or past the last max; the last bracket is then kept,
// and a wider candidate still matches it. With the FIRST_DOSE_MIN_AGE_DIFFERS
// Warn (Q43) each bracket reads its own record, so a group can match
// nothing while a candidate is younger than every max, and P1 is not
// claimed there.
func (s seriesModel) doseOneRanges(pl placement, ends []birthEnd, birthLo, today Date) []Range {
	upTo := today
	if len(pl.dated) > 0 {
		upTo = pl.dated[0].date.Hi
	}
	slot := func(it item) Range {
		if it.dose.dated {
			return it.dose.date
		}
		return Range{Lo: birthLo, Hi: upTo}
	}
	var out, uncounted []Range
	for _, b := range ends {
		w := s.walk(b, today, pl)
		var first *item // the first counted dated dose in the series
		tried := false  // an uncounted item at position 1 was seen at b
		for i := range w.seq {
			it := w.seq[i]
			switch {
			case it.pos == 1 && it.counted:
				out = append(out, slot(it))
			case it.pos == 1 && !tried:
				uncounted, tried = append(uncounted, slot(it)), true
			}
			if first == nil && it.counted && it.dose.dated && it.pos <= s.length {
				first = &w.seq[i]
			}
		}
		if len(pl.floating) == 0 {
			continue
		}
		switch {
		case first == nil:
			out = append(out, Range{Lo: birthLo, Hi: today})
		case first.dose.number == 0 && !numberedAfter(pl, first.dose.id):
			out = append(out, Range{Lo: birthLo, Hi: first.dose.date.Hi})
		}
	}
	switch {
	case len(out) > 0:
		return out
	case len(uncounted) > 0:
		return uncounted
	case len(pl.dated) > 0:
		return []Range{pl.dated[0].date}
	}
	return []Range{{Lo: birthLo, Hi: today}}
}

// numberedAfter reports whether a dated dose after the one with ID id in
// walk order has a user-entered number.
func numberedAfter(pl placement, id string) bool {
	i := slices.IndexFunc(pl.dated, func(g givenDose) bool { return g.id == id })
	return slices.ContainsFunc(pl.dated[i+1:], func(g givenDose) bool { return g.number > 0 })
}

// refs returns the series as SeriesRefs, in the given order.
func (dz diseaseModel) refs(series []int) []SeriesRef {
	out := make([]SeriesRef, 0, len(series))
	for _, c := range series {
		s := dz.series[c]
		out = append(out, SeriesRef{Product: s.product, Series: s.id})
	}
	return out
}
