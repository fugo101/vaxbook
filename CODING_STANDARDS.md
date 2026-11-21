# Coding standards

Read during review (`/code-review`'s Standards axis). These are judgement calls that no linter can check; mechanical rules live in `.golangci.yml`, `task gen:check` and the tests.

## Security limits survive every state transition

An attempt counter, resend wait, rate limit or lockout must hold across **every** path that changes the state it lives in. For each limit, list the operations that delete, replace or reset its row or key, and check that none of them hands the caller a fresh budget early. A test drives the limit through each of those paths.

_Why:_ the first identity slice deleted an OTP challenge after the 5th wrong guess, which also deleted the 60 s resend wait, so a caller could buy a fresh code and five fresh guesses with no wait at all (fixed in PR #4).

## Timestamps at database precision

A time value that is both written to Postgres and returned by the API comes from the module's clock truncated to microseconds (see `identity.Service.now`). Otherwise the response to a write and a later read of the same field disagree. Test harnesses run a fake clock with a sub-microsecond part so this shows up.

## Logs carry internal UUIDs only

Logs hold internal identifiers, route patterns, statuses and durations, never emails, nicknames, codes, request bodies or raw paths (SAD §9). Treat `err.Error()` and panic values as untrusted: before logging one, make sure it cannot carry request data (for example, a mailer error that quotes the recipient). Extend `TestLogsHoldNoEmailOrCode` when a new flow handles personal data.
