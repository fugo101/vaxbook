package engine_test

import (
	"bytes"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
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

func evaluate(t testing.TB, e *engine.Engine, in engine.Input) (engine.Output, []byte) {
	t.Helper()
	out, err := e.Evaluate(in)
	if err != nil {
		t.Fatalf("Evaluate(%+v): %v", in, err)
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
//     make it ⚪ on their own), and a blocking reason makes it ⚪ unless a
//     block of a higher layer masks it (SAD §5.3 step 10, layers 1–5; S2
//     has none, and notYetScheduled is layer 8);
//   - a next dose unless the series is complete or the bucket is ⚪ (a ⚪
//     assessment never carries a next dose, SAD §5.1);
//   - due ≥ earliest and overdue ≥ due + 7 days.
func checkAssessmentShape(t testing.TB, name string, a engine.Assessment) {
	t.Helper()
	checkReasonParams(t, a)
	if (a.Bucket == "") == (a.Block == "") {
		t.Fatalf("%s %s: bucket %q, block %q", name, a.Disease, a.Bucket, a.Block)
	}
	blocked := slices.ContainsFunc(a.Reasons, func(r engine.Reason) bool { return blockingCodes[r.Code] })
	masked := a.Block != "" && a.Block != engine.BlockNotYetScheduled
	if a.Bucket == engine.BucketUnknown && !blocked || blocked && a.Bucket != engine.BucketUnknown && !masked {
		t.Fatalf("%s %s: bucket %q, block %q with reasons %v", name, a.Disease, a.Bucket, a.Block, a.Reasons)
	}
	noNext := len(reasonsWith(a, engine.ReasonComplete)) > 0 || a.Bucket == engine.BucketUnknown
	if noNext != (a.NextDose == nil) {
		t.Fatalf("%s %s: nextDose %v with bucket %q and reasons %v", name, a.Disease, a.NextDose, a.Bucket, a.Reasons)
	}
	if nd := a.NextDose; nd != nil {
		if nd.Due.Before(nd.Earliest) {
			t.Fatalf("%s %s: due %s before earliest %s", name, a.Disease, nd.Due, nd.Earliest)
		}
		if nd.Overdue.Before(nd.Due.AddDays(engine.OverdueGraceDays)) {
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

// TestPropertyNoBirthDateNoAgeCheck: with no birth date, the minimum age
// never makes a dose invalid or possibly invalid (SAD §5.7, EQ19). Every
// random input is also evaluated with its birth date removed.
func TestPropertyNoBirthDateNoAgeCheck(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(19, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			in.Profile.Birth = engine.RecalledDate{Precision: engine.None}
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				if !a.Estimated {
					t.Fatalf("%s %s: no birth date but not estimated", fx.name, a.Disease)
				}
				for _, r := range a.Reasons {
					if slices.Contains(r.Params.Checks, engine.CheckMinAge) {
						t.Fatalf("%s %s: no birth date but %s checks the minimum age (input %+v)", fx.name, a.Disease, r.Code, in)
					}
				}
			}
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

// invalidDoses lists the dose and position of each DOSE_INVALID reason.
func invalidDoses(a engine.Assessment) []string {
	var out []string
	for _, r := range reasonsWith(a, engine.ReasonDoseInvalid) {
		out = append(out, fmt.Sprintf("%s#%d", r.Params.Dose, r.Params.Number))
	}
	return out
}

// visible is how an assessment shows: its bucket or block.
func visible(a engine.Assessment) string { return string(a.Bucket) + string(a.Block) }

// TestPropertyNarrowingBirthDate: narrowing a month- or year-precision birth
// date to a day inside its (today-capped) range never turns a ✅, 🟡 or
// notYetScheduled result into 🔴 or 🟠 (issue #13). The wide result is
// computed at both ends of the range, and steps 5–9 are monotone in the
// birth date between them.
func TestPropertyNarrowingBirthDate(t *testing.T) {
	t.Parallel()
	calm := map[string]bool{string(engine.BucketComplete): true, string(engine.BucketUpcoming): true, string(engine.BlockNotYetScheduled): true}
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(29, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			lo, hi, ok := testPeriod(in.Profile.Birth, in.Today)
			if !ok || lo == hi {
				continue
			}
			wide, _ := evaluate(t, fx.engine, in)
			narrow := cloneInput(in)
			narrow.Profile.Birth = engine.RecalledDate{Value: lo.AddDays(rng.IntN(daysBetween(lo, hi) + 1)).String(), Precision: engine.Day}
			got, _ := evaluate(t, fx.engine, narrow)
			for i, a := range wide.Assessments {
				b := got.Assessments[i]
				if calm[visible(a)] && (b.Bucket == engine.BucketOverdue || b.Bucket == engine.BucketDue) {
					t.Fatalf("%s %s: birth %v gives %s, narrowed to %s gives %s\nwide   %+v\nnarrow %+v",
						fx.name, a.Disease, in.Profile.Birth, visible(a), narrow.Profile.Birth.Value, visible(b), a, b)
				}
			}
		}
	}
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
//     earliest-due plan, A7, and that plan can change.) In S3a the
//     candidates do not depend on the birth date, and on dose dates only
//     through the placement, which the checked cases share (issue #14).
func TestPropertyNarrowingDoseDate(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(31, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			if in.Profile.Birth.Precision != engine.Day {
				continue
			}
			var wideDoses []int
			for i, r := range in.Doses {
				if lo, hi, ok := testPeriod(r.Date, in.Today); ok && lo != hi {
					wideDoses = append(wideDoses, i)
				}
			}
			if len(wideDoses) == 0 {
				continue
			}
			wide, _ := evaluate(t, fx.engine, in)
			narrow := cloneInput(in)
			i := wideDoses[rng.IntN(len(wideDoses))]
			lo, hi, _ := testPeriod(in.Doses[i].Date, in.Today)
			// A day before birth would be a record problem, not a narrower date.
			lo = slices.MaxFunc([]engine.Date{lo, mustDate(t, in.Profile.Birth.Value)}, engine.Date.Compare)
			if lo.After(hi) {
				continue
			}
			narrow.Doses[i].Date = engine.RecalledDate{Value: lo.AddDays(rng.IntN(daysBetween(lo, hi) + 1)).String(), Precision: engine.Day}
			got, _ := evaluate(t, fx.engine, narrow)
			for j, a := range wide.Assessments {
				b := got.Assessments[j]
				placed := len(reasonsWith(a, engine.ReasonOverlappingEstimatedDates)) == 0 && len(reasonsWith(a, engine.ReasonNumberingConflict)) == 0 &&
					len(reasonsWith(b, engine.ReasonNumberingConflict)) == 0
				if placed && len(reasonsWith(a, engine.ReasonDosePossiblyInvalid)) == 0 {
					if !slices.Equal(invalidDoses(a), invalidDoses(b)) ||
						len(reasonsWith(b, engine.ReasonDosePossiblyInvalid)) > 0 {
						t.Fatalf("%s %s: narrowing %s to %s changed validity\nwide   %+v\nnarrow %+v\ninput  %+v",
							fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, a, b, in)
					}
				}
				if a.NextDose == nil || b.NextDose == nil || a.NextDose.Number != b.NextDose.Number {
					continue
				}
				wn, nn := *a.NextDose, *b.NextDose
				onePlan := !slices.ContainsFunc(seriesUsed(t, fx.snap, a), func(sr ruleset.Series) bool { return len(sr.Plans) > 1 })
				if !slices.Equal(a.SeriesUsed, b.SeriesUsed) || nn.Due.After(wn.Due) || nn.Overdue.After(wn.Overdue) ||
					(onePlan && nn.Earliest.After(wn.Earliest)) {
					t.Fatalf("%s %s: narrowing %s to %s moved a date later\nwide   %+v\nnarrow %+v",
						fx.name, a.Disease, in.Doses[i].Date.Value, narrow.Doses[i].Date.Value, wn, nn)
				}
			}
		}
	}
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
// date is counted (the next dose number rises or the series completes) and
// adds no DOSE_INVALID. It uses day-precise inputs: with a month-precision
// dose, a new exact dose can overlap its range and make the order unknown.
// The new dose has the first product the assessment offers, or no product
// when it offers none, so the candidates can only narrow: a dose with no
// product would add every series of the disease, and its earliest date can
// be before the minimums of a series that was not a candidate (issue #14).
func TestPropertyDoseOnEarliestDateCounts(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(13, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomDayInput(t, rng, fx.snap)
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
				for _, id := range b.SeriesUsed {
					if !slices.Contains(a.SeriesUsed, id) {
						t.Fatalf("%s %s: a dose of %v on the earliest date added candidate %s: before %v, after %v",
							fx.name, a.Disease, a.Products, id, a.SeriesUsed, b.SeriesUsed)
					}
				}
				if !reflect.DeepEqual(reasonsWith(b, engine.ReasonDoseInvalid), reasonsWith(a, engine.ReasonDoseInvalid)) {
					t.Fatalf("%s %s: a dose on the earliest date %s changed DOSE_INVALID:\nbefore %v\nafter  %v",
						fx.name, a.Disease, a.NextDose.Earliest, a.Reasons, b.Reasons)
				}
				completed := len(reasonsWith(b, engine.ReasonComplete)) > 0
				if !completed && (b.NextDose == nil || b.NextDose.Number != a.NextDose.Number+1) {
					t.Fatalf("%s %s: a dose on the earliest date %s was not counted: before %+v, after %+v",
						fx.name, a.Disease, a.NextDose.Earliest, a, b)
				}
			}
		}
	}
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

// checkCandidates checks the S3a members of an assessment (issue #14):
//   - seriesUsed, when present, is not empty and names series of the
//     disease without repeats, in rule-set order;
//   - SAFEST_SERIES appears iff seriesUsed has two or more series, at most
//     once, and its candidates are seriesUsed with each series' product
//     (omitted for a generic series); with no seriesUsed there is neither;
//   - products present ⇒ nextDose present, and products are non-retired
//     catalog products that list the disease, in snapshot order without
//     repeats;
//   - with no dose of the disease, seriesUsed holds no series of a retired
//     product unless every series of the disease has one (Q38).
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
	switch {
	case len(used) < 2 && len(safest) > 0:
		fail("SAFEST_SERIES with seriesUsed %v", a.SeriesUsed)
	case len(used) >= 2 && len(safest) != 1:
		fail("seriesUsed %v with %d SAFEST_SERIES reasons", a.SeriesUsed, len(safest))
	case len(used) >= 2:
		var want []engine.SeriesRef
		for _, sr := range used {
			ref := engine.SeriesRef{Series: sr.ID}
			if sr.Product != nil {
				ref.Product = *sr.Product
			}
			want = append(want, ref)
		}
		if !slices.Equal(safest[0].Params.Candidates, want) {
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
	hasDose := slices.ContainsFunc(in.Doses, func(r engine.DoseRecord) bool { return slices.Contains(r.Diseases, a.Disease) })
	allRetired := !slices.ContainsFunc(s.Series, func(sr ruleset.Series) bool { return sr.Disease == a.Disease && !retired(sr) })
	if !hasDose && !allRetired && slices.ContainsFunc(used, retired) {
		fail("no dose, but seriesUsed %v has a retired product's series", a.SeriesUsed)
	}
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
// it never moves earliest, due or overdue earlier (issue #14).
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
				if b.NextDose.Earliest.Before(a.NextDose.Earliest) || b.NextDose.Due.Before(a.NextDose.Due) || b.NextDose.Overdue.Before(a.NextDose.Overdue) {
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
//   - When both have a next dose with the same number, the merged earliest,
//     due and overdue dates are each on or after the candidate's.
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
						(a.NextDose.Earliest.Before(b.NextDose.Earliest) || a.NextDose.Due.Before(b.NextDose.Due) || a.NextDose.Overdue.Before(b.NextDose.Overdue)) {
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
