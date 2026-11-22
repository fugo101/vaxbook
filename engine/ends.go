package engine

import (
	"fmt"
	"slices"
)

// endResult is steps 5–9 at one birth end, before step 4 compares the ends.
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

// compareEnds combines the results at the older and the later end of the
// birth-date range (SAD §5.3 step 4, F05.5, EQ17, A8).
//
// The ends agree when they have the same next dose number and the same
// blocking reasons, and either the same class or two classes in
// {notYetScheduled, 🟡, ✅ ON_TRACK}. Dates alone never make them differ: the
// dates driven by a minimum age or interval come from the later end. Then
// the later end's result is used. A ⚪ result keeps the older end's reasons
// that the later end lacks; any other result keeps only those that are not
// bucket reasons (bucketReason), which describe the later end's bucket.
//
// Otherwise the result is ⚪ BIRTH_DATE_IMPRECISE{hint} with no dates,
// followed by the later end's reasons and then the older end's reasons that
// are not already listed.
func compareEnds(older, later endResult, hint Hint) endResult {
	co, cl := older.class(), later.class()
	agree := older.k == later.k &&
		slices.EqualFunc(blockingOnly(older.reasons), blockingOnly(later.reasons), sameReason) &&
		(co == cl || distanceOnly(co) && distanceOnly(cl))
	// listed holds the reasons already in res.reasons, so that removing
	// duplicates stays linear in the number of reasons (one DOSE_INVALID
	// per record at each end).
	listed := map[string]bool{}
	if agree {
		res := later
		res.reasons = slices.Clone(later.reasons)
		for _, r := range later.reasons {
			listed[reasonKey(r)] = true
		}
		for _, r := range older.reasons {
			keep := later.bucket == BucketUnknown || !bucketReason(r.Code)
			if k := reasonKey(r); keep && !listed[k] {
				listed[k] = true
				res.reasons = append(res.reasons, r)
			}
		}
		return res
	}
	res := endResult{bucket: BucketUnknown, k: later.k}
	for _, r := range slices.Concat([]Reason{{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: hint}}}, later.reasons, older.reasons) {
		if k := reasonKey(r); !listed[k] {
			listed[k] = true
			res.reasons = append(res.reasons, r)
		}
	}
	return res
}

// bucketReason reports whether a reason describes one end's bucket: COMPLETE,
// ON_TRACK, ALTERNATIVE_PLANS, PRODUCT_NOT_IN_CATALOG and SAFEST_SERIES. With
// doses given, SAFEST_SERIES is emitted once before step 4 and is never in an
// end's reasons.
func bucketReason(c ReasonCode) bool {
	switch c {
	case ReasonComplete, ReasonOnTrack, ReasonAlternativePlans, ReasonProductNotInCatalog, ReasonSafestSeries:
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
	return fmt.Sprintf("%q %q %q %d %q %q %q %v %q", r.Code, p.Dose, p.Hint, p.Number, p.Series, p.Checks, p.Doses, p.Numbers, candidateKey(p.Candidates))
}
