package engine_test

import (
	"bytes"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// propertyRuns is the number of random inputs per fixture rule set. The
// generator is seeded, so every run (native and WASM) sees the same inputs.
const propertyRuns = 400

type fixture struct {
	name   string
	snap   *ruleset.Snapshot
	engine *engine.Engine
}

func fixtures(t *testing.T) []fixture {
	t.Helper()
	var out []fixture
	for _, path := range fixtureFiles(t) {
		name := fixtureName(path)
		out = append(out, fixture{name: name, snap: loadSnapshot(t, name), engine: loadEngine(t, name)})
	}
	return out
}

func mustDate(t testing.TB, s string) engine.Date {
	t.Helper()
	d, err := engine.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func daysBetween(a, b engine.Date) int {
	n := 0
	for d := a; d.Before(b); d = d.AddDays(1) {
		n++
	}
	return n
}

// randomDayInput draws a birth date, a today on or after it, and 0–6 dose
// records dated in [birth, today] for random non-empty subsets of the
// diseases, every date day-precise and no dose numbers (the S1 shape).
// About one record in 25 has an impossible date (A9), and about one in 25 is
// dated after today or before birth (record-scoped, ⚪). Products are drawn
// as randomProduct says.
func randomDayInput(t testing.TB, rng *rand.Rand, s *ruleset.Snapshot) engine.Input {
	t.Helper()
	return randomInputOf(t, rng, s, false)
}

// randomInput is randomDayInput with S2 dates: the birth date is day
// (70%), month (15%) or year (10%) precise or absent (5%); a dose date is
// day (70%), month (15%) or year (5%) precise, or level 4 (10%); every value
// is cut from a true date, so the true date lies in the stated period.
// About one record in 10 has dose numbers: its true rank among the records
// of the disease, or a random number from 1 to 4.
func randomInput(t testing.TB, rng *rand.Rand, s *ruleset.Snapshot) engine.Input {
	t.Helper()
	return randomInputOf(t, rng, s, true)
}

func randomInputOf(t testing.TB, rng *rand.Rand, s *ruleset.Snapshot, imprecise bool) engine.Input {
	t.Helper()
	base := mustDate(t, "1990-01-01")
	birth := base.AddDays(rng.IntN(41 * 365))
	// Ages up to 20 years, biased toward infancy where most milestones are.
	maxAge := []int{120, 400, 1000, 3700, 7300}[rng.IntN(5)]
	today := birth.AddDays(rng.IntN(maxAge + 1))
	span := daysBetween(birth, today)
	in := engine.Input{
		Today:   today,
		Profile: engine.Profile{Birth: engine.RecalledDate{Value: birth.String(), Precision: engine.Day}},
		Doses:   []engine.DoseRecord{},
	}
	if imprecise {
		switch n := rng.IntN(100); {
		case n < 15:
			in.Profile.Birth = engine.RecalledDate{Value: birth.String()[:7], Precision: engine.Month}
		case n < 25:
			in.Profile.Birth = engine.RecalledDate{Value: birth.String()[:4], Precision: engine.Year}
		case n < 30:
			in.Profile.Birth = engine.RecalledDate{Precision: engine.None}
		}
	}
	trueDates := map[string]engine.Date{}
	for i := range rng.IntN(7) {
		var diseases []string
		for _, d := range s.Diseases {
			if rng.IntN(2) == 0 {
				diseases = append(diseases, d.ID)
			}
		}
		if len(diseases) == 0 {
			diseases = []string{s.Diseases[rng.IntN(len(s.Diseases))].ID}
		}
		date := birth.AddDays(rng.IntN(span + 1))
		value := date.String()
		switch rng.IntN(50) {
		case 0, 1:
			value = value[:8] + "32"
		case 2:
			value = today.AddDays(1 + rng.IntN(30)).String()
		case 3:
			value = birth.AddDays(-1 - rng.IntN(30)).String()
		}
		rec := engine.DoseRecord{
			ID:       fmt.Sprintf("r%d", i),
			Diseases: diseases,
			Date:     engine.RecalledDate{Value: value, Precision: engine.Day},
		}
		if imprecise && value == date.String() {
			switch n := rng.IntN(100); {
			case n < 15:
				rec.Date = engine.RecalledDate{Value: value[:7], Precision: engine.Month}
			case n < 20:
				rec.Date = engine.RecalledDate{Value: value[:4], Precision: engine.Year}
			case n < 30:
				rec.Date = engine.RecalledDate{Precision: engine.Unknown}
			}
		}
		randomProduct(rng, s, &rec)
		trueDates[rec.ID] = date
		in.Doses = append(in.Doses, rec)
	}
	if imprecise {
		for i := range in.Doses {
			r := &in.Doses[i]
			if rng.IntN(10) != 0 {
				continue
			}
			r.DoseNumbers = map[string]engine.DoseNumber{}
			for _, dz := range r.Diseases {
				n := 1 + rng.IntN(4)
				if rng.IntN(2) == 0 {
					n = 1
					for _, o := range in.Doses {
						if o.ID != r.ID && slices.Contains(o.Diseases, dz) && trueDates[o.ID].Before(trueDates[r.ID]) {
							n++
						}
					}
				}
				r.DoseNumbers[dz] = engine.DoseNumber(n)
			}
		}
	}
	return in
}

// unknownProductID is a product ID no fixture has (UNKNOWN_PRODUCT).
const unknownProductID = "zz-unknown"

// randomProduct gives a dose record no product (45%), a random catalog
// product (30%; retired products and products that list none of the
// record's diseases included), a catalog product that lists one of its
// diseases (15%), an off-catalog product (8%) or a product ID the rule set
// does not have (2%, record-scoped ⚪). On a rule set with no products the
// catalog draws give no product.
func randomProduct(rng *rand.Rand, s *ruleset.Snapshot, rec *engine.DoseRecord) {
	str := func(v string) *string { return &v }
	switch n := rng.IntN(100); {
	case n < 45:
	case n < 75:
		if len(s.Products) > 0 {
			rec.Product = str(s.Products[rng.IntN(len(s.Products))].ID)
		}
	case n < 90:
		var listing []string
		for _, p := range s.Products {
			if slices.ContainsFunc(p.Diseases, func(d string) bool { return slices.Contains(rec.Diseases, d) }) {
				listing = append(listing, p.ID)
			}
		}
		if len(listing) > 0 {
			rec.Product = str(listing[rng.IntN(len(listing))])
		}
	case n < 98:
		rec.FreeProduct = str("Vắc xin khác")
	default:
		rec.Product = str(unknownProductID)
	}
}

func cloneInput(in engine.Input) engine.Input {
	out := in
	out.Doses = make([]engine.DoseRecord, len(in.Doses))
	for i, r := range in.Doses {
		r.Diseases = slices.Clone(r.Diseases)
		r.DoseNumbers = maps.Clone(r.DoseNumbers)
		out.Doses[i] = r
	}
	return out
}

// evaluate runs e on in and returns the output and its encoding. It also
// checks the one shape rule that needs the input: an outOfAge
// assessment has seriesUsed iff the disease has a dose record (issue #28,
// Q74).
func evaluate(t testing.TB, e *engine.Engine, in engine.Input) (engine.Output, []byte) {
	t.Helper()
	out, err := e.Evaluate(in)
	if err != nil {
		t.Fatalf("Evaluate(%+v): %v", in, err)
	}
	for _, a := range out.Assessments {
		dosed := slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) })
		if a.Block == engine.BlockOutOfAge && dosed != (a.SeriesUsed != nil) {
			t.Fatalf("%s: outOfAge with seriesUsed %v, doses given %v (input %+v)", a.Disease, a.SeriesUsed, dosed, in)
		}
	}
	b, err := engine.EncodeOutput(out)
	if err != nil {
		t.Fatal(err)
	}
	return out, b
}

// seriesUsed returns the snapshot series an assessment names in seriesUsed,
// in its order.
func seriesUsed(t testing.TB, s *ruleset.Snapshot, a engine.Assessment) []ruleset.Series {
	t.Helper()
	var out []ruleset.Series
	for _, id := range a.SeriesUsed {
		i := slices.IndexFunc(s.Series, func(sr ruleset.Series) bool { return sr.ID == id })
		if i < 0 || s.Series[i].Disease != a.Disease {
			t.Fatalf("%s: seriesUsed names %q, not a series of the disease", a.Disease, id)
		}
		out = append(out, s.Series[i])
	}
	return out
}

// testPeriod returns the dates a recalled date may fall on, capped at today
// (EQ7, EQ17). It is written independently of engine.RecalledDate.Period.
func testPeriod(r engine.RecalledDate, today engine.Date) (lo, hi engine.Date, ok bool) {
	var err1, err2 error
	switch r.Precision {
	case engine.Day:
		lo, err1 = engine.ParseDate(r.Value)
		hi = lo
	case engine.Month:
		lo, err1 = engine.ParseDate(r.Value + "-01")
		hi = lo.AddMonths(1).AddDays(-1)
	case engine.Year:
		lo, err1 = engine.ParseDate(r.Value + "-01-01")
		hi, err2 = engine.ParseDate(r.Value + "-12-31")
	default:
		return engine.Date{}, engine.Date{}, false
	}
	if err1 != nil || err2 != nil || len(r.Value) != map[engine.Precision]int{engine.Day: 10, engine.Month: 7, engine.Year: 4}[r.Precision] {
		return engine.Date{}, engine.Date{}, false
	}
	if hi.After(today) {
		hi = today
	}
	return lo, hi, true
}

// blockingCodes are the reason codes that make an assessment ⚪ (issue #13).
var blockingCodes = map[engine.ReasonCode]bool{
	engine.ReasonInvalidDate: true, engine.ReasonOutOfRange: true, engine.ReasonDateInFuture: true,
	engine.ReasonDateBeforeBirth: true, engine.ReasonUnexpectedValue: true, engine.ReasonUnknownProduct: true,
	engine.ReasonOverlappingEstimatedDates: true,
	engine.ReasonNumberingConflict:         true, engine.ReasonBirthDateImprecise: true, engine.ReasonDoseInvalid: true,
	engine.ReasonDosePossiblyInvalid: true, engine.ReasonPreviousDoseDateUnknown: true, engine.ReasonNoBirthDate: true,
	engine.ReasonDoseAfterMaxAge: true, engine.ReasonCheckProductName: true, engine.ReasonMixedProducts: true,
	engine.ReasonOutOfAgeUncertain: true,
}

func reasonsWith(a engine.Assessment, code engine.ReasonCode) []engine.Reason {
	var out []engine.Reason
	for _, r := range a.Reasons {
		if r.Code == code {
			out = append(out, r)
		}
	}
	return out
}

// checkAssessmentShape checks what every assessment satisfies:
//   - its reasons carry exactly their code's params;
//   - exactly one of bucket and block;
//   - bucket ⚪ only with a blocking reason (informational reasons never
//     make it ⚪ on their own), and a blocking reason makes it ⚪ (SAD §5.3
//     step 10), except under the outOfAge block, which layer 5 puts over
//     masked reasons (issue #28; notYetScheduled is layer 8);
//   - a next dose unless the series is complete, the bucket is ⚪ (a ⚪
//     assessment never carries a next dose, SAD §5.1) or the block is
//     outOfAge;
//   - outOfAge has OUT_OF_AGE, no products, and with no seriesUsed (no dose
//     given, Q74; evaluate checks that against the input) only OUT_OF_AGE;
//   - due ≥ earliest, and overdue ≥ due + 7 days, unless a cap reason
//     (DUE_CAPPED_BY_MAX_AGE, SERIES_MAY_NOT_COMPLETE) lowered them: then
//     due ≤ overdue ≤ its lastDay;
//   - a cap reason comes with a next dose that is never 🔴 (P3), or in a ⚪
//     from step 4's comparison (with BIRTH_DATE_IMPRECISE); never under
//     outOfAge and never in a ⚪ from steps 5–7 alone;
//   - OUT_OF_AGE_UNCERTAIN comes only in a ⚪ (so with no next dose), never
//     under outOfAge, only in an estimated assessment (it rests on an
//     imprecise or missing dose date), with its numbers strictly ascending.
func checkAssessmentShape(t testing.TB, name string, a engine.Assessment) {
	t.Helper()
	checkReasonParams(t, a)
	if (a.Bucket == "") == (a.Block == "") {
		t.Fatalf("%s %s: bucket %q, block %q", name, a.Disease, a.Bucket, a.Block)
	}
	outOfAge := a.Block == engine.BlockOutOfAge
	blocked := slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool { return blockingCodes[r.Code] })
	if a.Bucket == engine.BucketUnknown && !blocked || blocked && a.Bucket != engine.BucketUnknown && !outOfAge {
		t.Fatalf("%s %s: bucket %q, block %q with reasons %v", name, a.Disease, a.Bucket, a.Block, a.Reasons)
	}
	if outOfAge && (!hasCode(a, engine.ReasonOutOfAge) || a.Products != nil ||
		a.SeriesUsed == nil && !reflect.DeepEqual(a.Reasons, []engine.Reason{{Code: engine.ReasonOutOfAge}})) {
		t.Fatalf("%s %s: outOfAge with seriesUsed %v, products %v and reasons %v", name, a.Disease, a.SeriesUsed, a.Products, a.Reasons)
	}
	noNext := len(reasonsWith(a, engine.ReasonComplete)) > 0 || a.Bucket == engine.BucketUnknown || outOfAge
	if noNext != (a.NextDose == nil) {
		t.Fatalf("%s %s: nextDose %v with bucket %q and reasons %v", name, a.Disease, a.NextDose, a.Bucket, a.Reasons)
	}
	caps := append(reasonsWith(a, engine.ReasonDueCappedByMaxAge), reasonsWith(a, engine.ReasonSeriesMayNotComplete)...)
	switch {
	case len(caps) == 0:
	case outOfAge || a.Bucket == engine.BucketOverdue ||
		a.NextDose == nil && !hasCode(a, engine.ReasonBirthDateImprecise) ||
		a.NextDose != nil && len(caps) > 1:
		t.Fatalf("%s %s: cap reasons %v with bucket %q, block %q, nextDose %v and reasons %v", name, a.Disease, caps, a.Bucket, a.Block, a.NextDose, a.Reasons)
	}
	for _, r := range reasonsWith(a, engine.ReasonOutOfAgeUncertain) {
		if a.Bucket != engine.BucketUnknown || !a.Estimated || !slices.IsSorted(r.Params.Numbers) ||
			len(slices.Compact(slices.Clone(r.Params.Numbers))) != len(r.Params.Numbers) {
			t.Fatalf("%s %s: %v with bucket %q, block %q, estimated %v", name, a.Disease, r, a.Bucket, a.Block, a.Estimated)
		}
	}
	if nd := a.NextDose; nd != nil {
		if nd.Due.Before(nd.Earliest) {
			t.Fatalf("%s %s: due %s before earliest %s", name, a.Disease, nd.Due, nd.Earliest)
		}
		switch {
		case len(caps) > 0:
			if nd.Overdue.Before(nd.Due) || nd.Overdue.After(caps[0].Params.LastDay) {
				t.Fatalf("%s %s: capped due %s and overdue %s, last day %s", name, a.Disease, nd.Due, nd.Overdue, caps[0].Params.LastDay)
			}
		case nd.Overdue.Before(nd.Due.AddDays(engine.OverdueGraceDays)):
			t.Fatalf("%s %s: overdue %s before due %s + 7 days", name, a.Disease, nd.Overdue, nd.Due)
		}
	}
}

// TestPropertyNextDoseBounds checks every assessment of random S1 and S2
// inputs (checkAssessmentShape) and P1 for every next dose: for every
// candidate series in seriesUsed with at least k doses, and for some plan of
// it, the earliest allowed date is never before
//   - the minimum age at the later end of the birth-date range, capped at
//     today (SAD §5.7, EQ17), when there is a birth date;
//   - the later (today-capped) end of the latest dose range + the minimum
//     interval, when no record of the disease has a dose number (so the
//     latest dose in walk order is the one with the latest end).
func TestPropertyNextDoseBounds(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(11, uint64(len(fx.name))))
		for i := range 2 * propertyRuns {
			in := randomInput(t, rng, fx.snap)
			if i%2 == 0 {
				in = randomDayInput(t, rng, fx.snap)
			}
			out, _ := evaluate(t, fx.engine, in)
			_, birthHi, birthKnown := testPeriod(in.Profile.Birth, in.Today)
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				if a.NextDose == nil {
					continue
				}
				nd := *a.NextDose
				var latest engine.Date
				haveLatest, numbered := false, false
				for _, r := range in.Doses {
					if !slices.Contains(r.Diseases, a.Disease) {
						continue
					}
					numbered = numbered || len(r.DoseNumbers) > 0
					if _, hi, ok := testPeriod(r.Date, in.Today); ok && (!haveLatest || hi.After(latest)) {
						latest, haveLatest = hi, true
					}
				}
				for _, sr := range seriesUsed(t, fx.snap, a) {
					if len(sr.Plans[0].Doses) < nd.Number {
						continue
					}
					ok := false
					for _, p := range sr.Plans {
						r := p.Doses[nd.Number-1]
						if birthKnown && r.MinAge != nil && nd.Earliest.Before(birthHi.AddSpan(*r.MinAge)) {
							continue
						}
						if !numbered && haveLatest && r.MinInterval != nil && nd.Earliest.Before(latest.AddSpan(*r.MinInterval)) {
							continue
						}
						ok = true
					}
					if !ok {
						t.Fatalf("%s %s: earliest %s violates every plan's minimums of %s for dose %d (input %+v)",
							fx.name, a.Disease, nd.Earliest, sr.ID, nd.Number, in)
					}
				}
			}
		}
	}
}

// TestPropertyNoBirthDateNoAgeCheck (P2): with no birth date, no age-based
// check makes a dose invalid, possibly invalid or after a maximum age (SAD
// §5.7, EQ19, A6; issue #15): no DOSE_AFTER_MAX_AGE, and no reason lists
// minAge, maxAgeStart or maxAgeComplete. Every random input is also
// evaluated with its birth date removed, and so are the near-limit inputs
// of the fixtures with maximum ages.
func TestPropertyNoBirthDateNoAgeCheck(t *testing.T) {
	t.Parallel()
	check := func(fx fixture, in engine.Input) {
		in.Profile.Birth = engine.RecalledDate{Precision: engine.None}
		out, _ := evaluate(t, fx.engine, in)
		for _, a := range out.Assessments {
			checkAssessmentShape(t, fx.name, a)
			if !a.Estimated {
				t.Fatalf("%s %s: no birth date but not estimated", fx.name, a.Disease)
			}
			for _, r := range a.Reasons {
				if r.Code == engine.ReasonDoseAfterMaxAge || slices.ContainsFunc(r.Params.Checks, func(c engine.Check) bool {
					return c == engine.CheckMinAge || c == engine.CheckMaxAgeStart || c == engine.CheckMaxAgeComplete
				}) {
					t.Fatalf("%s %s: no birth date but %s checks an age (input %+v)", fx.name, a.Disease, r.Code, in)
				}
			}
		}
	}
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(19, uint64(len(fx.name))))
		for range propertyRuns {
			check(fx, randomInput(t, rng, fx.snap))
		}
		limits := ageLimits(fx.snap)
		for range len(limits) * propertyRuns / 4 {
			check(fx, nearLimitInput(t, rng, fx.snap, limits))
		}
	}
}

// TestPropertyEstimated: an assessment is estimated iff its result rests on
// an imprecise date, an undated dose or an off-catalog product. Every
// assessment of a day-precise input with no level-4 dose and no dose
// numbers is not estimated unless it has PRODUCT_NOT_IN_CATALOG (EQ14); an
// assessment is estimated when the birth date or a dose record of its
// disease is not day-precise, or when it has implied doses or
// PRODUCT_NOT_IN_CATALOG.
func TestPropertyEstimated(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(23, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomDayInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				if notInCatalog := len(reasonsWith(a, engine.ReasonProductNotInCatalog)) > 0; a.Estimated != notInCatalog {
					t.Fatalf("%s %s: day-precise input gives estimated %v with reasons %v", fx.name, a.Disease, a.Estimated, a.Reasons)
				}
			}
			in = randomInput(t, rng, fx.snap)
			out, _ = evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				want := in.Profile.Birth.Precision != engine.Day || len(reasonsWith(a, engine.ReasonImpliedEarlierDoses)) > 0 ||
					len(reasonsWith(a, engine.ReasonProductNotInCatalog)) > 0
				for _, r := range in.Doses {
					want = want || (slices.Contains(r.Diseases, a.Disease) && r.Date.Precision != engine.Day)
				}
				if want && !a.Estimated {
					t.Fatalf("%s %s: imprecise input but not estimated (input %+v)", fx.name, a.Disease, in)
				}
			}
		}
	}
}

// invalidDoses lists the dose and position of each DOSE_INVALID and
// DOSE_AFTER_MAX_AGE reason, once each: the doses the result does not count
// (issue #15, "dose and position"). The code and checks are left out: a
// dose that is not counted drops its possible checks (Q62), so narrowing its
// date can turn a hidden possible check into a definite one, adding a check
// or a DOSE_INVALID next to its DOSE_AFTER_MAX_AGE.
func invalidDoses(a engine.Assessment) []string {
	var out []string
	for _, r := range a.Reasons {
		if r.Code == engine.ReasonDoseInvalid || r.Code == engine.ReasonDoseAfterMaxAge {
			if k := fmt.Sprintf("%s#%d", r.Params.Dose, r.Params.Number); !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	return out
}

// disagreeingIn returns the records of disease whose DOSE_AFTER_MAX_AGE a
// maximum-age disagreement drops in the result of any of the inputs
// (engine.DisagreeingRecords).
func disagreeingIn(t testing.TB, e *engine.Engine, disease string, ins ...engine.Input) map[string]bool {
	t.Helper()
	q := map[string]bool{}
	for _, in := range ins {
		recs, err := engine.DisagreeingRecords(e, in)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range recs[disease] {
			q[id] = true
		}
	}
	return q
}

// symmetricDifference returns the keys in exactly one of the sorted lists a
// and b.
func symmetricDifference(a, b []string) []string {
	var out []string
	for _, k := range a {
		if !slices.Contains(b, k) {
			out = append(out, k)
		}
	}
	for _, k := range b {
		if !slices.Contains(a, k) {
			out = append(out, k)
		}
	}
	return out
}

// recordOf returns the record ID of an uncountedKeys key.
func recordOf(k string) string {
	id, _, _ := strings.Cut(k, "#")
	return id
}

// uncountedKeys is invalidDoses for the narrowing property: dose and
// position, except that an unnumbered level-4 record is keyed by its ID
// alone. Such a record that fills no gap floats and has no position (SAD
// §5.3 step 5): when no placement fits, the last one by ID is named with the
// number of the position that fails when every floating dose goes after the
// last dated dose (issue #15 In scope 4), and narrowing a dated range can
// change which position that is (a possible failure hidden on the named
// dose, Q62) without changing which records are counted.
func uncountedKeys(a engine.Assessment, in engine.Input) []string {
	floating := map[string]bool{}
	for _, r := range in.Doses {
		if r.Date.Precision == engine.Unknown && r.DoseNumbers[a.Disease] == 0 {
			floating[r.ID] = true
		}
	}
	var out []string
	for _, r := range a.Reasons {
		if r.Code != engine.ReasonDoseInvalid && r.Code != engine.ReasonDoseAfterMaxAge {
			continue
		}
		k := fmt.Sprintf("%s#%d", r.Params.Dose, r.Params.Number)
		if floating[r.Params.Dose] {
			k = r.Params.Dose
		}
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// productDisagreement reports whether a has CHECK_PRODUCT_NAME or
// MIXED_PRODUCTS, which name no dose.
func productDisagreement(a engine.Assessment) bool {
	return hasCode(a, engine.ReasonCheckProductName) || hasCode(a, engine.ReasonMixedProducts)
}

// bracketed reports whether some series of the disease has a start-age
// bracket. Only then can a narrower date choose another series by bracket
// (Q44): without brackets the S3b bracket choice keeps every candidate, so a
// seriesUsed change under narrowing would be a candidate-selection bug.
func bracketed(s *ruleset.Snapshot, disease string) bool {
	return slices.ContainsFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == disease && sr.StartAge != nil })
}

// visible is how an assessment shows: its bucket or block.
func visible(a engine.Assessment) string { return string(a.Bucket) + string(a.Block) }

// TestPropertyNarrowingBirthDate: narrowing a month- or year-precision birth
// date to a day inside its (today-capped) range never turns a ✅, 🟡 or
// notYetScheduled result into 🔴 or 🟠 (issue #13), when the narrowed result
// has the same seriesUsed as the wide one or the disease has no dose (issue
// #23, Q44 and its extension, SAD §5.7).
//   - With doses given, the wide result is computed at both ends of the
//     range under one set of candidates, and for fixed series steps 5–9 are
//     monotone in the birth date between them; a narrower birth date can
//     choose another start-age bracket, with other dates. The skip applies
//     only to diseases with start-age brackets (see bracketed); for the
//     others the clause is checked whatever seriesUsed does, as in S3a.
//   - With no dose given, the birth range is split wherever the start-age
//     choice changes and both ends of every sub-range are compared (SAD
//     §5.3 step 4), so a calm wide result means every day of the range is
//     calm, whichever bracket it chooses: the clause has no skip.
//
// Half the inputs drop every dose and take a month- or year-precision birth
// date; half of those move today to 0–199 days after a bracket's max is
// reached at the older birth end, where the birth range crosses a bracket
// change. On minimal-start-age-gap-late the count of the F1 shape (PR #27:
// the birth ends choose different brackets, the latest birth date is calm,
// the wide result is ⚪ BIRTH_DATE_IMPRECISE and the narrowed day is 🟠 or
// 🔴) guards against the generator no longer reaching it.
func TestPropertyNarrowingBirthDate(t *testing.T) {
	t.Parallel()
	calm := map[string]bool{string(engine.BucketComplete): true, string(engine.BucketUpcoming): true, string(engine.BlockNotYetScheduled): true}
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(29, uint64(len(fx.name))))
		maxes := bracketMaxes(fx.snap)
		f1 := 0
		for i := range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			if i%2 == 1 {
				in = noDoseImprecise(rng, in, maxes, 200)
			}
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok || lo == hi {
				continue
			}
			wide, _ := evaluate(t, fx.engine, in)
			narrow := cloneInput(in)
			narrow.Profile.Birth = engine.RecalledDate{Value: lo.AddDays(rng.IntN(daysBetween(lo, hi) + 1)).String(), Precision: engine.Day}
			got, _ := evaluate(t, fx.engine, narrow)
			for j, a := range wide.Assessments {
				b := got.Assessments[j]
				hasDose := slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) })
				if hasDose && bracketed(fx.snap, a.Disease) && !slices.Equal(a.SeriesUsed, b.SeriesUsed) {
					continue // Q44: another bracket, other dates
				}
				alarming := b.Bucket == engine.BucketOverdue || b.Bucket == engine.BucketDue
				if fx.name == "minimal-start-age-gap-late" && !hasDose && alarming && hasCode(a, engine.ReasonBirthDateImprecise) &&
					!slices.Equal(noDoseChoice(fx.snap, a.Disease, lo, in.Today), noDoseChoice(fx.snap, a.Disease, hi, in.Today)) {
					latest := cloneInput(in)
					latest.Profile.Birth = engine.RecalledDate{Value: hi.String(), Precision: engine.Day}
					if out, _ := evaluate(t, fx.engine, latest); calm[visible(out.Assessments[j])] {
						f1++
					}
				}
				if calm[visible(a)] && alarming {
					t.Fatalf("%s %s: birth %v gives %s, narrowed to %s gives %s\nwide   %+v\nnarrow %+v\ninput  %+v",
						fx.name, a.Disease, in.Profile.Birth, visible(a), narrow.Profile.Birth.Value, visible(b), a, b, in)
				}
			}
		}
		if fx.name == "minimal-start-age-gap-late" && f1 == 0 {
			t.Errorf("%s: no random input reached the F1 shape", fx.name)
		}
	}
}

// bracketMaxes returns the startAge max of every series of s that has one.
func bracketMaxes(s *ruleset.Snapshot) []ruleset.Span {
	var out []ruleset.Span
	for _, sr := range s.Series {
		if sr.StartAge != nil && sr.StartAge.Max != nil {
			out = append(out, *sr.StartAge.Max)
		}
	}
	return out
}

// noDoseImprecise drops every dose of in and cuts a day-precise birth date
// to its month or, half the time, its year. When maxes is not empty, half
// the time it then moves today to the older birth end + a random max + 0 to
// window−1 days: a bracket has just closed at the older end and may still be
// open at the later one, so the birth range crosses a start-age change.
func noDoseImprecise(rng *rand.Rand, in engine.Input, maxes []ruleset.Span, window int) engine.Input {
	in.Doses = []engine.DoseRecord{}
	if b, err := engine.ParseDate(in.Profile.Birth.Value); err == nil && in.Profile.Birth.Precision == engine.Day {
		in.Profile.Birth = engine.RecalledDate{Value: b.String()[:7], Precision: engine.Month}
		if rng.IntN(2) == 0 {
			in.Profile.Birth = engine.RecalledDate{Value: b.String()[:4], Precision: engine.Year}
		}
	}
	if lo, _, ok := testPeriod(in.Profile.Birth, in.Today); ok && len(maxes) > 0 && rng.IntN(2) == 0 {
		in.Today = lo.AddSpan(maxes[rng.IntN(len(maxes))]).AddDays(rng.IntN(window))
	}
	return in
}

// TestPropertyNarrowingDoseDate: narrowing a month- or year-precision dose
// date to a day inside its (today-capped) range, with a day-precise birth
// date. A narrower range can be later than safe for the next dose, since the
// next dose counts from the later end of the wide range (F04.2), so the
// issue's property is checked in the two forms that hold for dose ranges:
//   - validity is monotone: when the wide result has its doses placed (no
//     OVERLAPPING_ESTIMATED_DATES or NUMBERING_CONFLICT, which narrowing can
//     resolve) and no DOSE_POSSIBLY_INVALID, the narrowed result has
//     DOSE_INVALID for the same doses and positions (a check failed at one
//     end only can join its checks) and no DOSE_POSSIBLY_INVALID, unless it is
//     ⚪ NUMBERING_CONFLICT: an overlap resolved by user numbers can turn
//     out to contradict the narrowed time order;
//   - when neither result is ⚪ and the next dose number is the same, the
//     narrowed due and overdue dates are never later than the wide ones,
//     and neither is the earliest date when every candidate series has one
//     plan. (With alternative plans the earliest date comes from the
//     earliest-due plan, A7, and that plan can change.)
//
// Both clauses are checked only when the narrowed result has the same
// seriesUsed as the wide one (issue #23, Q44 and its extension, SAD §5.7): a
// narrower dose range can match fewer start-age brackets, and the
// most-doses rule then selects another series (TestNarrowingChangesBracket
// pins both kinds of change). The skip applies only to diseases with
// start-age brackets (see bracketed). For the others the test keeps S3a's
// stronger check: the candidates do not depend on the birth date, and on
// dose dates only through the placement (issue #14), so the date clause
// fails when seriesUsed differs and the validity clause has no guard.
//
// Since S4a (issue #15) the validity clause also covers maximum ages: the
// doses not counted (DOSE_INVALID and DOSE_AFTER_MAX_AGE, by dose and
// position; uncountedKeys) and the presence of CHECK_PRODUCT_NAME or
// MIXED_PRODUCTS stay the same. A wide result with a DOSE_POSSIBLY_INVALID,
// minimum or maximum, is skipped as before: a possible failure may narrow to
// a definite one. Two readings of #15 are reported to the owner, each pinned
// by a test:
//   - an unnumbered level-4 record has no position of its own, so it is
//     keyed by its ID (TestNarrowingNamedFloatingDoseShape);
//   - a record whose DOSE_AFTER_MAX_AGE a disagreement drops (Q52) can show
//     a new DOSE_INVALID when a possible check hidden on it (Q62) narrows to
//     a definite one, with the same disagreement. Only such a record's
//     entries may change (engine.DisagreeingRecords,
//     TestNarrowingDisagreeingRecordShape).
//
// The near-limit inputs of the fixtures with maximum ages run the same
// check.
//
// Since S4b (issue #28) step 7 also gives CHECK_PRODUCT_NAME and
// MIXED_PRODUCTS when one candidate is out of age and another is not, from
// F, which counts from the latest dose's R.Hi: narrowing that dose can move
// F earlier and end the disagreement (the wide result is the more cautious
// ⚪). So the validity clause compares the step-5 disagreements only
// (engine.DisagreeingRecords). The date clause skips a pair where a result
// has a cap reason and a candidate has a later dose with minIntervalFromFirst:
// C_c,k measures it from dose 1's R.Hi, so narrowing dose 1 can move the
// last allowed day, and the capped dates, later. No fixture has both yet.
//
// Since the owner decision of 2026-11-24 (PR #31), a certain outOfAge is
// judged from the earliest possible dose dates, so a third clause holds: a
// wide outOfAge with doses given stays outOfAge when narrowed, with the
// same seriesUsed and its doses placed.
func TestPropertyNarrowingDoseDate(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(31, uint64(len(fx.name))))
		for range propertyRuns {
			checkNarrowingDoseDate(t, fx, rng, randomInput(t, rng, fx.snap))
		}
		limits := ageLimits(fx.snap)
		for range len(limits) * propertyRuns / 2 {
			checkNarrowingDoseDate(t, fx, rng, nearLimitInput(t, rng, fx.snap, limits))
		}
	}
}

// checkNarrowingDoseDate narrows one imprecise dose date of in to a random
// day inside it and checks TestPropertyNarrowingDoseDate's clauses. It
// reports whether in had a dose to narrow.
func checkNarrowingDoseDate(t *testing.T, fx fixture, rng *rand.Rand, in engine.Input) bool {
	t.Helper()
	if in.Profile.Birth.Precision != engine.Day {
		return false
	}
	var wideDoses []int
	for i, r := range in.Doses {
		if lo, hi, ok := testPeriod(r.Date, in.Today); ok && lo != hi {
			wideDoses = append(wideDoses, i)
		}
	}
	if len(wideDoses) == 0 {
		return false
	}
	wide, _ := evaluate(t, fx.engine, in)
	narrow := cloneInput(in)
	i := wideDoses[rng.IntN(len(wideDoses))]
	lo, hi, _ := testPeriod(in.Doses[i].Date, in.Today)
	// A day before birth would be a record problem, not a narrower date.
	lo = slices.MaxFunc([]engine.Date{lo, mustDate(t, in.Profile.Birth.Value)}, engine.Date.Compare)
	if lo.After(hi) {
		return false
	}
	narrow.Doses[i].Date = engine.RecalledDate{Value: lo.AddDays(rng.IntN(daysBetween(lo, hi) + 1)).String(), Precision: engine.Day}
	got, _ := evaluate(t, fx.engine, narrow)
	for j, a := range wide.Assessments {
		b := got.Assessments[j]
		sameSeries := slices.Equal(a.SeriesUsed, b.SeriesUsed)
		if bracketed(fx.snap, a.Disease) && !sameSeries {
			continue // Q44: another bracket
		}
		placed := len(reasonsWith(a, engine.ReasonOverlappingEstimatedDates)) == 0 && len(reasonsWith(a, engine.ReasonNumberingConflict)) == 0 &&
			len(reasonsWith(b, engine.ReasonNumberingConflict)) == 0
		if placed && len(reasonsWith(a, engine.ReasonDosePossiblyInvalid)) == 0 {
			ia, ib := uncountedKeys(a, in), uncountedKeys(b, in)
			da, db := len(disagreeingIn(t, fx.engine, a.Disease, in)) > 0, len(disagreeingIn(t, fx.engine, a.Disease, narrow)) > 0
			same := slices.Equal(ia, ib) && da == db
			if !same && da == db {
				// Known exception (Q52 with Q62, reported to the owner;
				// TestNarrowingDisagreeingRecordShape): a record whose
				// DOSE_AFTER_MAX_AGE a disagreement drops shows no reason
				// from the candidate that does not count it, so a possible
				// check hidden there can narrow to a definite one. Only
				// that record's entries may change.
				q := disagreeingIn(t, fx.engine, a.Disease, in, narrow)
				same = !slices.ContainsFunc(symmetricDifference(ia, ib), func(k string) bool { return !q[recordOf(k)] })
			}
			if !same || len(reasonsWith(b, engine.ReasonDosePossiblyInvalid)) > 0 {
				t.Fatalf("%s %s: narrowing %s to %s changed validity\nwide   %+v\nnarrow %+v\ninput  %+v",
					fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, a, b, in)
			}
		}
		// Certain outOfAge is judged from the earliest possible dose dates
		// (owner 2026-11-24), so a narrower date never undoes it.
		if a.Block == engine.BlockOutOfAge && a.SeriesUsed != nil && sameSeries && placed && b.Block != engine.BlockOutOfAge {
			t.Fatalf("%s %s: narrowing %s to %s undid outOfAge\nwide   %+v\nnarrow %+v\ninput  %+v",
				fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, a, b, in)
		}
		if a.NextDose == nil || b.NextDose == nil || a.NextDose.Number != b.NextDose.Number {
			continue
		}
		wn, nn := *a.NextDose, *b.NextDose
		if (isCapped(a) || isCapped(b)) && slices.ContainsFunc(seriesUsed(t, fx.snap, a), hasFromFirst) {
			continue
		}
		onePlan := !slices.ContainsFunc(seriesUsed(t, fx.snap, a), func(sr ruleset.Series) bool { return len(sr.Plans) > 1 })
		if !sameSeries || nn.Due.After(wn.Due) || nn.Overdue.After(wn.Overdue) ||
			(onePlan && nn.Earliest.After(wn.Earliest)) {
			t.Fatalf("%s %s: narrowing %s to %s moved a date later or changed seriesUsed\nwide   %+v %v\nnarrow %+v %v",
				fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, wn, a.SeriesUsed, nn, b.SeriesUsed)
		}
	}
	return true
}

// TestNarrowingFloatingDoseShapes pins two inputs the narrowing property
// missed at 400 runs: an unnumbered level-4 dose that fits among the dated
// doses only at the late end of an imprecise neighbour's range. The wide
// result must carry DOSE_POSSIBLY_INVALID (⚪, not ✅) whenever some day
// inside that range makes a dose DOSE_INVALID, or the narrowing property
// (SAD §5.7) fails for that day.
func TestNarrowingFloatingDoseShapes(t *testing.T) {
	t.Parallel()
	ptr := func(s string) *string { return &s }
	cases := []struct {
		fixture, disease, today, birth, month, narrow string
		product                                       *string
	}{
		// Rotarix: the level-4 dose fits as dose 1 only when r1 is on or
		// after birth + 6w + 4w = 2022-12-20; r1 + 4w is after today.
		{"illustrative-products", "rota", "2022-12-24", "2022-10-11", "2022-12", "2022-12-11", ptr("rotarix")},
		// The same shape with one series and no products (S2).
		{"illustrative-vn", "measles", "2025-11-24", "2025-01-11", "2025-11", "2025-11-05", nil},
	}
	for _, c := range cases {
		t.Run(c.fixture+"/"+c.disease, func(t *testing.T) {
			t.Parallel()
			e := loadEngine(t, c.fixture)
			in := engine.Input{
				Today:   mustDate(t, c.today),
				Profile: engine.Profile{Birth: engine.RecalledDate{Value: c.birth, Precision: engine.Day}},
				Doses: []engine.DoseRecord{
					{ID: "r1", Diseases: []string{c.disease}, Date: engine.RecalledDate{Value: c.month, Precision: engine.Month}, Product: c.product},
					{ID: "r5", Diseases: []string{c.disease}, Date: engine.RecalledDate{Precision: engine.Unknown}, Product: c.product},
				},
			}
			wide, _ := evaluate(t, e, in)
			narrow := cloneInput(in)
			narrow.Doses[0].Date = engine.RecalledDate{Value: c.narrow, Precision: engine.Day}
			got, _ := evaluate(t, e, narrow)
			a, b := wide.Assessments[0], got.Assessments[0]
			if a.Disease != c.disease {
				a, b = wide.Assessments[1], got.Assessments[1]
			}
			if len(invalidDoses(b)) > 0 && !hasCode(a, engine.ReasonDosePossiblyInvalid) {
				t.Fatalf("narrowing %s to %s made %v DOSE_INVALID, but the wide result has no DOSE_POSSIBLY_INVALID\nwide   %+v\nnarrow %+v",
					c.month, c.narrow, invalidDoses(b), a, b)
			}
			if a.Bucket == engine.BucketComplete {
				t.Fatalf("wide result is ✅ although a day inside %s makes a dose invalid: %+v", c.month, a)
			}
		})
	}
}

// TestFloatingDoseWideMatchesEveryDay checks both directions for one
// unnumbered level-4 dose beside an imprecise dated dose (SAD §5.3 step 5,
// §5.7): the wide result has DOSE_POSSIBLY_INVALID exactly when some day
// inside the range leaves no placement, that is when narrowing the range
// to that day makes a dose DOSE_INVALID. The narrowing property checks
// only that the wide result is not missing the reason; this also checks
// that it is not added when two placements that each fit at one end
// together cover every day.
func TestFloatingDoseWideMatchesEveryDay(t *testing.T) {
	t.Parallel()
	e := loadEngine(t, "minimal-single")
	cases := []struct {
		birth, value string
		precision    engine.Precision
		from, to     string // the todays tried, every third day
	}{
		{"2026-01-01", "2026-03", engine.Month, "2026-03-20", "2026-05-10"},
		{"2025-12-01", "2026", engine.Year, "2026-02-01", "2026-04-30"},
	}
	for _, c := range cases {
		for today := mustDate(t, c.from); !today.After(mustDate(t, c.to)); today = today.AddDays(3) {
			in := engine.Input{
				Today:   today,
				Profile: engine.Profile{Birth: engine.RecalledDate{Value: c.birth, Precision: engine.Day}},
				Doses: []engine.DoseRecord{
					{ID: "r1", Diseases: []string{"single"}, Date: engine.RecalledDate{Value: c.value, Precision: c.precision}},
					{ID: "r5", Diseases: []string{"single"}, Date: engine.RecalledDate{Precision: engine.Unknown}},
				},
			}
			wide, _ := evaluate(t, e, in)
			period, _, err := in.Doses[0].Date.Period()
			if err != nil {
				t.Fatal(err)
			}
			gap := ""
			for d := period.Lo; !d.After(minDateOf(period.Hi, today)) && gap == ""; d = d.AddDays(1) {
				narrow := cloneInput(in)
				narrow.Doses[0].Date = engine.RecalledDate{Value: d.String(), Precision: engine.Day}
				got, _ := evaluate(t, e, narrow)
				if len(invalidDoses(got.Assessments[0])) > 0 {
					gap = d.String()
				}
			}
			a := wide.Assessments[0]
			if possibly := hasCode(a, engine.ReasonDosePossiblyInvalid); possibly != (gap != "") {
				t.Errorf("r1 %s, today %s: DOSE_POSSIBLY_INVALID %v, but the first day leaving no placement is %q\nwide %+v",
					c.value, today, possibly, gap, a)
			}
		}
	}
}

func minDateOf(a, b engine.Date) engine.Date {
	if b.Before(a) {
		return b
	}
	return a
}

// TestPropertyDoseOnEarliestDateCounts: a dose given on the earliest allowed
// date never fails a minimum check (no new DOSE_INVALID, and no
// DOSE_POSSIBLY_INVALID listing minAge, minInterval or minIntervalFromFirst
// on it), and unless a maximum age questions it (maxAgeQuestioned) it is
// counted (the next dose number rises or the series completes). Since S4b
// the next dose is emitted only when it can still be given (issue #28), so
// a maximum age no longer questions the new dose; the "counted" clause
// skips a next dose with SERIES_MAY_NOT_COMPLETE, after which dose k+1 can
// be past every limit (outOfAge). It uses day-precise inputs: with a
// month-precision dose, a new exact dose can overlap its range and make the
// order unknown.
// The new dose has the first product the assessment offers, or no product
// when it offers none, so the candidates can only narrow: a dose with no
// product would add every series of the disease, and its earliest date can
// be before the minimums of a series that was not a candidate (issue #14).
// When the disease had no dose, the new dose can select another start-age
// bracket of the same product group (its age at dose 1 is that of the
// earliest date, which can be years before today), so only product groups
// are compared then (issue #23).
// The random inputs rarely reach a maximum age, so the property also runs
// on day-precise near-limit inputs (nearLimitDayInput), and some of their
// next doses must be capped (DUE_CAPPED_BY_MAX_AGE, SERIES_MAY_NOT_COMPLETE)
// or take the maximum-age skip, or the clauses are never tried at a
// maximum age.
func TestPropertyDoseOnEarliestDateCounts(t *testing.T) {
	t.Parallel()
	atMaxAge := 0 // results that reach a maximum age, from the near-limit inputs
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(13, uint64(len(fx.name))))
		for range propertyRuns {
			checkDoseOnEarliestDate(t, fx, randomDayInput(t, rng, fx.snap))
		}
		limits := ageLimits(fx.snap)
		rng = rand.New(rand.NewPCG(53, uint64(len(fx.name))))
		for range len(limits) * propertyRuns {
			atMaxAge += checkDoseOnEarliestDate(t, fx, nearLimitDayInput(t, rng, fx.snap, limits))
		}
	}
	if atMaxAge == 0 {
		t.Fatal("no near-limit input put a dose on the earliest date of a capped next dose or at a maximum age: the property is vacuous there")
	}
}

// checkDoseOnEarliestDate checks TestPropertyDoseOnEarliestDateCounts's
// clauses on one day-precise input and returns how many of its next doses
// are capped or have a result with the new dose that a maximum age
// questions (maxAgeQuestioned).
func checkDoseOnEarliestDate(t *testing.T, fx fixture, in engine.Input) (atMaxAge int) {
	t.Helper()
	out, _ := evaluate(t, fx.engine, in)
	for i, a := range out.Assessments {
		if a.NextDose == nil {
			continue
		}
		next := cloneInput(in)
		next.Today = slices.MaxFunc([]engine.Date{in.Today, a.NextDose.Earliest}, engine.Date.Compare)
		rec := engine.DoseRecord{
			ID:       "zz-on-earliest",
			Diseases: []string{a.Disease},
			Date:     engine.RecalledDate{Value: a.NextDose.Earliest.String(), Precision: engine.Day},
		}
		if len(a.Products) > 0 {
			rec.Product = &a.Products[0]
		}
		next.Doses = append(next.Doses, rec)
		after, _ := evaluate(t, fx.engine, next)
		b := after.Assessments[i]
		mayNotComplete := hasCode(a, engine.ReasonSeriesMayNotComplete)
		if mayNotComplete || hasCode(a, engine.ReasonDueCappedByMaxAge) {
			atMaxAge++
		}
		if !reflect.DeepEqual(reasonsWith(b, engine.ReasonDoseInvalid), reasonsWith(a, engine.ReasonDoseInvalid)) {
			t.Fatalf("%s %s: a dose on the earliest date %s changed DOSE_INVALID:\nbefore %v\nafter  %v",
				fx.name, a.Disease, a.NextDose.Earliest, a.Reasons, b.Reasons)
		}
		for _, r := range reasonsWith(b, engine.ReasonDosePossiblyInvalid) {
			if r.Params.Dose == rec.ID && slices.ContainsFunc(r.Params.Checks, func(c engine.Check) bool {
				return c == engine.CheckMinAge || c == engine.CheckMinInterval || c == engine.CheckMinIntervalFromFirst
			}) {
				t.Fatalf("%s %s: a dose on the earliest date %s may fail a minimum: %v", fx.name, a.Disease, a.NextDose.Earliest, b.Reasons)
			}
		}
		// Before S4b's cap, E could be at or past a maximum age (issue
		// #15): the new dose was then after it, possibly after it, or
		// questioned by the candidates, so the "no new candidate" and
		// "counted" clauses skip those results. Since S4b a next dose
		// is emitted only when it can still be given (issue #28), so
		// this is a guard.
		if maxAgeQuestioned(b) {
			atMaxAge++
			continue
		}
		if mayNotComplete {
			continue
		}
		hadDose := slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) })
		key := func(id string) string {
			if hadDose {
				return id
			}
			return productGroup(t, fx.snap, a.Disease, id)
		}
		var before []string
		for _, id := range a.SeriesUsed {
			before = append(before, key(id))
		}
		for _, id := range b.SeriesUsed {
			if !slices.Contains(before, key(id)) {
				t.Fatalf("%s %s: a dose of %v on the earliest date added candidate %s: before %v, after %v",
					fx.name, a.Disease, a.Products, id, a.SeriesUsed, b.SeriesUsed)
			}
		}
		completed := len(reasonsWith(b, engine.ReasonComplete)) > 0
		if !completed && (b.NextDose == nil || b.NextDose.Number != a.NextDose.Number+1) {
			t.Fatalf("%s %s: a dose on the earliest date %s was not counted: before %+v, after %+v",
				fx.name, a.Disease, a.NextDose.Earliest, a, b)
		}
	}
	return atMaxAge
}

// maxAgeQuestioned reports whether a has a maximum-age reason:
// DOSE_AFTER_MAX_AGE, a DOSE_POSSIBLY_INVALID listing a maximum age,
// CHECK_PRODUCT_NAME or MIXED_PRODUCTS.
func maxAgeQuestioned(a engine.Assessment) bool {
	return slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool {
		switch r.Code {
		case engine.ReasonDoseAfterMaxAge, engine.ReasonCheckProductName, engine.ReasonMixedProducts:
			return true
		case engine.ReasonDosePossiblyInvalid:
			return slices.Contains(r.Params.Checks, engine.CheckMaxAgeStart) || slices.Contains(r.Params.Checks, engine.CheckMaxAgeComplete)
		}
		return false
	})
}

// productGroup names the product group of a series of the disease: its
// product, or "" for a generic series.
func productGroup(t testing.TB, s *ruleset.Snapshot, disease, id string) string {
	t.Helper()
	sr := seriesUsed(t, s, engine.Assessment{Disease: disease, SeriesUsed: []string{id}})[0]
	if sr.Product == nil {
		return ""
	}
	return *sr.Product
}

// TestPropertyDeterministic: the output bytes do not depend on the order of
// dose records or of diseases within a record, repeated evaluation gives the
// same bytes, and neither the input nor the snapshot is modified.
func TestPropertyDeterministic(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(17, uint64(len(fx.name))))
		snapBefore := loadSnapshot(t, fx.name)
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			orig := cloneInput(in)
			_, first := evaluate(t, fx.engine, in)
			_, again := evaluate(t, fx.engine, in)
			if !bytes.Equal(first, again) {
				t.Fatalf("%s: two evaluations differ:\n%s\n%s", fx.name, first, again)
			}
			if !reflect.DeepEqual(in, orig) {
				t.Fatalf("%s: Evaluate modified its input", fx.name)
			}
			shuffled := cloneInput(in)
			rng.Shuffle(len(shuffled.Doses), func(i, j int) { shuffled.Doses[i], shuffled.Doses[j] = shuffled.Doses[j], shuffled.Doses[i] })
			for _, r := range shuffled.Doses {
				rng.Shuffle(len(r.Diseases), func(i, j int) { r.Diseases[i], r.Diseases[j] = r.Diseases[j], r.Diseases[i] })
			}
			if _, b := evaluate(t, fx.engine, shuffled); !bytes.Equal(first, b) {
				t.Fatalf("%s: record order changed the output:\n%s\n%s", fx.name, first, b)
			}
		}
		if !reflect.DeepEqual(fx.snap, snapBefore) {
			t.Fatalf("%s: the snapshot was modified", fx.name)
		}
	}
}

// hasCode reports whether a has a reason with code c.
func hasCode(a engine.Assessment, c engine.ReasonCode) bool { return len(reasonsWith(a, c)) > 0 }

// isCapped reports whether a has a cap reason (issue #28): its due and
// overdue dates may have been lowered to a last allowed day.
func isCapped(a engine.Assessment) bool {
	return hasCode(a, engine.ReasonDueCappedByMaxAge) || hasCode(a, engine.ReasonSeriesMayNotComplete)
}

// hasFromFirst reports whether some dose of sr has minIntervalFromFirst.
func hasFromFirst(sr ruleset.Series) bool {
	return slices.ContainsFunc(sr.Plans, func(p ruleset.Plan) bool {
		return slices.ContainsFunc(p.Doses, func(r ruleset.DoseRule) bool { return r.MinIntervalFromFirst != nil })
	})
}

// checkCandidates checks the S3a members of an assessment (issue #14):
//   - seriesUsed, when present, is not empty and names series of the
//     disease without repeats, in rule-set order;
//   - SAFEST_SERIES appears iff seriesUsed has two or more series, at most
//     once, and its candidates are seriesUsed with each series' product
//     (omitted for a generic series); with no seriesUsed there is neither.
//     With no dose given and a birth date, seriesUsed is the later end's
//     choice after A3's removal (noDoseOracle, Q40; issue #28 In scope 1),
//     and the birth range can cross start-age changes, which split it into
//     sub-ranges with one choice each (issue #23, PR #27). Only in a ⚪
//     BIRTH_DATE_IMPRECISE whose ends chose different lists does each
//     evaluated date keep its own SAFEST_SERIES (A8): one per run of
//     evaluated dates (the sub-range ends) with the same choice of two or
//     more series, from the latest to the oldest, without repeats, so
//     seriesUsed may have one series, or none when the later end has no
//     candidate left, while a SAFEST_SERIES is present. Ends that chose the
//     same list, or agree on a result that is not ⚪, keep the rule above;
//   - with no dose given, outOfAge has neither seriesUsed nor SAFEST_SERIES
//     (with doses given it keeps both, Q74);
//   - products present ⇒ nextDose present, and products are non-retired
//     catalog products that list the disease, in snapshot order without
//     repeats;
//   - with no dose of the disease, seriesUsed holds no series of a retired
//     product unless no series of a non-retired product (or generic) can
//     be started at the later end of the birth-date range, or every one
//     that can is removed by A3 (Q38, Q50 (b), Q72).
func checkCandidates(t testing.TB, fx fixture, in engine.Input, a engine.Assessment) {
	t.Helper()
	s := fx.snap
	fail := func(format string, args ...any) {
		t.Helper()
		t.Fatalf("%s %s: "+format+"\nassessment %+v\ninput %+v", append(append([]any{fx.name, a.Disease}, args...), a, in)...)
	}
	if a.SeriesUsed != nil && len(a.SeriesUsed) == 0 {
		fail("empty seriesUsed")
	}
	used := seriesUsed(t, s, a)
	index := func(sr ruleset.Series) int {
		return slices.IndexFunc(s.Series, func(x ruleset.Series) bool { return x.ID == sr.ID })
	}
	for i := 1; i < len(used); i++ {
		if index(used[i-1]) >= index(used[i]) {
			fail("seriesUsed %v is not in rule-set order without repeats", a.SeriesUsed)
		}
	}
	safest := reasonsWith(a, engine.ReasonSafestSeries)
	hasDose := slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) })
	refs := func(used []ruleset.Series) []engine.SeriesRef {
		var out []engine.SeriesRef
		for _, sr := range used {
			ref := engine.SeriesRef{Series: sr.ID}
			if sr.Product != nil {
				ref.Product = *sr.Product
			}
			out = append(out, ref)
		}
		return out
	}
	differ := false // no dose, and the birth ends chose different lists
	if lo, hi, ok := testPeriod(in.Profile.Birth, in.Today); ok && !hasDose {
		o := noDoseOracle{s: s, disease: a.Disease, lo: lo, today: in.Today}
		loEnd, _ := o.choice(lo)
		hiEnd, _ := o.choice(hi)
		if !slices.Equal(a.SeriesUsed, hiEnd) && (len(a.SeriesUsed) > 0 || len(hiEnd) > 0) {
			fail("no dose: seriesUsed %v, want the later end's choice %v", a.SeriesUsed, hiEnd)
		}
		if differ = !slices.Equal(loEnd, hiEnd); differ && hasCode(a, engine.ReasonBirthDateImprecise) {
			// The evaluated dates are the ends of the sub-ranges where the
			// raw choice is fixed, latest first.
			var points []engine.Date
			for b := hi; !b.Before(lo); b = b.AddDays(-1) {
				switch {
				case b == hi || b == lo:
					points = append(points, b)
				case !slices.Equal(o.raw(b), o.raw(b.AddDays(1))) || !slices.Equal(o.raw(b), o.raw(b.AddDays(-1))):
					points = append(points, b)
				}
			}
			var runs [][]string // each run's choice, latest first
			for _, b := range points {
				if c, _ := o.choice(b); len(runs) == 0 || !slices.Equal(c, runs[len(runs)-1]) {
					runs = append(runs, c)
				}
			}
			var want [][]engine.SeriesRef
			for _, run := range runs {
				ref := refs(seriesUsed(t, s, engine.Assessment{Disease: a.Disease, SeriesUsed: run}))
				if len(run) >= 2 && !slices.ContainsFunc(want, func(w []engine.SeriesRef) bool { return slices.Equal(w, ref) }) {
					want = append(want, ref)
				}
			}
			var got [][]engine.SeriesRef
			for _, r := range safest {
				got = append(got, r.Params.Candidates)
			}
			if !reflect.DeepEqual(got, want) {
				fail("⚪ across start-age changes, the runs chose %v (latest first): SAFEST_SERIES %v, want %v", runs, got, want)
			}
		}
	}
	switch {
	case a.Block == engine.BlockOutOfAge && !hasDose && (a.SeriesUsed != nil || len(safest) > 0):
		fail("outOfAge with seriesUsed %v and %d SAFEST_SERIES", a.SeriesUsed, len(safest))
	case differ && hasCode(a, engine.ReasonBirthDateImprecise):
		// Checked above against each end's choice.
	case len(used) < 2 && len(safest) > 0:
		fail("SAFEST_SERIES with seriesUsed %v", a.SeriesUsed)
	case len(used) >= 2 && len(safest) != 1:
		fail("seriesUsed %v with %d SAFEST_SERIES reasons", a.SeriesUsed, len(safest))
	case len(used) >= 2:
		if want := refs(used); !slices.Equal(safest[0].Params.Candidates, want) {
			fail("SAFEST_SERIES candidates %v, want %v", safest[0].Params.Candidates, want)
		}
	}
	if len(a.Products) > 0 && a.NextDose == nil {
		fail("products %v without nextDose", a.Products)
	}
	last := -1
	for _, id := range a.Products {
		i := slices.IndexFunc(s.Products, func(p ruleset.Product) bool { return p.ID == id })
		if i < 0 || s.Products[i].Retired || !slices.Contains(s.Products[i].Diseases, a.Disease) || i <= last {
			fail("products %v: %q is not a non-retired product of the disease in snapshot order", a.Products, id)
		}
		last = i
	}
	retired := func(sr ruleset.Series) bool {
		return sr.Product != nil && slices.ContainsFunc(s.Products, func(p ruleset.Product) bool { return p.ID == *sr.Product && p.Retired })
	}
	if birthLo, birthHi, ok := testPeriod(in.Profile.Birth, in.Today); ok && !hasDose {
		o := noDoseOracle{s: s, disease: a.Disease, lo: birthLo, today: in.Today}
		var open []string
		for _, sr := range s.Series {
			if sr.Disease == a.Disease && !retired(sr) && bracketOpen(sr, birthHi, in.Today) {
				open = append(open, sr.ID)
			}
		}
		kept, _ := o.filter(o.raw(birthHi), birthHi, o.subStart(birthHi))
		openOther := len(open) > 0 && slices.ContainsFunc(kept, func(id string) bool { return slices.Contains(open, id) })
		if openOther && slices.ContainsFunc(used, retired) {
			fail("no dose, but seriesUsed %v has a retired product's series", a.SeriesUsed)
		}
	}
}

// bracketOpen reports whether a person born on b can still start series sr
// on today: no startAge, no max, or b + max after today (EQ10).
func bracketOpen(sr ruleset.Series, b, today engine.Date) bool {
	return sr.StartAge == nil || sr.StartAge.Max == nil || b.AddSpan(*sr.StartAge.Max).After(today)
}

// noDoseChoice returns the IDs of the series chosen at birth end b on today
// when no dose of the disease is given (issue #23 In scope 5), computed from
// the snapshot with Date.AddSpan alone: from the series of non-retired
// products (every series of the disease when none is left, Q38), each
// product group keeps its first bracket by min that is still open at b
// (bracketOpen: the bracket the age falls in, the next one above a gap, or
// the earliest one, EQ10, A5, Q33). When no group keeps one, the choice is
// redone over every series of the disease (Q50 (b)). In rule-set order; empty
// means outOfAge (Q24).
func noDoseChoice(s *ruleset.Snapshot, disease string, b, today engine.Date) []string {
	retired := func(sr ruleset.Series) bool {
		return sr.Product != nil && slices.ContainsFunc(s.Products, func(p ruleset.Product) bool { return p.ID == *sr.Product && p.Retired })
	}
	choose := func(keep func(ruleset.Series) bool) []string {
		first := map[string]int{} // product group ("" generic) → index of its chosen series
		for i, sr := range s.Series {
			if sr.Disease != disease || !keep(sr) || !bracketOpen(sr, b, today) {
				continue
			}
			g := ""
			if sr.Product != nil {
				g = *sr.Product
			}
			if j, ok := first[g]; !ok || (sr.StartAge != nil && b.AddSpan(sr.StartAge.Min).Before(b.AddSpan(s.Series[j].StartAge.Min))) {
				first[g] = i
			}
		}
		var out []string
		for i, sr := range s.Series {
			if slices.Contains(slices.Collect(maps.Values(first)), i) {
				out = append(out, sr.ID)
			}
		}
		return out
	}
	keep := func(sr ruleset.Series) bool { return !retired(sr) }
	if !slices.ContainsFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == disease && keep(sr) }) {
		keep = func(ruleset.Series) bool { return true }
	}
	if c := choose(keep); len(c) > 0 {
		return c
	}
	return choose(func(ruleset.Series) bool { return true })
}

// TestPropertyCandidates runs checkCandidates on random inputs.
func TestPropertyCandidates(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(37, uint64(len(fx.name))))
		for i := range 2 * propertyRuns {
			in := randomInput(t, rng, fx.snap)
			if i%2 == 0 {
				in = randomDayInput(t, rng, fx.snap)
			}
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				checkCandidates(t, fx, in, a)
			}
		}
	}
}

// TestPropertyUnknownProductNeverLessSafe (P1): replacing a dose's catalog
// product with no product never turns a result that is not ✅ COMPLETE into
// ✅ COMPLETE, and when both results have a next dose with the same number
// it never moves earliest earlier, nor due or overdue when neither result
// has a cap reason (issue #14; issue #28: a superset of candidates can lower
// the smallest L).
func TestPropertyUnknownProductNeverLessSafe(t *testing.T) {
	t.Parallel()
	complete := func(a engine.Assessment) bool {
		return a.Bucket == engine.BucketComplete && hasCode(a, engine.ReasonComplete)
	}
	for _, fx := range fixtures(t) {
		if len(fx.snap.Products) == 0 {
			continue
		}
		rng := rand.New(rand.NewPCG(41, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			var known []int
			for i, r := range in.Doses {
				if r.Product != nil && *r.Product != unknownProductID {
					known = append(known, i)
				}
			}
			if len(known) == 0 {
				continue
			}
			out, _ := evaluate(t, fx.engine, in)
			unknown := cloneInput(in)
			i := known[rng.IntN(len(known))]
			unknown.Doses[i].Product = nil
			got, _ := evaluate(t, fx.engine, unknown)
			for j, a := range out.Assessments {
				b := got.Assessments[j]
				if !complete(a) && complete(b) {
					t.Fatalf("%s %s: removing product %s of %s made the result complete\nknown   %+v\nunknown %+v\ninput %+v",
						fx.name, a.Disease, *in.Doses[i].Product, in.Doses[i].ID, a, b, in)
				}
				if a.NextDose == nil || b.NextDose == nil || a.NextDose.Number != b.NextDose.Number {
					continue
				}
				uncapped := !isCapped(a) && !isCapped(b)
				if b.NextDose.Earliest.Before(a.NextDose.Earliest) || uncapped && (b.NextDose.Due.Before(a.NextDose.Due) || b.NextDose.Overdue.Before(a.NextDose.Overdue)) {
					t.Fatalf("%s %s: removing product %s of %s moved a date earlier\nknown   %+v\nunknown %+v\ninput %+v",
						fx.name, a.Disease, *in.Doses[i].Product, in.Doses[i].ID, *a.NextDose, *b.NextDose, in)
				}
			}
		}
	}
}

// TestPropertyMergeCoversCandidates compares the merged result with each
// candidate alone: a snapshot that keeps only that series for the disease,
// so that every tier falls back to it (issue #14, EQ6 item 2).
//   - When both have a next dose with the same number, the merged earliest
//     date is on or after the candidate's, and so are due and overdue when
//     the merged result has no cap reason (the smallest L over the
//     candidates can lower them, issue #28).
//   - With a day-precise birth date and no blocking reason, every candidate
//     with a next dose has the merged next dose number: candidates disagree
//     on k only when some candidate has a blocking reason.
func TestPropertyMergeCoversCandidates(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		alone := map[string]*engine.Engine{}
		single := func(id string) *engine.Engine {
			if e, ok := alone[id]; ok {
				return e
			}
			s := loadSnapshot(t, fx.name)
			disease := s.Series[slices.IndexFunc(s.Series, func(sr ruleset.Series) bool { return sr.ID == id })].Disease
			s.Series = slices.DeleteFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == disease && sr.ID != id })
			e, err := engine.New(s)
			if err != nil {
				t.Fatalf("%s with %s alone: %v", fx.name, id, err)
			}
			alone[id] = e
			return e
		}
		rng := rand.New(rand.NewPCG(43, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			for j, a := range out.Assessments {
				if len(a.SeriesUsed) < 2 {
					continue
				}
				blocked := slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool { return blockingCodes[r.Code] })
				for _, id := range a.SeriesUsed {
					o, _ := evaluate(t, single(id), in)
					b := o.Assessments[j]
					if a.NextDose != nil && b.NextDose != nil && a.NextDose.Number == b.NextDose.Number &&
						(a.NextDose.Earliest.Before(b.NextDose.Earliest) || !isCapped(a) && (a.NextDose.Due.Before(b.NextDose.Due) || a.NextDose.Overdue.Before(b.NextDose.Overdue))) {
						t.Fatalf("%s %s: merged %+v is before %s alone %+v\ninput %+v", fx.name, a.Disease, *a.NextDose, id, *b.NextDose, in)
					}
					if in.Profile.Birth.Precision == engine.Day && !blocked && b.NextDose != nil &&
						(a.NextDose == nil || a.NextDose.Number != b.NextDose.Number) {
						t.Fatalf("%s %s: no blocking reason, but %s alone needs dose %d\nmerged %+v\nalone  %+v\ninput %+v",
							fx.name, a.Disease, id, b.NextDose.Number, a, b, in)
					}
				}
			}
		}
	}
}
