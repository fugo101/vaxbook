# Coding standards

Read during review (`/code-review`'s Standards axis). These are judgement calls that no linter can check; mechanical rules live in `.golangci.yml`, `task gen:check` and the tests.

## Security limits survive every state transition

An attempt counter, resend wait, rate limit or lockout must hold across **every** path that changes the state it lives in. For each limit, list the operations that delete, replace or reset its row or key, and check that none of them hands the caller a fresh budget early. A test drives the limit through each of those paths.

_Why:_ the first identity slice deleted an OTP challenge after the 5th wrong guess, which also deleted the 60 s resend wait, so a caller could buy a fresh code and five fresh guesses with no wait at all (fixed in PR #4).

## Timestamps at database precision

A time value that is both written to Postgres and returned by the API comes from the module's clock truncated to microseconds (see `identity.Service.now`). Otherwise the response to a write and a later read of the same field disagree. Test harnesses run a fake clock with a sub-microsecond part so this shows up.

## Logs carry internal UUIDs only

Logs hold internal identifiers, route patterns, statuses and durations, never emails, nicknames, codes, request bodies or raw paths (SAD §9). Treat `err.Error()` and panic values as untrusted: before logging one, make sure it cannot carry request data (for example, a mailer error that quotes the recipient). Extend `TestLogsHoldNoEmailOrCode` when a new flow handles personal data.

## The engine contract grows additively

Rule set snapshots, engine inputs and engine outputs (SAD §5.1) are stored and published: snapshot versions are immutable, and clients validate against `contracts/ruleset.schema.json`. So:

- A member added after S1 is optional (`omitzero`) unless every stored document is migrated in the same change.
- `null` is never valid. An optional member is omitted; a future three-state value uses an enum, never null.
- A field with neither `omitempty` nor `omitzero` is required. The decoders (`ruleset/contractjson`) and the schema test read this from the struct tag, so the tag is the decision.
- A contract migration (a rename or a type change) is its own commit. It changes the Go types, the schema and the fixture and golden files, and no decision logic, so the unchanged engine checks the migrated files byte for byte.

_Why:_ the S1 decoders accepted JSON null and missing members, so a truncated snapshot loaded with no assessments, and an input without `doses` showed doses the person already had as overdue (fixed in the S1 contract review).
