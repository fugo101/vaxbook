package engine

import "github.com/fugo101/vaxbook/ruleset"

type planDates struct {
	earliest, due, overdue Date
}

// next computes next dose k = counted + 1 for every plan and merges the plans
// (SAD §5.3 step 7, EQ6 merge 1): the earliest and due dates come from the
// plan with the earliest due date (A7), the overdue date from the latest
// plan. Plans with the same due date resolve to the latest earliest date
// (SAD §5.3 step 7 merge 1, P1; golden alt-plans-due-tie-latest-earliest).
func (s seriesModel) next(birth Date, doses, counted []givenDose) NextDose {
	k := len(counted) + 1
	var best planDates
	var overdue Date
	for i, p := range s.plans {
		pd := p.doses[k-1].dates(birth, doses, counted)
		if i == 0 || pd.due.Before(best.due) || (pd.due == best.due && pd.earliest.After(best.earliest)) {
			best = pd
		}
		overdue = maxDate(overdue, pd.overdue)
	}
	return NextDose{Number: k, Earliest: best.earliest, Due: best.due, Overdue: overdue}
}

// dates computes one plan's dates for its dose rule r:
//
//	earliest = max(birth + minAge, latest dose in time + minInterval)
//	due      = max(earliest, anchor + offset)
//	overdue  = due + max(buffer, OverdueGraceDays days)
//
// The latest dose in time is used even when it was not counted, so a dose
// given on the earliest date always passes the interval check. Anchors use
// counted doses. Each span is added to its anchor in one step.
func (r doseRule) dates(birth Date, doses, counted []givenDose) planDates {
	earliest := birth
	if r.minAge.ok {
		earliest = maxDate(earliest, birth.AddSpan(r.minAge.span))
	}
	if r.minInterval.ok && len(doses) > 0 {
		earliest = maxDate(earliest, doses[len(doses)-1].date.Hi.AddSpan(r.minInterval.span))
	}
	var anchor Date
	switch r.from {
	case ruleset.FromFirstDose:
		anchor = counted[0].date.Hi
	case ruleset.FromPreviousDose:
		anchor = counted[len(counted)-1].date.Hi
	default: // ruleset.FromBirth; ruleset.Validate rejects any other anchor
		anchor = birth
	}
	due := maxDate(earliest, anchor.AddSpan(r.offset))
	overdue := due.AddDays(OverdueGraceDays)
	if r.buffer.ok {
		overdue = maxDate(overdue, due.AddSpan(r.buffer.span))
	}
	return planDates{earliest: earliest, due: due, overdue: overdue}
}
