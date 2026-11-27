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
// c in (r.Lo, r.Hi] where noDoseAt changes, c − 1 and c: the ends of the
// sub-ranges of noDoseSubRanges.
func (dz diseaseModel) noDosePoints(r Range, today Date) []Date {
	var pts []Date
	for _, sub := range dz.noDoseSubRanges(r, today) {
		pts = append(pts, sub.Lo, sub.Hi)
	}
	return slices.Compact(pts)
}

// noDoseSubRanges returns the sub-ranges of r on which the no-dose choice
// is fixed, ascending: [r.Lo, c₁ − 1], [c₁, c₂ − 1], …, [c_k, r.Hi] for
// the dates c in (r.Lo, r.Hi] where noDoseAt changes.
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
// it. Since S4b the maximum ages remove candidates as the birth date grows
// inside a sub-range (A3, issue #28 Q65); a day inside then keeps a
// superset of the later end's candidates, which only adds product options.
func (dz diseaseModel) noDoseSubRanges(r Range, today Date) []Range {
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
	var subs []Range
	lo := r.Lo
	for _, c := range slices.Compact(cs) {
		if !slices.Equal(dz.noDoseAt(c.AddDays(-1), today), dz.noDoseAt(c, today)) {
			subs = append(subs, Range{Lo: lo, Hi: c.AddDays(-1)})
			lo = c
		}
	}
	return append(subs, Range{Lo: lo, Hi: r.Hi})
}

// noDoseResult is steps 3 and 5–9 at birth date b when no dose is given:
// S3b's bracket choice (noDoseAt), then A3's removal of the candidates
// already out of age (filterOutOfAge, SAD §5.3 step 3, F05.3, EQ10; issue
// #28 In scope 1, Q65, Q72), before evaluate, so that SAFEST_SERIES and
// products list only the candidates left.
//
// When every candidate of the non-retired choice is removed and the
// choice over every group differs, that choice is filtered instead (Q72,
// Q50 (b) extended; products are then omitted). When none is left, the
// result is outOfAge with OUT_OF_AGE if every removed candidate is
// certainly out at b, otherwise ⚪ BIRTH_DATE_IMPRECISE{fullBirthDate}; both
// with no candidates. S3b's outOfAge (no bracket open at b) is unchanged.
//
// A candidate removed only from b_old's limits is still a candidate for
// some birth date of the range, so catchUp reads its catch-up age too
// (evaluation.doubt, Q108).
func (dz diseaseModel) noDoseResult(arr placement, b birthEnd, today Date) endResult {
	primary, all := dz.openAt(dz.noDose, b.date, today), dz.openAt(dz.all, b.date, today)
	cands := primary
	if len(cands) == 0 {
		cands = all
	}
	if len(cands) == 0 {
		return dz.evaluate(arr, nil, today, nil, true).at(b)
	}
	kept, certain, doubt := dz.filterOutOfAge(arr, cands, b, today)
	if len(kept) == 0 && len(primary) > 0 && !slices.Equal(primary, all) {
		kept, certain, doubt = dz.filterOutOfAge(arr, all, b, today)
	}
	switch {
	case len(kept) > 0:
		ev := dz.evaluate(arr, kept, today, nil, true)
		ev.doubt = doubt
		return ev.at(b)
	case certain:
		return endResult{block: BlockOutOfAge, reasons: []Reason{{Code: ReasonOutOfAge}}}
	}
	return endResult{bucket: BucketUnknown, reasons: []Reason{{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: HintFullBirthDate}}}}
}

// filterOutOfAge returns the no-dose candidates (all at dose 1) that are not
// out of age at b (issue #28 In scope 1): a candidate is removed when its
// own F_c is past Limit_c,1; then, until nothing changes, when the merged
// F of the candidates left (from each one's plans not past its limit) is
// past a candidate's Limit_c,1, it is removed too, since it could be given
// only before the date the others need. The candidate with the latest
// earliest date is never removed that way, so the list empties only
// through the first rule. certain reports whether every candidate removed
// by the first rule is past its limit at b itself (Limit^b), and doubt
// lists the candidates removed by either rule that are not past Limit^b
// there, so not out at b (Q108).
func (dz diseaseModel) filterOutOfAge(arr placement, cands []int, b birthEnd, today Date) (kept []int, certain bool, doubt []removed) {
	ev := dz.evaluate(arr, cands, today, nil, false)
	rs := make([]candidateResult, len(cands))
	certain = true
	var left []int // indexes into cands
	for i, c := range cands {
		rs[i] = dz.series[c].run(b, today, ev.pls[i])
		out, sure := rs[i].outOf(today)
		if !out {
			left = append(left, i)
			continue
		}
		certain = certain && sure
		if !sure {
			doubt = append(doubt, removed{series: c, earliest: rs[i].lastF(today)})
		}
	}
	for {
		f := today
		for _, i := range left {
			nd, _ := rs[i].kept(today)
			f = maxDate(f, nd.Earliest)
		}
		n := len(left)
		left = slices.DeleteFunc(left, func(i int) bool {
			if !rs[i].limits.lim.exceededBy(f) {
				return false
			}
			if !rs[i].limits.limOwn.exceededBy(f) {
				doubt = append(doubt, removed{series: cands[i], earliest: rs[i].lastF(today)})
			}
			return true
		})
		if len(left) == n {
			break
		}
	}
	for _, i := range left {
		kept = append(kept, cands[i])
	}
	return kept, certain, doubt
}

// removed is a no-dose candidate filterOutOfAge removed although it is not
// out at the evaluated birth date: its series, and the latest earliest
// date of its plans (lastF), at or after any F it adds where it is kept.
type removed struct {
	series   int
	earliest Date
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
	// unknown: a dose record, dated or level 4, has no product or another
	// product, so the walk also runs leniently (doseOneRanges, Q119). An
	// implied dose is no record and has no reading as a catalog product.
	outside := func(g givenDose) bool {
		_, ok := dz.tier[g.product]
		return !ok
	}
	unknown := slices.ContainsFunc(arr.dated, outside) || slices.ContainsFunc(arr.numbered, outside) ||
		slices.ContainsFunc(arr.unnumbered, outside)
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
				for _, d := range s.doseOneRanges(arr.forLength(s.length), ends, birth.r.Lo, today, unknown, outside) {
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
//     floating doses, no record holds position 1 (firstHeld), and no
//     dated dose is counted or the first counted dated dose is an
//     insertion point of floatingFit (no number, and no numbered dated
//     dose after it). Its range is [birthLo, R.Hi of that dose, or today
//     when there is none];
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
//
// The walks run without the maximum-age checks (issue #15 In scope 2), so
// the bracket choice is S3b's: a dose 1 past a series' maximum age to start
// stays dose 1's record, and those checks apply in step 5 after the choice.
//
// Which record is counted at position 1 moves with the versions, and the
// bracket choice with it, in either direction (issue #39), so no one set
// of versions in the walk keeps a dose with no product or another product
// from being judged better than a dose of some catalog product it could
// be. With unknown (such a dose record is given), the walk runs twice
// (Q119): with the strict versions, which such a dose meets only within
// every catalog product's tier, as in step 5 (Q116), and with their
// lenient copies, all of which it meets. Every version a dose of a catalog
// product meets is a compiled one (compileVersions), so for every reading
// of such doses as catalog products the strict walk's versions are a
// subset of the reading's, and the reading's a subset of the lenient
// walk's. At a birth end, until a record counts at position 1 every walk
// tries the same records there in the same order, and whether one counts
// is monotone in the versions (datedVerdict). So every reading counts as
// dose 1 a record from the lenient walk's dose 1 up to the strict walk's,
// or up to the last record the strict walk tries when it counts none; a
// reading that counts none at every end reads the strict walk's fallback.
// A named record (outside false) meets the same versions in every walk and
// every reading, so when the strict walk tries it at position 1 and does
// not count it, no reading counts it there either. When the two walks
// differ, the candidate is therefore one range, the hull of both walks'
// candidates and every record with no product or another product that the
// strict walk tries at position 1 from the lenient walk's dose 1 on: every
// reading's dose 1 lies in it, and a wider candidate only adds matches
// (Q106's argument).
func (s seriesModel) doseOneRanges(pl placement, ends []birthEnd, birthLo, today Date, unknown bool, outside func(givenDose) bool) []Range {
	bare := s.withoutMaxAges()
	strict := bare.doseOneWalk(pl, ends, birthLo, today)
	if !unknown || s.lenient == nil {
		return strict.ranges
	}
	bare.versions = s.lenient
	lenient := bare.doseOneWalk(pl, ends, birthLo, today)
	if slices.Equal(strict.at, lenient.at) && slices.Equal(strict.ranges, lenient.ranges) {
		return strict.ranges
	}
	h := strict.ranges[0]
	widen := func(rs []Range) {
		for _, r := range rs {
			h = Range{Lo: minDate(h.Lo, r.Lo), Hi: maxDate(h.Hi, r.Hi)}
		}
	}
	widen(strict.ranges)
	widen(lenient.ranges)
	for e, k := range lenient.at {
		if k < 0 || k >= len(strict.tried[e]) {
			continue
		}
		for _, t := range strict.tried[e][k:] {
			if outside(t.dose) {
				widen([]Range{t.r})
			}
		}
	}
	return []Range{h}
}

// doseOneWalk is one run of doseOneRanges' walk, with s's versions as
// they are.
type doseOneWalk struct {
	// ranges are the dose-1 candidates, as doseOneRanges returns them for
	// one set of versions.
	ranges []Range
	// tried holds, per birth end, the items tried at position 1 in walk
	// order up to the one counted there, and at the index of that one, or
	// -1 when none is counted.
	tried [][]triedItem
	at    []int
}

// triedItem is an item a doseOneWalk tries at position 1: its dose and its
// candidate range.
type triedItem struct {
	dose givenDose
	r    Range
}

// doseOneWalk walks pl at each birth end and reads its dose-1 candidates
// (doseOneRanges).
func (s seriesModel) doseOneWalk(pl placement, ends []birthEnd, birthLo, today Date) doseOneWalk {
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
	dw := doseOneWalk{tried: make([][]triedItem, len(ends)), at: make([]int, len(ends))}
	var out, uncounted []Range
	for e, b := range ends {
		w := s.walk(b, today, pl)
		var first *item // the first counted dated dose in the series
		tried := false  // an uncounted item at position 1 was seen at b
		dw.at[e] = -1
		for i := range w.seq {
			it := w.seq[i]
			if it.pos == 1 && dw.at[e] < 0 {
				dw.tried[e] = append(dw.tried[e], triedItem{dose: it.dose, r: slot(it)})
				if it.counted {
					dw.at[e] = len(dw.tried[e]) - 1
				}
			}
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
		if len(pl.floating) == 0 || pl.firstHeld() {
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
		dw.ranges = out
	case len(uncounted) > 0:
		dw.ranges = uncounted
	case len(pl.dated) > 0:
		dw.ranges = []Range{pl.dated[0].date}
	default:
		dw.ranges = []Range{{Lo: birthLo, Hi: today}}
	}
	return dw
}

// firstHeld reports whether a record holds position 1 whatever the dates:
// an undated slot there (a level-4 dose numbered 1, a gap-filling level-4
// dose or an implied dose) or a dated dose numbered 1. A floating dose is
// never inserted in front of either, so it cannot be dose 1. Being
// independent of the dates, it keeps every series' candidates monotone in
// the dose ranges (P1), even when the series' own intervals decide which
// dated dose is counted first.
func (pl placement) firstHeld() bool {
	if _, ok := pl.fixed[1]; ok {
		return true
	}
	return slices.ContainsFunc(pl.dated, func(g givenDose) bool { return g.number == 1 })
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
