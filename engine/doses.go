package engine

import "slices"

// birthEnd is the birth date used by one evaluation of steps 5–9: one end of
// the birth-date range (SAD §5.3 step 4), or no birth date (known false,
// EQ19), in which case no age-based step runs.
type birthEnd struct {
	date  Date
	known bool
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
}

// walkResult is steps 5 and 6 at one birth end.
type walkResult struct {
	seq      []item // in time order; undated slots sit where their positions put them
	reasons  []Reason
	floating []givenDose
	k        int // next dose number: highest counted position + floating doses + 1
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
// slots (chain). The minimum age is checked when the birth date is known,
// and minIntervalFromFirst when dose 1 is a counted dated dose.
//
// Doses beyond the series length are not checked and not counted, but stay
// in the sequence as the dose adjacent in time (S1).
func (s seriesModel) walk(b birthEnd, today Date, pl placement) walkResult {
	w := walkResult{floating: pl.floating}
	last := 0 // the highest position reached
	taken := make(map[int]bool, len(pl.fixed))
	for q := range pl.fixed {
		taken[q] = true
	}
	insertSlots := func(below int) {
		for q := last + 1; q < below && q <= s.length; q++ {
			if g, ok := pl.fixed[q]; ok {
				w.seq = append(w.seq, item{pos: q, dose: g, counted: true})
				last = q
			}
		}
	}
	var first *Range // the counted dated dose at position 1
	for _, g := range pl.dated {
		pos := g.number
		if pos == 0 {
			pos = nextFree(last, taken)
		}
		insertSlots(pos)
		from, run := w.tail()
		it := item{pos: pos, dose: g}
		if pos > s.length {
			w.seq = append(w.seq, it)
			if g.number > 0 {
				last = max(last, pos)
			}
			continue
		}
		v, checks := s.check(pos, b, g.date, from, run, first)
		switch {
		case v == verdictDefinite:
			w.reasons = append(w.reasons, Reason{Code: ReasonDoseInvalid, Params: ReasonParams{Checks: checks, Dose: g.id, Number: pos}})
		case v == verdictPossible:
			w.reasons = append(w.reasons, Reason{Code: ReasonDosePossiblyInvalid, Params: ReasonParams{Checks: checks, Dose: g.id, Number: pos}})
			it.counted = true
		case len(run) > 0 && s.hasInterval(pos):
			w.reasons = append(w.reasons, Reason{Code: ReasonIntervalUnverifiable, Params: ReasonParams{Dose: g.id, Number: pos}})
			it.counted = true
		default:
			it.counted = true
		}
		if it.counted || g.number > 0 {
			last = max(last, pos)
		}
		if it.counted && pos == 1 {
			first = &g.date
		}
		w.seq = append(w.seq, it)
	}
	insertSlots(s.length + 1)
	w.checkTrailing(s, b, today, first)

	highest := 0
	for _, it := range w.seq {
		if it.counted && it.pos <= s.length {
			highest = max(highest, it.pos)
		}
	}
	w.k = highest + len(w.floating) + 1
	return w
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

// checkTrailing checks that the undated slots after the last dated dose can
// lie on or before today: when even their earliest placement is after today
// the last of them is DOSE_INVALID (not counted), and when only the
// placement from the latest end of the dated range before them is, it is
// DOSE_POSSIBLY_INVALID. The last trailing slot is always a record, because
// implied doses lie below a position some record takes.
func (w *walkResult) checkTrailing(s seriesModel, b birthEnd, today Date, first *Range) {
	from, run := w.tail()
	if len(run) == 0 {
		return
	}
	best, worst, bounded := s.chain(b, from, run, first)
	if !bounded {
		return
	}
	lastSlot := &w.seq[len(w.seq)-1]
	var v verdict
	switch {
	case best.After(today):
		v = verdictDefinite
	case worst.After(today):
		v = verdictPossible
	}
	if v == verdictOK || lastSlot.dose.id == "" {
		return
	}
	check := CheckMinInterval
	if b.known && s.ageBound(b, lastSlot.pos).After(today) {
		check = CheckMinAge
	}
	code := ReasonDosePossiblyInvalid
	if v == verdictDefinite {
		code = ReasonDoseInvalid
		lastSlot.counted = false
	}
	w.reasons = append(w.reasons, Reason{Code: code, Params: ReasonParams{Checks: []Check{check}, Dose: lastSlot.dose.id, Number: lastSlot.pos}})
}

// ageBound returns the earliest date the minimum age of position q allows
// under the most lenient plan, or the birth date when some plan has none.
func (s seriesModel) ageBound(b birthEnd, q int) Date {
	bound := Date{}
	for i, p := range s.plans {
		d := b.date
		if r := p.doses[q-1]; r.minAge.ok {
			d = b.date.AddSpan(r.minAge.span)
		}
		if i == 0 || d.Before(bound) {
			bound = d
		}
	}
	return bound
}

// hasInterval reports whether some plan has a minimum interval for position
// pos, so that an undated predecessor leaves something unverified.
func (s seriesModel) hasInterval(pos int) bool {
	return slices.ContainsFunc(s.plans, func(p planModel) bool { return p.doses[pos-1].minInterval.ok })
}

// check returns the verdict of a dose on range x for position pos at birth
// end b, and the checks it fails. from is the dated dose before it in time
// (nil if none) and run the undated slots between them; first is the counted
// dated dose at position 1 (nil if none).
//
// A plan's verdict is the worst of its checks; the dose's verdict is the
// best over plans (A7). The checks reported are those the plans with that
// verdict fail, merged in catalog order. For exact dates this is the S1
// check.
func (s seriesModel) check(pos int, b birthEnd, x Range, from *Range, run []item, first *Range) (verdict, []Check) {
	var chainBest, chainWorst Date
	chained := false
	if len(run) > 0 {
		chainBest, chainWorst, chained = s.chain(b, from, run, first)
	}
	best := verdictDefinite + 1
	var failed [3]bool // minAge, minInterval, minIntervalFromFirst
	for _, p := range s.plans {
		r := p.doses[pos-1]
		var vs [3]verdict
		if b.known && r.minAge.ok {
			lim := b.date.AddSpan(r.minAge.span)
			vs[0] = against(x, lim, lim)
		}
		switch {
		case len(run) == 0 && from != nil && r.minInterval.ok:
			vs[1] = against(x, from.Lo.AddSpan(r.minInterval.span), from.Hi.AddSpan(r.minInterval.span))
		case chained:
			lo, hi := chainBest, chainWorst
			if r.minInterval.ok {
				lo, hi = lo.AddSpan(r.minInterval.span), hi.AddSpan(r.minInterval.span)
			}
			vs[1] = against(x, lo, hi)
		}
		if first != nil && r.minFromFirst.ok {
			vs[2] = against(x, first.Lo.AddSpan(r.minFromFirst.span), first.Hi.AddSpan(r.minFromFirst.span))
		}
		v := max(vs[0], vs[1], vs[2])
		if v < best {
			best, failed = v, [3]bool{}
		}
		if v == best {
			for i, c := range vs {
				failed[i] = failed[i] || c != verdictOK
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
// minIntervalFromFirst_q (dose 1 dated), under the most lenient plan for
// that slot. A run at the start of the series has no lower bound but the
// birth date. bounded is false when nothing bounds the last slot (no birth
// date and no dated dose before the run), so any placement is feasible.
func (s seriesModel) chain(b birthEnd, from *Range, run []item, first *Range) (best, worst Date, bounded bool) {
	if from != nil {
		best, worst, bounded = from.Lo, from.Hi, true
	}
	for _, it := range run {
		var nb, nw Date
		set, unbounded := false, false
		for _, p := range s.plans {
			r := p.doses[it.pos-1]
			lo, hi, ok := best, worst, bounded
			if ok && r.minInterval.ok {
				lo, hi = lo.AddSpan(r.minInterval.span), hi.AddSpan(r.minInterval.span)
			}
			if b.known && r.minAge.ok {
				a := b.date.AddSpan(r.minAge.span)
				lo, hi = later(lo, a, ok), later(hi, a, ok)
				ok = true
			}
			if first != nil && r.minFromFirst.ok {
				f1, f2 := first.Lo.AddSpan(r.minFromFirst.span), first.Hi.AddSpan(r.minFromFirst.span)
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
