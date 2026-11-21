package engine_test

import (
	"bytes"
	"fmt"
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

// randomInput draws a birth date, a today on or after it, and 0–6 dose
// records dated in [birth, today] for random non-empty subsets of the
// diseases. About one record in 25 has an impossible date (A9).
func randomInput(t testing.TB, rng *rand.Rand, s *ruleset.Snapshot) engine.Input {
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
		value := birth.AddDays(rng.IntN(span + 1)).String()
		if rng.IntN(25) == 0 {
			value = value[:8] + "32"
		}
		in.Doses = append(in.Doses, engine.DoseRecord{
			ID:       fmt.Sprintf("r%d", i),
			Diseases: diseases,
			Date:     engine.RecalledDate{Value: value, Precision: engine.Day},
		})
	}
	return in
}

func cloneInput(in engine.Input) engine.Input {
	out := in
	out.Doses = make([]engine.DoseRecord, len(in.Doses))
	for i, r := range in.Doses {
		r.Diseases = slices.Clone(r.Diseases)
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

func seriesOf(s *ruleset.Snapshot, disease string) ruleset.Series {
	for _, sr := range s.Series {
		if sr.Disease == disease {
			return sr
		}
	}
	panic("no series for " + disease)
}

// datedDoses returns the parseable dose dates of a disease, oldest first.
func datedDoses(in engine.Input, disease string) []engine.Date {
	var out []engine.Date
	for _, r := range in.Doses {
		if !slices.Contains(r.Diseases, disease) {
			continue
		}
		if d, err := engine.ParseDate(r.Date.Value); err == nil {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, engine.Date.Compare)
	return out
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

// TestPropertyNextDoseBounds checks every next dose of random inputs:
//   - P1: the earliest allowed date is never before birth + minimum age, and
//     never before the latest dose + minimum interval, for the plan it comes
//     from (some plan satisfies both);
//   - due ≥ earliest and overdue ≥ due + 7 days;
//   - exactly one of bucket and block; a next dose unless the series is
//     complete or a date is invalid.
func TestPropertyNextDoseBounds(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(11, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			birth := mustDate(t, in.Profile.Birth.Value)
			for _, a := range out.Assessments {
				checkReasonParams(t, a)
				if (a.Bucket == "") == (a.Block == "") {
					t.Fatalf("%s %s: bucket %q, block %q", fx.name, a.Disease, a.Bucket, a.Block)
				}
				noNext := len(reasonsWith(a, engine.ReasonComplete)) > 0 || len(reasonsWith(a, engine.ReasonInvalidDate)) > 0
				if noNext != (a.NextDose == nil) {
					t.Fatalf("%s %s: nextDose %v with reasons %v", fx.name, a.Disease, a.NextDose, a.Reasons)
				}
				if a.NextDose == nil {
					continue
				}
				nd := *a.NextDose
				if nd.Due.Before(nd.Earliest) {
					t.Fatalf("%s %s: due %s before earliest %s", fx.name, a.Disease, nd.Due, nd.Earliest)
				}
				if nd.Overdue.Before(nd.Due.AddDays(engine.OverdueGraceDays)) {
					t.Fatalf("%s %s: overdue %s before due %s + 7 days", fx.name, a.Disease, nd.Overdue, nd.Due)
				}
				doses := datedDoses(in, a.Disease)
				sr := seriesOf(fx.snap, a.Disease)
				ok := false
				for _, p := range sr.Plans {
					r := p.Doses[nd.Number-1]
					if r.MinAge != nil && nd.Earliest.Before(birth.AddSpan(*r.MinAge)) {
						continue
					}
					if r.MinInterval != nil && len(doses) > 0 && nd.Earliest.Before(doses[len(doses)-1].AddSpan(*r.MinInterval)) {
						continue
					}
					ok = true
				}
				if !ok {
					t.Fatalf("%s %s: earliest %s violates every plan's minimums for dose %d (birth %s, doses %v)",
						fx.name, a.Disease, nd.Earliest, nd.Number, birth, doses)
				}
			}
		}
	}
}

// TestPropertyDoseOnEarliestDateCounts: a dose given on the earliest allowed
// date is counted (the next dose number rises or the series completes) and
// adds no DOSE_INVALID.
func TestPropertyDoseOnEarliestDateCounts(t *testing.T) {
	t.Parallel()
	for _, fx := range fixtures(t) {
		rng := rand.New(rand.NewPCG(13, uint64(len(fx.name))))
		for range propertyRuns {
			in := randomInput(t, rng, fx.snap)
			out, _ := evaluate(t, fx.engine, in)
			for i, a := range out.Assessments {
				if a.NextDose == nil {
					continue
				}
				next := cloneInput(in)
				next.Today = slices.MaxFunc([]engine.Date{in.Today, a.NextDose.Earliest}, engine.Date.Compare)
				next.Doses = append(next.Doses, engine.DoseRecord{
					ID:       "zz-on-earliest",
					Diseases: []string{a.Disease},
					Date:     engine.RecalledDate{Value: a.NextDose.Earliest.String(), Precision: engine.Day},
				})
				after, _ := evaluate(t, fx.engine, next)
				b := after.Assessments[i]
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
