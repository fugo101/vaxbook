package engine

// Range is a closed date interval [Lo, Hi]. Every date inside the engine is a
// Range; an exact date is the Range with Lo == Hi. Imprecise dates (S2) widen
// it without changing the computations that consume it.
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
