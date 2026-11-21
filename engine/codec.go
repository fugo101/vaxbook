package engine

import (
	"encoding/json/v2"
	"fmt"
)

// DecodeInput parses an Input strictly: unknown, duplicate or wrongly cased
// members and malformed dates in "today" are errors. Recalled dates are kept
// as text; Validate and Evaluate judge them.
func DecodeInput(b []byte) (Input, error) {
	var in Input
	if err := json.Unmarshal(b, &in, json.RejectUnknownMembers(true)); err != nil {
		return Input{}, fmt.Errorf("decode engine input: %w", err)
	}
	return in, nil
}

// EncodeOutput encodes an Output as compact JSON with members in struct order
// and map keys sorted. The native server, the WASM build and the golden tests
// all use it, so their bytes are identical.
func EncodeOutput(o Output) ([]byte, error) {
	b, err := json.Marshal(o, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode engine output: %w", err)
	}
	return b, nil
}
