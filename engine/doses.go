package engine

// walk validates the doses in time order (SAD §5.3 step 5) and returns the
// counted doses and a DOSE_INVALID reason per dose that is not counted.
//
// A dose is checked for the next free position. It is valid when it meets
// the minimums of that position in at least one plan; the plan is never
// inferred (A7). The minimum interval is measured to the dose adjacent in
// time, counted or not, never to dose j−2 (A4). Doses past the end of the
// series are ignored in S1.
func (s seriesModel) walk(birth Date, doses []givenDose) (counted []givenDose, reasons []Reason) {
	var prev *givenDose // the dose before g in time, counted or not
	for i := range doses {
		g, before := doses[i], prev
		prev = &doses[i]
		pos := len(counted) + 1
		if pos > s.length {
			continue
		}
		failed := s.check(pos, birth, g.date, before)
		if len(failed) == 0 {
			counted = append(counted, g)
			continue
		}
		reasons = append(reasons, Reason{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: failed, Dose: g.id, Number: pos}})
	}
	return counted, reasons
}

// check returns nil when a dose on date meets the minimums of position pos in
// some plan, otherwise the union over plans of the failed checks, in catalog
// order.
//
// Only a definite violation fails: the latest possible dose date is before
// the minimum. For exact dates (S1) this is the plain comparison; minimums
// are inclusive.
func (s seriesModel) check(pos int, birth Date, date Range, prev *givenDose) []Check {
	var age, interval bool
	for _, p := range s.plans {
		r := p.doses[pos-1]
		failAge := r.minAge.ok && date.Hi.Before(birth.AddSpan(r.minAge.span))
		failInterval := r.minInterval.ok && prev != nil && date.Hi.Before(prev.date.Lo.AddSpan(r.minInterval.span))
		if !failAge && !failInterval {
			return nil
		}
		age = age || failAge
		interval = interval || failInterval
	}
	var failed []Check
	if age {
		failed = append(failed, CheckMinAge)
	}
	if interval {
		failed = append(failed, CheckMinInterval)
	}
	return failed
}
