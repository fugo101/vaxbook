package engine

import (
	"encoding/json/v2"
	"fmt"

	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

// DecodeInput parses an Input strictly, with the shape rules of
// contracts/ruleset.schema.json: unknown, duplicate or wrongly cased
// members, missing required members, JSON null and a malformed "today" are
// errors. A recalled date's value is any string, kept as text; Validate and
// Evaluate judge it.
func DecodeInput(b []byte) (Input, error) {
	var in Input
	if err := contractjson.Unmarshal(b, &in); err != nil {
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
