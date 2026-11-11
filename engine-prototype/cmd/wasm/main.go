//go:build js && wasm

// PROTOTYPE — throwaway. Browser entry point: exposes the same engine to JS.
package main

import (
	"encoding/json"
	"syscall/js"

	"vaxbook.local/engine-prototype/engine"
)

var eng *engine.Engine

func main() {
	js.Global().Set("vaxbookLoadRuleSet", js.FuncOf(func(this js.Value, args []js.Value) any {
		var rs engine.RuleSet
		if err := json.Unmarshal([]byte(args[0].String()), &rs); err != nil {
			return err.Error()
		}
		e, err := engine.New(&rs)
		if err != nil {
			return err.Error()
		}
		eng = e
		return ""
	}))
	js.Global().Set("vaxbookEvaluate", js.FuncOf(func(this js.Value, args []js.Value) any {
		var in engine.Input
		if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
			b, _ := json.Marshal(map[string]string{"error": err.Error()})
			return string(b)
		}
		b, _ := json.Marshal(eng.Evaluate(in))
		return string(b)
	}))
	select {}
}
