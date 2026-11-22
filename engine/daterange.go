package engine

// Range is a closed date interval [Lo, Hi]. Every date inside the engine is a
// Range; an exact date is the Range with Lo == Hi. A month- or year-precision
// date is the Range of its period, capped at today (EQ7, EQ17).
type Range struct {
	Lo, Hi Date
}

// Exact returns the Range holding only d.
func Exact(d Date) Range { return Range{Lo: d, Hi: d} }

// IsExact reports whether r holds a single day.
func (r Range) IsExact() bool { return r.Lo == r.Hi }

func (r Range) String() string {
	if r.IsExact() {
		return r.Lo.String()
	}
	return "[" + r.Lo.String() + ", " + r.Hi.String() + "]"
}

// CapAt returns r with Hi lowered to today when it is later: [Lo, min(Hi,
// today)]. The caller guarantees Lo ≤ today.
func (r Range) CapAt(today Date) Range { return Range{Lo: r.Lo, Hi: minDate(r.Hi, today)} }

// Intersects reports whether r and o share at least one day.
func (r Range) Intersects(o Range) bool { return !r.Hi.Before(o.Lo) && !o.Hi.Before(r.Lo) }
