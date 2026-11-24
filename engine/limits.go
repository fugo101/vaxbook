package engine

import "slices"

// The maximum-age limits of the next dose (SAD §5.3 steps 3, 4 and 7,
// F05.4a, F05.4b, EQ15; issue #28). Every limit is a last allowed day: one
// day before the exclusive limit date of S4a's checks.
//
// At an evaluated birth date b, for a candidate series c needing dose k:
//
//	L_c       = b_old + maxAgeComplete − 1 day
//	S_c       = min(b_old + maxAgeStart, b_s + the group's last bracket max) − 1 day
//	Limit_c,k = L_c, and min(L_c, S_c) when k = 1
//	C_c,k     = the latest day for dose k from which doses k+1…n, each on its
//	            earliest date, still end by L_c (lastAllowed)
//	L_c,k     = min(C_c,k, S_c when k = 1)
//
// b_old is the older end of the whole birth range and b_s the older end of
// the evaluated sub-range (Q65, Q73; birthEnd). The same values with b in
// place of b_old and b_s (Limit^b, L^b), with F_c^lo from the earliest
// possible dose dates (ownFLo), decide only whether an out-of-age
// conclusion is certain at b.

// boundKind is the state of a limit: absent (no limit), impossible (no day
// works) or a date.
type boundKind uint8

const (
	boundAbsent boundKind = iota
	boundImpossible
	boundDate
)

// bound is a last allowed day with an explicit state, since a zero Date
// cannot tell "no limit" from "no day works". The zero value is absent.
type bound struct {
	kind boundKind
	day  Date // set for boundDate
}

func dateBound(d Date) bound { return bound{kind: boundDate, day: d} }

// present reports whether the bound limits anything.
func (x bound) present() bool { return x.kind != boundAbsent }

// min is the stricter of two bounds: impossible wins, absent is the
// identity, otherwise the earlier day.
func (x bound) min(y bound) bound {
	switch {
	case x.kind == boundImpossible || y.kind == boundAbsent:
		return x
	case y.kind == boundImpossible || x.kind == boundAbsent:
		return y
	case y.day.Before(x.day):
		return y
	}
	return x
}

// exceededBy reports whether a dose on d is past the bound.
func (x bound) exceededBy(d Date) bool {
	switch x.kind {
	case boundImpossible:
		return true
	case boundDate:
		return d.After(x.day)
	}
	return false
}

// limits are the maximum-age limits of a candidate at its next dose k
// (issue #28 In scope 9: kept in endResult.cands for later slices).
type limits struct {
	lc, lcOwn   bound // L_c from b_old, and at b (EQ11)
	lim, limOwn bound // Limit_c,k from b_old and b_s, and at b
	lk          bound // L_c,k; absent when step 7 stops
}

// completeBound is L_c for a person born on bo: absent without
// maxAgeComplete.
func (s seriesModel) completeBound(bo Date) bound {
	if !s.maxComplete.ok {
		return bound{}
	}
	return dateBound(bo.AddSpan(s.maxComplete.span).AddDays(-1))
}

// startBound is S_c: dose 1's start limit with maxAgeStart counted from
// ageBirth and the group's last bracket max from bracketBirth, less one day.
func (s seriesModel) startBound(ageBirth, bracketBirth Date) bound {
	lim, ok := s.startLimitAt(ageBirth, bracketBirth)
	if !ok {
		return bound{}
	}
	return dateBound(lim.AddDays(-1))
}

// limitsAt computes the limits of series s for next dose k at birth date b.
// first is the first-dose anchor step 7 used (nil if none); stopped is set
// when step 7 stopped, so C_c,k is not needed.
func (s seriesModel) limitsAt(k int, b birthEnd, first *Range, stopped bool) limits {
	if !b.known || !s.hasMaxAges() {
		return limits{}
	}
	l := limits{lc: s.completeBound(b.older()), lcOwn: s.completeBound(b.date)}
	l.lim, l.limOwn = l.lc, l.lcOwn
	var sc bound
	if k == 1 {
		sc = s.startBound(b.older(), b.subStart())
		l.lim, l.limOwn = l.lim.min(sc), l.limOwn.min(s.startBound(b.date, b.date))
	}
	if !stopped {
		l.lk = s.lastAllowed(k, b, first, l.lc).min(sc)
	}
	return l
}

// lastAllowed is C_c,k (Q70): the latest day d for dose k such that doses
// k+1…n, each on max(previous + minInterval, dose 1 + minIntervalFromFirst,
// b_late + minAge) under the most lenient plan for that dose (chain, as in
// step 5, A7), end on or before L_c. Dose 1 is d when k = 1, else first,
// step 7's first-dose anchor at its R.Hi. It is absent when L_c is, and L_c
// when k is the last dose.
//
// The chain end never decreases as d grows (AddSpan is monotone, month-end
// clamping included), so d is found by bisection over [b_old, L_c]. When
// even b_old fails, C_c,k is impossible: any day before b_old is before
// today, so it compares with F like impossible does.
func (s seriesModel) lastAllowed(k int, b birthEnd, first *Range, lc bound) bound {
	if lc.kind != boundDate || k >= s.length {
		return lc
	}
	late := birthEnd{date: b.later(), known: true}
	run := make([]item, 0, s.length-k)
	for j := k + 1; j <= s.length; j++ {
		run = append(run, item{pos: j})
	}
	var anchor *Range
	if first != nil {
		anchor = &Range{Lo: first.Hi, Hi: first.Hi}
	}
	fits := func(d Date) bool {
		dk := Range{Lo: d, Hi: d}
		f := anchor
		if k == 1 {
			f = &dk
		}
		best, _, bounded := s.chain(late, &dk, run, f)
		return !bounded || !best.After(lc.day)
	}
	lo, hi := b.older(), lc.day
	switch {
	case !fits(lo):
		return bound{kind: boundImpossible}
	case fits(hi):
		return lc
	}
	for lo.AddDays(1).Before(hi) {
		mid := lo.AddDays(int(hi.days-lo.days) / 2)
		if fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return dateBound(lo)
}

// ownF is the candidate's own F_c (Q86): today, or the earliest date of its
// most lenient plan when later.
func (r candidateResult) ownF(today Date) Date {
	f := today
	for i, p := range r.next.plans {
		if i == 0 || p.earliest.Before(f) {
			f = p.earliest
		}
	}
	return maxDate(today, f)
}

// ownFLo is the candidate's own F_c^lo (owner 2026-11-24): today, or the
// smallest earliest date of its plans from the earliest possible anchors
// (nextResult.lows) when later. It equals ownF when every dose date
// involved is one day and dose 1 is not the EQ20c stand-in.
func (r candidateResult) ownFLo(today Date) Date {
	f := today
	for i, d := range r.next.lows {
		if i == 0 || d.Before(f) {
			f = d
		}
	}
	return maxDate(today, f)
}

// kept returns next dose k from the candidate's plans whose own F is not
// past Limit_c,k (Q88), with A7's choice made over them, and how many plans
// are kept.
func (r candidateResult) kept(today Date) (NextDose, int) {
	return choosePlans(r.next.next.Number, r.next.plans, func(p planDates) bool {
		return !r.limits.lim.exceededBy(maxDate(today, p.earliest))
	})
}

// outOf reports whether an incomplete candidate is out of age, and
// certainly so at the evaluated date (issue #28 In scope 2): out when its
// step 7 runs and its own F_c is past Limit_c,k (Q86, Q88: every plan is
// dropped), or step 7 stopped on PREVIOUS_DOSE_DATE_UNKNOWN and today is
// past L_c (EQ11; never without maxAgeComplete, Q75). Certain when it is
// out even at the young end and from the earliest possible dose dates:
// F_c^lo past Limit^b_c,k (owner 2026-11-24), or for EQ11 today past L^b_c,
// which involves no dose date. A stop on NO_BIRTH_DATE or a missing anchor
// is never out.
func (r candidateResult) outOf(today Date) (out, certain bool) {
	switch {
	case !r.next.stop:
		return r.limits.lim.exceededBy(r.ownF(today)), r.limits.limOwn.exceededBy(r.ownFLo(today))
	case hasReason(r.next.reasons, ReasonPreviousDoseDateUnknown):
		return r.limits.lc.exceededBy(today), r.limits.lcOwn.exceededBy(today)
	}
	return false, false
}

// uncertainCauses reports, for a candidate that is out but not certainly
// out, what its certainty waits on: the birth date (out from F_c^lo, or not
// out at b from F_c) and the dose dates (out at b from F_c, or not out from
// F_c^lo). With X = Limit^b exceeded by F_c and Y = Limit exceeded by
// F_c^lo, birth = ¬X ∨ Y and dose = X ∨ ¬Y: one alone when only that input
// is imprecise, both when neither alone or each alone would decide. An
// EQ11 stop involves no dose date: birth only.
func (r candidateResult) uncertainCauses(today Date) (birth, dose bool) {
	if r.next.stop {
		return true, false
	}
	x := r.limits.limOwn.exceededBy(r.ownF(today))
	y := r.limits.lim.exceededBy(r.ownFLo(today))
	return !x || y, x || !y
}

// deciding are the positions, ascending, whose exact date could decide
// whether a candidate that is out but not certainly out can still have its
// next dose (OUT_OF_AGE_UNCERTAIN's numbers, owner 2026-11-24). A position
// decides through a plan whose earliest date from the earliest possible
// anchors (F^lo = max(today, nextResult.lows)) is not past Limit^b_c,k, so
// that the plan could still be in age, and whose term from that position's
// latest possible date (planTerms) is above F^lo and past Limit_c,k, so
// that the term alone could put it out: with the other dates at their
// earliest, that position's date then moves the plan across a limit the
// birth range allows. A term that never exceeds the limit, or that the
// other terms dominate, cannot decide, and an exact date never does (its
// term is at most F^lo). Every position that can decide is listed: some
// plan must be in age at one of its dates and out at another.
func (r candidateResult) deciding(today Date) []int {
	var out []int
	for i, lo := range r.next.lows {
		lo = maxDate(today, lo)
		if r.limits.limOwn.exceededBy(lo) {
			continue
		}
		t := r.next.terms[i]
		if t.hasFirst && t.first.After(lo) && r.limits.lim.exceededBy(t.first) {
			out = append(out, 1)
		}
		if t.hasLatest && t.latest.After(lo) && r.limits.lim.exceededBy(t.latest) {
			out = append(out, r.next.latestPos)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// hasReason reports whether reasons hold one with code c.
func hasReason(reasons []Reason, c ReasonCode) bool {
	for _, r := range reasons {
		if r.Code == c {
			return true
		}
	}
	return false
}
