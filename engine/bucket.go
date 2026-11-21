package engine

// classify assigns the bucket or block of a next dose (SAD §5.3 step 9).
// The thresholds are added to today, never subtracted from the due date, and
// every boundary is inclusive:
//
//	today ≥ overdue                             → 🔴 overdue
//	today ≥ earliest and due ≤ today + 30 days  → 🟠 due
//	due ≤ today + 6 months                      → 🟡 upcoming
//	at least one dose counted                   → ✅ complete, ON_TRACK
//	otherwise                                   → notYetScheduled block
func classify(today Date, nd NextDose, anyCounted bool) (Bucket, Block, ReasonCode) {
	switch {
	case !today.Before(nd.Overdue):
		return BucketOverdue, "", ""
	case !today.Before(nd.Earliest) && !nd.Due.After(today.AddDays(dueSoonDays)):
		return BucketDue, "", ""
	case !nd.Due.After(today.AddMonths(upcomingMonths)):
		return BucketUpcoming, "", ""
	case anyCounted:
		return BucketComplete, "", ReasonOnTrack
	}
	return "", BlockNotYetScheduled, ""
}
