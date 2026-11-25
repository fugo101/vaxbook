package engine

import "testing"

// DisagreeingRecords returns, for each disease of e, the records whose
// DOSE_AFTER_MAX_AGE a maximum-age disagreement dropped (issue #15 In scope
// 5, Q52), by ID. It is test-only: the narrowing property limits its Q52/Q62
// exception to these records. A disease with none is absent.
func DisagreeingRecords(e *Engine, in Input) (map[string][]string, error) {
	birth, byDisease, err := e.prepare(in)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, tl := range e.diseases {
		dz := tl.at(in.Today)
		if _, res := dz.assess(birth, in.Today, byDisease[dz.id]); len(res.disagreeing) > 0 {
			out[dz.id] = res.disagreeing
		}
	}
	return out, nil
}

// CandidateView is one candidate series of the final result of steps 4–9
// for a disease, as the S4b properties read it (issue #28): complete or
// not, its next dose number, the per-plan earliest dates of that dose
// (none when step 7 stopped), whether step 7 stopped on
// PREVIOUS_DOSE_DATE_UNKNOWN, and whether position 1 holds a dated record
// (else dose 1 is undated or implied, and step 7 may use the EQ20c
// stand-in). Counted maps each record the walk counts in the series to its
// position (issue #29: the tolerance property compares walks that count the
// same records at the same positions).
type CandidateView struct {
	Series           string
	Complete         bool
	K                int
	Earliest         []Date
	StoppedOnUnknown bool
	FirstDated       bool
	Counted          map[string]int
}

// ResultView is the final result of steps 4–9 of one disease: its
// candidates as walked, and the counted view (the same walks when no record
// is questioned). For compared results it is the latest evaluated date's.
type ResultView struct {
	Walked, Counted []CandidateView
}

// ResultViews returns the ResultView of each disease of e that reaches
// steps 4–9 (test-only).
func ResultViews(e *Engine, in Input) (map[string]ResultView, error) {
	birth, byDisease, err := e.prepare(in)
	if err != nil {
		return nil, err
	}
	view := func(dz diseaseModel, series []int, rs []candidateResult) []CandidateView {
		var out []CandidateView
		for i, r := range rs {
			v := CandidateView{Series: dz.series[series[i]].id, Complete: r.complete, K: r.walk.k,
				StoppedOnUnknown: r.next.stop && hasReason(r.next.reasons, ReasonPreviousDoseDateUnknown)}
			for _, p := range r.next.plans {
				v.Earliest = append(v.Earliest, p.earliest)
			}
			v.Counted = map[string]int{}
			for _, it := range r.walk.seq {
				if it.pos == 1 {
					v.FirstDated = it.dose.dated
				}
				if it.counted && it.pos <= dz.series[series[i]].length && it.dose.id != "" {
					v.Counted[it.dose.id] = it.pos
				}
			}
			for _, g := range r.walk.floating {
				v.Counted[g.id] = 0
			}
			out = append(out, v)
		}
		return out
	}
	out := map[string]ResultView{}
	for _, tl := range e.diseases {
		dz := tl.at(in.Today)
		_, res := dz.assess(birth, in.Today, byDisease[dz.id])
		if res.cands == nil {
			continue
		}
		v := ResultView{Walked: view(dz, res.series, res.cands)}
		v.Counted = v.Walked
		if res.counted != nil {
			v.Counted = view(dz, res.series, res.counted)
		}
		out[dz.id] = v
	}
	return out, nil
}

// onlySegment returns disease i of e, failing unless the disease has one
// validity segment (a windowless snapshot): the internal tests that read a
// diseaseModel directly run on such snapshots (issue #29 In scope 2).
func onlySegment(t testing.TB, e *Engine, i int) diseaseModel {
	t.Helper()
	return e.diseases[i].only(t)
}

// only returns the timeline's model, failing unless it has one segment.
func (tl diseaseTimeline) only(t testing.TB) diseaseModel {
	t.Helper()
	if len(tl.models) != 1 {
		t.Fatalf("disease %s has %d validity segments, want 1", tl.id, len(tl.models))
	}
	return tl.models[0]
}
