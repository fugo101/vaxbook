package engine

import (
	"cmp"
	"slices"
	"strings"
)

// candidates returns the tier union of a disease (SAD §5.3 step 3), in
// rule-set order: the candidate series before start-age brackets, which
// byAgeAtDoseOne (doses given) or noDoseAt (no dose, per birth end) then
// choose from within each product group. It depends on the dose products
// only, never on the birth date.
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
// run at a birth date. With doses given both birth ends share one
// evaluation; with no dose given each birth date step 4 evaluates has its
// own, since the brackets can differ.
type evaluation struct {
	dz    diseaseModel
	cands []int       // the candidate series, in rule-set order
	pls   []placement // the placement in each candidate
	today Date
	// free are the IDs of the disease's records with an off-catalog
	// product, by ID: PRODUCT_NOT_IN_CATALOG when complete (EQ14).
	free []string
	// safest is SAFEST_SERIES as a bucket reason of each birth date, set
	// when no dose is given and two or more candidates remain.
	safest *Reason
	// unknownProduct: some dose of the disease, implied doses included, has
	// an unknown product (anyUnknownProduct). It chooses CHECK_PRODUCT_NAME
	// over MIXED_PRODUCTS when the candidates disagree on a maximum age.
	unknownProduct bool
}

// evaluate prepares steps 5–9 over cands: each candidate's placement, and
// with perEndSafest SAFEST_SERIES as a bucket reason when two or more
// candidates remain (no dose given).
func (dz diseaseModel) evaluate(arr placement, cands []int, today Date, free []string, perEndSafest bool) evaluation {
	ev := evaluation{dz: dz, cands: cands, today: today, free: free}
	for _, c := range cands {
		ev.pls = append(ev.pls, arr.forLength(dz.series[c].length))
	}
	if perEndSafest && len(cands) > 1 {
		ev.safest = &Reason{Code: ReasonSafestSeries, Params: ReasonParams{Candidates: dz.refs(cands)}}
	}
	return ev
}

// candidateResult is steps 5–7 of one candidate series at one birth end:
// complete or not, k and per-dose validity (walk), and the per-plan dates
// of dose k (next.plans). The merge reads it, and it stays whole in
// endResult for the slices that merge more (S4: the EQ15 cap over the
// candidates at k, issue #14 In scope 6).
type candidateResult struct {
	walk     walkResult
	complete bool       // every dose of the series counted (step 6)
	next     nextResult // step 7, when not complete
	// limits are the maximum-age limits of next dose k, when not complete
	// (issue #28; zero without a maximum age or a birth date).
	limits limits
}

// run runs steps 5–7 of series s at one birth end.
func (s seriesModel) run(b birthEnd, today Date, pl placement) candidateResult {
	w := s.walk(b, today, pl)
	r := candidateResult{walk: w, complete: w.k > s.length}
	if !r.complete {
		r.next = s.next(b, w)
		r.limits = s.limitsAt(w.k, b, r.next.first, r.next.stop)
	}
	return r
}

// at runs steps 5–9 at one birth end, per candidate series, and merges the
// candidates (SAD §5.3 steps 5–7, EQ6 item 2, F05.3, A1), then draws the
// maximum-age conclusions of the next dose (issue #28, conclude).
//
//   - Dose validity is per candidate, and every candidate's DOSE_INVALID,
//     DOSE_POSSIBLY_INVALID and INTERVAL_UNVERIFIABLE are kept (Q31), and
//     its DOSE_AFTER_MAX_AGE unless the candidates disagree on that record
//     (disagreements): then CHECK_PRODUCT_NAME or MIXED_PRODUCTS replaces
//     them (issue #15 In scope 5). Counting, k and the masked dates stay
//     per candidate.
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
// With no candidate (no dose given, and past every start-age bracket at b)
// the result is the outOfAge block with OUT_OF_AGE, k 0 and no series
// (issue #23, Q24).
//
// Reasons of one end, in order: SAFEST_SERIES (no dose given); the walk
// reasons by dose in walk order, then by candidate; CHECK_PRODUCT_NAME or
// MIXED_PRODUCTS; COMPLETE and
// PRODUCT_NOT_IN_CATALOG; or OUT_OF_AGE; or PREVIOUS_DOSE_DATE_UNKNOWN,
// NO_BIRTH_DATE, FIRST_DOSE_DATE_ASSUMED, ALTERNATIVE_PLANS,
// DUE_CAPPED_BY_MAX_AGE or SERIES_MAY_NOT_COMPLETE, and ON_TRACK. For a
// single candidate this is the S2 result.
func (ev evaluation) at(b birthEnd) endResult {
	if len(ev.cands) == 0 {
		return endResult{block: BlockOutOfAge, reasons: []Reason{{Code: ReasonOutOfAge}}}
	}
	rs := make([]candidateResult, len(ev.cands))
	for i, c := range ev.cands {
		rs[i] = ev.dz.series[c].run(b, ev.today, ev.pls[i])
	}
	res, step7At, atK := ev.asWalked(rs)
	if res.complete {
		return res
	}
	return ev.conclude(b, rs, res, step7At, atK)
}

// asWalked is the S3a/S4a result of the walks (the as-walked view, issue
// #28): the reasons, and the merged and masked dates. step7At is where the
// step-7 reasons start in res.reasons, and atK the indexes into rs of the
// candidates at k (nil when every candidate is complete).
func (ev evaluation) asWalked(rs []candidateResult) (res endResult, step7At int, atK []int) {
	res = endResult{cands: rs, series: ev.cands}
	if ev.safest != nil {
		res.reasons = append(res.reasons, *ev.safest)
	}
	drop := ev.disagreements(rs)
	res.reasons = append(res.reasons, ev.walkReasons(rs, drop)...)
	if len(drop) > 0 {
		for d := range drop {
			res.disagreeing = append(res.disagreeing, d)
		}
		slices.Sort(res.disagreeing)
		res.reasons = append(res.reasons, Reason{Code: ev.productCode()})
	}
	step7At = len(res.reasons)

	// Step 6: complete when every candidate is.
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
		return res, step7At, nil
	}
	res.k = rs[atK[0]].walk.k

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
		return res, step7At, atK
	}
	ev.merge(&res, rs, atK, false)
	return res, step7At, atK
}

// merge merges next dose k over the candidates at k into res (SAD §5.3 step
// 7, EQ6 merge 2): earliest, due and overdue each take their latest value,
// with FIRST_DOSE_DATE_ASSUMED and ALTERNATIVE_PLANS for each candidate
// with two or more plans; then step 9 classifies the merged dates and step
// 10's blocking mask applies.
//
// With capped set (issue #28 In scope 3, the EQ15 cap) each candidate's
// dates come from its plans not past Limit_c,k (Q88), ALTERNATIVE_PLANS is
// given only for two or more such plans, and the merged dates are capped:
// with F = max(today, merged earliest), L_k the smallest L_c,k and Lim the
// smallest Limit_c,k over the candidates at k,
//   - F ≤ L_k: due and overdue are lowered to L_k where later, with
//     DUE_CAPPED_BY_MAX_AGE{lastDay: L_k} when either is lowered;
//   - L_k < F ≤ Lim: due and overdue are lowered to Lim where later, with
//     SERIES_MAY_NOT_COMPLETE{lastDay: Lim} (Q66).
//
// A capped dose is never 🔴 (step 9, neverRed); the earliest date is never
// changed, and since F is on or before the cap day due stays on or after
// it. The cap reason goes after ALTERNATIVE_PLANS and before ON_TRACK.
func (ev evaluation) merge(res *endResult, rs []candidateResult, atK []int, capped bool) {
	listed := map[string]bool{}
	var merged NextDose
	var series []int
	var alternatives []Reason
	var lk, lim bound
	for j, i := range atK {
		r := rs[i]
		for _, x := range r.next.reasons { // FIRST_DOSE_DATE_ASSUMED
			if key := reasonKey(x); !listed[key] {
				listed[key] = true
				res.reasons = append(res.reasons, x)
			}
		}
		nd, plans := r.next.next, len(ev.dz.series[ev.cands[i]].plans)
		if capped {
			nd, plans = r.kept(ev.today)
			lk, lim = lk.min(r.limits.lk), lim.min(r.limits.lim)
		}
		if j == 0 {
			merged = nd
		} else {
			merged.Earliest = maxDate(merged.Earliest, nd.Earliest)
			merged.Due = maxDate(merged.Due, nd.Due)
			merged.Overdue = maxDate(merged.Overdue, nd.Overdue)
		}
		series = append(series, ev.cands[i])
		if plans > 1 {
			alternatives = append(alternatives, Reason{Code: ReasonAlternativePlans, Params: ReasonParams{Series: ev.dz.series[ev.cands[i]].id}})
		}
	}
	res.reasons = append(res.reasons, alternatives...)
	neverRed := false
	if capped {
		f := maxDate(ev.today, merged.Earliest)
		var code ReasonCode
		day := lk
		switch {
		case !lk.present():
		case !lk.exceededBy(f):
			code = ReasonDueCappedByMaxAge
		case !lim.exceededBy(f):
			code, day = ReasonSeriesMayNotComplete, lim
		}
		if code != "" {
			due, overdue := minDate(merged.Due, day.day), minDate(merged.Overdue, day.day)
			if code == ReasonSeriesMayNotComplete || due != merged.Due || overdue != merged.Overdue {
				merged.Due, merged.Overdue, neverRed = due, overdue, true
				res.reasons = append(res.reasons, Reason{Code: code, Params: ReasonParams{LastDay: day.day}})
			}
		}
	}

	// Step 9: bucket; step 10: a blocking reason masks it.
	bucket, block, reason := classify(ev.today, merged, res.k > 1, neverRed)
	if reason != "" {
		res.reasons = append(res.reasons, Reason{Code: reason})
	}
	res.next, res.hasNext = merged, true
	res.products = ev.dz.productsFor(series)
	if blocking(res.reasons) {
		res.bucket = BucketUnknown
		return
	}
	res.bucket, res.block = bucket, block
}

// productCode is the reason for candidates that disagree on a maximum age:
// CHECK_PRODUCT_NAME when some dose has an unknown product, else
// MIXED_PRODUCTS (F05.3; issue #15 In scope 6).
func (ev evaluation) productCode() ReasonCode {
	if ev.unknownProduct {
		return ReasonCheckProductName
	}
	return ReasonMixedProducts
}

// conclude draws the maximum-age conclusions of the next dose from res, the
// as-walked result, which is not complete (SAD §5.3 step 7 "Maximum age, no
// next dose possible", step 10 layer 5, F05.10 "Precedence", F05.11, A1,
// EQ11, EQ16, P1; issue #28 In scope 2, Q63, Q64, Q69, Q75, Q85, Q86).
//
// The conclusions come from the counted view: each candidate walked again
// with every record counted (countedView), the same walks as rs when no
// record is questioned. Each incomplete candidate is out or certainly out
// (candidateResult.outOf), at its own k.
//
//   - Layer 5: no candidate complete and every incomplete one certainly out
//     → outOfAge with OUT_OF_AGE, and every as-walked reason kept as a
//     masked reason: the reasons from steps 5–6 before OUT_OF_AGE, and when
//     the as-walked result is blocking its step-7 reasons after it.
//   - Data ⚪: otherwise a blocking as-walked result stays unchanged, with
//     its masked dates uncapped (a complete candidate in the counted view
//     keeps it too, F05.10).
//   - Clean result (the two views are the same): every incomplete
//     candidate out, none complete, not all certain → ⚪
//     BIRTH_DATE_IMPRECISE{fullBirthDate}; some out, or merged-out (its
//     step 7 runs and the merged F is past Limit_c,k) → ⚪
//     CHECK_PRODUCT_NAME or MIXED_PRODUCTS; otherwise the EQ15 cap (merge).
//     These conclusions emit no step-7 reason.
//
// S5 (issue #16) adds the other layers around this; its layer 6 compares
// the capped due date with historyStatusAt.
func (ev evaluation) conclude(b birthEnd, rs []candidateResult, res endResult, step7At int, atK []int) endResult {
	cv := rs
	if ev.questioned(rs) {
		cv = ev.countedView(b)
		res.counted = cv
	}
	anyComplete, allOut, allCertain, someOut := false, true, true, false
	for _, r := range cv {
		if r.complete {
			anyComplete = true
			continue
		}
		out, certain := r.outOf(ev.today)
		allOut, allCertain, someOut = allOut && out, allCertain && certain, someOut || out
	}
	head := slices.Clip(res.reasons[:step7At])
	conclusion := func(r endResult, reasons ...Reason) endResult {
		r.reasons = append(head, reasons...)
		r.next, r.hasNext, r.products = NextDose{}, false, nil
		return r
	}
	if !anyComplete && allOut && allCertain {
		out := conclusion(res, Reason{Code: ReasonOutOfAge})
		out.bucket, out.block, out.k = "", BlockOutOfAge, 0
		if blocking(res.reasons) {
			out.reasons = append(out.reasons, res.reasons[step7At:]...)
		}
		return out
	}
	if blocking(res.reasons) {
		return res
	}
	limited := someOut
	for _, i := range atK {
		limited = limited || rs[i].limits.lim.present()
	}
	if !limited {
		return res
	}
	unknown := res
	unknown.bucket, unknown.block = BucketUnknown, ""
	switch {
	case !anyComplete && allOut:
		return conclusion(unknown, Reason{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: HintFullBirthDate}})
	case someOut || ev.mergedOut(rs, atK):
		return conclusion(unknown, Reason{Code: ev.productCode()})
	}
	res.reasons = head
	ev.merge(&res, rs, atK, true)
	return res
}

// mergedOut reports whether a candidate at k is past its Limit_c,k at the
// merged F = max(today, the latest earliest date of the candidates at k,
// each from its plans not past its limit): it could be given only before
// the date the other candidates need (issue #28 In scope 2, Q86).
func (ev evaluation) mergedOut(rs []candidateResult, atK []int) bool {
	f := ev.today
	for _, i := range atK {
		nd, _ := rs[i].kept(ev.today)
		f = maxDate(f, nd.Earliest)
	}
	return slices.ContainsFunc(atK, func(i int) bool { return rs[i].limits.lim.exceededBy(f) })
}

// questioned reports whether some candidate does not count a record: it
// gives it DOSE_INVALID or DOSE_AFTER_MAX_AGE (issue #28, Q64).
func (ev evaluation) questioned(rs []candidateResult) bool {
	return slices.ContainsFunc(rs, func(r candidateResult) bool {
		return slices.ContainsFunc(r.walk.reasons, func(x Reason) bool {
			return x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge
		})
	})
}

// countedView runs steps 5–7 of each candidate with every record counted,
// at the position it reaches when all records count, and every floating
// dose counted, with no placement search (issue #28, Q85; walk with
// forceAll). It never derives n = 0 or out of age from a dose being not
// counted (SAD §5.3 step 5).
func (ev evaluation) countedView(b birthEnd) []candidateResult {
	cv := make([]candidateResult, len(ev.cands))
	for i, c := range ev.cands {
		pl := ev.pls[i]
		pl.forceAll = true
		cv[i] = ev.dz.series[c].run(b, ev.today, pl)
	}
	return cv
}

// walkReasons merges the step-5 reasons of the candidates: by dose in walk
// order, then by candidate in rule-set order, then in the candidate's own
// order, without repeats (A8). The dated doses come first, in placement
// order, which every candidate shares. The undated records after them are
// ordered by the position their reason names, which is where the walk puts
// them (a numbered or gap-filling level-4 dose has the same position in
// every candidate that reaches it), and then by ID. A reason on an undated
// record names it only after the last dated dose: the trailing slot that
// cannot lie by today, a level-4 dose beyond the series, or a floating dose.
// One candidate's reasons are already in that order; for one dose under one
// candidate the codes go DOSE_INVALID, DOSE_AFTER_MAX_AGE,
// DOSE_POSSIBLY_INVALID, INTERVAL_UNVERIFIABLE. A DOSE_AFTER_MAX_AGE on a
// record in drop is left out (disagreements).
func (ev evaluation) walkReasons(rs []candidateResult, drop map[string]bool) []Reason {
	dated := ev.pls[0].dated
	rank := make(map[string]int, len(dated))
	for i, g := range dated {
		rank[g.id] = i
	}
	type entry struct {
		rank, pos int
		id        string
		cand, idx int
		r         Reason
	}
	var all []entry
	for c, r := range rs {
		for i, x := range r.walk.reasons {
			if x.Code == ReasonDoseAfterMaxAge && drop[x.Params.Dose] {
				continue
			}
			e := entry{cand: c, idx: i, r: x}
			if rk, ok := rank[x.Params.Dose]; ok {
				e.rank = rk
			} else {
				e.rank, e.pos, e.id = len(dated), x.Params.Number, x.Params.Dose
			}
			all = append(all, e)
		}
	}
	slices.SortFunc(all, func(x, y entry) int {
		return cmp.Or(cmp.Compare(x.rank, y.rank), cmp.Compare(x.pos, y.pos), strings.Compare(x.id, y.id),
			cmp.Compare(x.cand, y.cand), cmp.Compare(doseCodeRank(x.r.Code), doseCodeRank(y.r.Code)), cmp.Compare(x.idx, y.idx))
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

// doseCodeRank orders the per-dose codes of one dose under one candidate.
func doseCodeRank(c ReasonCode) int {
	switch c {
	case ReasonDoseInvalid:
		return 0
	case ReasonDoseAfterMaxAge:
		return 1
	case ReasonDosePossiblyInvalid:
		return 2
	}
	return 3
}

// disagreements returns the records on which the candidates disagree on a
// maximum age (SAD §5.3 step 5, F05.3, F05.10, EQ16; issue #15 In scope 5,
// Q52): some candidate gives the record DOSE_AFTER_MAX_AGE and another
// accepts it. A single candidate never disagrees. Each candidate's
// acceptances are indexed once (acceptance), so the pass is linear in the
// reasons and records.
func (ev evaluation) disagreements(rs []candidateResult) map[string]bool {
	if len(rs) < 2 {
		return nil
	}
	acc := make([]map[string]bool, len(rs))
	for j := range rs {
		acc[j] = ev.acceptance(rs, j)
	}
	var drop map[string]bool
	for i, r := range rs {
		for _, x := range r.walk.reasons {
			d := x.Params.Dose
			if x.Code != ReasonDoseAfterMaxAge || drop[d] {
				continue
			}
			for j := range rs {
				if j != i && acc[j][d] {
					if drop == nil {
						drop = map[string]bool{}
					}
					drop[d] = true
					break
				}
			}
		}
	}
	return drop
}

// acceptance returns the records candidate j accepts: it counts them (a
// counted item at a position up to the series length, or a floating dose
// that fits; with no reason, INTERVAL_UNVERIFIABLE or DOSE_POSSIBLY_INVALID),
// or it is complete and leaves them unchecked beyond its length (a dated
// dose past the length or a numbered level-4 dose beyond the series). A
// candidate that gives a record DOSE_INVALID or DOSE_AFTER_MAX_AGE, or leaves
// it unchecked while incomplete, does not accept it (Q52). Only accepted
// records are in the map.
func (ev evaluation) acceptance(rs []candidateResult, j int) map[string]bool {
	r, length := rs[j], ev.dz.series[ev.cands[j]].length
	rejected := map[string]bool{}
	for _, x := range r.walk.reasons {
		if x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge {
			rejected[x.Params.Dose] = true
		}
	}
	acc := map[string]bool{}
	seen := map[string]bool{}
	accept := func(d string, ok bool) {
		if !seen[d] {
			seen[d] = true
			if ok && !rejected[d] {
				acc[d] = true
			}
		}
	}
	for _, g := range r.walk.floating {
		accept(g.id, true)
	}
	for _, it := range r.walk.seq {
		if it.dose.id == "" {
			continue
		}
		if it.pos > length {
			accept(it.dose.id, r.complete)
		} else {
			accept(it.dose.id, it.counted)
		}
	}
	for _, g := range ev.pls[j].beyond {
		accept(g.id, r.complete)
	}
	return acc
}

// anyUnknownProduct reports whether some dose of the disease has an unknown
// product, for the choice between CHECK_PRODUCT_NAME and MIXED_PRODUCTS
// (F05.3; issue #15 In scope 6, Q53, Q54). A dose is named when it has a
// catalog product that lists the disease, retired or not, or an off-catalog
// product (freeProduct); it is unknown with neither, with a catalog product
// that does not list the disease, or when it is implied (Q30). This is not
// S3a's candidate rule: a retired or off-catalog product is named here.
func (dz diseaseModel) anyUnknownProduct(doses []givenDose, implied bool) bool {
	return implied || slices.ContainsFunc(doses, func(g givenDose) bool {
		return !g.free && (g.product == "" || !dz.listing[g.product])
	})
}

// candidateKey encodes candidates for reasonKey.
func candidateKey(cs []SeriesRef) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Product + "/" + c.Series
	}
	return strings.Join(parts, ",")
}
