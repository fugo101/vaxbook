package engine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	dz := onlySegment(t, maxAgeTestEngine(t, true), 0)
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
// limits of illustrative-max-age. Since S4b step 7 can also give
// CHECK_PRODUCT_NAME or MIXED_PRODUCTS (issue #28 In scope 2), so only the
// step-5 reasons (asWalked, before step7At) are compared.
func TestPropertyMaxAgeDisagreement(t *testing.T) {
	t.Parallel()
	listing := map[string]bool{"rotarix": true, "rotateq": true, "rotavin": true}
	products := []string{"", "rotarix", "rotateq", "rotavin", "othervax", "free"}
	base := d(t, "2026-01-01")
	agreed, disagreed := 0, 0
	for _, retired := range []bool{false, true} {
		dz := onlySegment(t, maxAgeTestEngine(t, retired), 0)
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
			walked, step7At, _ := ev.asWalked(res.cands)
			step5 := walked.reasons[:step7At]
			cpn, mixed := hasReason(step5, ReasonCheckProductName), hasReason(step5, ReasonMixedProducts)
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

// TestBirthHintForAgeLimitReasons (issue #15 Q58, SAD §5.3 step 4): when the
// compared results differ, the hint is fullBirthDate whatever the birth
// precision as soon as any of them has an age-limit reason: OUT_OF_AGE,
// DOSE_AFTER_MAX_AGE, CHECK_PRODUCT_NAME, MIXED_PRODUCTS, a
// DOSE_POSSIBLY_INVALID listing a maximum age, and since S4b (issue #28 In
// scope 6) DUE_CAPPED_BY_MAX_AGE, SERIES_MAY_NOT_COMPLETE,
// OUT_OF_AGE_UNCERTAIN (owner 2026-11-24), and a compared result that is
// itself ⚪ BIRTH_DATE_IMPRECISE{fullBirthDate}, whose reason is then listed
// once. A minimum-only reason keeps
// the precision's hint (birthMonth here). The reason is on the older end's
// ⚪ result, and the later end is 🟠 with none; the same holds with the two
// swapped.
func TestBirthHintForAgeLimitReasons(t *testing.T) {
	t.Parallel()
	checked := func(code ReasonCode, checks ...Check) Reason {
		return Reason{Code: code, Params: ReasonParams{Checks: checks, Dose: "r1", Number: 1}}
	}
	cases := []struct {
		name   string
		reason Reason
		want   Hint
	}{
		{"out of age", Reason{Code: ReasonOutOfAge}, HintFullBirthDate},
		{"dose after max age", checked(ReasonDoseAfterMaxAge, CheckMaxAgeStart), HintFullBirthDate},
		{"check product name", Reason{Code: ReasonCheckProductName}, HintFullBirthDate},
		{"mixed products", Reason{Code: ReasonMixedProducts}, HintFullBirthDate},
		{"possibly after max age to start", checked(ReasonDosePossiblyInvalid, CheckMaxAgeStart), HintFullBirthDate},
		{"possibly after max age to complete", checked(ReasonDosePossiblyInvalid, CheckMinAge, CheckMaxAgeComplete), HintFullBirthDate},
		{"possibly before min age", checked(ReasonDosePossiblyInvalid, CheckMinAge), HintBirthMonth},
		{"dose invalid", checked(ReasonDoseInvalid, CheckMinInterval), HintBirthMonth},
		{"due capped by max age", Reason{Code: ReasonDueCappedByMaxAge, Params: ReasonParams{LastDay: d(t, "2026-05-31")}}, HintFullBirthDate},
		{"series may not complete", Reason{Code: ReasonSeriesMayNotComplete, Params: ReasonParams{LastDay: d(t, "2026-08-12")}}, HintFullBirthDate},
		{"age-limit ⚪ at one date", Reason{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: HintFullBirthDate}}, HintFullBirthDate},
		{"out of age uncertain", Reason{Code: ReasonOutOfAgeUncertain, Params: ReasonParams{Numbers: []int{1}}}, HintFullBirthDate},
		{"catch-up ask doctor", Reason{Code: ReasonCatchUpAskDoctor}, HintFullBirthDate},
	}
	for _, tc := range cases {
		withReason := endResult{bucket: BucketUnknown, k: 1, reasons: []Reason{tc.reason}}
		plain := endResult{bucket: BucketDue, k: 1}
		for end, rs := range [][]endResult{{withReason, plain}, {plain, withReason}} {
			res := compareResults(rs, HintBirthMonth)
			want := Reason{Code: ReasonBirthDateImprecise, Params: ReasonParams{Hint: tc.want}}
			if res.bucket != BucketUnknown || len(res.reasons) == 0 || !sameReason(res.reasons[0], want) {
				t.Errorf("%s (reason at end %d): %s %v, want ⚪ led by %v", tc.name, end, res.bucket, res.reasons, want)
			}
			if n := len(slices.DeleteFunc(slices.Clone(res.reasons), func(r Reason) bool { return r.Code != ReasonBirthDateImprecise })); n != 1 {
				t.Errorf("%s (reason at end %d): %d BIRTH_DATE_IMPRECISE reasons in %v, want 1", tc.name, end, n, res.reasons)
			}
		}
	}
}

// TestReasonKeyHasLastDay (issue #28): two DUE_CAPPED_BY_MAX_AGE reasons with
// different last days are different reasons, so step 4 never merges them
// silently (possible across no-dose sub-ranges, through the start limit's
// bracket term).
func TestReasonKeyHasLastDay(t *testing.T) {
	t.Parallel()
	a := Reason{Code: ReasonDueCappedByMaxAge, Params: ReasonParams{LastDay: d(t, "2026-04-15")}}
	b := Reason{Code: ReasonDueCappedByMaxAge, Params: ReasonParams{LastDay: d(t, "2026-04-16")}}
	if sameReason(a, b) {
		t.Errorf("reasonKey(%v) = reasonKey(%v)", a, b)
	}
	if !sameReason(a, a) {
		t.Errorf("reasonKey is not stable")
	}
}

// TestPropertyUndatedDefiniteMaxAge (issue #28 In scope 0, Q84, Q87): a
// definite maximum-age verdict on an undated record holds for every exact
// day the record could have. On both maximum-age fixtures, with an exact
// birth, one to three exact dated doses (a quarter numbered) and one undated
// record u (a third numbered), each with a random product or none: when u
// gets DOSE_AFTER_MAX_AGE, no day d in [birth, today] for u gives u neither
// DOSE_INVALID nor DOSE_AFTER_MAX_AGE while giving no other record a
// definite reason it lacked. Days are skipped that give a step-2 reason,
// CHECK_PRODUCT_NAME or MIXED_PRODUCTS, or that move a dated record some
// candidate counts to another position: the verdict is about u's place in
// time, not about a different arrangement of the other records.
func TestPropertyUndatedDefiniteMaxAge(t *testing.T) {
	t.Parallel()
	type fixture struct {
		name, birth, base string
		products          []string
		span              int
	}
	checked := 0
	for _, fx := range []fixture{
		{"illustrative-max-age", "2026-01-01", "2026-01-01", []string{"", "rotarix", "rotateq", "rotavin"}, 330},
		{"minimal-max-age-complete", "2006-06-01", "2024-05-01", []string{"", "za", "zb"}, 2*365 + 200},
	} {
		b, err := os.ReadFile(filepath.Join("testdata", "rulesets", fx.name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		s, err := ruleset.Decode(b)
		if err != nil {
			t.Fatal(err)
		}
		e, err := New(s)
		if err != nil {
			t.Fatal(err)
		}
		dz := onlySegment(t, e, 0)
		birthDay, base := d(t, fx.birth), d(t, fx.base)
		birth := birthRange{r: Exact(birthDay), known: true, precision: Day}
		dose := func(id string, date *Date, rng *rand.Rand, numberOdds int) givenDose {
			g := givenDose{id: id, product: fx.products[rng.IntN(len(fx.products))]}
			if date != nil {
				g.date, g.dated = Exact(*date), true
			}
			if rng.IntN(numberOdds) == 0 {
				g.number = 1 + rng.IntN(3)
			}
			return g
		}
		// positions returns the position of each dated record per candidate
		// series, and whether that candidate counts it.
		type place struct {
			pos     int
			counted bool
		}
		positions := func(res endResult) map[string]place {
			out := map[string]place{}
			for i, r := range res.cands {
				for _, it := range r.walk.seq {
					if it.dose.dated {
						out[fmt.Sprintf("%d %s", res.series[i], it.dose.id)] = place{it.pos, it.counted}
					}
				}
			}
			return out
		}
		rng := rand.New(rand.NewPCG(71, uint64(len(fx.name))))
		for range 12000 {
			today := base.AddDays(30 + rng.IntN(fx.span))
			var doses []givenDose
			for i := range 1 + rng.IntN(3) {
				day := base.AddDays(rng.IntN(int(today.days-base.days) + 1))
				doses = append(doses, dose(fmt.Sprintf("d%d", i), &day, rng, 4))
			}
			u := dose("u", nil, rng, 3)
			a, res := dz.assess(birth, today, append(slices.Clone(doses), u))
			if res.cands == nil || !slices.ContainsFunc(a.Reasons, func(r Reason) bool { return r.Code == ReasonDoseAfterMaxAge && r.Params.Dose == "u" }) {
				continue
			}
			checked++
			had := definiteIDs(a.Reasons)
			pos := positions(res)
			for day := birthDay; !day.After(today); day = day.AddDays(1) {
				ud := u
				ud.date, ud.dated = Exact(day), true
				a2, res2 := dz.assess(birth, today, append(slices.Clone(doses), ud))
				if res2.cands == nil || hasReason(a2.Reasons, ReasonCheckProductName) || hasReason(a2.Reasons, ReasonMixedProducts) {
					continue
				}
				moved := false
				for k, p := range positions(res2) {
					if q, ok := pos[k]; ok && q.pos != p.pos && (p.counted || q.counted) {
						moved = true
					}
				}
				if moved {
					continue
				}
				now := definiteIDs(a2.Reasons)
				if now["u"] {
					continue
				}
				gained := false
				for id := range now {
					gained = gained || !had[id]
				}
				if !gained {
					t.Fatalf("%s: u has DOSE_AFTER_MAX_AGE, but on %v it has no definite reason and no other record gains one\ntoday %v doses %+v u %+v\nundated %+v\ndated %+v",
						fx.name, day, today, doses, u, a.Reasons, a2.Reasons)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no undated record got DOSE_AFTER_MAX_AGE; the property is vacuous")
	}
	t.Logf("%d undated records with DOSE_AFTER_MAX_AGE checked", checked)
}

// TestLastAllowedByForwardChain (issue #28 Q70): C_c,k is the latest day for
// dose k from which the later doses, each on its earliest date under the
// most lenient plan for that dose, end by L_c, found by bisection. Generic
// in-test series, birth 2010-01-01 and maxAgeComplete 15y (L = 2024-12-31)
// unless stated.
func TestLastAllowedByForwardChain(t *testing.T) {
	t.Parallel()
	series := func(maxComplete string, plans ...string) seriesModel {
		t.Helper()
		var ps []string
		for i, p := range plans {
			ps = append(ps, `{"id": "`+string(rune('a'+i))+`", "doses": [`+p+`]}`)
		}
		s, err := ruleset.Decode([]byte(`{"version": "v", "illustrative": true, "diseases": [{"id": "q", "name": "Q"}], "products": [], ` +
			`"series": [{"id": "q-1", "disease": "q", "stage": "childhood", "maxAgeComplete": "` + maxComplete + `", "plans": [` + strings.Join(ps, ", ") + `]}]}`))
		if err != nil {
			t.Fatal(err)
		}
		e, err := New(s)
		if err != nil {
			t.Fatal(err)
		}
		return onlySegment(t, e, 0).series[0]
	}
	dose := func(fields string) string {
		return `{` + fields + `"recommended": {"from": "birth", "offset": "P9Y"}}`
	}
	first := dose(`"minAge": "P9Y", `)
	hpv := series("P15Y", strings.Join([]string{first, dose(`"minInterval": "P4W", `), dose(`"minInterval": "P12W", "minIntervalFromFirst": "P5M", `)}, ", "))
	b := birthEnd{date: d(t, "2010-01-01"), known: true}
	at := func(s string) *Range { r := Exact(d(t, s)); return &r }
	day := func(s string) bound { return dateBound(d(t, s)) }
	impossible := bound{kind: boundImpossible}
	cases := []struct {
		name  string
		s     seriesModel
		k     int
		b     birthEnd
		first *Range
		want  bound
	}{
		// Dose 3 is at least d + 5m, later than d + 16w: d + 5m ≤ L. The
		// sum of the minimum intervals alone (L − 16w = 09-10) would leave
		// no room for dose 3.
		{"minIntervalFromFirst of a later dose", hpv, 1, b, nil, day("2024-07-31")},
		// Dose 1 on 05-01 (+ 5m = 10-01 ≤ L): dose 3 at d + 12w ≤ L.
		{"first-dose anchor that fits", hpv, 2, b, at("2024-05-01"), day("2024-10-08")},
		// Dose 1 on 09-01: dose 3 is at least 2025-02-01 > L.
		{"first-dose anchor past the limit", hpv, 2, b, at("2024-09-01"), impossible},
		{"last dose", hpv, 3, b, nil, day("2024-12-31")},
		// Dose 2's minimum age 179m (2024-12-01) puts dose 3 at least
		// 2025-01-26: no day for dose 1 works, although the intervals alone
		// would allow L − 12w.
		{"binding later minimum age", series("P15Y", strings.Join([]string{first, dose(`"minAge": "P179M", "minInterval": "P4W", `), dose(`"minInterval": "P8W", `)}, ", ")), 1, b, nil, impossible},
		// Birth 2025-04-01, maxAgeComplete 1y: L = 2026-03-31. 01-31 + 1m +
		// 1m = 03-28 (month-end clamping) fits, 02-01 + 1m + 1m = 04-01
		// does not; stepping back from L by months would give 01-28.
		{"month-end clamping", series("P1Y", strings.Join([]string{dose(`"minAge": "P0D", `), dose(`"minInterval": "P1M", `), dose(`"minInterval": "P1M", `)}, ", ")),
			1, birthEnd{date: d(t, "2025-04-01"), known: true}, nil, day("2026-01-31")},
		// Plan a is more lenient for dose 2 (4w), plan b for dose 3 (4w):
		// the chain takes each, d + 8w ≤ L.
		{"most lenient plan per dose", series("P15Y",
			strings.Join([]string{first, dose(`"minInterval": "P4W", `), dose(`"minInterval": "P12W", `)}, ", "),
			strings.Join([]string{first, dose(`"minInterval": "P8W", `), dose(`"minInterval": "P4W", `)}, ", ")), 1, b, nil, day("2024-11-05")},
	}
	for _, tc := range cases {
		lc := tc.s.completeBound(tc.b.older())
		if got := tc.s.lastAllowed(tc.k, tc.b, tc.first, lc); got != tc.want {
			t.Errorf("%s: C_%d = %+v, want %+v", tc.name, tc.k, got, tc.want)
		}
	}
	if got := hpv.lastAllowed(1, b, nil, bound{}); got.present() {
		t.Errorf("without maxAgeComplete: C_1 = %+v, want absent", got)
	}
}
