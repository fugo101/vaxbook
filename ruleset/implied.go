package ruleset

// MinAgeImplied reports whether dose k (1-based) of p has no minimum age, or
// one implied by dose 1's minimum age plus the minimum intervals of doses
// 2…k (SAD §5.2, EQ19):
//
//	minAge_k ≤ minAge_1 + Σ minInterval_j, j = 2…k (a missing interval counts 0)
//
// The engine needs it for a profile without a birth date: the minimum age of
// such a dose is then met by the intervals alone, so the dose can be
// scheduled without one (SAD §5.3 step 7). Dose 1 is always implied.
//
// Spans are compared per unit family. When every span involved is in
// months or years (1y = 12m), or every one is in days or weeks (1w = 7d),
// the comparison is exact in that unit. Otherwise it is conservative: a
// month of minAge_k counts 31 days and a month of the sum counts 28 days, so
// a mixed case is implied only if it holds for every month length.
//
// The formula adds the spans as numbers. The engine adds each span to its
// anchor in one step, and a chain of month additions that clamps at a month
// end can fall up to 3 days short of a single addition of the sum
// (2026-01-31 + 1m + 1m = 2026-03-28, + 2m = 2026-03-31). The SAD formula is
// kept as written; that difference is accepted.
func MinAgeImplied(p Plan, k int) bool {
	if k <= 1 || k > len(p.Doses) || p.Doses[k-1].MinAge == nil {
		return true
	}
	if p.Doses[0].MinAge == nil {
		return false // Validate rejects this plan (A7)
	}
	var lhs, rhs monthsDays
	lhs.add(p.Doses[k-1].MinAge)
	rhs.add(p.Doses[0].MinAge)
	for j := 1; j < k; j++ {
		rhs.add(p.Doses[j].MinInterval)
	}
	switch {
	case lhs.days == 0 && rhs.days == 0:
		return lhs.months <= rhs.months
	case lhs.months == 0 && rhs.months == 0:
		return lhs.days <= rhs.days
	}
	return lhs.months*31+lhs.days <= rhs.months*28+rhs.days
}

// monthsDays is a sum of spans split into its month family (months, years)
// and its day family (days, weeks).
type monthsDays struct{ months, days int }

func (m *monthsDays) add(s *Span) {
	if s == nil {
		return
	}
	switch s.Unit {
	case Days:
		m.days += s.N
	case Weeks:
		m.days += 7 * s.N
	case Months:
		m.months += s.N
	case Years:
		m.months += 12 * s.N
	}
}
