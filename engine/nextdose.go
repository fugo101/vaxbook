package engine

import (
	"slices"
	"strings"

	"github.com/fugo101/vaxbook/ruleset"
)

type planDates struct {
	earliest, due, overdue Date
}

// anchors are the dose dates step 7 measures from, all R.Hi of a today-capped
// range (SAD §5.3 step 7).
type anchors struct {
	latest   *Range // the last dated dose in time, counted or not
	previous *Range // dose k−1, when it is dated
	first    *Range // dose 1, or the dose standing in for it
}

// nextResult is step 7 at one birth end. When stop is set the next dose has
// no dates and reasons hold the blocking reason that stopped it.
type nextResult struct {
	next NextDose
	// plans are the dates of dose k in each plan, in rule-set order, before
	// the plans are merged into next: kept per candidate for the EQ15 cap
	// of S4 (issue #14 In scope 6). Empty when step 7 stops.
	plans   []planDates
	reasons []Reason
	stop    bool
	// first is the first-dose anchor the dates used (an.first, possibly the
	// EQ20c stand-in): C_c,k measures minIntervalFromFirst from it.
	first *Range
	// lows are each plan's earliest date of dose k from the earliest
	// possible anchors, parallel to plans: R.Lo of the latest dose and of
	// dose 1, with no dose-1 term when dose 1 is the EQ20c stand-in (the
	// real dose 1 is earlier by an unknown amount). They decide only
	// whether an out-of-age conclusion is certain (owner 2026-11-24).
	lows []Date
	// loose are the positions, ascending, whose exact date could move a
	// plan's earliest date earlier: the latest dated dose when some plan
	// has a minInterval and its range is not one day, and 1 when some plan
	// has a minIntervalFromFirst and dose 1 is the stand-in or its range is
	// not one day (OUT_OF_AGE_UNCERTAIN's numbers).
	loose []int
}

// next computes next dose k for every plan and merges the plans (SAD §5.3
// step 7, EQ6 merge 1): the earliest and due dates come from the plan with
// the earliest due date (A7), the overdue date from the latest plan. Plans
// with the same due date resolve to the latest earliest date (P1).
//
// It stops with a blocking reason instead of dates when:
//   - dose k−1 is undated, or an unnumbered level-4 dose may be the latest
//     dose: PREVIOUS_DOSE_DATE_UNKNOWN for each such record (F04.2);
//   - there is no birth date and no dose is counted, or dose k's minimum
//     age is not implied by the intervals in some plan, or the series has
//     a maximum age to complete (A6, issue #15 Q59), or a needed dose
//     anchor is missing: NO_BIRTH_DATE (EQ19). The maximum age to start
//     and the bracket limit do not stop it: they apply to dose 1 only, and
//     with no birth date and doses given dose 1 is always counted (no age
//     is checked and dose 1 has no interval), so k = 1 does not arise;
//   - a needed dose anchor is missing although the birth date is known:
//     no reason of its own, because a DOSE_INVALID already blocks.
//
// When dose 1 is undated and a plan anchors dose k on the first dose or has
// a minIntervalFromFirst, the earliest counted dated dose stands in for it:
// FIRST_DOSE_DATE_ASSUMED (EQ20c).
func (s seriesModel) next(b birthEnd, w walkResult) nextResult {
	k := w.k
	var firstSlot, prevSlot, standIn *item
	for i := range w.seq {
		it := &w.seq[i]
		if !it.counted || it.pos > s.length {
			continue
		}
		if firstSlot == nil || it.pos < firstSlot.pos {
			firstSlot = it
		}
		if prevSlot == nil || it.pos > prevSlot.pos {
			prevSlot = it
		}
		if standIn == nil && it.dose.dated {
			standIn = it
		}
	}

	var unknown []string
	for _, g := range w.floating {
		unknown = append(unknown, g.id)
	}
	if prevSlot != nil && !prevSlot.dose.dated && prevSlot.dose.id != "" {
		unknown = append(unknown, prevSlot.dose.id)
	}
	if len(unknown) > 0 {
		slices.SortFunc(unknown, strings.Compare)
		var r nextResult
		for _, id := range slices.Compact(unknown) {
			r.reasons = append(r.reasons, Reason{Code: ReasonPreviousDoseDateUnknown, Params: ReasonParams{Dose: id}})
		}
		r.stop = true
		return r
	}
	noBirth := nextResult{reasons: []Reason{{Code: ReasonNoBirthDate}}, stop: true}
	if !b.known && (prevSlot == nil || s.maxComplete.ok || slices.ContainsFunc(s.plans, func(p planModel) bool { return !p.doses[k-1].minAgeImplied })) {
		return noBirth
	}

	var an anchors
	var latestPos int
	for i := len(w.seq) - 1; i >= 0; i-- {
		if w.seq[i].dose.dated {
			an.latest, latestPos = &w.seq[i].dose.date, w.seq[i].pos
			break
		}
	}
	if prevSlot != nil && prevSlot.dose.dated {
		an.previous = &prevSlot.dose.date
	}
	var r nextResult
	if firstSlot != nil {
		switch {
		case firstSlot.dose.dated:
			an.first = &firstSlot.dose.date
		case standIn != nil:
			an.first = &standIn.dose.date
			if slices.ContainsFunc(s.plans, func(p planModel) bool {
				d := p.doses[k-1]
				return d.from == ruleset.FromFirstDose || d.minFromFirst.ok
			}) {
				r.reasons = append(r.reasons, Reason{Code: ReasonFirstDoseDateAssumed, Params: ReasonParams{Dose: standIn.dose.id}})
			}
		}
	}

	// The earliest possible anchors: R.Lo of the latest dose and of dose 1,
	// and no dose-1 anchor when it is the stand-in.
	lowLatest, lowFirst := an.latest.lower(), (*Range)(nil)
	if an.first != nil && firstSlot.dose.dated {
		lowFirst = an.first.lower()
	}
	looseLatest, looseFirst := false, false
	for _, p := range s.plans {
		d := p.doses[k-1]
		pd, ok := d.dates(b, an)
		if !ok && !b.known {
			return noBirth
		}
		if !ok {
			// A first-dose or previous-dose anchor is missing although the
			// birth date is known: every counted dose it could come from is
			// implied, which happens only when the record that implies them
			// is not counted (DOSE_INVALID). That reason already makes the
			// assessment ⚪, so step 7 stops without a reason of its own.
			return nextResult{stop: true}
		}
		r.plans = append(r.plans, pd)
		low, _ := d.earliest(b, lowLatest, lowFirst)
		r.lows = append(r.lows, low)
		looseLatest = looseLatest || d.minInterval.ok && an.latest != nil && an.latest.Lo != an.latest.Hi
		looseFirst = looseFirst || d.minFromFirst.ok && an.first != nil && (lowFirst == nil || an.first.Lo != an.first.Hi)
	}
	if looseFirst {
		r.loose = append(r.loose, 1)
	}
	if looseLatest {
		r.loose = append(r.loose, latestPos)
	}
	slices.Sort(r.loose)
	r.loose = slices.Compact(r.loose)
	r.first = an.first
	r.next, _ = choosePlans(k, r.plans, func(planDates) bool { return true })
	return r
}

// lower is the range's earlier end as a one-day range, or nil for nil.
func (x *Range) lower() *Range {
	if x == nil {
		return nil
	}
	return &Range{Lo: x.Lo, Hi: x.Lo}
}

// choosePlans merges the plans that keep accepts into next dose k (A7, EQ6
// merge 1): the earliest and due dates from the plan with the earliest due
// date, the latest earliest date on a tie (P1), and the overdue date from
// the latest plan. It also returns how many plans it kept; S4b drops the
// plans past a maximum age before the choice (Q88).
func choosePlans(k int, plans []planDates, keep func(planDates) bool) (NextDose, int) {
	var best planDates
	var overdue Date
	n := 0
	for _, pd := range plans {
		if !keep(pd) {
			continue
		}
		if n == 0 || pd.due.Before(best.due) || (pd.due == best.due && pd.earliest.After(best.earliest)) {
			best = pd
		}
		if n == 0 || pd.overdue.After(overdue) {
			overdue = pd.overdue
		}
		n++
	}
	return NextDose{Number: k, Earliest: best.earliest, Due: best.due, Overdue: overdue}, n
}

// dates computes one plan's dates for its dose rule r:
//
//	earliest = max(birth + minAge, latest dose in time + minInterval,
//	               dose 1 + minIntervalFromFirst)
//	due      = max(earliest, anchor + offset)
//	overdue  = due + max(buffer, OverdueGraceDays days)
//
// Dose dates are the later (today-capped) end of their ranges. The latest
// dose in time is used even when it was not counted, so a dose given on the
// earliest date always passes the interval check. Each span is added to its
// anchor in one step.
//
// Without a birth date (EQ19) there is no minimum-age term, the earliest
// date is at least the latest dose, and a birth-anchored milestone becomes
// dose k−1 + (offset_k − offset_{k−1}), or the earliest date when dose k−1
// is not birth-anchored in the same unit family. ok is false when a needed
// anchor is missing.
func (r doseRule) dates(b birthEnd, an anchors) (planDates, bool) {
	earliest, ok := r.earliest(b, an.latest, an.first)
	if !ok {
		return planDates{}, false
	}
	due := earliest
	switch r.from {
	case ruleset.FromFirstDose:
		if an.first == nil {
			return planDates{}, false
		}
		due = maxDate(earliest, an.first.Hi.AddSpan(r.offset))
	case ruleset.FromPreviousDose:
		if an.previous == nil {
			return planDates{}, false
		}
		due = maxDate(earliest, an.previous.Hi.AddSpan(r.offset))
	default: // ruleset.FromBirth; ruleset.Validate rejects any other anchor
		switch {
		case b.known:
			due = maxDate(earliest, b.date.AddSpan(r.offset))
		case r.birthGap.ok && an.previous != nil:
			due = maxDate(earliest, an.previous.Hi.AddSpan(r.birthGap.span))
		}
	}
	overdue := due.AddDays(OverdueGraceDays)
	if r.buffer.ok {
		overdue = maxDate(overdue, due.AddSpan(r.buffer.span))
	}
	return planDates{earliest: earliest, due: due, overdue: overdue}, true
}

// earliest is the earliest date of dates, from the R.Hi of the given latest
// and first anchors (each nil when absent): max(birth + minAge, latest +
// minInterval, first + minIntervalFromFirst), and with no birth date at
// least latest. ok is false when no term applies.
func (r doseRule) earliest(b birthEnd, latest, first *Range) (Date, bool) {
	var earliest Date
	set := false
	raise := func(d Date) {
		if !set || d.After(earliest) {
			earliest, set = d, true
		}
	}
	if b.known {
		raise(b.date)
		if r.minAge.ok {
			raise(b.date.AddSpan(r.minAge.span))
		}
	} else if latest != nil {
		raise(latest.Hi)
	}
	if r.minInterval.ok && latest != nil {
		raise(latest.Hi.AddSpan(r.minInterval.span))
	}
	if r.minFromFirst.ok && first != nil {
		raise(first.Hi.AddSpan(r.minFromFirst.span))
	}
	return earliest, set
}
