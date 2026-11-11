# The recommendation engine is written once in Go, runs natively on the server and as WASM in the browser

The engine is a pure Go package (no I/O, does not read the clock; "today" is an input parameter). The server runs the native build to send reminder emails, generate notifications and run test cases. The browser runs the `js/wasm` build to show recommendations to **all** users, including those with an account. Reason: guests and people who have withdrawn consent only have data on their device, so recommendations must be computable locally; sending their health data to the server for computation, even without storing it, is still processing data without consent. Rewriting the engine in two languages would make the two versions drift apart, and this is the part with the highest medical risk.

A prototype on 2026-11-10 (Go 1.27.3) confirmed this approach is feasible:
- Native and WASM produce byte-identical JSON results on 20/20 scenario steps.
- WASM is about 906 KB with brotli compression (1.23 MB gzip).
- Each computation takes about 0.1 ms; with a 4× CPU slowdown emulation, about 0.25 ms.

## Consequences

- The browser only downloads the WASM after the user ticks the disclaimer, and the service worker caches it. The server serves pre-compressed brotli files.
- Every API response carries the `X-RuleSet-Version` header. The client reloads the rule snapshot when the version differs, so that results in the browser match the email.
- Fallback if WASM becomes a burden: an engine written in TypeScript, with the Go server calling it through a Node sidecar.

## Considered Options

- Engine runs only on the server: guest data would have to leave the device.
- One TypeScript implementation for the browser and one Go implementation for the server: the two implementations would drift apart.
- TinyGo: produces smaller files, but its `encoding/json` and `time` are not stable.
