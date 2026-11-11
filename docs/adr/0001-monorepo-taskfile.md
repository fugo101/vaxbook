# One repo for Go, React and the engine; orchestrated with Taskfile

All of VaxBook (Go API, React web, engine, OpenAPI contract) lives in one repo, and builds are orchestrated with Taskfile. Reasons: there is only one operator; most changes touch both the API and the UI; and the web depends directly on the WASM build of the Go engine (ADR-0002), so splitting repos would force publishing artifacts back and forth between repos.

## Considered Options

- Two repos (backend / frontend): API contract versions must be kept in sync, and two separate CIs must run.
- Three repos (plus a separate repo for the engine): same as above, plus a WASM release cycle.
