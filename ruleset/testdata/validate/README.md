# Rule set validation cases

Rejected rule-set snapshots for `ruleset.Validate`. **Every snapshot here is illustrative, not medically verified** (each carries `"illustrative": true`), and each one deliberately breaks an engine-semantics check of SAD §5.2.

Each file is `{ "snapshot": Snapshot, "problems": [Problem] }` and is decoded strictly (unknown members fail). `problems` is the complete expected result of `ruleset.Validate(snapshot)`, written by hand before the code, in document order of `path`. Members of a Problem are written in Go struct order (`code`, `severity`, `path`, then `params` when present).

| File | Check | Expected |
|---|---|---|
| `reject-plans-different-dose-counts.json` | EQ5: every plan in one series has the same number of doses | plan 1 has 2 doses where plan 0 has 3 → one `reject` Problem at `/series/0/plans/1` |
| `reject-dose-rule-without-minimum.json` | A7: every dose rule has a minimum age or a minimum interval | dose 2 of plan 0 has neither → one `reject` Problem at `/series/0/plans/0/doses/1` |
| `reject-reports-every-problem.json` | both defects in one snapshot | both Problems, so `Validate` reports every problem, not the first |

Notation: plan 0 sets the dose count of its series, and every other plan with a different count is reported at that plan's path. The codes `PLAN_DOSE_COUNT_MISMATCH` and `DOSE_RULE_NO_MINIMUM` are the first entries of the closed `ruleset` Problem catalog introduced in S1; renaming them is a format change reviewed in the PR.
