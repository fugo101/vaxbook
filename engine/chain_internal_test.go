package engine

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fugo101/vaxbook/ruleset"
)

// gapFixture is a fixture disease of the S2 chain gap checks (issue #35).
type gapFixture struct {
	name, disease, birth, base string
	products                   []string
	span                       int
}

// diseaseOf loads fixture fx and returns its disease model.
func (fx gapFixture) diseaseOf(t *testing.T) diseaseModel {
	t.Helper()
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
	for _, dz := range e.diseases {
		if dz.id == fx.disease {
			return dz
		}
	}
	t.Fatalf("%s has no disease %s", fx.name, fx.disease)
	return diseaseModel{}
}

// TestPropertyUndatedDefiniteMinInterval (issue #35, Q100 (a); SAD §5.7): a
// definite DOSE_INVALID listing minInterval on an undated record holds for
// every exact day the record could have. On minimal-single,
// illustrative-vn pertussis and illustrative-max-age, with an exact birth,
// one to three exact dated doses (a quarter numbered 1–3) and one undated
// record u (half numbered 1–3), each with a random product or none: when u
// gets DOSE_INVALID listing minInterval, no day d in [birth, today] for u
// gives u no definite reason while giving no other record a definite reason
// it lacked. The days skipped are those of the S4b property
// (TestPropertyUndatedDefiniteMaxAge): a step-2 reason, CHECK_PRODUCT_NAME
// or MIXED_PRODUCTS, or a dated record some candidate counts moving to
// another position. It fails when no checked record lies behind an
// uncounted unnumbered dated dose (the shape the gap search covers).
func TestPropertyUndatedDefiniteMinInterval(t *testing.T) {
	t.Parallel()
	checked, behind, fails := 0, 0, 0
	for _, fx := range []gapFixture{
		{"minimal-single", "single", "2026-01-01", "2026-01-01", []string{""}, 200},
		{"illustrative-vn", "pertussis", "2026-01-01", "2026-01-01", []string{""}, 300},
		{"illustrative-max-age", "rota", "2026-01-01", "2026-01-01", []string{"", "rotarix", "rotateq", "rotavin"}, 330},
	} {
		dz := fx.diseaseOf(t)
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
		// clearDay returns the first day in [birth, today] on which u dated
		// that day has no definite reason and no other record gains one, the
		// days above skipped; ok is false when there is none.
		clearDay := func(today Date, doses []givenDose, u givenDose, a Assessment, res endResult) (day Date, a2 Assessment, ok bool) {
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
					return day, a2, true
				}
			}
			return Date{}, Assessment{}, false
		}
		rng := rand.New(rand.NewPCG(91, uint64(len(fx.name)+len(fx.disease))))
		pairChecked, pairBehind, pairFails := checked, behind, fails
		for range 6000 {
			today := base.AddDays(30 + rng.IntN(fx.span))
			var doses []givenDose
			for i := range 1 + rng.IntN(3) {
				day := base.AddDays(rng.IntN(int(today.days-base.days) + 1))
				doses = append(doses, dose(fmt.Sprintf("d%d", i), &day, rng, 4))
			}
			u := dose("u", nil, rng, 2)
			a, res := dz.assess(birth, today, append(slices.Clone(doses), u))
			if res.cands == nil || !slices.ContainsFunc(a.Reasons, func(r Reason) bool {
				return r.Code == ReasonDoseInvalid && r.Params.Dose == "u" && slices.Contains(r.Params.Checks, CheckMinInterval)
			}) {
				continue
			}
			checked++
			if slices.ContainsFunc(res.cands, func(c candidateResult) bool { return behindUncounted(c.walk.seq) }) {
				behind++
			}
			if day, a2, ok := clearDay(today, doses, u, a, res); ok {
				if fails++; fails <= 3 {
					t.Errorf("%s: u has DOSE_INVALID{minInterval}, but on %v it has no definite reason and no other record gains one\ntoday %v doses %+v u %+v\nundated %+v\ndated %+v",
						fx.name, day, today, doses, u, a.Reasons, a2.Reasons)
				}
			}
		}
		t.Logf("%s/%s: %d checked, %d behind an uncounted dose, %d counterexamples",
			fx.name, fx.disease, checked-pairChecked, behind-pairBehind, fails-pairFails)
	}
	if fails > 0 {
		t.Errorf("%d counterexamples", fails)
	}
	if behind == 0 {
		t.Fatal("no checked record lies behind an uncounted unnumbered dated dose; the property is vacuous")
	}
}

// behindUncounted reports whether the undated slots at the end of walk seq
// follow an uncounted unnumbered dated dose.
func behindUncounted(seq []item) bool {
	j := len(seq)
	for j > 0 && !seq[j-1].dose.dated {
		j--
	}
	return j > 0 && j < len(seq) && !seq[j-1].counted && seq[j-1].dose.number == 0
}
