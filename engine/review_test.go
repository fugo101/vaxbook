package engine_test

import (
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
)

// reviewCase is one input of TestS2ReviewRegressions with the expected
// assessment of one disease, derived by hand from the spec (never from
// engine output). reasons is the compact JSON of the reasons.
type reviewCase struct {
	name    string
	ruleset string
	today   string
	birth   string // a day-precise birth date
	doses   string // the JSON array of dose records
	disease string
	bucket  engine.Bucket
	est     bool
	reasons string
}

// TestS2ReviewRegressions pins the S2 review findings that no golden file
// covers. The derivations use the notation of testdata/golden/README.md.
func TestS2ReviewRegressions(t *testing.T) {
	t.Parallel()
	for _, c := range s2ReviewCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			checkReviewCase(t, c)
		})
	}
}

func checkReviewCase(t *testing.T, c reviewCase) {
	t.Helper()
	src := fmt.Sprintf(`{"today": %q, "profile": {"birth": {"value": %q, "precision": "day"}}, "doses": %s}`, c.today, c.birth, c.doses)
	in, err := engine.DecodeInput([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := loadEngine(t, c.ruleset).Evaluate(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range out.Assessments {
		if a.Disease != c.disease {
			continue
		}
		got, err := json.Marshal(a.Reasons, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Join(strings.Fields(c.reasons), "")
		if a.Bucket != c.bucket || a.Estimated != c.est || string(got) != want {
			t.Errorf("%s: bucket %q, estimated %v, reasons\n  %s\nwant bucket %q, estimated %v, reasons\n  %s",
				c.disease, a.Bucket, a.Estimated, got, c.bucket, c.est, want)
		}
		return
	}
	t.Fatalf("no assessment for %s", c.disease)
}

var s2ReviewCases = []reviewCase{
	{
		// hpv h2 #2: dose 1 is implied, u1 ≥ b + 9y = 2025-01-01, so h2 must
		// be ≥ 01-29 → DOSE_INVALID{minInterval}, not counted. k = 2 and the
		// only counted dose is implied, so step 7 has no first-dose date and
		// stops. The birth date is known, so it is not NO_BIRTH_DATE; the
		// DOSE_INVALID already makes the assessment ⚪.
		name: "known birth date, step 7 without a dated anchor", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2016-01-01",
		doses:   `[{"id": "h2", "diseases": ["hpv"], "date": {"value": "2025-01-10", "precision": "day"}, "doseNumbers": {"hpv": 2}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1]}},
			{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"h2","number":2}}]`,
	},
	// minIntervalFromFirst against an undated dose 1 (SAD §5.3 step 5: no
	// placement of the undated doses satisfies every constraint →
	// DOSE_INVALID). hpv: dose 1 min age 9y; dose 2 4w; dose 3 12w and 5m
	// from dose 1. b 2010-05-20, so dose 1 ≥ 2019-05-20 and dose 3 ≥
	// 2019-10-20 wherever dose 1 lies.
	{
		// r2 ≥ u1 + 28d = 06-17 ✓ (INTERVAL_UNVERIFIABLE). r3 ≥ r2 + 84d =
		// 09-12 ✓, but < 10-20 → DOSE_INVALID{minIntervalFromFirst}, not
		// counted. k = 3; dose 1 is undated, so r2 stands in for it
		// (FIRST_DOSE_DATE_ASSUMED); the masked bucket is overdue.
		name: "minIntervalFromFirst from a level-4 dose 1", ruleset: "illustrative-vn", today: "2026-06-15", birth: "2010-05-20",
		doses: `[{"id": "r1", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 1}},
			{"id": "r2", "diseases": ["hpv"], "date": {"value": "2019-06-20", "precision": "day"}},
			{"id": "r3", "diseases": ["hpv"], "date": {"value": "2019-09-15", "precision": "day"}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"INTERVAL_UNVERIFIABLE","params":{"dose":"r2","number":2}},
			{"code":"DOSE_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}},
			{"code":"FIRST_DOSE_DATE_ASSUMED","params":{"dose":"r2"}}]`,
	},
	{
		// Doses 1 and 2 are implied. Chain: u1 ≥ 05-20, u2 ≥ 06-17, so r3 ≥
		// 09-09 ✓; r3 < u1 + 5m = 10-20 → DOSE_INVALID. k = 3, and no counted
		// dose is dated, so step 7 has no first-dose date and stops; the
		// DOSE_INVALID already makes it ⚪ (no NO_BIRTH_DATE: the birth date
		// is known).
		name: "minIntervalFromFirst from an implied dose 1", ruleset: "illustrative-vn", today: "2026-06-15", birth: "2010-05-20",
		doses:   `[{"id": "r3", "diseases": ["hpv"], "date": {"value": "2019-09-20", "precision": "day"}, "doseNumbers": {"hpv": 3}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1,2]}},
			{"code":"DOSE_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}}]`,
	},
	{
		// r3 ≥ u2 + 84d = 06-17 + 84d = 09-09 ✓, but < u1 + 5m = 10-20 →
		// DOSE_INVALID. Dose k − 1 = 2 is r2, undated → PREVIOUS_DOSE_DATE_UNKNOWN.
		name: "minIntervalFromFirst after a run of level-4 doses", ruleset: "illustrative-vn", today: "2019-09-15", birth: "2010-05-20",
		doses: `[{"id": "r1", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 1}},
			{"id": "r2", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 2}},
			{"id": "r3", "diseases": ["hpv"], "date": {"value": "2019-09-10", "precision": "day"}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}},
			{"code":"PREVIOUS_DOSE_DATE_UNKNOWN","params":{"dose":"r2"}}]`,
	},
	{
		// Every dose undated: u3 ≥ max(u2 + 84d = 09-09, u1 + 5m = 10-20) =
		// 10-20 > today → r3 DOSE_INVALID, not counted. Only
		// minIntervalFromFirst fails: the intervals alone fit by 09-09.
		name: "trailing level-4 doses with minIntervalFromFirst after today", ruleset: "illustrative-vn", today: "2019-09-15", birth: "2010-05-20",
		doses: `[{"id": "r1", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 1}},
			{"id": "r2", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 2}},
			{"id": "r3", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 3}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}},
			{"code":"PREVIOUS_DOSE_DATE_UNKNOWN","params":{"dose":"r2"}}]`,
	},
	{
		// r1 m 2026-06 is dose 1, R = [06-01, 06-30]; r2 and r3 trail it.
		// Intervals: u2 ≥ 06-29 (worst 07-28), u3 ≥ 09-21 (worst 10-20), both
		// ≤ today. From dose 1: u3 ≥ 06-01 + 5m = 11-01 > today at every end
		// → r3 DOSE_INVALID{minIntervalFromFirst}, not counted; dose k − 1 = 2
		// is r2, undated.
		name: "trailing slot fails minIntervalFromFirst at every end", ruleset: "illustrative-vn", today: "2026-10-25", birth: "2010-05-20",
		doses: `[{"id": "r1", "diseases": ["hpv"], "date": {"value": "2026-06", "precision": "month"}},
			{"id": "r2", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 2}},
			{"id": "r3", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 3}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}},
			{"code":"PREVIOUS_DOSE_DATE_UNKNOWN","params":{"dose":"r2"}}]`,
	},
	{
		// Same, today 2026-11-20: 06-01 + 5m = 11-01 ≤ today < 06-30 + 5m =
		// 11-30 → r3 DOSE_POSSIBLY_INVALID{minIntervalFromFirst}, counted →
		// the masked bucket is complete.
		name: "trailing slot fails minIntervalFromFirst at one end", ruleset: "illustrative-vn", today: "2026-11-20", birth: "2010-05-20",
		doses: `[{"id": "r1", "diseases": ["hpv"], "date": {"value": "2026-06", "precision": "month"}},
			{"id": "r2", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 2}},
			{"id": "r3", "diseases": ["hpv"], "date": {"precision": "unknown"}, "doseNumbers": {"hpv": 3}}]`,
		disease: "hpv", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_POSSIBLY_INVALID","params":{"checks":["minIntervalFromFirst"],"dose":"r3","number":3}},
			{"code":"COMPLETE"}]`,
	},
	// Doses beyond the series length (SAD §5.3 steps 2 and 5). pertussis: dose
	// 1 min age 6w; doses 2 and 3 min interval 4w; 3 doses.
	{
		// r1 #5 implies doses 1–3, which must lie before r1: u1 ≥ b + 42d =
		// 12-13 > r1 11-20, so no placement exists → DOSE_INVALID on r1, the
		// dated dose after the run (a chain failure is minInterval). The
		// implied doses stay counted, so k = 4 and the masked bucket is
		// complete.
		name: "number beyond the series, implied doses cannot fit before it", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-11-01",
		doses:   `[{"id": "r1", "diseases": ["pertussis"], "date": {"value": "2026-11-20", "precision": "day"}, "doseNumbers": {"pertussis": 5}}]`,
		disease: "pertussis", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1,2,3]}},
			{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"r1","number":5}},
			{"code":"COMPLETE"}]`,
	},
	{
		// u #4 is level 4 and implies doses 1–3, which trail: u3 ≥ 12-13 +
		// 28d + 28d = 2027-02-07 > today. The last trailing slot is implied,
		// so the record whose number implies it carries the DOSE_INVALID,
		// with that number.
		name: "level-4 number beyond the series, implied doses not feasible by today", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-11-01",
		doses:   `[{"id": "u", "diseases": ["pertussis"], "date": {"precision": "unknown"}, "doseNumbers": {"pertussis": 4}}]`,
		disease: "pertussis", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1,2,3]}},
			{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"u","number":4}},
			{"code":"COMPLETE"}]`,
	},
	{
		// a 11-20 is dose 1 (≥ b + 42d ✓); u #7 implies doses 2 and 3, which
		// trail a: u3 ≥ 11-20 + 28d + 28d = 2027-01-15 > today.
		name: "implied doses after a dated dose not feasible by today", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-01-01",
		doses: `[{"id": "a", "diseases": ["pertussis"], "date": {"value": "2026-11-20", "precision": "day"}},
			{"id": "u", "diseases": ["pertussis"], "date": {"precision": "unknown"}, "doseNumbers": {"pertussis": 7}}]`,
		disease: "pertussis", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[2,3]}},
			{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"u","number":7}},
			{"code":"COMPLETE"}]`,
	},
	{
		// a 11-01 #2 (dose 1 implied, u1 ≥ 02-12, a ≥ 03-12 ✓,
		// INTERVAL_UNVERIFIABLE). u #3 is level 4. b 11-05 takes position 4,
		// beyond the series, so u lies between a and b: u ≥ a + 28d = 11-29 >
		// b → DOSE_INVALID on b, the dated dose after the run. Doses 1–3 stay
		// counted → the masked bucket is complete.
		name: "undated dose before a dose beyond the series cannot fit", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-01-01",
		doses: `[{"id": "a", "diseases": ["pertussis"], "date": {"value": "2026-11-01", "precision": "day"}, "doseNumbers": {"pertussis": 2}},
			{"id": "b", "diseases": ["pertussis"], "date": {"value": "2026-11-05", "precision": "day"}},
			{"id": "u", "diseases": ["pertussis"], "date": {"precision": "unknown"}, "doseNumbers": {"pertussis": 3}}]`,
		disease: "pertussis", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1]}},
			{"code":"INTERVAL_UNVERIFIABLE","params":{"dose":"a","number":2}},
			{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"b","number":4}},
			{"code":"COMPLETE"}]`,
	},
	{
		// Control: a 10-01 #2, so u ≥ 10-29 ≤ b 11-05 fits → complete.
		name: "undated dose before a dose beyond the series fits", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-01-01",
		doses: `[{"id": "a", "diseases": ["pertussis"], "date": {"value": "2026-10-01", "precision": "day"}, "doseNumbers": {"pertussis": 2}},
			{"id": "b", "diseases": ["pertussis"], "date": {"value": "2026-11-05", "precision": "day"}},
			{"id": "u", "diseases": ["pertussis"], "date": {"precision": "unknown"}, "doseNumbers": {"pertussis": 3}}]`,
		disease: "pertussis", bucket: engine.BucketComplete, est: true,
		reasons: `[{"code":"IMPLIED_EARLIER_DOSES","params":{"numbers":[1]}},
			{"code":"INTERVAL_UNVERIFIABLE","params":{"dose":"a","number":2}},
			{"code":"COMPLETE"}]`,
	},
	// Unnumbered level-4 doses that fill no gap (floating) still count, so
	// the counted doses must all fit by today (SAD §5.3 step 5). measles: dose
	// 1 min age 9m; dose 2 min interval 4w.
	{
		// b 2026-10-01: dose 1 ≥ 2027-07-01 and dose 2 ≥ 07-29, both after
		// today, so two doses cannot have been given → the last floating
		// dose by ID, f2, is DOSE_INVALID as dose 2 (a chain failure is
		// minInterval) and is not counted. k = 2 and f1 may be the latest
		// dose → PREVIOUS_DOSE_DATE_UNKNOWN{f1}.
		name: "floating level-4 doses not feasible by today", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-10-01",
		doses: `[{"id": "f1", "diseases": ["measles"], "date": {"precision": "unknown"}},
			{"id": "f2", "diseases": ["measles"], "date": {"precision": "unknown"}}]`,
		disease: "measles", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"f2","number":2}},
			{"code":"PREVIOUS_DOSE_DATE_UNKNOWN","params":{"dose":"f1"}}]`,
	},
	{
		// b 2026-02-01: m1 11-20 ≥ b + 9m = 11-01 is dose 1. With f, two
		// doses are counted, and dose 2 ≥ 11-01 + 28d = 11-29 > today → f
		// DOSE_INVALID, not counted. k = 2: E = m1 + 28d = 12-18, Du = b +
		// 18m = 2027-08-01 > T6m, 1 dose → the masked bucket is complete +
		// ON_TRACK.
		name: "floating level-4 dose with a dated dose not feasible by today", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2026-02-01",
		doses: `[{"id": "m1", "diseases": ["measles"], "date": {"value": "2026-11-20", "precision": "day"}},
			{"id": "f", "diseases": ["measles"], "date": {"precision": "unknown"}}]`,
		disease: "measles", bucket: engine.BucketUnknown, est: true,
		reasons: `[{"code":"DOSE_INVALID","params":{"checks":["minInterval"],"dose":"f","number":2}},
			{"code":"ON_TRACK"}]`,
	},
	{
		// Control: b 2025-01-01, so dose 2 ≥ 2025-10-01 + 28d = 10-29 ≤ today
		// (f may be dose 1, before m1) → complete.
		name: "floating level-4 dose with a dated dose feasible", ruleset: "illustrative-vn", today: "2026-11-22", birth: "2025-01-01",
		doses: `[{"id": "m1", "diseases": ["measles"], "date": {"value": "2026-11-01", "precision": "day"}},
			{"id": "f", "diseases": ["measles"], "date": {"precision": "unknown"}}]`,
		disease: "measles", bucket: engine.BucketComplete, est: true,
		reasons: `[{"code":"COMPLETE"}]`,
	},
}
