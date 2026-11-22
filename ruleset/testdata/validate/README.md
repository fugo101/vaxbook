# Rule set validation cases

Rejected rule-set snapshots for `ruleset.Validate`. **Every snapshot here is illustrative, not medically verified** (each carries `"illustrative": true`), and each one deliberately breaks an engine-semantics check of SAD §5.2.

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

Every snapshot here is shape-valid, so only `ruleset.Validate` rejects it, except the files named `reject-schema-*`: they break a schema keyword that `ruleset.Validate` mirrors (`uniqueItems`, `minLength`, the `Id` pattern), and `contracts` checks that the schema rejects them too.

Notation: plan 0 sets the dose count of its series, and every other plan with a different count is reported at that plan's path. The codes `PLAN_DOSE_COUNT_MISMATCH` and `DOSE_RULE_NO_MINIMUM` are the first entries of the closed `ruleset` Problem catalog introduced in S1; renaming them is a format change reviewed in the PR.
