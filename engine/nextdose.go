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
//     age is not implied by the intervals in some plan, or a needed dose
//     anchor is missing: NO_BIRTH_DATE (EQ19);
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
	if !b.known && (prevSlot == nil || slices.ContainsFunc(s.plans, func(p planModel) bool { return !p.doses[k-1].minAgeImplied })) {
		return noBirth
	}

	var an anchors
	for i := len(w.seq) - 1; i >= 0; i-- {
		if w.seq[i].dose.dated {
			an.latest = &w.seq[i].dose.date
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

	var best planDates
	var overdue Date
	for i, p := range s.plans {
		pd, ok := p.doses[k-1].dates(b, an)
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
		if i == 0 || pd.due.Before(best.due) || (pd.due == best.due && pd.earliest.After(best.earliest)) {
			best = pd
		}
		overdue = maxDate(overdue, pd.overdue)
	}
	r.next = NextDose{Number: k, Earliest: best.earliest, Due: best.due, Overdue: overdue}
	return r
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
	} else if an.latest != nil {
		raise(an.latest.Hi)
	}
	if r.minInterval.ok && an.latest != nil {
		raise(an.latest.Hi.AddSpan(r.minInterval.span))
	}
	if r.minFromFirst.ok && an.first != nil {
		raise(an.first.Hi.AddSpan(r.minFromFirst.span))
	}
	if !set {
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
