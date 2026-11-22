package engine

import (
	"cmp"
	"slices"
	"strings"
)

// candidates returns the candidate series of a disease (SAD §5.3 step 3),
// in rule-set order. They depend on the dose products only, never on the
// birth date.
//
// With doses given, each record contributes the tier of its product: a
// non-retired catalog product that lists the disease gives its own series,
// else the generic series, else every series; any other record (a retired
// product, an off-catalog product, no product, or a product that does not
// list the disease) gives every series (A2, EQ14, F11.3, Q36). Implied
// doses are unknown-product doses (Q30). The candidates are the union, so
// one record's specific series never hides another record's fallback.
//
// With no dose given: every series but those of retired products, or every
// series when none is left (Q38).
func (dz diseaseModel) candidates(doses []givenDose, implied bool) []int {
	if len(doses) == 0 {
		return dz.noDose
	}
	if implied {
		return dz.all
	}
	in := make([]bool, len(dz.series))
	for _, g := range doses {
		tier, ok := dz.tier[g.product]
		if !ok {
			return dz.all
		}
		for _, i := range tier {
			in[i] = true
		}
	}
	var out []int
	for i, ok := range in {
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// productsFor returns the non-retired products for a next dose of the
// given candidate series, in snapshot order: a product series offers its
// product unless it is retired, and a generic series every non-retired
// product of the disease with no series of its own. Empty is nil.
func (dz diseaseModel) productsFor(series []int) []string {
	offered := map[string]bool{}
	for _, c := range series {
		switch s := dz.series[c]; {
		case s.product == "":
			for _, p := range dz.genericProducts {
				offered[p] = true
			}
		case !s.retired:
			offered[s.product] = true
		}
	}
	var out []string
	for _, p := range dz.catalog {
		if offered[p] {
			out = append(out, p)
		}
	}
	return out
}

// evaluation is steps 5–9 of one disease over its candidate series, ready to
// run at a birth end.
type evaluation struct {
	dz    diseaseModel
	cands []int       // the candidate series, in rule-set order
	pls   []placement // the placement in each candidate
	today Date
	// free are the IDs of the disease's records with an off-catalog
	// product, by ID: PRODUCT_NOT_IN_CATALOG when complete (EQ14).
	free []string
	// safest is SAFEST_SERIES as a bucket reason of each end, set when no
	// dose is given and two or more candidates remain.
	safest *Reason
}

// candidateResult is steps 5–7 of one candidate series at one birth end.
// The merge reads it, and it stays whole in endResult for the slices that
// merge more (S4: the EQ15 cap over the candidates at k).
type candidateResult struct {
	walk     walkResult
	complete bool       // every dose of the series counted (step 6)
	next     nextResult // step 7, when not complete
}

// run runs steps 5–7 of series s at one birth end.
func (s seriesModel) run(b birthEnd, today Date, pl placement) candidateResult {
	w := s.walk(b, today, pl)
	r := candidateResult{walk: w, complete: w.k > s.length}
	if !r.complete {
		r.next = s.next(b, w)
	}
	return r
}

// at runs steps 5–9 at one birth end, per candidate series, and merges the
// candidates (SAD §5.3 steps 5–7, EQ6 item 2, F05.3, A1). Step 10 has one
// layer in S3a: any blocking reason makes the result ⚪ (layer 7, data ⚪),
// and the masked bucket's reasons and dates stay.
//
//   - Dose validity is per candidate, and every candidate's DOSE_INVALID,
//     DOSE_POSSIBLY_INVALID and INTERVAL_UNVERIFIABLE are kept (Q31).
//   - Every candidate complete → ✅ COMPLETE, with PRODUCT_NOT_IN_CATALOG
//     when a record has an off-catalog product. A complete candidate never
//     makes the result complete on its own.
//   - Otherwise k is the smallest next dose number among the candidates
//     still needing a dose, and only the candidates at k are merged: when
//     any of them cannot date dose k, step 7 stops with their reasons; else
//     earliest, due and overdue each take their latest value over them,
//     each with alternative plans adds ALTERNATIVE_PLANS (Q28), and the
//     bucket is classified once from the merged dates.
//
// Reasons of one end, in order: SAFEST_SERIES (no dose given); the walk
// reasons by dose in walk order, then by candidate; COMPLETE and
// PRODUCT_NOT_IN_CATALOG; or PREVIOUS_DOSE_DATE_UNKNOWN, NO_BIRTH_DATE,
// FIRST_DOSE_DATE_ASSUMED, ALTERNATIVE_PLANS and ON_TRACK. For a single
// candidate this is the S2 result.
func (ev evaluation) at(b birthEnd) endResult {
	rs := make([]candidateResult, len(ev.cands))
	for i, c := range ev.cands {
		rs[i] = ev.dz.series[c].run(b, ev.today, ev.pls[i])
	}
	res := endResult{cands: rs}
	if ev.safest != nil {
		res.reasons = append(res.reasons, *ev.safest)
	}
	res.reasons = append(res.reasons, ev.walkReasons(rs)...)

	// Step 6: complete when every candidate is.
	var atK []int // indexes into rs of the candidates at k
	for i, r := range rs {
		switch {
		case r.complete:
			res.k = max(res.k, r.walk.k)
		case atK == nil || r.walk.k < rs[atK[0]].walk.k:
			atK = []int{i}
		case r.walk.k == rs[atK[0]].walk.k:
			atK = append(atK, i)
		}
	}
	if atK == nil {
		res.complete, res.bucket = true, BucketComplete
		res.reasons = append(res.reasons, Reason{Code: ReasonComplete})
		if len(ev.free) > 0 {
			res.reasons = append(res.reasons, Reason{Code: ReasonProductNotInCatalog, Params: ReasonParams{Doses: ev.free}})
		}
		if blocking(res.reasons) {
			res.bucket = BucketUnknown
		}
		return res
	}
	k := rs[atK[0]].walk.k
	res.k = k

	// Step 7: next dose k. A candidate at k that cannot date it stops the
	// merge: a ⚪ assessment does not emit dates (SAD §5.1), and the other
	// candidates' dates alone could be unsafe for it.
	var unknown []string
	noBirth, stop := false, false
	for _, i := range atK {
		n := rs[i].next
		if !n.stop {
			continue
		}
		stop = true
		for _, r := range n.reasons {
			switch r.Code {
			case ReasonPreviousDoseDateUnknown:
				unknown = append(unknown, r.Params.Dose)
			case ReasonNoBirthDate:
				noBirth = true
			}
		}
	}
	if stop {
		slices.Sort(unknown)
		for _, id := range slices.Compact(unknown) {
			res.reasons = append(res.reasons, Reason{Code: ReasonPreviousDoseDateUnknown, Params: ReasonParams{Dose: id}})
		}
		if noBirth {
			res.reasons = append(res.reasons, Reason{Code: ReasonNoBirthDate})
		}
		res.bucket = BucketUnknown
		return res
	}
	listed := map[string]bool{}
	var merged NextDose
	var series []int
	for j, i := range atK {
		n := rs[i].next
		for _, r := range n.reasons { // FIRST_DOSE_DATE_ASSUMED
			if key := reasonKey(r); !listed[key] {
				listed[key] = true
				res.reasons = append(res.reasons, r)
			}
		}
		if j == 0 {
			merged = n.next
		} else {
			merged.Earliest = maxDate(merged.Earliest, n.next.Earliest)
			merged.Due = maxDate(merged.Due, n.next.Due)
			merged.Overdue = maxDate(merged.Overdue, n.next.Overdue)
		}
		series = append(series, ev.cands[i])
	}
	for _, c := range series {
		if s := ev.dz.series[c]; len(s.plans) > 1 {
			res.reasons = append(res.reasons, Reason{Code: ReasonAlternativePlans, Params: ReasonParams{Series: s.id}})
		}
	}

	// Step 9: bucket; step 10: a blocking reason masks it.
	bucket, block, reason := classify(ev.today, merged, k > 1)
	if reason != "" {
		res.reasons = append(res.reasons, Reason{Code: reason})
	}
	res.next, res.hasNext = merged, true
	res.products = ev.dz.productsFor(series)
	if blocking(res.reasons) {
		res.bucket = BucketUnknown
		return res
	}
	res.bucket, res.block = bucket, block
	return res
}

// walkReasons merges the step-5 reasons of the candidates: by dose in walk
// order (the dated doses in placement order, which every candidate shares,
// then the undated records), then by candidate in rule-set order, then in
// the candidate's own order, without repeats (A8). One candidate's reasons
// are already in that order.
func (ev evaluation) walkReasons(rs []candidateResult) []Reason {
	dated := ev.pls[0].dated
	rank := make(map[string]int, len(dated))
	for i, g := range dated {
		rank[g.id] = i
	}
	type entry struct {
		rank, cand, idx int
		r               Reason
	}
	var all []entry
	for c, r := range rs {
		for i, x := range r.walk.reasons {
			rk, ok := rank[x.Params.Dose]
			if !ok {
				rk = len(dated)
			}
			all = append(all, entry{rk, c, i, x})
		}
	}
	slices.SortFunc(all, func(x, y entry) int {
		return cmp.Or(cmp.Compare(x.rank, y.rank), cmp.Compare(x.cand, y.cand), cmp.Compare(x.idx, y.idx))
	})
	var out []Reason
	listed := map[string]bool{}
	for _, e := range all {
		if key := reasonKey(e.r); !listed[key] {
			listed[key] = true
			out = append(out, e.r)
		}
	}
	return out
}

// candidateKey encodes candidates for reasonKey.
func candidateKey(cs []SeriesRef) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Product + "/" + c.Series
	}
	return strings.Join(parts, ",")
}
