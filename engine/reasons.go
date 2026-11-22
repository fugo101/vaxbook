package engine

import "fmt"

// blocks reports whether reason code c is blocking: it makes the assessment
// ⚪ BucketUnknown with no next dose (SAD §5.3 step 10, issue #13). The
// informational codes explain a result without masking it. The split is a
// closed switch, so a new code must be classified here.
func (c ReasonCode) blocks() bool {
	switch c {
	case ReasonInvalidDate, ReasonOutOfRange, ReasonDateInFuture, ReasonDateBeforeBirth, ReasonUnexpectedValue,
		ReasonOverlappingEstimatedDates, ReasonNumberingConflict, ReasonBirthDateImprecise,
		ReasonDoseInvalid, ReasonDosePossiblyInvalid, ReasonPreviousDoseDateUnknown, ReasonNoBirthDate:
		return true
	case ReasonImpliedEarlierDoses, ReasonIntervalUnverifiable, ReasonFirstDoseDateAssumed,
		ReasonAlternativePlans, ReasonComplete, ReasonOnTrack:
		return false
	}
	panic(fmt.Sprintf("engine: reason code %q is not classified as blocking or informational", string(c)))
}

// blocking reports whether any of reasons is blocking.
func blocking(reasons []Reason) bool {
	for _, r := range reasons {
		if r.Code.blocks() {
			return true
		}
	}
	return false
}
