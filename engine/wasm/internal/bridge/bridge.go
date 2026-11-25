// Package bridge is the JSON-in, JSON-out surface of the engine's WASM build.
// It has no syscall/js dependency, so it is linted and tested natively as
// well as under js/wasm; engine/wasm only wires it to JavaScript.
package bridge

import (
	"encoding/json/v2"
	"errors"
	"sync/atomic"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
)

// Bridge holds the engine for the rule set loaded last.
type Bridge struct {
	engine atomic.Pointer[engine.Engine]
}

// Error kinds of a failed call.
const (
	kindDecode    = "decode"    // the JSON does not match the contract
	kindRuleSet   = "ruleset"   // ruleset.Validate rejects the snapshot; problems listed
	kindInput     = "input"     // engine.Validate problems stop Evaluate; problems listed
	kindNotLoaded = "notLoaded" // Evaluate before a successful LoadRuleSet
	kindInternal  = "internal"  // an engine bug; never expected
)

type callError struct {
	Kind     string `json:"kind"`
	Message  string `json:"message,omitempty"`
	Problems any    `json:"problems,omitempty"`
}

type errorResult struct {
	Error callError `json:"error"`
}

func fail(kind, message string, problems any) []byte {
	b, err := json.Marshal(errorResult{Error: callError{Kind: kind, Message: message, Problems: problems}}, json.Deterministic(true))
	if err != nil {
		return []byte(`{"error":{"kind":"internal"}}`)
	}
	return b
}

// LoadRuleSet decodes and validates a snapshot and makes it the current rule
// set. It returns {"ok":true} or {"error":{kind, message?, problems?}}; on
// error the previous rule set stays loaded.
func (b *Bridge) LoadRuleSet(snapshot []byte) (res []byte) {
	defer func() {
		if r := recover(); r != nil {
			res = fail(kindInternal, "", nil)
		}
	}()
	s, err := ruleset.Decode(snapshot)
	if err != nil {
		return fail(kindDecode, err.Error(), nil)
	}
	e, err := engine.New(s)
	var rejected *ruleset.RejectedError
	switch {
	case errors.As(err, &rejected):
		return fail(kindRuleSet, "", rejected.Problems)
	case err != nil:
		return fail(kindInternal, err.Error(), nil)
	}
	b.engine.Store(e)
	return []byte(`{"ok":true}`)
}

// Evaluate runs the current engine on an Input. It returns
// {"output":<engine.EncodeOutput bytes>} or {"error":{kind, message?,
// problems?}}. The output bytes are exactly those of the native encoder.
func (b *Bridge) Evaluate(input []byte) (res []byte) {
	defer func() {
		if r := recover(); r != nil {
			res = fail(kindInternal, "", nil)
		}
	}()
	e := b.engine.Load()
	if e == nil {
		return fail(kindNotLoaded, "", nil)
	}
	in, err := engine.DecodeInput(input)
	if err != nil {
		return fail(kindDecode, err.Error(), nil)
	}
	out, err := e.Evaluate(in)
	var inputErr *engine.InputError
	switch {
	case errors.As(err, &inputErr):
		return fail(kindInput, "", inputErr.Problems)
	case err != nil:
		return fail(kindInternal, err.Error(), nil)
	}
	enc, err := engine.EncodeOutput(out)
	if err != nil {
		return fail(kindInternal, err.Error(), nil)
	}
	res = make([]byte, 0, len(enc)+len(`{"output":}`))
	res = append(res, `{"output":`...)
	res = append(res, enc...)
	return append(res, '}')
}
