# Fixture rule sets

Shared, named rule-set snapshots used by the engine golden files (`../golden/*.json`, member `ruleset` = file name without `.json`) and by the property tests.

**Every file here is illustrative, not medically verified.** Each snapshot carries `"note": "Illustrative, not medically verified"`. Ages, intervals and milestones were chosen to exercise engine semantics (SAD §5.1–§5.3), not copied from a cited schedule. Never use them as clinical data, and never copy them into an editorial rule set.

Rules for this folder:

- A fixture holds only fields of the current snapshot contract. New fields arrive with the slice that adds them.
- Every fixture passes `ruleset.Validate` with zero problems and loads with `engine.New`. Rule sets that must be rejected live in `ruleset/testdata/validate/`.
- Every fixture is referenced by at least one golden file.
- Changing a fixture changes every golden that uses it: re-derive those goldens by hand.

| File | Shape | Used for |
|---|---|---|
| `illustrative-vn.json` | `pertussis` with alternative plans 2-3-4 and 2-4-6 months (3 doses each); `measles` with one 2-dose plan (9 and 18 months) | F13.2 cases, alternative-plan merge, A9 |
| `minimal-single.json` | one plan: dose 1 min age 6w at 2m; dose 2 min interval 4w at 4m; no buffer | dose validity, A4, buffer 0 boundaries, inclusive minimum interval and earliest date, complete |
| `minimal-alt-plans.json` | plans `fast` (4w, at 3m) and `slow` (8w, at 4m) for dose 2 | earliest date taken from the earliest-due plan (A7), per-plan dose validity |
| `minimal-buffer.json` | dose 1 at 2m, dose 2 at 12m, buffer 14d on both | buffer > 7 days, 30-day and 6-month boundaries |
| `minimal-late-start.json` | one dose, min age 9m at 9m, buffer 14d | first dose at the 6-month boundary, month clamping |
| `minimal-anchors.json` | dose 1 at 9y; dose 2 at `firstDose` + 2m; dose 3 at `previousDose` + 4m (min interval 12w) | milestone anchors, minimum age in years |
| `minimal-short-buffer.json` | one dose, min age 6w at 2m, buffer 3d | the 7-day grace is a floor, not a default (O = Du + max(buffer, 7d)) |
| `minimal-alt-plans-crossed.json` | plans `early-due` (8w, at 3m) and `late-due` (4w, at 4m) for dose 2: the earliest-due plan has the later earliest date | earliest date from the earliest-due plan, not the smallest earliest date (A7) |
| `minimal-first-dose-anchor.json` | dose 1 at 9y; doses 2 and 3 at `firstDose` + 2m and + 6m (HPV 0-2-6 shape) | `firstDose` anchor for a dose after dose 2 |
