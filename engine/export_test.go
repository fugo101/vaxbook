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
