package engine_test

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Property tests of maximum ages (issue #15, S4a). randomInputOf draws ages
// up to 20 years, which never reaches minimal-max-age-complete's 20-year
// limit and rarely lands near the 12–32 week rota limits, so these
// properties also draw near-limit inputs (nearLimitInput). Changing
// randomInputOf would shift every seeded sequence of the earlier
// properties.

// ageLimits returns the maximum ages of a snapshot's series (maxAgeStart,
// maxAgeComplete) and its start-age bracket maxes, without repeats; empty
// for a snapshot with none.
func ageLimits(s *ruleset.Snapshot) []ruleset.Span {
	var out []ruleset.Span
	add := func(sp *ruleset.Span) {
		if sp != nil && !slices.Contains(out, *sp) {
			out = append(out, *sp)
		}
	}
	for _, sr := range s.Series {
		add(sr.MaxAgeStart)
		add(sr.MaxAgeComplete)
		if sr.StartAge != nil {
			add(sr.StartAge.Max)
		}
	}
	return out
}

// nearLimitInput draws an input whose today is within 60 days of birth + one
// of limits, with 1–4 dose records dated in the 20 weeks before today: day
// (50%), month (25%) or level 4 (25%) precise, about one in ten numbered
// with its true rank or a random number from 1 to 4, and products as
// randomProduct draws them. The birth date is day (80%) or month (10%)
// precise, or absent (10%). limits must not be empty.
func nearLimitInput(t testing.TB, rng *rand.Rand, s *ruleset.Snapshot, limits []ruleset.Span) engine.Input {
	t.Helper()
	birth := mustDate(t, "1990-01-01").AddDays(rng.IntN(36 * 365))
	today := birth.AddSpan(limits[rng.IntN(len(limits))]).AddDays(rng.IntN(121) - 60)
	if today.Before(birth) {
		today = birth
	}
	from := today.AddDays(-140)
	if from.Before(birth) {
		from = birth
	}
	in := engine.Input{Today: today, Profile: engine.Profile{Birth: engine.RecalledDate{Value: birth.String(), Precision: engine.Day}}}
	switch rng.IntN(10) {
	case 0:
		in.Profile.Birth = engine.RecalledDate{Value: birth.String()[:7], Precision: engine.Month}
	case 1:
		in.Profile.Birth = engine.RecalledDate{Precision: engine.None}
	}
	trueDates := map[string]engine.Date{}
	for i := range 1 + rng.IntN(4) {
		var diseases []string
		for _, d := range s.Diseases {
			if rng.IntN(2) == 0 {
				diseases = append(diseases, d.ID)
			}
		}
		if len(diseases) == 0 {
			diseases = []string{s.Diseases[rng.IntN(len(s.Diseases))].ID}
		}
		date := from.AddDays(rng.IntN(daysBetween(from, today) + 1))
		rec := engine.DoseRecord{ID: fmt.Sprintf("r%d", i), Diseases: diseases, Date: engine.RecalledDate{Value: date.String(), Precision: engine.Day}}
		switch n := rng.IntN(100); {
		case n < 25:
			rec.Date = engine.RecalledDate{Value: date.String()[:7], Precision: engine.Month}
		case n < 50:
			rec.Date = engine.RecalledDate{Precision: engine.Unknown}
		}
		randomProduct(rng, s, &rec)
		trueDates[rec.ID] = date
		in.Doses = append(in.Doses, rec)
	}
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
	return in
}

// TestPropertyProductDisagreement (issue #15 In scope 5, 6; Q53, Q54):
// CHECK_PRODUCT_NAME and MIXED_PRODUCTS appear only with a dose of the
// disease and two or more series in seriesUsed, never both. MIXED_PRODUCTS
// appears iff every record of the disease is named (a catalog product that
// lists the disease, retired allowed, or an off-catalog product) and there
// is no implied dose (hasImpliedDose, including one above every series
// length, which IMPLIED_EARLIER_DOSES does not list); otherwise it is
// CHECK_PRODUCT_NAME. Named and unknown are re-derived from the input and
// the snapshot. The near-limit inputs must reach both reasons and
// DOSE_AFTER_MAX_AGE, or the property is vacuous.
func TestPropertyProductDisagreement(t *testing.T) {
	t.Parallel()
	seen := map[engine.ReasonCode]int{}
	for _, fx := range fixtures(t) {
		listing := map[string]map[string]bool{} // product → diseases it lists
		for _, p := range fx.snap.Products {
			listing[p.ID] = map[string]bool{}
			for _, d := range p.Diseases {
				listing[p.ID][d] = true
			}
		}
		rng := rand.New(rand.NewPCG(47, uint64(len(fx.name))))
		inputs := make([]engine.Input, 0, propertyRuns)
		for range propertyRuns {
			inputs = append(inputs, randomInput(t, rng, fx.snap))
		}
		limits := ageLimits(fx.snap)
		for range len(limits) * propertyRuns {
			inputs = append(inputs, nearLimitInput(t, rng, fx.snap, limits))
		}
		for _, in := range inputs {
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				checkAssessmentShape(t, fx.name, a)
				for _, c := range []engine.ReasonCode{engine.ReasonCheckProductName, engine.ReasonMixedProducts, engine.ReasonDoseAfterMaxAge} {
					if hasCode(a, c) {
						seen[c]++
					}
				}
				cpn, mixed := hasCode(a, engine.ReasonCheckProductName), hasCode(a, engine.ReasonMixedProducts)
				if !cpn && !mixed {
					continue
				}
				named, any := true, false
				for _, r := range in.Doses {
					if !slices.Contains(r.Diseases, a.Disease) {
						continue
					}
					any = true
					named = named && (r.FreeProduct != nil || r.Product != nil && listing[*r.Product][a.Disease])
				}
				named = named && !hasImpliedDose(t, in, a.Disease)
				switch {
				case cpn && mixed:
					t.Fatalf("%s %s: both CHECK_PRODUCT_NAME and MIXED_PRODUCTS: %+v", fx.name, a.Disease, a)
				case !any || len(a.SeriesUsed) < 2:
					t.Fatalf("%s %s: %v with no dose or one series: %+v\ninput %+v", fx.name, a.Disease, a.Reasons, a, in)
				case mixed != named:
					t.Fatalf("%s %s: MIXED_PRODUCTS %v but every dose named %v: %+v\ninput %+v", fx.name, a.Disease, mixed, named, a, in)
				}
			}
		}
	}
	for _, c := range []engine.ReasonCode{engine.ReasonCheckProductName, engine.ReasonMixedProducts, engine.ReasonDoseAfterMaxAge} {
		if seen[c] == 0 {
			t.Errorf("no input reached %s; the property is vacuous", c)
		}
	}
}

// TestPropertyMaxAgeCoverage: the near-limit inputs reach the maximum-age
// outcomes on the fixtures with maximum ages, so the properties that run them
// (TestPropertyNarrowingDoseDate, TestPropertyNoBirthDateNoAgeCheck) cannot
// become vacuous: DOSE_AFTER_MAX_AGE and DOSE_POSSIBLY_INVALID for each
// maximum-age check, on a dated and on an undated record. The one exception
// is a possible maxAgeStart on an undated record: a slot at position 1
// follows only an uncounted dated dose (before its minimum age, or already
// definitely past the start limit), so its chain dates never straddle the
// limit.
func TestPropertyMaxAgeCoverage(t *testing.T) {
	t.Parallel()
	seen := map[string]int{}
	for _, fx := range fixtures(t) {
		limits := ageLimits(fx.snap)
		if len(limits) == 0 {
			continue
		}
		undated := func(in engine.Input, id string) bool {
			i := slices.IndexFunc(in.Doses, func(r engine.DoseRecord) bool { return r.ID == id })
			return i >= 0 && in.Doses[i].Date.Precision == engine.Unknown
		}
		rng := rand.New(rand.NewPCG(53, uint64(len(fx.name))))
		for range len(limits) * propertyRuns {
			in := nearLimitInput(t, rng, fx.snap, limits)
			out, _ := evaluate(t, fx.engine, in)
			for _, a := range out.Assessments {
				for _, r := range a.Reasons {
					if r.Code != engine.ReasonDoseAfterMaxAge && r.Code != engine.ReasonDosePossiblyInvalid {
						continue
					}
					for _, c := range r.Params.Checks {
						if c == engine.CheckMaxAgeStart || c == engine.CheckMaxAgeComplete {
							seen[fmt.Sprintf("%s %s undated=%v", r.Code, c, undated(in, r.Params.Dose))]++
						}
					}
				}
			}
		}
	}
	for _, code := range []engine.ReasonCode{engine.ReasonDoseAfterMaxAge, engine.ReasonDosePossiblyInvalid} {
		for _, c := range []engine.Check{engine.CheckMaxAgeStart, engine.CheckMaxAgeComplete} {
			for _, u := range []bool{false, true} {
				if code == engine.ReasonDosePossiblyInvalid && c == engine.CheckMaxAgeStart && u {
					continue
				}
				if k := fmt.Sprintf("%s %s undated=%v", code, c, u); seen[k] == 0 {
					t.Errorf("near-limit inputs never reach %s", k)
				}
			}
		}
	}
}

// hasImpliedDose re-derives from the records alone whether the disease has an
// implied dose (SAD §5.3 step 2, F04.1), with no series length: the records
// take positions 1, 2, … (a user-entered number its own, an unnumbered dated
// dose the next free position after the last one reached, in time order),
// unnumbered level-4 doses fill the gaps, and the positions left below the
// highest one taken are implied. So there is one iff the highest position
// taken is above the number of records. Within one exact day the unnumbered
// doses go first, as orderSameDay fills the free positions around the
// numbered ones. Only meaningful when step 2 does not block.
func hasImpliedDose(t testing.TB, in engine.Input, disease string) bool {
	t.Helper()
	type rec struct {
		lo, hi engine.Date
		number int
		id     string
	}
	var dated []rec
	taken := map[int]bool{}
	n, highest := 0, 0
	for _, r := range in.Doses {
		if !slices.Contains(r.Diseases, disease) {
			continue
		}
		n++
		num := int(r.DoseNumbers[disease])
		if num > 0 {
			taken[num] = true
			highest = max(highest, num)
		}
		if lo, hi, ok := testPeriod(r.Date, in.Today); ok {
			dated = append(dated, rec{lo: lo, hi: hi, number: num, id: r.ID})
		}
	}
	slices.SortFunc(dated, func(x, y rec) int {
		return cmp.Or(x.lo.Compare(y.lo), x.hi.Compare(y.hi), cmp.Compare(min(x.number, 1), min(y.number, 1)), strings.Compare(x.id, y.id))
	})
	last := 0
	for _, d := range dated {
		if d.number > 0 {
			last = max(last, d.number)
			continue
		}
		for last++; taken[last]; last++ {
		}
		taken[last] = true
		highest = max(highest, last)
	}
	return highest > n
}
