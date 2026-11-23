package engine

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// maxAgeTestEngine is illustrative-max-age plus a disease other and a
// product othervax that lists only other (the Q36 sub-case), with rotateq
// retired when retired is set.
func maxAgeTestEngine(t *testing.T, retired bool) *Engine {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "rulesets", "illustrative-max-age.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ruleset.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	s.Diseases = append(s.Diseases, ruleset.Disease{ID: "other", Name: "Other"})
	s.Products = append(s.Products, ruleset.Product{ID: "othervax", Name: "Othervax", Diseases: []string{"other"}})
	s.Products[1].Retired = retired
	other := s.Series[2]
	other.ID, other.Disease, other.Product, other.MaxAgeStart = "other-1", "other", new("othervax"), nil
	s.Series = append(s.Series, other)
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestAnyUnknownProduct (issue #15 In scope 6, Q53, Q54): the classifier for
// CHECK_PRODUCT_NAME and MIXED_PRODUCTS. Named: a catalog product that lists
// the disease, retired or not, or an off-catalog product. Unknown: no
// product, a catalog product that does not list the disease, or an implied
// dose.
func TestAnyUnknownProduct(t *testing.T) {
	t.Parallel()
	dz := maxAgeTestEngine(t, true).diseases[0]
	named := givenDose{id: "n", product: "rotarix"}
	cases := []struct {
		name    string
		doses   []givenDose
		implied bool
		want    bool
	}{
		{"catalog product listing the disease", []givenDose{named}, false, false},
		{"retired product listing the disease", []givenDose{named, {id: "r", product: "rotateq"}}, false, false},
		{"off-catalog product", []givenDose{named, {id: "f", free: true}}, false, false},
		{"no product", []givenDose{named, {id: "x"}}, false, true},
		{"catalog product not listing the disease", []givenDose{named, {id: "o", product: "othervax"}}, false, true},
		{"implied dose with named records", []givenDose{named}, true, true},
	}
	for _, tc := range cases {
		if got := dz.anyUnknownProduct(tc.doses, tc.implied); got != tc.want {
			t.Errorf("%s: anyUnknownProduct = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestPropertyMaxAgeDisagreement (issue #15 In scope 5, Q52), against the
// per-candidate walks kept whole in endResult.cands: for every record that
// some candidate gives DOSE_AFTER_MAX_AGE, the reason is in the result iff
// no other candidate accepts the record, where a candidate accepts it when
// it has no DOSE_INVALID or DOSE_AFTER_MAX_AGE on it and counts it (it is
// walked at a position up to the series length, or floats and fits), or is
// complete and leaves it beyond its length. CHECK_PRODUCT_NAME or
// MIXED_PRODUCTS is there iff some record disagrees, never both, and
// CHECK_PRODUCT_NAME iff some dose of the disease has no product, a product
// that does not list rota, or is implied. The generator covers every rota
// product (rotateq retired in half the runs), othervax, an off-catalog
// product and none, exact and month dates and level-4 records near the
// limits of illustrative-max-age.
func TestPropertyMaxAgeDisagreement(t *testing.T) {
	t.Parallel()
	listing := map[string]bool{"rotarix": true, "rotateq": true, "rotavin": true}
	products := []string{"", "rotarix", "rotateq", "rotavin", "othervax", "free"}
	base := d(t, "2026-01-01")
	agreed, disagreed := 0, 0
	for _, retired := range []bool{false, true} {
		dz := maxAgeTestEngine(t, retired).diseases[0]
		rng := rand.New(rand.NewPCG(43, map[bool]uint64{false: 1, true: 2}[retired]))
		for range 3000 {
			today := base.AddDays(60 + rng.IntN(240))
			n := 1 + rng.IntN(3)
			var doses []givenDose
			for i := range n {
				g := givenDose{id: string(rune('a' + i))}
				switch p := products[rng.IntN(len(products))]; p {
				case "free":
					g.free = true
				default:
					g.product = p
				}
				switch day := base.AddDays(40 + rng.IntN(int(today.days-base.days)-39)); rng.IntN(5) {
				case 0:
					// level 4, sometimes numbered
					if rng.IntN(2) == 0 {
						g.number = 1 + rng.IntN(4)
					}
				case 1:
					_, _, dom := day.YMD()
					lo := day.AddDays(1 - dom)
					hi := minDate(lo.AddMonths(1).AddDays(-1), today)
					g.date, g.dated, g.imprecise = Range{Lo: lo, Hi: hi}, true, true
				default:
					g.date, g.dated = Exact(day), true
				}
				doses = append(doses, g)
			}
			arr := arrange(doses)
			if arr.blocked() {
				continue
			}
			implied := len(arr.forLength(math.MaxInt).implied) > 0
			birth := birthRange{r: Exact(base), known: true, precision: Day}
			cands, _ := dz.byAgeAtDoseOne(arr, dz.candidates(doses, implied), birth, today)
			ev := dz.evaluate(arr, cands, today, nil, false)
			ev.unknownProduct = dz.anyUnknownProduct(doses, implied)
			res := ev.at(birthEnd{date: base, known: true})

			wantUnknown := implied || slices.ContainsFunc(doses, func(g givenDose) bool {
				return !g.free && !listing[g.product]
			})
			accepts := func(j int, id string) bool {
				r := res.cands[j]
				if slices.ContainsFunc(r.walk.reasons, func(x Reason) bool {
					return x.Params.Dose == id && (x.Code == ReasonDoseInvalid || x.Code == ReasonDoseAfterMaxAge)
				}) {
					return false
				}
				length := dz.series[cands[j]].length
				if slices.ContainsFunc(r.walk.floating, func(g givenDose) bool { return g.id == id }) {
					return true
				}
				if i := slices.IndexFunc(r.walk.seq, func(it item) bool { return it.dose.id == id }); i >= 0 {
					return r.walk.seq[i].pos <= length || r.complete
				}
				return r.complete && slices.ContainsFunc(ev.pls[j].beyond, func(g givenDose) bool { return g.id == id })
			}
			anyDisagree := false
			for i, r := range res.cands {
				for _, x := range r.walk.reasons {
					if x.Code != ReasonDoseAfterMaxAge {
						continue
					}
					disagree := false
					for j := range res.cands {
						disagree = disagree || j != i && accepts(j, x.Params.Dose)
					}
					inResult := slices.ContainsFunc(res.reasons, func(y Reason) bool { return reasonKey(y) == reasonKey(x) })
					if inResult == disagree {
						t.Fatalf("DOSE_AFTER_MAX_AGE %+v in result %v, disagreement %v\ndoses %+v today %v\nreasons %+v", x, inResult, disagree, doses, today, res.reasons)
					}
					if disagree {
						disagreed++
					} else {
						agreed++
					}
					anyDisagree = anyDisagree || disagree
				}
			}
			cpn, mixed := hasReason(res.reasons, ReasonCheckProductName), hasReason(res.reasons, ReasonMixedProducts)
			switch {
			case cpn && mixed:
				t.Fatalf("both CHECK_PRODUCT_NAME and MIXED_PRODUCTS: %+v", res.reasons)
			case anyDisagree != (cpn || mixed):
				t.Fatalf("disagreement %v but reasons %+v\ndoses %+v today %v", anyDisagree, res.reasons, doses, today)
			case anyDisagree && cpn != wantUnknown:
				t.Fatalf("CHECK_PRODUCT_NAME %v, unknown product %v\ndoses %+v", cpn, wantUnknown, doses)
			}
		}
	}
	// The generator must reach both sides, or the property is vacuous.
	if agreed == 0 || disagreed == 0 {
		t.Fatalf("DOSE_AFTER_MAX_AGE kept %d times and dropped %d times; want both", agreed, disagreed)
	}
}

func hasReason(rs []Reason, c ReasonCode) bool {
	return slices.ContainsFunc(rs, func(r Reason) bool { return r.Code == c })
}
