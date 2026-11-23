package ruleset

// maxAges checks a series' maxAgeStart and maxAgeComplete (SAD §5.2, issue
// #15, Q60), at path p (the series), in document order: the maxAgeStart
// problems, then the maxAgeComplete problems.
//
//   - A zero Span (Go-built only; JSON "P0D" decodes to a valid zero span)
//     is REQUIRED, and an invalid one INVALID_VALUE. Either makes the member
//     unusable, and the relational checks below skip it.
//   - Each member is compared with the dose-1 minAge of every plan, and
//     maxAgeStart also with startAge.min of a valid bracket: when some pair
//     is certainly not after (SpanAtMost(max, min)) the series can never
//     start, MAX_AGE_NOT_AFTER_MIN_AGE (Reject); otherwise, when some pair
//     is not certainly after (spanLess(min, max)), MAX_AGE_MAY_NOT_BE_AFTER_MIN_AGE
//     (Warn). At most one of the two per member.
//   - MAX_AGE_START_AFTER_COMPLETE (Warn, at maxAgeStart) when both are
//     usable and maxAgeStart is not certainly at most maxAgeComplete.
//   - SERIES_CANNOT_COMPLETE{id} (Warn, at maxAgeComplete) per plan whose
//     age at the last dose has a lower bound that maxAgeComplete is
//     certainly at or before (lastDoseBounds). It is skipped when
//     maxAgeComplete already has the Reject, which says more.
//
// The Reject takes only the certain case: the 31/28 rule cannot prove that
// P2M is after P8W, although two calendar months always are, so the
// uncertain band is a Warn (unlike START_AGE_EMPTY for brackets).
func (v *validator) maxAges(p string, sr Series) {
	startP, completeP := p+"/maxAgeStart", p+"/maxAgeComplete"
	start, startOK := v.maxAgeSpan(startP, sr.MaxAgeStart)
	// maxAgeComplete's own span problem is reported after maxAgeStart's
	// relational problems, in document order; only its usability is
	// needed before then.
	complete, completeOK := usableMaxAge(sr.MaxAgeComplete)

	var mins []Span // the dose-1 minimum ages of the plans that have a valid one
	for _, pl := range sr.Plans {
		if len(pl.Doses) > 0 && pl.Doses[0].MinAge != nil && pl.Doses[0].MinAge.valid() {
			mins = append(mins, *pl.Doses[0].MinAge)
		}
	}
	if startOK {
		pairs := mins
		if validBracket(sr.StartAge) {
			pairs = append(pairs[:len(pairs):len(pairs)], sr.StartAge.Min)
		}
		v.maxAgeAfter(startP, start, pairs)
		if completeOK && !SpanAtMost(start, complete) {
			v.warn(MaxAgeStartAfterComplete, startP, ProblemParams{})
		}
	}
	v.maxAgeSpan(completeP, sr.MaxAgeComplete)
	if !completeOK || !v.maxAgeAfter(completeP, complete, mins) {
		return
	}
	for _, pl := range sr.Plans {
		bounds, ok := lastDoseBounds(pl)
		if !ok {
			continue
		}
		for _, b := range bounds {
			if of(complete).atMost(b) {
				v.warn(SeriesCannotComplete, completeP, ProblemParams{ID: pl.ID})
				break
			}
		}
	}
}

// maxAgeSpan reports a zero or invalid maximum age and returns the span and
// whether it is present and usable.
func (v *validator) maxAgeSpan(path string, s *Span) (Span, bool) {
	switch {
	case s == nil:
	case *s == Span{}:
		v.reject(Required, path, ProblemParams{})
	case !s.valid():
		v.reject(InvalidValue, path, ProblemParams{Value: s.String()})
	}
	return usableMaxAge(s)
}

// usableMaxAge returns a maximum age and whether it is present, not zero and
// valid, without reporting anything.
func usableMaxAge(s *Span) (Span, bool) {
	if s == nil || *s == (Span{}) || !s.valid() {
		return Span{}, false
	}
	return *s, true
}

// maxAgeAfter compares a maximum age with the minimums it must be after and
// reports MAX_AGE_NOT_AFTER_MIN_AGE or MAX_AGE_MAY_NOT_BE_AFTER_MIN_AGE at
// path. It returns false when it reported the Reject.
func (v *validator) maxAgeAfter(path string, limit Span, mins []Span) bool {
	uncertain := false
	for _, m := range mins {
		switch {
		case SpanAtMost(limit, m):
			v.reject(MaxAgeNotAfterMinAge, path, ProblemParams{})
			return false
		case !spanLess(m, limit):
			uncertain = true
		}
	}
	if uncertain {
		v.warn(MaxAgeMayNotBeAfterMinAge, path, ProblemParams{})
	}
	return true
}

// lastDoseBounds returns lower bounds of the age at the last dose of plan pl,
// with the spans added as numbers (issue #15): for every dose j with a
// minAge, minAge_j + Σ minInterval_i (i = j+1…n), and for every dose j with
// a minIntervalFromFirst, minAge_1 + minIntervalFromFirst_j + Σ minInterval_i
// (i = j+1…n); a missing interval counts 0. A calendar chain of month
// additions can fall up to 3 days short of the sum (SAD §5.2, MinAgeImplied),
// so SERIES_CANNOT_COMPLETE is a Warn. ok is false when the plan has no
// doses, no dose-1 minAge, or a span that is not valid (it has its own
// problem).
func lastDoseBounds(pl Plan) (bounds []monthsDays, ok bool) {
	if len(pl.Doses) == 0 || pl.Doses[0].MinAge == nil {
		return nil, false
	}
	for _, d := range pl.Doses {
		for _, s := range []*Span{d.MinAge, d.MinInterval, d.MinIntervalFromFirst} {
			if s != nil && !s.valid() {
				return nil, false
			}
		}
	}
	n := len(pl.Doses)
	after := make([]monthsDays, n) // after[j]: Σ minInterval_i, i = j+1…n (0-based j)
	for j := n - 2; j >= 0; j-- {
		after[j] = after[j+1]
		after[j].add(pl.Doses[j+1].MinInterval)
	}
	first := of(*pl.Doses[0].MinAge)
	for j, d := range pl.Doses {
		if d.MinAge != nil {
			b := after[j]
			b.add(d.MinAge)
			bounds = append(bounds, b)
		}
		if d.MinIntervalFromFirst != nil {
			b := after[j]
			b.add(d.MinIntervalFromFirst)
			b.months += first.months
			b.days += first.days
			bounds = append(bounds, b)
		}
	}
	return bounds, true
}
