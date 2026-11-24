package engine

import (
	"fmt"
	"slices"
)

// endResult is steps 5–9 at one birth date, before step 4 compares the
// results across the birth-date range.
type endResult struct {
	bucket   Bucket
	block    Block
	next     NextDose
	hasNext  bool // next holds the dates; set even when bucket is ⚪ (masked)
	k        int
	complete bool // COMPLETE
	reasons  []Reason
	products []string          // the products for next; set with hasNext
	cands    []candidateResult // per candidate series, for later merges (S4)
	series   []int             // the end's candidate series, in rule-set order (seriesUsed)
	// disagreeing are the records whose DOSE_AFTER_MAX_AGE a maximum-age
	// disagreement dropped (issue #15 In scope 5), by ID; for compared
	// results, those of every result. This is narrower than S4b's
	// questioned records (issue #28: any record that some candidate gives
	// DOSE_INVALID or DOSE_AFTER_MAX_AGE), which its counted view counts;
	// an agreed DOSE_AFTER_MAX_AGE or a DOSE_INVALID is not listed here.
	disagreeing []string
}

// class is the result as the user sees it: ⚪ whenever a reason blocks, and
// ✅ complete told apart from ✅ ON_TRACK.
func (r endResult) class() string {
	switch {
	case r.bucket == BucketUnknown:
		return string(BucketUnknown)
	case r.block != "":
		return string(r.block)
	case r.bucket == BucketComplete && !r.complete:
		return "onTrack"
	}
	return string(r.bucket)
}

// distanceOnly reports whether a class differs from the others in its set
// only by how far away the next dose is (SAD §5.3 step 4, F05.5).
func distanceOnly(class string) bool {
	return class == string(BlockNotYetScheduled) || class == string(BucketUpcoming) || class == "onTrack"
}

// compareResults combines the results that step 4 computes across the
// birth-date range, in ascending birth order: the older and the later end,
// or with no dose given both ends of every sub-range where the start-age
// choice is fixed (SAD §5.3 step 4, F05.5, EQ17, A8; noDosePoints).
//
// Two results agree when they have the same next dose number and the same
// blocking reasons, and either the same class or two classes in
// {notYetScheduled, 🟡, ✅ ON_TRACK}. Dates alone never make them differ: the
// dates driven by a minimum age or interval come from the latest birth date.
// Agreement is an equivalence, so the results all agree when each agrees
// with the latest one. Then the latest result is used, with its series
// (seriesUsed) and products (issue #23 Q40, amended in PR #27). A ⚪ result
// keeps the other results' reasons that it lacks; any other result keeps
// only those that are not bucket reasons (bucketReason), which describe the
// latest result's bucket. They are added from the latest birth date down.
//
// Otherwise the result is ⚪ BIRTH_DATE_IMPRECISE{hint} with no dates,
// followed by every result's reasons, from the latest birth date down,
// without repeats. It keeps the latest result's candidate series
// (seriesUsed, Q40) but not its products. When any of the results has an
// age-limit reason (ageLimit), the hint is fullBirthDate whatever the birth
// precision (SAD §5.3 step 4: an end hits an age limit; issue #15 Q58). On
// S3b's inputs this is "some result is outOfAge and another is not": two
// outOfAge results agree.
func compareResults(rs []endResult, hint Hint) endResult {
	later := rs[len(rs)-1]
	agree, limit := true, ageLimit(later.reasons)
	for _, r := range rs[:len(rs)-1] {
		co, cl := r.class(), later.class()
		agree = agree && r.k == later.k &&
			slices.EqualFunc(blockingOnly(r.reasons), blockingOnly(later.reasons), sameReason) &&
			(co == cl || distanceOnly(co) && distanceOnly(cl))
		limit = limit || ageLimit(r.reasons)
	}
	// listed holds the reasons already in res.reasons, so that removing
	// duplicates stays linear in the number of reasons (one DOSE_INVALID
	// per record at each end).
	listed := map[string]bool{}
	add := func(res *endResult, r Reason) {
		if k := reasonKey(r); !listed[k] {
			listed[k] = true
			res.reasons = append(res.reasons, r)
		}
	}
	var disagreeing []string
	for _, r := range rs {
		disagreeing = append(disagreeing, r.disagreeing...)
	}
	slices.Sort(disagreeing)
	disagreeing = slices.Compact(disagreeing)
	if agree {
		res := later
		res.disagreeing = disagreeing
		res.reasons = slices.Clone(later.reasons)
		for _, r := range later.reasons {
			listed[reasonKey(r)] = true
		}
		for i := len(rs) - 2; i >= 0; i-- {
			for _, r := range rs[i].reasons {
				if later.bucket == BucketUnknown || !bucketReason(r.Code) {
					add(&res, r)
				}
			}
		}
		return res
	}
	if limit {
		hint = HintFullBirthDate
	}
	res := endResult{bucket: BucketUnknown, k: later.k, series: later.series, disagreeing: disagreeing}
	add(&res, Reason{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: hint}})
	for i := len(rs) - 1; i >= 0; i-- {
		for _, r := range rs[i].reasons {
			add(&res, r)
		}
	}
	return res
}

// ageLimit reports whether reasons hold an age-limit reason (issue #15
// Q58): OUT_OF_AGE, DOSE_AFTER_MAX_AGE, CHECK_PRODUCT_NAME, MIXED_PRODUCTS,
// a DOSE_POSSIBLY_INVALID listing a maximum age, and since S4b (issue #28
// In scope 6) DUE_CAPPED_BY_MAX_AGE, SERIES_MAY_NOT_COMPLETE and
// BIRTH_DATE_IMPRECISE{fullBirthDate}. An end's own reasons hold
// BIRTH_DATE_IMPRECISE only when it is an age-limit ⚪ (not certainly out
// of age at that date), so that result's reason then has the same key as
// the comparison's and is listed once.
func ageLimit(reasons []Reason) bool {
	for _, r := range reasons {
		switch r.Code {
		case ReasonOutOfAge, ReasonDoseAfterMaxAge, ReasonCheckProductName, ReasonMixedProducts,
			ReasonDueCappedByMaxAge, ReasonSeriesMayNotComplete:
			return true
		case ReasonBirthDateImprecise:
			if r.Params.Hint == HintFullBirthDate {
				return true
			}
		case ReasonDosePossiblyInvalid:
			if slices.Contains(r.Params.Checks, CheckMaxAgeStart) || slices.Contains(r.Params.Checks, CheckMaxAgeComplete) {
				return true
			}
		}
	}
	return false
}

// bucketReason reports whether a reason describes one end's bucket: COMPLETE,
// ON_TRACK, ALTERNATIVE_PLANS, PRODUCT_NOT_IN_CATALOG, SAFEST_SERIES,
// OUT_OF_AGE, DUE_CAPPED_BY_MAX_AGE and SERIES_MAY_NOT_COMPLETE. With doses given, SAFEST_SERIES is emitted once before step 4
// and is never in an end's reasons; with no dose given the sub-ranges of the
// birth range can choose different brackets, so each one's SAFEST_SERIES and
// OUT_OF_AGE can differ.
func bucketReason(c ReasonCode) bool {
	switch c {
	case ReasonComplete, ReasonOnTrack, ReasonAlternativePlans, ReasonProductNotInCatalog, ReasonSafestSeries, ReasonOutOfAge,
		ReasonDueCappedByMaxAge, ReasonSeriesMayNotComplete:
		return true
	}
	return false
}

func blockingOnly(reasons []Reason) []Reason {
	var out []Reason
	for _, r := range reasons {
		if r.Code.blocks() {
			out = append(out, r)
		}
	}
	return out
}

// sameReason reports whether two reasons have the same code and params.
func sameReason(a, b Reason) bool { return reasonKey(a) == reasonKey(b) }

// reasonKey encodes a reason's code and params; two reasons have the same
// key iff they have the same code and params.
func reasonKey(r Reason) string {
	p := r.Params
	return fmt.Sprintf("%q %q %q %d %q %q %q %v %q %d", r.Code, p.Dose, p.Hint, p.Number, p.Series, p.Checks, p.Doses, p.Numbers, candidateKey(p.Candidates), p.LastDay.days)
}
