package engine

import (
	"maps"
	"slices"
)

// birthEnd is the birth date used by one evaluation of steps 5–9: one end of
// the birth-date range (SAD §5.3 step 4), or no birth date (known false,
// EQ19), in which case no age-based step runs.
//
// The maximum ages of the next dose also read the whole range (issue #28,
// Q65): old is its older end (b_old) and late its later end capped at today
// (b_late); start is the older end of the evaluated sub-range (b_s), which
// chose the start-age bracket (Q45, Q73), and with doses given b_old. A
// zero value means date. Given doses are still validated at date (S4a).
type birthEnd struct {
	date             Date
	known            bool
	old, late, start Date
}

// older returns b_old, later b_late and subStart b_s.
func (b birthEnd) older() Date    { return orDate(b.old, b.date) }
func (b birthEnd) later() Date    { return orDate(b.late, b.date) }
func (b birthEnd) subStart() Date { return orDate(b.start, b.older()) }

// orDate returns d, or def when d is the zero Date.
func orDate(d, def Date) Date {
	if d.IsZero() {
		return def
	}
	return d
}

// verdict is the outcome of a dose-validity check over date ranges.
type verdict uint8

const (
	verdictOK       verdict = iota // met at every end of the ranges
	verdictPossible                // violated at one end only: DOSE_POSSIBLY_INVALID
	verdictDefinite                // violated at every end: DOSE_INVALID
)

// against checks a dose range x against a minimum. definite is the minimum
// at the end of the other range most favourable to x, possible the minimum
// at its least favourable end; for exact dates they are equal. Minimums are
// inclusive.
func against(x Range, definite, possible Date) verdict {
	switch {
	case x.Hi.Before(definite):
		return verdictDefinite
	case x.Lo.Before(possible):
		return verdictPossible
	}
	return verdictOK
}

// item is one dose in the walk sequence: a dated dose, or an undated slot
// (a level-4 dose with a position, or an implied dose with id "").
type item struct {
	pos     int
	dose    givenDose
	counted bool
	// maxDef and maxPos are an undated slot's maximum-age checks, failed
	// definitely and possibly (issue #15 In scope 4, Q56), set when the run
	// of slots closes (closeRun, checkTrailing, slotMax).
	maxDef, maxPos []Check
	// best and worst are an undated slot's earliest chain dates, from the
	// dated dose before its run (bounded false when nothing bounds them).
	// best is also taken from the chain that skips the uncounted unnumbered
	// dated doses before the run; lowOpen is set when that chain is
	// unbounded, so that no maximum age fails definitely (issue #28 In
	// scope 0c, Q87).
	best, worst      Date
	bounded, lowOpen bool
	// runBest is, for a dated dose after a run of undated slots (hasRun),
	// the chain's best date: the run lies on or before the dose only when
	// runBest ≤ its Hi (issue #28 In scope 0b, the guard). guardOK is the
	// same test for one floating dose inserted in front of an unnumbered
	// dated dose, computed without that walk.
	runBest         Date
	hasRun, guardOK bool
	// step is a dated dose's step-2 position (placement.steps), which the
	// S2 chain gap search reads as q_k (issue #35).
	step int
}

// slotMax checks undated slot it against the maximum ages (issue #28 In
// scope 0a, Q84). u is its upper bound: the range of the dated dose that
// closes its run, in the series or beyond it, or [today, today] for a
// trailing slot. The slot lies in [best, min(worst, u.Hi)] when its chain
// fits (best ≤ u.Hi), and in u itself when the chain overshoots u, which
// the closing record already reports as a chain failure (S2). A lowOpen
// slot fails no maximum age definitely.
func (s seriesModel) slotMax(b birthEnd, it item, u Range) (def, poss []Check) {
	if !it.bounded {
		return nil, nil
	}
	x := Range{Lo: it.best, Hi: minDate(it.worst, u.Hi)}
	if it.best.After(u.Hi) {
		x = u
	}
	def, poss = s.maxAges(it.pos, b, x)
	if it.lowOpen {
		def, poss = nil, unionChecks(poss, def)
	}
	return def, poss
}

// doseVerdict is the validity of one dose at one position under one series
// (SAD §5.3 step 5, issue #15 In scope 3): the verdict of the minimum checks
// (best over plans, A7) with the checks that reach it, and the maximum-age
// checks failed definitely and possibly (series-level). An undated slot's
// maximum-age checks are added to the record that carries them.
type doseVerdict struct {
	minV           verdict
	minChecks      []Check
	maxDef, maxPos []Check
}

// addMax adds maximum-age checks, keeping each list in catalog order.
func (d *doseVerdict) addMax(def, poss []Check) {
	d.maxDef, d.maxPos = unionChecks(d.maxDef, def), unionChecks(d.maxPos, poss)
}

// counted reports whether the dose counts: no definite failure.
func (d doseVerdict) counted() bool {
	return d.minV != verdictDefinite && len(d.maxDef) == 0
}

// reasons returns the dose's reasons, in the order of one dose under one
// candidate: DOSE_INVALID for a definite minimum failure, DOSE_AFTER_MAX_AGE
// for a definite maximum failure, or, when the dose counts, one
// DOSE_POSSIBLY_INVALID listing every possible check, minimum and maximum,
// in catalog order. A dose that is not counted drops its possible checks
// (Q62). With no maximum age this is S2's verdict.
func (d doseVerdict) reasons(id string, pos int) []Reason {
	var out []Reason
	if d.minV == verdictDefinite {
		out = append(out, Reason{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: d.minChecks, Dose: id, Number: pos}})
	}
	if len(d.maxDef) > 0 {
		out = append(out, Reason{Code: ReasonDoseAfterMaxAge, Params: ReasonParams{Checks: d.maxDef, Dose: id, Number: pos}})
	}
	if d.counted() {
		var poss []Check
		if d.minV == verdictPossible {
			poss = d.minChecks
		}
		if poss = unionChecks(poss, d.maxPos); len(poss) > 0 {
			out = append(out, Reason{Code: ReasonDosePossiblyInvalid, Params: ReasonParams{Checks: poss, Dose: id, Number: pos}})
		}
	}
	return out
}

// checkOrder is the catalog order of the checks.
var checkOrder = []Check{CheckMinAge, CheckMinInterval, CheckMinIntervalFromFirst, CheckMaxAgeStart, CheckMaxAgeComplete}

// unionChecks returns the checks in a or b, in catalog order; nil when both
// are empty. It never modifies a or b.
func unionChecks(a, b []Check) []Check {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	var out []Check
	for _, c := range checkOrder {
		if slices.Contains(a, c) || slices.Contains(b, c) {
			out = append(out, c)
		}
	}
	return out
}

// walkResult is steps 5 and 6 at one birth end.
type walkResult struct {
	seq      []item // in time order; undated slots sit where their positions put them
	reasons  []Reason
	floating []givenDose
	k        int // next dose number: highest counted position + floating doses + 1
	// lowered are the records whose definite minimum-interval failure as
	// the last trailing undated slot the gap search made possible, so that
	// they count (issue #35): the slot's ID, or the record beyond the series
	// that carries an implied slot's verdict. A slot that a maximum age
	// still fails definitely is not in it.
	lowered map[string]bool
}

// walk validates the doses (SAD §5.3 step 5, F05.10) at one birth end.
//
// The dated doses are walked in placement order. A dose with a user-entered
// number takes that position; any other dose takes the next free position
// after the last one reached, skipping the undated slots, which are inserted
// in front of the first dose that passes them. As in S1, an unnumbered dose
// that is DOSE_INVALID does not advance the position, so the next dose is
// checked for the same position.
//
// A dose meets a position when some plan's minimums all hold (A7). The
// minimum interval is measured to the dose adjacent in time, counted or not,
// never to dose j−2 (A4): to a dated predecessor P, the dose range R is
// definitely invalid when R.Hi < P.Lo + minInterval and possibly invalid
// when R.Lo < P.Hi + minInterval. Behind undated slots the interval is
// waived (INTERVAL_UNVERIFIABLE) and replaced by the feasibility of the
// slots (chain), from the dated dose before the run; the last slot after
// the last dated dose must fit by today from a dated dose before the run
// that the positions allow (checkTrailing, gapBound, issue #35). The
// minimum age is checked when the birth date is known.
// minIntervalFromFirst is checked against dose 1: a counted dated dose, or
// an undated slot at its earliest dates, so that a dose that no placement
// of dose 1 allows is DOSE_INVALID.
//
// Doses beyond the series length are not checked and not counted, but stay
// in the sequence as the dose adjacent in time (S1). The undated slots
// before such a dose must still fit before it.
//
// The floating level-4 doses have no position: they are counted when some
// placement among the dated doses fits (checkFloating). Before that, a dated
// dose that a placement of them in front of it would count is rescued
// (rescue, issue #28 In scope 0b).
//
// A placement with forceAll is the counted view (issue #28, Q85): every
// record counts at the position it reaches when all records count, and
// every floating dose counts, with no placement search. Its reasons are not
// used.
func (s seriesModel) walk(b birthEnd, today Date, pl placement) walkResult {
	w := s.walkWith(b, today, pl, nil)
	w.floating = pl.floating
	if !pl.forceAll {
		if rescued, rw, first := s.rescue(w, b, today, pl); len(rescued) > 0 {
			pl.force = maps.Clone(rescued)
			w = s.forceLater(b, today, pl, rw, first, w.lowered)
			pl.force = w.pinned(pl.force, rescued)
		}
		s.checkFloating(&w, b, today, pl)
	}
	highest := 0
	for _, it := range w.seq {
		if it.counted && it.pos <= s.length {
			highest = max(highest, it.pos)
		}
	}
	w.k = highest + len(w.floating) + 1
	return w
}

// walkWith is the walk of steps 5 and 6 with the floating doses ins[i]
// inserted as undated slots in front of dated dose i, and ins[len(pl.dated)]
// after the last dated dose and the undated slots of the placement. An
// inserted dose takes the next free position; one beyond the series length
// is not checked. ins nil walks the placement alone.
func (s seriesModel) walkWith(b birthEnd, today Date, pl placement, ins [][]givenDose) walkResult {
	var w walkResult
	last := 0 // the highest position reached
	taken := make(map[int]bool, len(pl.fixed))
	for q := range pl.fixed {
		taken[q] = true
	}
	// first is dose 1: the counted dated dose at position 1, or, when
	// position 1 is an undated slot, the earliest dates that slot can have
	// (chain), so that minIntervalFromFirst is checked against the most
	// favourable placement of dose 1 (EQ20b).
	var first *Range
	// firstBr is dose 1's range for the bracket match of the versions
	// (datedVerdict, issue #29 In scope 3, Q93): a counted dated dose 1's
	// range, or for an undated slot at position 1 [b_old, Hi of the dated
	// dose it lies before, or today], as doseOneRanges reads an undated dose
	// 1; nil when no dose 1 is counted, and datedVerdict then reads [b_old,
	// R.Hi] of the dose it judges (Q106). It is never narrower than where
	// dose 1 can lie, so a wider range can only weaken a match (P1).
	var firstBr *Range
	upTo := today
	if len(pl.dated) > 0 {
		upTo = pl.dated[0].date.Hi
	}
	//
	// Every slot is also checked against the maximum ages at the earliest
	// dates the chain gives it, from the dated dose before its run: best
	// from that dose's Lo, worst from its Hi (issue #15 In scope 4, Q56).
	// first stays the slot's chain dates even when its own maximum age makes
	// it not counted. before is the Hi of the dated dose the slot lies
	// before (today after every dated dose).
	appendSlot := func(it item, before Date) {
		from, run := w.tail()
		if it.pos == 1 {
			if lo, hi, bounded := s.chain(b, from, []item{it}, nil, true); bounded {
				first = &Range{Lo: lo, Hi: hi}
			}
			firstBr = &Range{Lo: b.older(), Hi: before}
		}
		if b.known && s.hasMaxAges() {
			if best, worst, bounded := s.chain(b, from, append(slices.Clip(run), it), first, true); bounded {
				it.best, it.worst, it.bounded = best, worst, true
				if fb, ok, skipped := w.chainSkipping(s, b, it, first); skipped && ok {
					it.best = minDate(it.best, fb)
				} else if skipped {
					it.lowOpen = true
				}
			}
		}
		w.seq = append(w.seq, it)
		last = it.pos
	}
	insertSlots := func(below int) {
		for q := last + 1; q < below && q <= s.length; q++ {
			if g, ok := pl.fixed[q]; ok {
				appendSlot(item{pos: q, dose: g, counted: true}, upTo)
			}
		}
	}
	insertFloating := func(i int) {
		if ins == nil {
			return
		}
		before := today
		if i < len(pl.dated) {
			before = pl.dated[i].date.Hi
		}
		for _, g := range ins[i] {
			q := nextFree(last, taken)
			insertSlots(q)
			if q > s.length {
				last = q
				continue
			}
			appendSlot(item{pos: q, dose: g, counted: true}, before)
		}
	}
	for i, g := range pl.dated {
		insertFloating(i)
		pos := g.number
		if pos == 0 {
			pos = nextFree(last, taken)
		}
		insertSlots(pos)
		from, run := w.tail()
		// The dated dose closes the run of slots before it: their records
		// report their own maximum ages, and the implied ones' are carried
		// by this dose.
		carry := w.closeRun(s, b, pl, run, g.date)
		it := item{pos: pos, dose: g}
		if i < len(pl.steps) {
			it.step = pl.steps[i]
		}
		pd := s.predecessor(b, from, run, first)
		it.runBest, it.hasRun = pd.chainBest, pd.run && pd.chained
		if pos > s.length {
			// Not checked itself, but the undated slots before it must fit
			// on or before it; when they do not, it carries the chain
			// failure, as a dated dose after the run in the series does,
			// and so the maximum ages of the implied slots before it.
			if len(run) > 0 {
				if best, worst, bounded := s.chain(b, from, run, first, true); bounded {
					if carry.minV = against(g.date, best, worst); carry.minV != verdictOK {
						carry.minChecks = []Check{CheckMinInterval}
					}
				}
			}
			w.reasons = append(w.reasons, carry.reasons(g.id, pos)...)
			w.seq = append(w.seq, it)
			if g.number > 0 {
				last = max(last, pos)
			}
			continue
		}
		dv := s.datedVerdict(pos, b, g.date, pd, first, firstBr, g.product)
		dv.addMax(carry.maxDef, carry.maxPos)
		if pl.forced(g.id, pos) {
			dv = dv.demoted()
		}
		reasons := dv.reasons(g.id, pos)
		it.counted = dv.counted()
		// A dose that counts with no reason has its interval waived behind
		// the undated slots; a possible failure says more.
		if it.counted && len(reasons) == 0 && len(run) > 0 && s.hasInterval(pos) {
			reasons = append(reasons, Reason{Code: ReasonIntervalUnverifiable, Params: ReasonParams{Dose: g.id, Number: pos}})
		}
		w.reasons = append(w.reasons, reasons...)
		if it.counted || g.number > 0 {
			last = max(last, pos)
		}
		if it.counted && pos == 1 {
			first, firstBr = &g.date, &g.date
		}
		// The guard of a floating dose inserted in front of an uncounted
		// unnumbered dose: it takes the dose's position, after the same run.
		if !it.counted && g.number == 0 {
			best, _, bounded := s.chain(b, from, append(slices.Clip(run), item{pos: pos}), first, true)
			it.guardOK = !bounded || !g.date.Hi.Before(best)
		}
		w.seq = append(w.seq, it)
	}
	insertSlots(s.length + 1)
	insertFloating(len(pl.dated))
	w.checkTrailing(s, b, today, first, pl)
	return w
}

// checkFloating checks that the floating level-4 doses fit among the dated
// doses (F05.10, EQ20: no feasible placement → DOSE_INVALID), with
// floatingFit.
//
// A placement that fits at every end of the ranges adds nothing. When the
// placements that fit do so only at one end, the dated ranges are split
// (covered): when every day inside them still has a placement that fits,
// nothing is added either. Otherwise some day leaves no placement, and the
// first placement tried that fits at one end adds its DOSE_POSSIBLY_INVALID
// reasons (on records the walk without the floating doses has no
// DOSE_INVALID or DOSE_POSSIBLY_INVALID on): the floating doses stay
// counted, but the result is ⚪, not ✅ (F05.10). Without them, narrowing a
// dose range could make a counted dose invalid (SAD §5.7).
//
// When no placement fits, the last floating dose by ID is not counted, with
// the number and checks it fails when all of them are placed after the last
// dated dose (failedIn): DOSE_INVALID when it cannot lie by today (checked
// like a dose given today, as a trailing slot is), and since S4a
// DOSE_AFTER_MAX_AGE when a maximum age fails there (issue #15 In scope 4).
// The other floating doses stay counted, with any DOSE_POSSIBLY_INVALID they
// get there.
func (s seriesModel) checkFloating(w *walkResult, b birthEnd, today Date, pl placement) {
	fit, fallback, failed := s.floatingFit(*w, b, today, pl)
	switch fit {
	case fitEverywhere:
	case fitAtOneEnd:
		budget := coverBudget
		if lo, hi, ok := splitWidest(pl); ok && s.covered(b, today, lo, &budget) && s.covered(b, today, hi, &budget) {
			return
		}
		w.reasons = append(w.reasons, fallback...)
	case fitNowhere:
		w.reasons = append(w.reasons, failed...)
		w.floating = slices.DeleteFunc(slices.Clone(w.floating), func(g givenDose) bool {
			return slices.ContainsFunc(failed, func(r Reason) bool {
				return r.Params.Dose == g.id && (r.Code == ReasonDoseInvalid || r.Code == ReasonDoseAfterMaxAge)
			})
		})
	}
}

// fit is how the placements of the floating doses fit over the dated ranges.
type fit uint8

const (
	fitEverywhere fit = iota // some placement fits at every end, or none is needed
	fitAtOneEnd              // no placement fits at every end, but some fits at one end
	fitNowhere               // every placement adds a DOSE_INVALID
)

// coverBudget is the most date boxes covered checks for one series at one
// birth end. One imprecise range takes about two per halving (a year takes
// about 18); past the budget the result is the conservative ⚪.
const coverBudget = 512

// floatingFit tries the placements of the floating doses, given w, the walk
// of pl without them. Only as many of them as the series still needs are
// placed, the last ones by ID. A placement puts each of them in front of a
// counted dated dose that has no number and no numbered dated dose after it
// (moving that dose and the ones after it up one position), or after every
// dated dose and undated slot. It fits when its walk has no DOSE_INVALID on
// a record w has none on: the dated doses still meet their (possibly moved)
// positions, the slots fit before the dated dose after them, and the last
// dose fits by today. It fits only at one end when its walk also has a
// DOSE_POSSIBLY_INVALID on a record w has no DOSE_INVALID or
// DOSE_POSSIBLY_INVALID on. Placements in front of an uncounted dated dose
// are tried as described below (since S4b); trying fewer placements can
// only report more DOSE_INVALID (P1).
//
// A DOSE_AFTER_MAX_AGE is bad like a DOSE_INVALID (issue #15 In scope 4):
// records w already has one on do not count against a placement, and a new
// one makes it not fit. A maximum-age DOSE_POSSIBLY_INVALID is a
// DOSE_POSSIBLY_INVALID.
//
// A forced record's reasons at the position it has in w are its verdict
// from the rescue (forceLater), so they never count against a placement.
// Nor does a definite reason on a record that w's gap search demoted
// (walkResult.lowered, issue #35 Q103): it is treated like a record w has
// DOSE_INVALID on.
// A rescued dose is judged like any other record at any other position, so
// a placement that moves it where it fails does not fit (placement.forced).
//
// fallback is the DOSE_POSSIBLY_INVALID reasons of the first placement
// tried that fits only at one end; failed is the reasons on the floating
// doses with all of them after the last dated dose (failedIn).
//
// Since S4b a placement is also tried in front of an unnumbered dated dose D
// that w does not count (uncountedPoints, issue #28 In scope 0b): D is
// walked at the next free position after the inserted doses, and its own
// reasons do not count against the placement, but the guard must hold: the
// inserted doses lie on or before D (the chain's best date ≤ D.Hi).
func (s seriesModel) floatingFit(w walkResult, b birthEnd, today Date, pl placement) (f fit, fallback []Reason, failed []Reason) {
	counted := 0
	movable := map[string]bool{}
	for _, it := range w.seq {
		if it.counted && it.pos <= s.length {
			counted++
			if it.dose.dated && it.dose.number == 0 {
				movable[it.dose.id] = true
			}
		}
	}
	if len(pl.floating) == 0 || counted >= s.length {
		return fitEverywhere, nil, nil
	}
	fl := pl.floating[len(pl.floating)-min(len(pl.floating), s.length-counted):]
	invalid, flagged := map[string]bool{}, map[string]bool{}
	for _, r := range w.reasons {
		switch r.Code {
		case ReasonDoseInvalid, ReasonDoseAfterMaxAge:
			invalid[r.Params.Dose] = true
			flagged[r.Params.Dose] = true
		case ReasonDosePossiblyInvalid:
			flagged[r.Params.Dose] = true
		}
	}
	// A demoted slot counts with its DOSE_POSSIBLY_INVALID in w, so it is
	// flagged already.
	for id := range w.lowered {
		invalid[id] = true
	}
	isBad := func(x Reason) bool {
		return (x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge) && !invalid[x.Params.Dose]
	}
	settled := map[string]int{} // the forced records at their positions in w
	for _, it := range w.seq {
		if _, ok := pl.force[it.dose.id]; ok && it.dose.id != "" {
			settled[it.dose.id] = it.pos
		}
	}
	n := len(pl.dated)
	ins := make([][]givenDose, n+1)
	uncounted := s.uncountedPoints(w, pl)
	// assess reports whether a placement's walk adds a DOSE_INVALID or a
	// DOSE_AFTER_MAX_AGE, or breaks the guard of an uncounted dose with
	// floating doses in front of it (bad), and the DOSE_POSSIBLY_INVALID
	// reasons it adds.
	assess := func(r walkResult) (bad bool, possible []Reason) {
		for _, i := range uncounted {
			if len(ins[i]) > 0 && !r.guardHolds(pl.dated[i].id) {
				return true, nil
			}
		}
		for _, x := range r.reasons {
			// A forced record at its position in w has its verdict from the
			// rescue: forceLater may have dropped its reason (beyond the
			// series in the rescuing walk).
			if p, ok := settled[x.Params.Dose]; ok && p == x.Params.Number {
				continue
			}
			switch {
			case isBad(x):
				return true, nil
			case x.Code == ReasonDosePossiblyInvalid && !flagged[x.Params.Dose]:
				possible = append(possible, x)
			}
		}
		return false, possible
	}
	// fitsEverywhere reports whether a placement fits at every end, and
	// records the first one that fits only at one end.
	fitsEverywhere := func(r walkResult) bool {
		bad, possible := assess(r)
		if bad {
			return false
		}
		if len(possible) == 0 {
			return true
		}
		if fallback == nil {
			fallback = possible
		}
		return false
	}

	ins[n] = fl
	all := s.walkWith(b, today, pl, ins)
	failed = failedIn(all, fl, isBad)
	if fitsEverywhere(all) {
		return fitEverywhere, nil, nil
	}
	// The insertion points, ascending: in front of the movable dated doses
	// after the last numbered one and the uncounted points, then after
	// everything.
	var points []int
	for i := n - 1; i >= 0 && pl.dated[i].number == 0; i-- {
		if movable[pl.dated[i].id] {
			points = append(points, i)
		}
	}
	points = append(points, uncounted...)
	slices.Sort(points)
	points = append(points, n)
	// Every way to share the doses (in ID order) among the points, except
	// all of them after everything, which was tried above.
	var try func(p int, rest []givenDose) bool
	try = func(p int, rest []givenDose) bool {
		if p == len(points)-1 {
			if len(rest) == len(fl) {
				return false
			}
			ins[n] = rest
			return fitsEverywhere(s.walkWith(b, today, pl, ins))
		}
		for c := len(rest); c >= 0; c-- {
			ins[points[p]] = rest[:c]
			if try(p+1, rest[c:]) {
				return true
			}
		}
		ins[points[p]] = nil
		return false
	}
	clear(ins)
	switch {
	case try(0, fl):
		return fitEverywhere, nil, nil
	case fallback != nil:
		return fitAtOneEnd, fallback, failed
	}
	return fitNowhere, nil, failed
}

// uncountedPoints returns the insertion points of floatingFit and rescue in
// front of the unnumbered in-series dated doses that w does not count,
// after the last numbered dated dose, as ascending indexes into pl.dated
// (issue #28 In scope 0b). In each run of such doses (no counted dated dose
// between them) only the first whose guard holds in w (item.guardOK) gets a
// point, so there is at most one point per run, and the runs are bounded by
// the counted doses, which keeps the search bounded.
func (s seriesModel) uncountedPoints(w walkResult, pl placement) []int {
	start := len(pl.dated)
	for start > 0 && pl.dated[start-1].number == 0 {
		start--
	}
	if start == len(pl.dated) {
		return nil
	}
	state := make(map[string]item, len(pl.dated)-start)
	for _, it := range w.seq {
		if it.dose.dated {
			state[it.dose.id] = it
		}
	}
	var points []int
	done := false // the current run has its point
	for i := start; i < len(pl.dated); i++ {
		it := state[pl.dated[i].id]
		switch {
		case it.counted:
			done = false
		case it.pos > s.length:
			return points
		case !done && it.guardOK:
			points, done = append(points, i), true
		}
	}
	return points
}

// guardHolds reports whether the undated slots in front of dated dose id
// lie on or before it in walk r: the chain's best date is not after its Hi
// (issue #28 In scope 0b).
func (w walkResult) guardHolds(id string) bool {
	for _, it := range w.seq {
		if it.dose.dated && it.dose.id == id {
			return !it.hasRun || !it.dose.date.Hi.Before(it.runBest)
		}
	}
	return true
}

// countsAt reports whether walk r counts dated dose id in the series.
func (w walkResult) countsAt(id string, length int) bool {
	for _, it := range w.seq {
		if it.dose.dated && it.dose.id == id {
			return it.counted && it.pos <= length
		}
	}
	return false
}

// definiteIDs returns the records that reasons give DOSE_INVALID or
// DOSE_AFTER_MAX_AGE.
func definiteIDs(reasons []Reason) map[string]bool {
	out := map[string]bool{}
	for _, x := range reasons {
		if x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge {
			out[x.Params.Dose] = true
		}
	}
	return out
}

// rescue finds the dated doses that a placement of floating doses in front
// of them counts (issue #28 In scope 0b, Q83). For each uncounted point D
// (uncountedPoints) it tries the last c floating doses by ID in front of
// D, for c from as many as fit before the series ends down to 1: D lands
// at least c positions after its position in w, so with more of them it
// is beyond the series and cannot count, and trying them would make the
// rescue quadratic in the floating doses. A placement is skipped when its
// guard fails, or when it gives DOSE_INVALID or DOSE_AFTER_MAX_AGE to a
// record that has neither in w; D's own reasons do not count, since D is
// already bad in w, and neither do those of a slot that w's gap search
// demoted (walkResult.lowered, issue #35 Q103), which some placement in
// time fits. When D counts at its new position it is rescued; a
// placement that fits at one end only also rescues. It returns the rescued
// doses, the first walk that rescues one (rw, for the earliest rescued
// dose in walk order) and that dose's ID.
func (s seriesModel) rescue(w walkResult, b birthEnd, today Date, pl placement) (rescued map[string]int, rw walkResult, first string) {
	if len(pl.floating) == 0 {
		return nil, walkResult{}, ""
	}
	points := s.uncountedPoints(w, pl)
	if len(points) == 0 {
		return nil, walkResult{}, ""
	}
	bad := definiteIDs(w.reasons)
	for id := range w.lowered {
		bad[id] = true
	}
	n := len(pl.dated)
	pos := make(map[string]int, len(points))
	for _, it := range w.seq {
		if it.dose.dated {
			pos[it.dose.id] = it.pos
		}
	}
	for _, i := range points {
		id := pl.dated[i].id
		for c := min(len(pl.floating), s.length-pos[id]); c >= 1; c-- {
			ins := make([][]givenDose, n+1)
			ins[i] = pl.floating[len(pl.floating)-c:]
			r := s.walkWith(b, today, pl, ins)
			if !r.guardHolds(id) || slices.ContainsFunc(r.reasons, func(x Reason) bool {
				return (x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge) && !bad[x.Params.Dose]
			}) {
				continue
			}
			if r.countsAt(id, s.length) {
				if rescued == nil {
					rescued, rw, first = map[string]int{}, r, id
				}
				rescued[id] = 0
				break
			}
		}
	}
	return rescued, rw, first
}

// forceLater walks pl with its rescued doses forced counted, then settles
// the records after the earliest rescued dose first in the redone walk,
// dated doses and level-4 slots alike (issue #28 In scope 0b): such a
// record keeps a DOSE_INVALID or DOSE_AFTER_MAX_AGE only if rw, the walk of
// the rescuing placement, gives it one too. Otherwise it is forced counted
// with the DOSE_POSSIBLY_INVALID of the redone walk, or with no reason when
// rw puts it beyond the series. A slot that the gap search of the walk
// before the rescue demoted (lowered, issue #35 Q103) is not kept definite
// by rw either. Forcing a record can only move the records
// after it up, so this repeats until nothing more is forced, at most length
// rounds.
func (s seriesModel) forceLater(b birthEnd, today Date, pl placement, rw walkResult, first string, lowered map[string]bool) walkResult {
	w := s.walkWith(b, today, pl, nil)
	w.floating = pl.floating
	keep := definiteIDs(rw.reasons)
	for id := range lowered {
		delete(keep, id)
	}
	beyond := map[string]bool{}
	for _, it := range rw.seq {
		if it.dose.dated && it.pos > s.length {
			beyond[it.dose.id] = true
		}
	}
	drop := map[string]bool{}
	for range s.length {
		more := false
		after := w.recordsAfter(first)
		for id := range definiteIDs(w.reasons) {
			if _, forced := pl.force[id]; after[id] && !keep[id] && !forced {
				pl.force[id], more = 0, true
				drop[id] = beyond[id]
			}
		}
		if !more {
			break
		}
		w = s.walkWith(b, today, pl, nil)
		w.floating = pl.floating
	}
	w.reasons = slices.DeleteFunc(w.reasons, func(x Reason) bool {
		return x.Code == ReasonDosePossiblyInvalid && drop[x.Params.Dose]
	})
	return w
}

// recordsAfter returns the records after dated dose id in walk w: the
// dated doses and the level-4 slots, by ID.
func (w walkResult) recordsAfter(id string) map[string]bool {
	after := map[string]bool{}
	seen := false
	for _, it := range w.seq {
		if seen && it.dose.id != "" {
			after[it.dose.id] = true
		}
		seen = seen || (it.dose.dated && it.dose.id == id)
	}
	return after
}

// failedIn returns the reasons for the floating doses fl when no placement
// fits, from walk r, all of them placed after the last dated dose. As in S2,
// the last floating dose by ID is named, with the numbers and checks it fails
// there (issue #15 In scope 4: DOSE_INVALID, or DOSE_AFTER_MAX_AGE when a
// maximum age fails), so it alone is not counted. Another dose of fl can be
// the one that fails while the last does not, because the last is placed
// beyond the series, where it is not checked: for example a dose that
// cannot lie by today behind a numbered dose 1 (DOSE_INVALID{minInterval},
// TestNoFloatingPlacementNamesLastByID). The first such reason is then given
// to the last floating dose, with its checks and number. The
// DOSE_POSSIBLY_INVALID reasons r gives the other doses of fl are kept on
// them, as they stay counted; the named dose drops its own (Q62). When no
// dose of fl fails, it is the first bad reason of r. Since S4b's rescue
// (issue #28 In scope 0b) a dose at position 1 past the start limit behind
// an uncounted unnumbered dated dose no longer reaches this branch: the
// placement in front of that dose rescues it.
func failedIn(r walkResult, fl []givenDose, isBad func(Reason) bool) []Reason {
	last := fl[len(fl)-1].id
	var own, other, possible, firstBad []Reason
	for _, x := range r.reasons {
		if isBad(x) && firstBad == nil {
			firstBad = []Reason{x}
		}
		if !slices.ContainsFunc(fl, func(g givenDose) bool { return g.id == x.Params.Dose }) {
			continue
		}
		switch {
		case isBad(x) && x.Params.Dose == last:
			own = append(own, x)
		case isBad(x):
			if other == nil {
				x.Params.Dose = last
				other = []Reason{x}
			}
		case x.Code == ReasonDosePossiblyInvalid && x.Params.Dose != last:
			possible = append(possible, x)
		}
	}
	switch {
	case own != nil:
		return append(own, possible...)
	case other != nil:
		return append(other, possible...)
	}
	return firstBad
}

// covered reports whether every day inside the dated ranges of pl has a
// placement of the floating doses that fits (floatingFit over that day's
// walk). A box of ranges where some placement fits at every end is
// covered, and one where every placement adds a DOSE_INVALID is not; any
// other box is split in two at the middle of its widest range. The walk
// order stays that of pl: imprecise ranges overlap no other range (an
// overlap is blocking unless numbers order it). budget counts the boxes
// checked; when it runs out the answer is false, which only keeps the
// conservative ⚪.
func (s seriesModel) covered(b birthEnd, today Date, pl placement, budget *int) bool {
	if *budget <= 0 {
		return false
	}
	*budget--
	w := s.walkWith(b, today, pl, nil)
	switch f, _, _ := s.floatingFit(w, b, today, pl); f {
	case fitEverywhere:
		return true
	case fitNowhere:
		return false
	}
	lo, hi, ok := splitWidest(pl)
	return ok && s.covered(b, today, lo, budget) && s.covered(b, today, hi, budget)
}

// splitWidest returns pl with its widest dated range (the first by walk
// order on a tie) cut into its first and second half; ok is false when every
// dated range is a single day.
func splitWidest(pl placement) (lo, hi placement, ok bool) {
	widest, width := -1, int32(0)
	for i, g := range pl.dated {
		if d := g.date.Hi.days - g.date.Lo.days; d > width {
			widest, width = i, d
		}
	}
	if widest < 0 {
		return pl, pl, false
	}
	r := pl.dated[widest].date
	mid := r.Lo.AddDays(int(width / 2))
	lo, hi = pl, pl
	lo.dated, hi.dated = slices.Clone(pl.dated), slices.Clone(pl.dated)
	lo.dated[widest].date = Range{Lo: r.Lo, Hi: mid}
	hi.dated[widest].date = Range{Lo: mid.AddDays(1), Hi: r.Hi}
	return lo, hi, true
}

// tail returns the dated dose before the undated slots at the end of the
// sequence (nil if there is none) and those slots.
func (w *walkResult) tail() (from *Range, run []item) {
	j := len(w.seq)
	for j > 0 && !w.seq[j-1].dose.dated {
		j--
	}
	if j > 0 {
		from = &w.seq[j-1].dose.date
	}
	return from, w.seq[j:]
}

// closeRun reports the maximum-age checks of a run of undated slots that a
// record after it closes (issue #15 In scope 4, Q56), each slot bounded by u
// (slotMax): a level-4 slot carries its own (DOSE_AFTER_MAX_AGE, not
// counted, or DOSE_POSSIBLY_INVALID), and the implied slots' checks are
// returned for the record that carries them, as a chain failure of the run
// is (S2). An implied slot stays counted, and a forced level-4 slot counts
// with its failures demoted (doseVerdict.demoted).
func (w *walkResult) closeRun(s seriesModel, b birthEnd, pl placement, run []item, u Range) (carry doseVerdict) {
	for i := range run {
		it := &run[i]
		it.maxDef, it.maxPos = s.slotMax(b, *it, u)
		if it.dose.id == "" {
			carry.addMax(it.maxDef, it.maxPos)
			continue
		}
		var dv doseVerdict
		dv.addMax(it.maxDef, it.maxPos)
		forced := pl.forced(it.dose.id, it.pos)
		if forced {
			dv = dv.demoted()
		}
		w.reasons = append(w.reasons, dv.reasons(it.dose.id, it.pos)...)
		it.counted = dv.counted() || forced
	}
	return carry
}

// chainSkipping returns the best chain date of undated slot it from the
// chain that skips the uncounted unnumbered dated doses before it (issue #28
// In scope 0c, Q87): it starts at the dated dose before them that is
// counted or numbered, or at birth, and runs through the undated slots in
// between. skipped is false when no such dose is skipped (the chain is the
// slot's own); ok is false when the chain is unbounded.
func (w *walkResult) chainSkipping(s seriesModel, b birthEnd, it item, first *Range) (best Date, ok, skipped bool) {
	j := len(w.seq)
	var run []item
	for ; j > 0; j-- {
		x := w.seq[j-1]
		if x.dose.dated && (x.counted || x.dose.number > 0) {
			break
		}
		if x.dose.dated {
			skipped = true
		} else {
			run = append(run, x)
		}
	}
	if !skipped {
		return Date{}, false, false
	}
	var from *Range
	if j > 0 {
		from = &w.seq[j-1].dose.date
	}
	slices.Reverse(run)
	best, _, ok = s.chain(b, from, append(run, it), first, true)
	return best, ok, true
}

// gapBound is the definite end of the minimum interval of the last trailing
// undated slot, from the gaps before the uncounted doses that it may lie in
// (issue #35, Q98, Q99): best is the earliest predecessor chain end over
// the gaps that fit, and open is set when such a chain is unbounded. Unset,
// the walked threshold stands.
type gapBound struct {
	set, open bool
	best      Date
}

// lower returns the definite minimum-interval threshold def of a dose on x,
// lowered to max(best, best + minInterval − tol) (best alone without a
// minimum interval), tol the dose rule's tolerance in days (issue #29 In
// scope 6). An open bound never fails x definitely.
func (gb gapBound) lower(x Range, def Date, minInterval optSpan, tol int) Date {
	switch {
	case !gb.set:
		return def
	case gb.open:
		return minDate(def, x.Hi)
	}
	end := gb.best
	if minInterval.ok {
		end = maxDate(end, end.AddSpan(minInterval.span).AddDays(-tol))
	}
	return minDate(def, end)
}

// gapBound returns the gap bound of last, the last trailing undated slot at
// position n = last.pos (issue #35, Q98, Q99 (a)). Before the trailing run
// the walk has the uncounted unnumbered dated doses U1 < … < Um (undated
// slots between them skipped), back to P, the latest dated dose that is
// counted or numbered, or birth. They hold no position in the walk, so last
// may lie before them. For k < m, gap k lies after Uk (P, or birth, for k =
// 0) and before U(k+1). With q_k Uk's step-2 position (P's, or 0), a gap
// with q_k ≥ n is not tried: dated there, last would contradict the
// numbers. Otherwise positions q_k + 1 … n − 1 lie between Uk and last
// (undated slots or implied doses), chained from Uk under the most lenient
// plan per position, as chain does; gap 0 chains through position 1, so
// its dose 1 is that position at its earliest dates, as the walk takes an
// undated dose 1. The gap fits when last's earliest date there is before
// U(k+1).Hi, or on it when last walks first on that day. order is the
// number of the record a day can be given to: last's own number, or for an
// implied last the number N of its carrier beyond the series (0 for an
// unnumbered last). Dated on U(k+1)'s day, that record walks before U(k+1)
// only when U(k+1)'s step-2 position q_(k+1) is above order (orderSameDay),
// and otherwise measures its chain from U(k+1), which is gap k + 1. An
// unnumbered last has no such order and fits on the day (the safe side).
// Gap m, behind Um, is the walk's own chain, which check measures. The
// result is unset when no gap fits or m = 0. It reads only the walk, two
// chains per gap, and q_k grows with k, so at most n − 1 gaps are chained.
func (w *walkResult) gapBound(s seriesModel, b birthEnd, first *Range, last item, order int) gapBound {
	p, us := w.beforeTrailing()
	var gb gapBound
	if len(us) == 0 {
		return gb
	}
	var from *Range
	q := 0
	if p != nil {
		from, q = &p.dose.date, p.step
	}
	for k, next := range us {
		if k > 0 {
			from, q = &us[k-1].dose.date, us[k-1].step
		}
		if q >= last.pos {
			break
		}
		run := make([]item, 0, last.pos-q)
		for p := q + 1; p < last.pos; p++ {
			run = append(run, item{pos: p})
		}
		// Gap 0 chains through position 1, so its dose 1 is that position
		// at its earliest dates (EQ20b), not the walk's.
		f := first
		if q == 0 {
			f = nil
			if lo, hi, ok := s.chain(b, nil, []item{{pos: 1}}, nil, true); ok {
				f = &Range{Lo: lo, Hi: hi}
			}
		}
		if end, _, ok := s.chain(b, from, append(slices.Clip(run), last), f, true); ok &&
			(end.After(next.dose.date.Hi) || end == next.dose.date.Hi && order > 0 && next.step < order) {
			continue
		}
		pred, _, ok := s.chain(b, from, run, f, true)
		switch {
		case !ok:
			gb.set, gb.open = true, true
		case !gb.set || pred.Before(gb.best):
			gb.set, gb.best = true, pred
		}
	}
	return gb
}

// beforeTrailing scans back from the last item of the walk (the last
// trailing undated slot) over the undated slots and the uncounted
// unnumbered dated doses to P, the latest dated dose that is counted or
// numbered (nil when there is none). It returns P and those uncounted
// dated doses U1 < … < Um, in time order.
func (w *walkResult) beforeTrailing() (p *item, us []item) {
	seq := w.seq[:len(w.seq)-1]
	j := len(seq)
	for ; j > 0; j-- {
		x := seq[j-1]
		if !x.dose.dated {
			continue
		}
		if x.counted || x.dose.number > 0 {
			break
		}
		us = append(us, x)
	}
	slices.Reverse(us)
	if j > 0 {
		p = &seq[j-1]
	}
	return p, us
}

// envelope returns, for the last trailing fixed slot last, a copy of s
// whose plans are s's plus those of every version of s that could apply to
// it (issue #29 In scope 3, Q105 (b), F05.10): the version applies to
// last's product (an implied slot has none, so c's group only, Q94) and to
// its position (Q96), and its segment meets [P.Lo, today], P the latest
// counted or numbered dated dose before the run (beforeTrailing), else
// [b_old, today], else (−∞, today]. Every placement of last lies after P,
// since the gap search searches only the gaps after P. There is no bracket
// test: last has no date to take an age from, and leaving it out only adds
// plans. ok is false when no version is added.
func (w *walkResult) envelope(s seriesModel, b birthEnd, today Date, last item) (env seriesModel, ok bool) {
	if len(s.versions) == 0 {
		return s, false
	}
	var lo Date
	bounded := true
	switch p, _ := w.beforeTrailing(); {
	case p != nil:
		lo = p.dose.date.Lo
	case b.known:
		lo = b.older()
	default:
		bounded = false
	}
	env = s
	for _, v := range s.versions {
		if !v.applies(last.dose.product) || !v.lastDose(last.pos, s.length) {
			continue
		}
		if v.seg.hasFrom && v.seg.from.After(today) || bounded && v.seg.hasTo && !lo.Before(v.seg.to) {
			continue
		}
		if !ok {
			env.plans, ok = slices.Clip(s.plans), true
		}
		env.plans = append(env.plans, v.s.plans...)
	}
	return env, ok
}

// forced reports whether the walk counts dated dose or level-4 slot id at
// position pos whatever its checks (issue #28 In scope 0e): a rescued
// dose, a record after it (forceLater), or every record in the counted
// view. Once the rescue is settled, a rescued dose is pinned to its
// position (walkResult.pinned): the floating fit then judges it afresh at
// any other position, so that a placement that moves it where it fails
// does not fit. A record forced by forceLater is not pinned: the rescuing
// walk may put it beyond the series, which the fit, placing only the
// floating doses the series still needs, does not reach.
func (pl placement) forced(id string, pos int) bool {
	if pl.forceAll {
		return true
	}
	p, ok := pl.force[id]
	return ok && (p == 0 || p == pos)
}

// pinned returns force with the records in pin pinned to their positions
// in w.
func (w walkResult) pinned(force, pin map[string]int) map[string]int {
	out := maps.Clone(force)
	for _, it := range w.seq {
		if _, ok := pin[it.dose.id]; ok && it.dose.id != "" {
			out[it.dose.id] = it.pos
		}
	}
	return out
}

// demoted returns the verdict of a forced record: a definite failure becomes
// possible, so the record counts with one DOSE_POSSIBLY_INVALID listing the
// same checks and its own possible ones (issue #28 In scope 0b, Q83).
func (d doseVerdict) demoted() doseVerdict {
	if d.minV == verdictDefinite {
		d.minV = verdictPossible
	}
	d.maxDef, d.maxPos = nil, unionChecks(d.maxPos, d.maxDef)
	return d
}

// checkTrailing checks that the undated slots after the last dated dose can
// lie on or before today. The last of them is checked like a dose given
// today (check), with the slots before it as its run: when even its
// earliest placement is after today it is DOSE_INVALID (not counted), and
// when only the placement from the latest end of the ranges is, it is
// DOSE_POSSIBLY_INVALID. Its checks name the minimums that push it past
// today; a failure of the chain before it is minInterval.
//
// The chain runs from the dated dose before the run. When that dose is
// uncounted and unnumbered, it holds no position, so a last slot that is a
// fixed slot (pl.fixed, not an inserted floating dose) may lie before it
// (issue #35, Q98, Q99): when its minimum interval fails definitely, the
// definite threshold is lowered to the gaps that fit (gapBound), so the
// slot is DOSE_INVALID only when it fails in every gap and behind the
// uncounted doses, and is otherwise DOSE_POSSIBLY_INVALID and counted
// (walkResult.lowered). Its minimum age, maximum ages and carrier are
// unchanged. A forced slot and the counted view skip the search.
//
// Since S4c, a last fixed slot that is still definite is checked again on
// the envelope of its series' versions (envelope, issue #29 Q105 (b)), the
// check and the gap search alike: a day in an older segment may clear it
// although today's rule fails it on every day (F05.10). When the envelope
// does not fail it definitely, it is DOSE_POSSIBLY_INVALID and counted
// like a slot the gap search demoted. noGapSearch turns this off too.
//
// When the last trailing slot is an implied dose, the record whose number
// implies it is a level-4 dose beyond the series (an in-series or dated
// record would follow it). The lowest such record carries the reason, with
// its number, and the implied doses stay counted.
//
// The maximum ages of the trailing slots are reported as closeRun does,
// except that the last slot's record merges its own and the implied slots'
// into one verdict with its by-today check, so DOSE_INVALID comes before
// DOSE_AFTER_MAX_AGE and one DOSE_POSSIBLY_INVALID lists every possible
// check (Q62). When that record is forced, the verdict is demoted.
func (w *walkResult) checkTrailing(s seriesModel, b birthEnd, today Date, first *Range, pl placement) {
	from, run := w.tail()
	if len(run) == 0 {
		return
	}
	u := Range{Lo: today, Hi: today}
	carry := w.closeRun(s, b, pl, run[:len(run)-1], u)
	lastSlot := &w.seq[len(w.seq)-1]
	lastSlot.maxDef, lastSlot.maxPos = s.slotMax(b, *lastSlot, u)
	carry.addMax(lastSlot.maxDef, lastSlot.maxPos)
	carrier, number := lastSlot.dose.id, lastSlot.pos
	if carrier == "" {
		if len(pl.beyond) == 0 {
			return
		}
		carrier, number = pl.beyond[0].id, pl.beyond[0].number
	}
	carry.minV, carry.minChecks = s.check(lastSlot.pos, b, u, from, run[:len(run)-1], first, gapBound{})
	forced := lastSlot.dose.id != "" && pl.forced(lastSlot.dose.id, lastSlot.pos)
	if carry.minV == verdictDefinite && !forced && !pl.forceAll && !pl.noGapSearch {
		if g, ok := pl.fixed[lastSlot.pos]; ok && g.id == lastSlot.dose.id {
			order := lastSlot.dose.number
			if lastSlot.dose.id == "" {
				order = number // the carrier beyond the series
			}
			// demote takes the slot's verdict once it is not definite. Only
			// a slot that now counts is demoted for 0b (Q103): one that a
			// maximum age still fails definitely keeps that verdict, in the
			// rescue too.
			demote := func(v verdict, checks []Check) {
				carry.minV, carry.minChecks = v, checks
				if carry.counted() {
					if w.lowered == nil {
						w.lowered = map[string]bool{}
					}
					w.lowered[carrier] = true
				}
			}
			if gb := w.gapBound(s, b, first, *lastSlot, order); gb.set {
				if v, checks := s.check(lastSlot.pos, b, u, from, run[:len(run)-1], first, gb); v != verdictDefinite {
					demote(v, checks)
				}
			}
			// The envelope (Q105 (b)): still definite, the check and the gap
			// search are rerun with the versions' plans added. Not definite
			// there, the slot is possible, with the envelope's checks (c's
			// when the envelope passes it): never OK, and no date moves.
			if carry.minV == verdictDefinite {
				if env, ok := w.envelope(s, b, today, *lastSlot); ok {
					gb := w.gapBound(env, b, first, *lastSlot, order)
					if v, checks := env.check(lastSlot.pos, b, u, from, run[:len(run)-1], first, gb); v != verdictDefinite {
						if v == verdictOK {
							checks = carry.minChecks
						}
						demote(verdictPossible, checks)
					}
				}
			}
		}
	}
	if forced {
		carry = carry.demoted()
	}
	w.reasons = append(w.reasons, carry.reasons(carrier, number)...)
	if lastSlot.dose.id != "" && !carry.counted() && !forced {
		lastSlot.counted = false
	}
}

// hasInterval reports whether some plan has a minimum interval for position
// pos, so that an undated predecessor leaves something unverified.
func (s seriesModel) hasInterval(pos int) bool {
	return slices.ContainsFunc(s.plans, func(p planModel) bool { return p.doses[pos-1].minInterval.ok })
}

// check returns the verdict of a dose on range x for position pos at birth
// end b, and the checks it fails. from is the dated dose before it in time
// (nil if none) and run the undated slots between them; first is the counted
// dated dose at position 1, or the earliest dates of an undated dose 1
// (nil if neither). gb lowers the definite end of the minimum interval of
// the last trailing undated slot to the gaps before the uncounted dated
// doses that it may lie in (gapBound, issue #35); the possible end stays
// the one from from. A dated dose passes the zero gapBound.
//
// Each minimum threshold is (anchor + span) − tol, tol the plan's dose rule
// tolerance in days, at the definite and the possible end alike; behind
// undated slots it is never before the tolerant chain date itself (issue
// #29 In scope 6, Q79). With no tolerance this is S4a's check.
//
// A plan's verdict is the worst of its checks; the dose's verdict is the
// best over plans (A7). The checks reported are those that reach that
// verdict in the plans that have it, merged in catalog order: a
// DOSE_INVALID lists the checks failed at every end, not one failed at one
// end only. For exact dates this is the S1 check.
func (s seriesModel) check(pos int, b birthEnd, x Range, from *Range, run []item, first *Range, gb gapBound) (verdict, []Check) {
	return s.checkWith(pos, b, x, s.predecessor(b, from, run, first), first, gb)
}

// predecessor is what a dose's minimum interval is measured from: the dated
// dose before it in time (from, nil if none) and, behind a run of undated
// slots, the chain's earliest dates for the last of them (best from from.Lo,
// worst from from.Hi; chained false when the chain is unbounded).
type predecessor struct {
	from                  *Range
	run, chained          bool
	chainBest, chainWorst Date
}

// predecessor returns the predecessor of a dose behind run under s, with
// the tolerant chain (issue #29 Q90).
func (s seriesModel) predecessor(b birthEnd, from *Range, run []item, first *Range) predecessor {
	p := predecessor{from: from, run: len(run) > 0}
	if p.run {
		p.chainBest, p.chainWorst, p.chained = s.chain(b, from, run, first, true)
	}
	return p
}

// checkWith is check with the predecessor given: a version of another
// segment is judged against today's chain dates (issue #29 In scope 3,
// P1).
func (s seriesModel) checkWith(pos int, b birthEnd, x Range, pd predecessor, first *Range, gb gapBound) (verdict, []Check) {
	from, chainBest, chainWorst, chained := pd.from, pd.chainBest, pd.chainWorst, pd.chained
	best := verdictDefinite + 1
	var failed [3]bool // minAge, minInterval, minIntervalFromFirst
	for _, p := range s.plans {
		r := p.doses[pos-1]
		t := r.tol
		var vs [3]verdict
		if b.known && r.minAge.ok {
			lim := b.date.AddSpan(r.minAge.span).AddDays(-t)
			vs[0] = against(x, lim, lim)
		}
		switch {
		case !pd.run && from != nil && r.minInterval.ok:
			vs[1] = against(x, gb.lower(x, from.Lo.AddSpan(r.minInterval.span).AddDays(-t), r.minInterval, t), from.Hi.AddSpan(r.minInterval.span).AddDays(-t))
		case chained:
			lo, hi := chainBest, chainWorst
			if r.minInterval.ok {
				lo, hi = maxDate(lo, lo.AddSpan(r.minInterval.span).AddDays(-t)), maxDate(hi, hi.AddSpan(r.minInterval.span).AddDays(-t))
			}
			vs[1] = against(x, gb.lower(x, lo, r.minInterval, t), hi)
		}
		if first != nil && r.minFromFirst.ok {
			vs[2] = against(x, first.Lo.AddSpan(r.minFromFirst.span).AddDays(-t), first.Hi.AddSpan(r.minFromFirst.span).AddDays(-t))
		}
		v := max(vs[0], vs[1], vs[2])
		if v < best {
			best, failed = v, [3]bool{}
		}
		if v == best {
			for i, c := range vs {
				failed[i] = failed[i] || c >= best
			}
		}
	}
	if best == verdictOK {
		return verdictOK, nil
	}
	var checks []Check
	for i, c := range []Check{CheckMinAge, CheckMinInterval, CheckMinIntervalFromFirst} {
		if failed[i] {
			checks = append(checks, c)
		}
	}
	return best, checks
}

// chain returns the earliest dates the last of a run of undated slots can
// have (EQ8, EQ20b): best starting from from.Lo, worst from from.Hi. Each
// slot q is on or after the slot before it, and takes the latest of the
// previous slot + minInterval_q, birth + minAge_q (birth known) and dose 1 +
// minIntervalFromFirst_q (first set), under the most lenient plan for
// that slot. A run at the start of the series has no lower bound but the
// birth date. bounded is false when nothing bounds the last slot (no birth
// date and no dated dose before the run), so any placement is feasible.
//
// With tolerant set, each slot's minInterval, minAge and
// minIntervalFromFirst terms are lowered by that slot's tolerance, as a
// dated dose's checks are (check): the chain then says where a recorded
// dose could lie, so every step-5 use passes true (issue #29 In scope 6,
// Q79, Q90). Being on or after the slot before is an order, not a minimum,
// and stays. C_c,k (lastAllowed) computes the next dose's last day and
// passes false (EQ27).
func (s seriesModel) chain(b birthEnd, from *Range, run []item, first *Range, tolerant bool) (best, worst Date, bounded bool) {
	if from != nil {
		best, worst, bounded = from.Lo, from.Hi, true
	}
	for _, it := range run {
		var nb, nw Date
		set, unbounded := false, false
		for _, p := range s.plans {
			r := p.doses[it.pos-1]
			t := 0
			if tolerant {
				t = r.tol
			}
			lo, hi, ok := best, worst, bounded
			if ok && r.minInterval.ok {
				lo, hi = maxDate(lo, lo.AddSpan(r.minInterval.span).AddDays(-t)), maxDate(hi, hi.AddSpan(r.minInterval.span).AddDays(-t))
			}
			if b.known && r.minAge.ok {
				a := b.date.AddSpan(r.minAge.span).AddDays(-t)
				lo, hi = later(lo, a, ok), later(hi, a, ok)
				ok = true
			}
			if first != nil && r.minFromFirst.ok {
				f1, f2 := first.Lo.AddSpan(r.minFromFirst.span).AddDays(-t), first.Hi.AddSpan(r.minFromFirst.span).AddDays(-t)
				lo, hi = later(lo, f1, ok), later(hi, f2, ok)
				ok = true
			}
			if !ok {
				unbounded = true
				break
			}
			if !set || lo.Before(nb) {
				nb = lo
			}
			if !set || hi.Before(nw) {
				nw = hi
			}
			set = true
		}
		if unbounded {
			best, worst, bounded = Date{}, Date{}, false
			continue
		}
		best, worst, bounded = nb, nw, true
	}
	return best, worst, bounded
}

// later returns max(a, b), or b when a is not set.
func later(a, b Date, aSet bool) Date {
	if !aSet {
		return b
	}
	return maxDate(a, b)
}
