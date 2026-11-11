# PROTOTYPE: VaxBook engine (Go → native + WASM)

> **Throwaway code.** It exists to answer one question. Do not import it into the real codebase; the real engine will be rewritten in `engine/` following the SAD.

**Question:** Can a pure Go engine handle the 5 hardest cases in the spec? Specifically:
- typed rule data, "today" passed in as a parameter, output as reason codes;
- run natively (server) and as WASM (browser) with **byte-identical** results;
- how big is the WASM file, and how long does one evaluation take?

## Run

```sh
cd engine-prototype
go run ./cmd/build                     # go.mod pins toolchain go1.27.3 (downloaded automatically)
open dist/vaxbook-engine-prototype.html
```

- The HTML file is self-contained (WASM is inlined as base64). Open it by double-clicking.
- It has 5 guided scenarios plus a "Free play" tab.
- The "Run native ↔ WASM parity check" button compares every step against the golden output from the native build.

## Layout

| Path | Contents |
|---|---|
| `engine/` | Pure engine: `date.go` (civil dates, month clamp, weeks = 7 days), `types.go` (rule set / input / output), `engine.go` |
| `scenarios/ruleset.json` | **Illustrative** rule data, not medically verified |
| `scenarios/scenarios.go` | The 5 scenarios, defined once and shared by native and WASM |
| `cmd/wasm` | `js/wasm` entry point: `vaxbookLoadRuleSet`, `vaxbookEvaluate` |
| `cmd/build` | Builds the WASM, measures it, runs golden natively, and inlines everything into one HTML file |
| `web/template.html` | Page shell (throwaway) |

Results and decisions are recorded in `docs/sad.md` (Prototype section) and the engine ADR.
