# Rule set validation cases

Rule-set snapshots for `ruleset.Validate`. **Every snapshot here is illustrative, not medically verified** (each carries `"illustrative": true`). Each `reject-*` file deliberately breaks an engine-semantics check of SAD §5.2 and expects at least one `reject` Problem. Each `warn-*` file (since S3b) is accepted, with only `warn` Problems.

Each file is `{ "snapshot": Snapshot, "problems": [Problem] }` and is decoded strictly (unknown members fail). `problems` is the complete expected result of `ruleset.Validate(snapshot)`, written by hand before the code, in document order of `path`. Members of a Problem are written in Go struct order (`code`, `severity`, `path`, then `params` when present).

| File | Check | Expected |
|---|---|---|
| `reject-plans-different-dose-counts.json` | EQ5: every plan in one series has the same number of doses | plan 1 has 2 doses where plan 0 has 3 → one `reject` Problem at `/series/0/plans/1` |
| `reject-dose-rule-without-minimum.json` | A7: every dose rule has a minimum age or a minimum interval | dose 2 of plan 0 has neither → one `reject` Problem at `/series/0/plans/0/doses/1` |
| `reject-reports-every-problem.json` | both defects in one snapshot | both Problems, so `Validate` reports every problem, not the first |
| `reject-first-dose-interval-from-first.json` | dose 1 has no earlier dose, so it has no `minIntervalFromFirst` (S2) | one `FIRST_DOSE_HAS_INTERVAL` at `/series/0/plans/0/doses/0/minIntervalFromFirst`; dose 2 has only a `minIntervalFromFirst`, which satisfies A7, so it has no problem |
| `reject-product-disease-mismatch.json` | a series' product lists the series' disease (S3a) | product `p` lists only `a`, and series `b-p` of disease `b` names it → one `PRODUCT_DISEASE_MISMATCH{disease: b, product: p}` at `/series/1/product`; series `a-p` is valid |
| `reject-series-product-unknown.json` | a series' product is in the catalog (S3a) | series `a-q` names `q`, which is not a product → one `UNKNOWN_REFERENCE{product: q}` at `/series/0/product` |
| `reject-schema-product-duplicate-disease.json` | a product lists each disease once (S3a) | product `p` lists `a` twice → one `DUPLICATE_VALUE{disease: a}` at `/products/0/diseases/1` |
| `reject-schema-series-product-empty.json` | a present series product is not empty (S3a) | `"product": ""` → one `REQUIRED` at `/series/0/product`; the series does not become generic |
| `reject-start-age-empty.json` | a start-age bracket's `max` is certainly after its `min` for every month length (S3b) | `[P4W, P1M)` is empty when the month has 28 days → one `START_AGE_EMPTY` at `/series/0/startAge` |
| `reject-start-age-required.json` | EQ26: every series of a product group with more than one series has a `startAge` (S3b) | product `p` has `a-p-young` `[P0D, P1Y)` and `a-p-old` with no bracket → one `START_AGE_REQUIRED{disease: a, product: p}` at `/series/1` |
| `reject-start-age-overlap.json` | EQ26: the brackets of one product group do not overlap (S3b) | generic `a-young` `[P9Y, P15Y)` then `a-old` `[P14Y, —)` → one `START_AGE_OVERLAP{id: a-young}` at `/series/1/startAge` |
| `warn-first-dose-min-age-above-start-age.json` | dose 1's `minAge` is at most `startAge.min` (S3b) | `[P9Y, —)` with dose-1 `minAge` P10Y → accepted with one `FIRST_DOSE_MIN_AGE_ABOVE_START_AGE` Warn at `/series/0/plans/0/doses/0/minAge` |
| `warn-first-dose-min-age-differs.json` | the dose-1 `minAge` is the same across a product group (S3b, Q43) | `a-young` `[P9Y, P15Y)` with `minAge` P9Y, then `a-old` `[P15Y, —)` with P12Y → accepted with one `FIRST_DOSE_MIN_AGE_DIFFERS{id: a-young}` Warn at `/series/1/plans/0/doses/0/minAge` |

Every snapshot here is shape-valid, so only `ruleset.Validate` rejects it (and the schema accepts every `warn-*` file), except the files named `reject-schema-*`: they break a schema keyword that `ruleset.Validate` mirrors (`uniqueItems`, `minLength`, the `Id` pattern), and `contracts` checks that the schema rejects them too.

Notation: plan 0 sets the dose count of its series, and every other plan with a different count is reported at that plan's path. The codes `PLAN_DOSE_COUNT_MISMATCH` and `DOSE_RULE_NO_MINIMUM` are the first entries of the closed `ruleset` Problem catalog introduced in S1; renaming them is a format change reviewed in the PR.
