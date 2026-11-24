package engine

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
	for _, dz := range e.diseases {
		if _, res := dz.assess(birth, in.Today, byDisease[dz.id]); len(res.disagreeing) > 0 {
			out[dz.id] = res.disagreeing
		}
	}
	return out, nil
}

// CandidateView is one candidate series of the final result of steps 4–9
// for a disease, as the S4b properties read it (issue #28): complete or
// not, its next dose number, the per-plan earliest dates of that dose
// (none when step 7 stopped) and whether step 7 stopped on
// PREVIOUS_DOSE_DATE_UNKNOWN.
type CandidateView struct {
	Series           string
	Complete         bool
	K                int
	Earliest         []Date
	StoppedOnUnknown bool
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
			out = append(out, v)
		}
		return out
	}
	out := map[string]ResultView{}
	for _, dz := range e.diseases {
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
