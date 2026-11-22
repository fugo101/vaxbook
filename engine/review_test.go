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
}
