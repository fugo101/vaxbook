# Golden files

Each file is `{ "ruleset": "<fixture name>", "input": Input, "output": Output }` and is decoded with the contract's shape rules (unknown or missing members and `null` fail). `ruleset` names a file in `../rulesets/`.

**The expected `output` is written by hand from the spec (SAD §5.1–§5.3, spec F05 and F13.2) before the engine code exists.** A changed expectation is re-derived by hand here first. An expectation is never written from engine output.

`TestGolden` compares compact bytes, so it enforces the canonical encoding: members of `output` in Go struct order (`ruleSetVersion`, `today`, `assessments`; `disease`, `bucket` or `block`, `nextDose`, `reasons`; `number`, `earliest`, `due`, `overdue`; `code`, `params`), `params` keys sorted, optional members omitted when empty. It decodes the expectation into an Output and re-encodes it to tell the cases apart:

- the expectation does not decode (an unknown, renamed, missing or `null` member, a wrong type, a bad date): an error, never rewritten;
- the same values in a different encoding (member order, spacing): fails with a hint; `go test ./engine -run TestGolden -update` rewrites the `output` member with the canonical encoding of the expectation itself;
- different values: fails with "re-derive by hand"; `-update` never writes it.

**A contract migration is its own commit.** It changes the Go types, the schema and the fixture/golden files (by hand or with an uncommitted script) and no decision logic. The unchanged engine then checks the migrated files byte for byte.

## Notation

- E = earliest allowed date, Du = due date, O = overdue date.
- Per plan: E = max(birth + minAge, latest dose in time + minInterval); Du = max(E, anchor + offset); O = max(Du + buffer, Du + 7d).
- Alternative plans: E and Du from the plan with the smallest Du (SAD §5.3 step 7 merge 1, A7), even when another plan has a smaller E; O = the latest O of all plans. Ties (two plans with the same Du but different E) are not specified by the SAD and no golden depends on them.
- T30 = today + 30 days; T6m = today + 6 months (always added to today).
- Bucket: today ≥ O → overdue; else today ≥ E and Du ≤ T30 → due; else Du ≤ T6m → upcoming; else ≥ 1 dose counted → complete + `ON_TRACK`; else `notYetScheduled`. `DOSE_INVALID`, or a record-scoped date problem (`INVALID_DATE`, `OUT_OF_RANGE`, `DATE_IN_FUTURE`, `DATE_BEFORE_BIRTH`) → unknown with no nextDose (SAD §5.1); the reasons of the masked bucket (`COMPLETE`, `ON_TRACK`, `ALTERNATIVE_PLANS`) stay.
- Months keep the day of month and clamp to the month end; 1w = 7d; 1y = 12m. All doses are precision `day`.
- Fixtures write spans as ISO 8601 durations with one designator (`"P6W"`, `"P2M"`, `"P14D"`); the derivations below write them short (6w, 2m, 14d).

## Derivations

### illustrative-vn

- `f13-5-week-old-upcoming`: birth 2026-01-10, today 2026-02-14 (5 weeks old), no doses. Pertussis dose 1 in both plans: E = 01-10 + 42d = 02-21; Du = max(02-21, 01-10 + 2m = 03-10) = 03-10; O = 03-17. T30 = 03-16; today < E and Du ≤ T30 → upcoming, `ALTERNATIVE_PLANS`. Measles: E = Du = 01-10 + 9m = 10-10, O = 10-17; T6m = 08-14 < Du, no dose → `notYetScheduled`.
- `f13-no-dose-not-yet-scheduled`: birth = today = 2026-03-01. Pertussis: E = 03-01 + 42d = 04-12, Du = 05-01, O = 05-08; T30 = 03-31 < Du ≤ T6m = 09-01 → upcoming, `ALTERNATIVE_PLANS`. Measles: E = Du = 12-01, O = 12-08; Du > T6m = 09-01, no dose → `notYetScheduled`.
- `f13-on-track`: birth 2025-01-15, today 2025-11-01. Pertussis p1 03-15 ≥ 01-15 + 42d = 02-26; p2 04-15 ≥ 03-15 + 28d = 04-12; p3 05-15 ≥ 04-15 + 28d = 05-13: 3 of 3 counted → complete, `COMPLETE`, no nextDose. Measles m1 2025-10-15 = 01-15 + 9m (inclusive) → counted; dose 2: E = 10-15 + 28d = 11-12, Du = 01-15 + 18m = 2026-07-15, O = 2026-07-22; T6m = 2026-05-01 < Du, 1 dose → complete, `ON_TRACK`.
- `alt-plans-due-window`: birth 2026-01-10, p1 03-10 (≥ 02-21, counted), today 04-20. Dose 2: plan 2-3-4 E = 03-10 + 28d = 04-07, Du = 01-10 + 3m = 04-10, O = 04-17; plan 2-4-6 E = 04-07, Du = 05-10, O = 05-17. Merged E 04-07, Du 04-10, O 05-17. today ≥ E, Du ≤ T30 = 05-20, today < O → due (2-3-4 alone would be overdue), `ALTERNATIVE_PLANS`. Measles E = Du = 10-10, O = 10-17; Du ≤ T6m = 10-20 → upcoming.
- `alt-plans-overdue-from-latest-plan`: as above, today 2026-05-17 = merged O → overdue, `ALTERNATIVE_PLANS`. Measles Du 10-10 ≤ T6m = 11-17 → upcoming.
- `a9-invalid-date-every-disease-of-record`: r1 value "2026-02-30" (no such day) on pertussis and measles → both unknown, `INVALID_DATE{dose: r1}`, no nextDose.
- `a9-invalid-date-other-disease-unaffected`: r1 "2026-02-30" on pertussis only → pertussis unknown, `INVALID_DATE{dose: r1}`. Measles has no records: E = Du = 10-10, O = 10-17; today 2026-04-01, T6m = 10-01 < Du → `notYetScheduled`.
- `record-dose-in-future`: birth 2026-01-10, today 2026-04-01; r1 2026-04-02 (after today) on pertussis → record-scoped (SAD §5.1): pertussis unknown `DATE_IN_FUTURE{dose: r1}`, no nextDose. Measles: E = Du = 10-10, O = 10-17; T6m = 10-01 < Du, no dose → `notYetScheduled`.
- `record-dose-before-birth`: same dates; r1 2026-01-09 (before birth) on pertussis and measles → both unknown `DATE_BEFORE_BIRTH{dose: r1}`, no nextDose.
- `dose-invalid-masks-on-track`: birth 2025-01-15, today 2025-11-01. Pertussis p1, p2, p3 as in `f13-on-track` → complete, `COMPLETE`. Measles m1 10-15 = 01-15 + 9m counted; m2 10-20 < 10-15 + 28d = 11-12 → `DOSE_INVALID{checks: [minInterval], dose: m2, number: 2}`. Dose 2: E = m2 10-20 + 28d = 11-17, Du = max(E, 01-15 + 18m = 2026-07-15) = 2026-07-15, O = 2026-07-22; today < E, Du > T6m = 2026-05-01, 1 dose counted → the bucket would be complete `ON_TRACK`, which the invalid dose masks (SAD §5.3 step 10) → unknown, `DOSE_INVALID` then `ON_TRACK`, no nextDose.
- `a9-invalid-date-not-iso`: birth 2026-01-10, today 2026-04-01; r1 "10/02/2026" on pertussis and r2 "2026-1-5" on measles are not YYYY-MM-DD → pertussis unknown `INVALID_DATE{dose: r1}`, measles unknown `INVALID_DATE{dose: r2}`, no nextDose. (The schema accepts any string as a recalled date value, so a client that validates against it reaches ⚪ too.)
- `a9-invalid-date-empty-value`: same dates; r1 value "" with precision day on pertussis → pertussis unknown `INVALID_DATE{dose: r1}`. Measles: E = Du = 10-10, O = 10-17; T6m = 10-01 < Du, no dose → `notYetScheduled`.

### minimal-single (birth 2026-01-10; dose 1 min age 6w = 02-21, at 2m = 03-10; dose 2 min interval 4w, at 4m = 05-10)

- `f13-dose-before-min-age`: today 02-20, r1 02-14 < 02-21 → `DOSE_INVALID{checks: [minAge], dose: r1, number: 1}`, not counted → unknown, no nextDose.
- `f13-dose-before-min-interval`: today 04-01, r1 03-10 counted; r2 03-31 < 03-10 + 28d = 04-07 → `DOSE_INVALID{checks: [minInterval], dose: r2, number: 2}` → unknown, no nextDose. (The earliest date after an uncounted dose, measured from that dose, is pinned by `TestNextAfterUncountedDose`.)
- `a4-interval-to-adjacent-dose`: today 04-15. The input lists the doses out of time order (r3, r1, r2); the engine walks them in time order (SAD §5.3 step 2). r1 03-10 counted. r2 03-24 < 04-07 → invalid. r3 04-14 < r2 03-24 + 28d = 04-21 → invalid (measured to the adjacent r2; against r1 it would pass and complete the series) → unknown, no nextDose, two `DOSE_INVALID` reasons in dose time order.
- `complete`: today 06-01, r1 03-10 counted, r2 05-10 ≥ 04-07 counted → complete, `COMPLETE`.
- `boundary-dose-on-min-interval-counted`: today 04-10, r1 03-10 counted, r2 04-07 = r1 + 28d exactly → the minimum interval is met on the day it is reached, counted → complete, `COMPLETE`. (An exclusive check would give unknown, `DOSE_INVALID{checks: [minInterval], dose: r2, number: 2}`.)
- `boundary-due-on-earliest-date`: no doses, today 02-21 = E, Du 03-10 ≤ T30 = 03-23, today < O 03-17 → due (the 🟠 condition today ≥ E is inclusive; today 02-20 would be upcoming).
- `f13-buffer0-due-on-due-date`: no doses, today 03-10 = Du: today ≥ E 02-21, Du ≤ T30, today < O 03-17 → due.
- `buffer0-due-day-before-overdue`: today 03-16 < O 03-17 → due.
- `f13-buffer0-overdue-at-due-plus-7`: today 03-17 = Du + 7d = O → overdue.

### minimal-alt-plans (birth 2026-01-10, r1 03-10 counted; dose 2: fast 4w at 3m, slow 8w at 4m)

- `a7-earliest-from-earliest-due-plan`: today 03-20. fast E = 04-07, Du = 04-10, O = 04-17; slow E = 03-10 + 56d = 05-05, Du = 05-10, O = 05-17. Merged E 04-07 (from fast, not the latest E 05-05), Du 04-10, O 05-17. today < E, Du ≤ T30 = 04-19 → upcoming, `ALTERNATIVE_PLANS`.
- `alt-plans-dose-valid-under-one-plan`: today 04-20, r2 04-14 ≥ 04-07 (fast) though < 05-05 (slow) → counted → complete, `COMPLETE` (no step 7, so no `ALTERNATIVE_PLANS`).
- `alt-plans-dose-invalid-under-every-plan`: today 04-01, r2 03-31 < 04-07 (fast) and < 05-05 (slow) → `DOSE_INVALID{checks: [minInterval], dose: r2, number: 2}`, not counted under any plan. Step 7 still merges the plans → unknown, no nextDose, `DOSE_INVALID` then `ALTERNATIVE_PLANS`.

### minimal-buffer (buffer 14d: O = Du + 14d)

- `boundary-buffer14-due-at-due-plus-7`: birth 2026-01-10, no doses, today 03-17. E = 02-21, Du = 03-10, O = 03-24. today = Du + 7d < O → due (the 7-day grace does not apply when buffer > 7d).
- `boundary-buffer14-overdue-at-due-plus-14`: today 03-24 = O → overdue.
- `boundary-due-at-today-plus-30`: birth 2025-01-10, r1 2025-03-10 (≥ 02-21, counted), today 2025-12-11. Dose 2: E = 2025-04-07, Du = 01-10 + 12m = 2026-01-10, O = 2026-01-24. T30 = 2025-12-11 + 30d = 2026-01-10 = Du (inclusive), today ≥ E → due.
- `boundary-due-at-today-plus-31`: as above, today 2025-12-10: T30 = 2026-01-09 < Du ≤ T6m = 2026-06-10 → upcoming.
- `boundary-due-at-today-plus-6-months-clamped`: birth 2026-02-28, r1 2026-04-28 (≥ 02-28 + 42d = 04-11, counted), today 2026-08-31. Dose 2: E = 04-28 + 28d = 05-26, Du = 02-28 + 12m = 2027-02-28, O = 2027-03-14. T6m = 2026-08-31 + 6m = 2027-02-31 → clamped to 2027-02-28 = Du (inclusive) → upcoming. This pins inclusivity at T6m; a non-clamping T6m (2027-03-03) gives the same output, so the clamp itself is pinned by the next case.
- `boundary-due-past-clamped-6-months-on-track`: birth 2026-03-01, r1 2026-05-01 (≥ 03-01 + 42d = 04-12, counted), today 2026-08-31. Dose 2: E = 05-01 + 28d = 05-29, Du = 03-01 + 12m = 2027-03-01, O = 2027-03-15. T30 = 2026-09-30; T6m = 2027-02-31 → 2027-02-28 < Du, 1 dose counted → complete, `ON_TRACK`. (A non-clamping T6m of 2027-03-03 would give upcoming.)
- `boundary-due-past-6-months-on-track`: as `boundary-due-at-today-plus-6-months-clamped`, today 2026-08-27: T6m = 2027-02-27 < Du, 1 dose counted → complete, `ON_TRACK`.
- `boundary-6-months-added-to-today`: birth 2026-03-31, r1 2026-05-31 (≥ 03-31 + 42d = 05-12, counted), today 2026-09-30. Dose 2: E = 05-31 + 28d = 06-28, Du = 2027-03-31, O = 2027-04-14. T6m = 2026-09-30 + 6m = 2027-03-30 < Du → complete, `ON_TRACK`. (Subtracting 6m from Du would give 2026-09-30 ≤ today and wrongly give upcoming.)

### minimal-late-start (one dose, min age 9m at 9m, buffer 14d; today 2026-08-31, T6m = 2027-02-31 → 2027-02-28; no doses)

- `boundary-first-dose-at-6-months`: birth 2026-05-28: E = Du = 2027-02-28, O = 2027-03-14; Du = T6m → upcoming.
- `boundary-first-dose-clamped-onto-6-months`: birth 2026-05-31: + 9m = 2027-02-31 → 2027-02-28 = E = Du, O = 2027-03-14 → upcoming.
- `boundary-first-dose-past-6-months-not-yet-scheduled`: birth 2026-06-01: E = Du = 2027-03-01, O = 2027-03-15; Du > T6m = 2027-02-28, no dose → `notYetScheduled` (a non-clamping T6m of 2027-03-03 would give upcoming).

### minimal-anchors (dose 1 min age 9y; birth 2015-03-31, so 9y = 2024-03-31, unless a case states otherwise)

- `anchor-first-dose-clamped`: r1 2024-12-31 ≥ 2024-03-31 counted; today 2025-02-10. Dose 2: E = 12-31 + 28d = 2025-01-28; Du = max(E, firstDose + 2m = 2025-02-31 → 2025-02-28) = 2025-02-28; O = 2025-03-07. today ≥ E, Du ≤ T30 = 2025-03-12 → due. (For dose 2 the first and previous dose are both r1; `anchor-first-dose-for-dose-3` separates them.)
- `boundary-min-age-in-years-clamped`: birth 2016-02-29, r1 2025-02-27, today 2025-03-05. Min age 9y = 108m = 2025-02-29 → clamped to 2025-02-28 > r1 → `DOSE_INVALID{checks: [minAge], dose: r1, number: 1}`, not counted → unknown, no nextDose. (9 × 365 days gives 2025-02-26 and would count r1; a non-clamping year gives 2025-03-01.)
- `anchor-previous-dose`: r1 2024-12-31, r2 2025-03-03 ≥ 2025-01-28 counted; today 2025-03-10. Dose 3: E = 03-03 + 84d = 2025-05-26; Du = max(E, previousDose + 4m = 2025-07-03) = 2025-07-03; O = 2025-07-10. T30 = 2025-04-09 < Du ≤ T6m = 2025-09-10 → upcoming.

### minimal-short-buffer (one dose, min age 6w at 2m, buffer 3d)

- `short-buffer-due-before-grace-floor`: birth 2026-01-10, no doses, today 03-13. E = 02-21, Du = 03-10, O = Du + max(3d, 7d) = 03-17. today ≥ E, Du ≤ T30, today < O → due. (Treating the 7-day grace as a default instead of a floor would give O = 03-13 and overdue.)

### minimal-alt-plans-crossed (birth 2026-01-10, r1 03-10 counted; dose 2: `early-due` 8w at 3m, `late-due` 4w at 4m)

- `a7-earliest-from-earliest-due-plan-not-min`: today 04-10. early-due E = 03-10 + 56d = 05-05, Du = max(05-05, 04-10) = 05-05, O = 05-12; late-due E = 03-10 + 28d = 04-07, Du = 05-10, O = 05-17. Merged E 05-05 (from the earliest-due plan, not the smallest E 04-07), Du 05-05, O 05-17. today < E, Du ≤ T30 = 05-10 → upcoming, `ALTERNATIVE_PLANS`. (min E would give due.)

### minimal-first-dose-anchor (birth 2015-03-31; dose 1 min age 9y at 9y; dose 2 4w at `firstDose` + 2m; dose 3 12w at `firstDose` + 6m)

- `anchor-first-dose-for-dose-3`: r1 2024-12-31 ≥ 2024-03-31 counted; r2 2025-03-03 ≥ 12-31 + 28d = 2025-01-28 counted; today 2025-05-01. Dose 3: E = 03-03 + 84d = 2025-05-26; Du = max(E, firstDose + 6m = 2025-06-31 → 2025-06-30) = 2025-06-30; O = 2025-07-07. T30 = 2025-05-31 < Du ≤ T6m = 2025-11-01 → upcoming. (Reading the anchor as the previous dose would give Du = 2025-09-03.)
