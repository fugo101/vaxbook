//go:build js && wasm

// Command wasm is the engine's browser entry point (ADR-0002). It registers
// two global JavaScript functions, each taking and returning a JSON string:
//
//	vaxbookLoadRuleSet(snapshot) → {"ok":true} | {"error":{kind, message?, problems?}}
//	vaxbookEvaluate(input)       → {"output":Output} | {"error":{kind, message?, problems?}}
//
// The logic lives in package bridge, which is tested natively and as WASM;
// main_test.go calls the globals themselves through syscall/js.
package main

import (
	"syscall/js"

	"github.com/fugo101/vaxbook/engine/wasm/internal/bridge"
)

func main() {
	register()
	select {} // keep the Go runtime alive for later calls
}

// register sets the two global functions, sharing one fresh Bridge.
func register() {
	var b bridge.Bridge
	js.Global().Set("vaxbookLoadRuleSet", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return string(b.LoadRuleSet([]byte(firstString(args))))
	}))
	js.Global().Set("vaxbookEvaluate", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return string(b.Evaluate([]byte(firstString(args))))
	}))
}

// firstString returns the first argument if it is a string; anything else
// becomes "" and is reported as a decode error.
func firstString(args []js.Value) string {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return ""
	}
	return args[0].String()
}
