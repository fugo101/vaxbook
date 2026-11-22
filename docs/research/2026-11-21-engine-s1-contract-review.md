# Engine S1 contract review: decisions (2026-11-21)

The owner merged S1 (#20, issue #12, parent #11) and asked for four points to be fixed **if the fix is better**. Five designers proposed changes. This record is the judged result, and the follow-up PR implements it. Sources: SAD §3, §4, §5.1–§5.7; spec F04, F05, F13.2, Appendix C; GLOSSARY; ADR-0002/0003/0004; issues #11, #13–#19.

## Checked facts

These were checked against the code and by running it, not taken from the proposals:

- **json/v2 (go1.27):**
  - JSON `null` decodes to the zero value with no error, and `UnmarshalText` is never called for it.
  - Missing members decode with no error.
  - Member names are matched case-sensitively.
  - A nil slice encodes as `[]`.
  - There is no "required" option.
- **The current engine:**
  - `{"version":"x"}` decodes and loads, and `Evaluate` then returns `"assessments":[]`.
  - An Input without `doses` evaluates as if no doses were given (an 11-week-old shows pertussis `overdue`).
  - `{"value":null,"precision":"day"}` gives ⚪ `INVALID_DATE`.
  - The schema rejects `"10/02/2026"`, but the engine assesses it as ⚪ `INVALID_DATE`.
- **Test data:**
  - There are 36 goldens. `DOSE_INVALID` appears in **5** of them, not 6. The same 5 are the only ⚪ goldens that carry `nextDose`: `a4-interval-to-adjacent-dose`, `alt-plans-dose-invalid-under-every-plan`, `boundary-min-age-in-years-clamped`, `f13-dose-before-min-age`, `f13-dose-before-min-interval`.
  - No golden or fixture contains `null`.
  - No golden contains a span.
- **Repo checks:**
  - depguard and `TestImportsStdlibOnly` match allowed imports by prefix, so a package under `ruleset/` needs no configuration change.
  - forbidigo is already enabled for the non-test files of `engine/` and `ruleset/`.
- **New golden expectations:** each one below that does not depend on a semantic change was run on the current engine and matches the hand derivation (`a9-*` ×2, the dose-1 repeat, the alternative-plan tie, the masked `ON_TRACK`).

## Rules that apply to every item

- **R1. Shape versus content.** The decoders (`ruleset.Decode`, `engine.DecodeInput`) and the schema agree on shape: member names, required members, JSON types, no null, and the syntax of the values Go parses itself (`Span`, `Date`).
  - Every content rule belongs to `ruleset.Validate` / `engine.Validate`.
  - So does every schema keyword the decoders do not enforce: enums, `minLength`, `minItems`, `uniqueItems` and the `Id` pattern.
- **R2. Agreement, stated precisely.**
  - I1: schema-valid ⇒ decodes.
  - I2: decodes and Validate reports nothing (no Reject for a snapshot) ⇒ schema-valid.
  - I3: for an Input, decodes and `Evaluate` returns an Output ⇒ schema-valid. In other words, the schema never rejects an Input the engine assesses, so ⚪ `INVALID_DATE` (A9) stays reachable.
  - I4: every Output validates against `$defs/Output`.
  - A test enforces all four (2.5).
- **R3. Null is never valid** in a snapshot, Input or Output. An optional member is omitted. A future three-state value uses an enum, never null.
- **R4. Requiredness comes from the struct tags.** A member whose field has neither `omitempty` nor `omitzero` is required, in the decoders and in the schema test alike.
- **R5. The contract grows additively.** A member added after S1 is optional unless every stored document is migrated.
- **R6. A golden expectation is never written from engine output.** `-update` only re-encodes a hand-written expectation.

## Decisions

### Item 1: the schema rejects malformed dates that the engine handles

**1.1 Governing rule: CHANGE.** Adopt R1 and R2.
- The schema root description becomes: "Go structs in ruleset/ and engine/ are the source (SAD §3). This schema checks shape. null is never valid; an optional member is omitted; members added after S1 are optional. Content checks live in ruleset.Validate and engine.Validate. They include the keywords the Go decoders do not enforce (enums, minLength, minItems, uniqueItems, the Id pattern). The Input schema accepts every Input that Evaluate assesses, so ⚪ INVALID_DATE (A9) stays reachable for a client that validates against it."
- SAD §5.1 gets a "Contract documents" bullet saying the same thing. It adds one sentence: a future write API may use a stricter entry schema of its own (for example, a dose date that is a valid `Date`). The engine Input schema stays permissive.

Reason: EQ27/A9 say `Evaluate` never coerces and an unparseable dose date gives ⚪. A client that validates against the published schema must not reject such a record. The two sides cannot be exactly equal, because some checks need `today` or the rule set.

**1.2 `$defs/RecalledDate.value`: CHANGE.** It becomes `{"type": "string"}` with no pattern, length or format. Its description: "YYYY-MM-DD for precision day; YYYY-MM and YYYY arrive with month and year in S2. Any string is accepted; engine.Validate reports one that does not parse as INVALID_DATE (⚪ on a dose record, input error on the birth date)." In Go only the doc comments of `engine/recalled.go` change.

Reason: the old pattern let `"2026-02-30"` through but rejected `"10/02/2026"`, `"2026-1-5"` and `""`, and the engine treats all of these the same way. A per-precision pattern would duplicate `ParseDate`, would need rewriting in S2, and TS generators ignore it anyway.

**1.3 No `maxLength`: KEEP.** A cap would create a third outcome for one class of value. The output never echoes the value, so a long value costs nothing.

**1.4 Decode error versus `INVALID_DATE`: CHANGE.** The JSON type decides:
- A non-string `value`, including `null`, is a decode error (R3).
- Any string, including `""`, or a missing `value` with precision `day`, is `INVALID_DATE` at `<path>/value`.
- `today` stays a strict `Date`.

The `InvalidDate` doc becomes: "a value that is not a parseable date, including an empty or absent value".

**1.5 One `RecalledDate` def for the birth date and dose dates: KEEP.** A stricter birth schema would replace coded, path-addressed Problems (ADR-0003) with generic schema errors. It would also need a second def for one Go struct.

**1.6 Tests: CHANGE (trimmed).**
- `contracts/schema_test.go`: replace the single calendar-invalid assertion with a table. `$defs/RecalledDate` must accept each of these values: `"2026-02-30"`, `"10/02/2026"`, `"2026-1-5"`, `""`, `" 2026-01-01"`, `"2026-01-01T00:00:00Z"`, a value with fullwidth digits, `"2026-01"`, `"2026"` and a 200-character string.
- Add `{"value": null, …}` and `{"value": 20260210, …}` to `TestSchemaRejects`.
- `engine`: extend `TestEvaluateGarbageDoseDateIsInvalidDate` with the same strings on a dose, and `TestEvaluateInputErrors` with them on the birth date.
- Two new goldens (see the golden list).
- No separate fuzz target, because 2.5 covers the crossing.

**1.7 Write-API guidance: KEEP, as a note.** One sentence in SAD §5.1, as in 1.1. No code.

### Item 2: the Go decoders are looser than the schema

**2.1 Null policy: CHANGE.** R3. The decoders return `null is not allowed at "<pointer>"`.

**2.2 Required members: CHANGE.** R4 is enforced by both decoders, with errors like `missing required member at "/series"`.
- This turns `{"version":"x"}` into an error, and likewise a missing `doses`, `profile`, `today`, `recommended`, `offset`, `id`, `diseases`, `date` or `precision`.
- `RecalledDate.value` stays optional (1.4).
- The golden test decodes the expected `output` with the same function.

**2.3 Mechanism: CHANGE.** Add a new stdlib-only package `ruleset/contractjson`:

```go
// Unmarshal decodes b into v (non-nil pointer to struct): json/v2 with
// RejectUnknownMembers, then a shape pass that rejects JSON null anywhere and
// any missing member whose field has neither omitempty nor omitzero.
// encoding.TextUnmarshaler types (Span, Date) are leaves; pointers are
// dereferenced; slices and map values are walked.
func Unmarshal(b []byte, v any) error

type Kind string // KindNull, KindMissing
type Error struct{ Pointer string; Kind Kind } // Error() = `<kind> at "<pointer>"`

type Member struct{ Name string; Type reflect.Type; Required bool }
func Members(t reflect.Type) []Member // field order; panics on embedded fields
```

- The implementer can do the shape pass with a `jsontext` token walk or over an `any` tree. Either way, errors carry RFC 6901 pointers that match `Problem.Path`.
- `ruleset.Decode` and `engine.DecodeInput` call it.
- Add the forbidigo pattern `^json\.Unmarshal` for the non-test files of `engine/` and `ruleset/`. The one call site in `contractjson` gets a `//nolint:forbidigo` with a reason.

Reason: this is a real safety bug (missing doses can show doses the person already had as overdue). The rules come from the tags, so S2–S8 add members without touching the decoder.

**2.4 One definition of "required": CHANGE.** `TestSchemaMatchesStructs` replaces its private `jsonMembers` with `contractjson.Members`.

**2.5 Agreement test: CHANGE (trimmed).** Add `contracts/agreement_test.go` with `TestSchemaAndGoAgree`, which checks I1–I4 on deterministic mutations of the seeds.
- Seeds: every fixture rule set, every `ruleset/testdata/validate` snapshot, and every golden input (the latter using the engine built from its fixture).
- Mutations, applied to every node:
  - delete the member;
  - set it to `null`;
  - replace it with each other JSON kind (`"x"`, `0`, `true`, `[]`, `{}`);
  - add an unknown member `"zz"`;
  - on arrays: replace with `[]`, and append `null`;
  - on strings: replace with `""`, `"2026-02-30"`, `"10/02/2026"`, `"P06W"` and `"P1000D"`.
- A failure names the seed, the mutation and the pointer.
- No `Fuzz` target.
- SAD §5.7 "Schema check" names the four invariants.

Reason: it guards the exact class of drift behind Items 1 and 2, and each member a later slice adds to a fixture or golden is covered without editing the test.

**2.6 Empty `diseases`: CHANGE.** `ruleset.Validate` reports `EMPTY` (Reject) at `/diseases`, and the schema gets `"minItems": 1`. The `Empty` doc gains "a snapshot has no diseases". This is an inline test case, not a `testdata/validate` file, because those files must be shape-valid. Reason: an empty snapshot silently gives no assessments, and Go-built snapshots skip Decode.

**2.7 Empty `series`: KEEP.** S1 already rejects it with `ErrUnsupported`. Later slices may have diseases without a series (info-only).

**2.8 Two Go layers with a named owner per keyword: KEEP.** This is R1, written in the `contractjson` package doc and the schema description.

**2.9 Conditional requiredness of `value`: KEEP optional.** Struct tags cannot express it. A schema `if/then` would break I3 (see 4b).

**2.10 Additive rule: CHANGE.** Add R5 as one line in SAD §5.1, and as a short "Engine contract" section in `CODING_STANDARDS.md` (additive members, R3, R4, a contract migration is its own commit).

### Item 3: the golden `-update` flag

**3.1 Format-only enforced by code: CHANGE (adjusted).** Add `compareGolden(expected jsontext.Value, got []byte) (canonical []byte, d goldenDiff, err error)`.
- It decodes `expected` with `contractjson.Unmarshal` into `engine.Output` and re-encodes it with `engine.EncodeOutput`.
- An invalid expectation is an error that is never rewritten: unknown, renamed, missing or **null** member, wrong type, bad date. This differs from the proposal, which treated an explicit `null` as format; under R3 it is invalid.
- Outcomes:
  - `goldenSame`: pass.
  - `goldenFormat` (same value, different bytes, such as member order or whitespace): with `-update`, write `canonical`; without it, fail with a hint.
  - `goldenValues`: always fail with "re-derive by hand".
- `writeGolden` takes `canonical`.
- New `TestCompareGolden` table:
  - indented → same;
  - reordered members (including `params` members) → format;
  - due date +1 day, a changed bucket, swapped reasons → values;
  - renamed member, extra member, `"today": null`, missing `disease` → error.
- New flag help text: "rewrite golden outputs whose only difference from the engine output is encoding; any value difference fails and is never written".

**3.2 Renames and type changes: CHANGE (docs only).** No rename map and no tool. The golden README states: "A contract migration is its own commit. It changes the Go types, the schema and the fixture/golden files (by hand or with an uncommitted script) and no decision logic. The unchanged engine then checks the migrated files byte for byte."

**3.3 Docs: CHANGE.**
- SAD §5.7 second bullet and the golden README: replace "pure format changes only, diff reviewed" with the rule as 3.1 enforces it.
- The README states that `TestGolden` enforces the canonical member order.

### Item 4 (a–d): contract shapes

**4a Span notation: CHANGE (trimmed), OWNER-QUESTION 1.** Write spans as ISO 8601 durations with one designator: `"P6W"`, `"P2M"`, `"P9Y"`, `"P0D"`.

```json
"Span": {"type": "string", "pattern": "^P(0|[1-9][0-9]{0,2})[DWMY]$", "format": "duration",
  "description": "ISO 8601 duration with one designator: P<n>D days, P<n>W weeks (7 days), P<n>M months (day of month clamped to the month end), P<n>Y years (12 months); n in 0..999, no leading zeros, upper case. Compound durations are not accepted yet."}
```

- The Go model and the arithmetic stay as they are (`Span{N, Unit}`). The unit constants become the designators `D W M Y`, and `ParseSpan`/`String` read and write `P<n><U>`.
- `format: duration` is only an annotation. Every allowed string is valid under RFC 3339 Appendix A.
- **Compound spans** (`P14W6D`, `P1Y6M`) are **not adopted.** No sourced rule needs one. Exclusive maximum ages (SAD: L = max − 1 day) turn "14 weeks 6 days" into `P15W`. Compound arithmetic adds an order rule to the riskiest code. If a sourced rule ever needs one, the extension is additive: designators in Y M W D order, years and months added in one month step, then weeks and days (Temporal semantics).

Reason: snapshot versions are immutable, so the notation is permanent after the first publish. The standard form avoids the "6m = minutes" misreading in human and AI review (ADR-0003) and is understood by JSON Schema, Temporal and `Intl.DurationFormat`. It costs a mechanical rewrite of the fixtures and changes no golden.

**4b `RecalledDate {value, precision}`: KEEP.**
- It mirrors storage (SAD §6.1) and F04.2. The precision is chosen, not inferred.
- The proposed schema `if/then` (value required for day/month/year) is **rejected**. A dose with precision `day` and no value is assessed as ⚪ `INVALID_DATE`, so the conditional would break I3.
- A value given with `unknown`/`none` should be reported (not silently ignored), but that is deferred to S2 (#13), where those precisions become evaluable. In S1 they already stop evaluation.

**4c Typed params: CHANGE (trimmed).**

```go
type Reason struct {
	Code   ReasonCode   `json:"code"`
	Params ReasonParams `json:"params,omitzero"`
}
// ReasonParams: every reason parameter; each code uses the members its doc
// lists. Alphabetical JSON-name order, every field omitzero, no booleans
// (omitzero would drop false). Grows additively.
type ReasonParams struct {
	Checks []Check `json:"checks,omitzero"` // DOSE_INVALID, catalog order
	Dose   string  `json:"dose,omitzero"`   // record ID: INVALID_DATE, OUT_OF_RANGE, DATE_IN_FUTURE, DATE_BEFORE_BIRTH, DOSE_INVALID
	Number int     `json:"number,omitzero"` // DOSE_INVALID: position checked, ≥ 1
	Series string  `json:"series,omitzero"` // ALTERNATIVE_PLANS
}
type Check string // CheckMinAge "minAge", CheckMinInterval "minInterval"
```

- Schema: `$defs/Reason.params` → `$defs/ReasonParams`, which is a closed object with `minProperties: 1`:
  - `checks`: an array of the `Check` enum, `minItems` 1, `uniqueItems`;
  - `number`: integer ≥ 1;
  - `dose`: string, `minLength` 1;
  - `series`: `Id`.
- `TestSchemaMatchesStructs` and `TestSchemaEnumsMatchConstants` cover it.
- **No per-code `oneOf` in S1.** It can be added when TS generation ships with the first consumer (#11). Outputs that already comply stay valid. Instead, a Go test table `code → allowed params` checks every reason in the goldens and in the property tests.
- Problems get the same treatment:
  - `engine.ProblemParams{Disease, ID, Precision, Value}` and `ruleset.ProblemParams{Disease, ID, Value}`, all strings and omitzero;
  - the JSON bytes stay identical.
- SAD §5.4 gets one paragraph: params are typed members. IDs and enums are strings, counts are integers, dates are `Date` strings, lists are arrays, and there are no booleans.

Reason: every consumer (TS UI, Go email templates, ADR-0003) would otherwise parse `"1"` and `"minAge,minInterval"` back into types. SAD §5.3 step 3's "never an empty element" rule for `SAFEST_SERIES` already exists only because of comma-joined lists.

**4d Illustrative marker: CHANGE.**
- Remove `note`. Add ``Illustrative bool `json:"illustrative,omitzero"` `` after `Version`, documented as "fixture or demo rule set, not medically verified; the publish path never sets it".
- Schema: `"illustrative": {"type": "boolean"}`. This differs from the proposed `const: true`: Go accepts `false`, so `const` would break I2.
- Fixtures carry `"illustrative": true`. `TestFixtureRuleSets` asserts the flag instead of comparing the note string.
- SAD §5.2 lists the member (fixtures only), and SAD §5.7 and the fixtures README use the new wording.

Reason: `note` is not in the SAD, and free text cannot be checked by a machine. A flag lets a future publish gate or server guard refuse such snapshots.

### Item 4e: semantic defaults

**Q1. A `DOSE_INVALID` ⚪ stays, and masked reasons are kept: KEEP.**
- F05.10 says literally that a too-early dose places the disease in ⚪. SAD §5.3 step 10 keeps masked reasons (`COMPLETE`, `ON_TRACK`).
- The proposed "resolved by a valid repeat clears ⚪" is **not adopted** in this PR. With Q4 unchanged, a dose 1 repeated one day after the invalid one would be counted and would clear ⚪. Whether a repeat must be spaced from the invalid dose is a clinical rule (P2).
- It is recorded as an open clinical question (see below). New golden `dose-invalid-masks-on-track` pins the masked reasons.

**Q2. `nextDose` in a ⚪ assessment: CHANGE.**
- Invariant: `bucket == "unknown"` ⇒ no `nextDose`. `assess` still computes it internally but does not emit it.
- Schema `Assessment`: `"if": {"required": ["bucket"], "properties": {"bucket": {"const": "unknown"}}}, "then": {"not": {"required": ["nextDose"]}}`.
- A property test asserts the invariant.
- An internal (package `engine`) test of `seriesModel.next` pins "earliest after an uncounted latest dose = that dose + minInterval", which the 5 goldens can no longer show.
- The doc on `Assessment.NextDose` and SAD §5.1 state the invariant.

Reason: the spec is silent on dates for ⚪. A ⚪ date is a guessed branch (P1). Most later ⚪ reasons have no single date anyway. Adding the member later is additive, while removing it later breaks consumers.

**Q3 and the whole-profile failure: CHANGE.**
- On `/doses/{i}/date/value`, the problems `INVALID_DATE`, `OUT_OF_RANGE`, `DATE_IN_FUTURE` and `DATE_BEFORE_BIRTH` are **record-scoped**. Every rule-set disease of that record becomes ⚪ with no `nextDose`. The reason code equals the problem code (the `INVALID_DATE` precedent), with `params {dose}`. Reasons are sorted by record ID. The ⚪ is a data ⚪ (F05.11 layer 7): steps 2–9 are skipped, but precedence layers 1–5 still win over it and keep the record reasons as masked reasons (F04.6, A8).
- Every other problem stops evaluation: all `/today` and `/profile/birth` problems, plus `REQUIRED`, `DUPLICATE_ID`, `DUPLICATE_VALUE`, `INVALID_VALUE`, `UNSUPPORTED_PRECISION` and **`UNKNOWN_DISEASE`**.
- Mechanism:
  - add `type Scope string` with `ScopeInput` and `ScopeRecord`;
  - add `Problem.Scope` with `json:"-"`. It is Go-only for now, and serializing it later is additive;
  - Validate sets the scope when it creates the problem;
  - `Evaluate` builds the per-record reasons from an internal validation result, replacing the path-string filter;
  - `InputError` holds only `ScopeInput` problems. The doc reads "Evaluate returns *InputError iff some problem has ScopeInput".
- Add the three new reason codes to the schema enum and to SAD §5.4.
- Documentation:
  - SAD §5.1 "Input errors" and §5.3 step 1 are updated;
  - SAD §17 gets item 6: "F05.1: a dose date that is out of range, after today or before the birth date is treated like an unparseable one (⚪ for the record's diseases); apply in the next spec bump".
- Validate still reports these problems, so saving (F04.4) still blocks them.

Reason:
- Stored data can reach this state through an edited birth date, client/server clock skew or a guest import.
- Today one such record blanks the whole person and silently drops every reminder for them (SAD §7.2).
- F05.1 lists only profile-level input errors.

The proposed "ignore an unknown disease entry" is **rejected**. Dropping a listed disease silently can show a false 🔴 and prompt an extra dose. Soft delete keeps catalog entities (F11.3), so the case comes only from client bugs. It would also break I3 against the `Id` pattern.

**Q4. A repeated dose 1 gets its earliest date from the minimum age only: KEEP.**
- Add to SAD §5.3 step 7: "A dose 1 repeated after an uncounted dose 1 has earliest = minimum age: dose 1 has no minimum interval in the rule model (FIRST_DOSE_HAS_INTERVAL)."
- New golden `repeat-dose-1-counted-from-min-age` pins it.
- It joins Q1 as an open clinical question.

**Q6. Plans tie on due date, so the latest earliest date wins: KEEP and document.**
- SAD §5.3 step 7 merge 1 gains "Plans with the same due date: the earliest allowed date is the latest of theirs (P1)".
- The golden README Notation states the rule.
- The `nextdose.go` comment cites the SAD.
- New fixture `minimal-alt-plans-tied` and golden `alt-plans-due-tie-latest-earliest`. Taking the minimum E would give `due`, so the golden tells the two rules apart.

## Owner question

1. **Span notation (4a).** The spec is silent, and the notation is permanent after the first published snapshot. **Default, applied:** ISO 8601 with one designator (`P6W`). The alternative is to keep `"6w"`. Either way, compound spans stay out until a sourced rule needs one.

## Open for clinical review (no behaviour change in this PR)

Add one row to the SAD §15 table.

- **Q1:** should a too-early dose that a valid dose has repeated still keep the disease ⚪? Today it does, per the literal F05.10. The cost is that ⚪ sends no reminders for the rest of the series.
- **Q4:** should a repeated dose be spaced from the invalid dose?
  - If yes, the additive path is rule data: allow a sourced "interval from a preceding uncounted dose" on dose 1. `check()` and `dates()` already measure to the adjacent dose in time (A4).
  - Decide Q1 and Q4 together, because Q1 alone would clear ⚪ after a badly spaced repeat.

## Implementation order (TDD)

Write each test first and watch it fail, then commit it together with its fix. Run `go tool -modfile=tools/go.mod task gen:check lint test` before each commit.

1. **Strict decoding (Item 2).**
   - `contractjson` tests: null at a nested pointer, a missing required member with its pointer, an omitted optional member, an unknown member, `Members` order and requiredness.
   - Then the package itself, and the wiring into both decoders.
   - Move the `"missing offset"` and `"missing recommended"` cases of `ruleset/validate_test.go` to `TestDecodeIsStrict`, and keep `Required` coverage through a Go-built snapshot.
   - New `DecodeInput` cases: doses missing or null, value null, today or profile missing.
   - `EMPTY` at `/diseases` plus `minItems`; the schema test switches to `Members`; the forbidigo rule.
   - Docs: schema description, SAD §5.1, `CODING_STANDARDS.md`.
2. **Any string as a recalled date value (Item 1).**
   - The schema and engine tables from 1.6, and goldens `a9-invalid-date-not-iso` and `a9-invalid-date-empty-value` (red against the old pattern).
   - Then drop the pattern and update the descriptions and doc comments.
3. **Agreement test (2.5).** It must pass after steps 1–2 and keep passing through every later step.
4. **Golden harness (Item 3).** `TestCompareGolden`, then `compareGolden`, the `TestGolden` branches, `writeGolden(canonical)` and the flag text. Docs: SAD §5.7 and the golden README. No golden changes.
5. **Contract migration: Span notation (4a).**
   - `span_test` cases:
     - accepted: `P6W`, `P0D`, `P999Y`;
     - rejected: `P`, `6w`, `P6w`, `P06W`, `P1000D`, `PT6M`, `P1Y6M`, `-P1D`.
   - `TestSpanPatternMatchesParseSpan` (exhaustive over n ∈ 0..1200, all designators, both cases, with and without leading zeros).
   - Then the code and schema, and rewrite the spans in the 9 fixtures, the 3 validate files and the Go test literals with an uncommitted script.
   - No decision-logic change.
6. **Contract migration: illustrative flag (4d).**
7. **Contract migration: typed params (4c).**
   - Schema and catalog tests first, then the structs.
   - Rewrite the `DOSE_INVALID` params in the 5 goldens with an uncommitted script, and update the README notation (`DOSE_INVALID{checks: [minAge], dose: r1, number: 1}`).
   - No decision-logic change.
8. **Q2.**
   - Property test, schema `if/then` and the internal `next()` test.
   - Edit `nextDose` out of the 5 goldens by hand and re-derive their README lines ("→ unknown, no nextDose"). Add golden `dose-invalid-masks-on-track`.
   - Then change `assess`.
9. **Q3.**
   - Scope pinned on every `TestValidate*` row.
   - New `TestValidateScopeMatchesEvaluate`: Evaluate fails iff some problem is `ScopeInput`, and each record-scoped problem appears as a reason on every disease of its record.
   - `TestEvaluateInputErrors` loses "future dose" and "dose before birth".
   - Goldens `record-dose-in-future` and `record-dose-before-birth`.
   - Then implement, and update SAD §5.1, §5.3 step 1, §5.4 and §17.
10. **Q4, Q6 and the Q1/Q4 notes.**
    - Goldens `repeat-dose-1-counted-from-min-age` and `alt-plans-due-tie-latest-earliest`, plus fixture `minimal-alt-plans-tied` and its README row. These pin the current behaviour.
    - SAD §5.3 step 7 sentences, the §15 row and the `nextdose.go` comment.
11. **Review and PR.** An adversarial review of the logic changes (Q2, Q3) per #11, then the PR.

## Golden and fixture changes

**Existing goldens.** Only these 5 change: `a4-interval-to-adjacent-dose`, `alt-plans-dose-invalid-under-every-plan`, `boundary-min-age-in-years-clamped`, `f13-dose-before-min-age`, `f13-dose-before-min-interval`. Each changes twice:

- **4c, a format/contract migration with no value change:** `"check":"minAge"` → `"checks":["minAge"]` and `"number":"1"` → `1`.
- **Q2, a spec-driven value change:** `nextDose` is removed, and the README lines are re-derived by hand.

The other 31 existing goldens stay byte-identical.

**New goldens (hand-written; fixture, inputs and expected outputs):**

| File | Fixture | Input | Expected |
|---|---|---|---|
| `a9-invalid-date-not-iso` | `illustrative-vn` | birth 2026-01-10, today 2026-04-01; r1 `"10/02/2026"` (pertussis), r2 `"2026-1-5"` (measles) | both unknown, `INVALID_DATE{dose}` |
| `a9-invalid-date-empty-value` | `illustrative-vn` | same dates; r1 `""` precision day (pertussis) | pertussis unknown `INVALID_DATE{dose: r1}`; measles `notYetScheduled` (E = Du = 10-10, O = 10-17, T6m = 10-01) |
| `record-dose-in-future` | `illustrative-vn` | same dates; r1 2026-04-02 (pertussis) | pertussis unknown `DATE_IN_FUTURE{dose: r1}`; measles `notYetScheduled` as above |
| `record-dose-before-birth` | `illustrative-vn` | same dates; r1 2026-01-09 (pertussis, measles) | both unknown `DATE_BEFORE_BIRTH{dose: r1}` |
| `dose-invalid-masks-on-track` | `illustrative-vn` | birth 2025-01-15, today 2025-11-01; pertussis 03-15/04-15/05-15; measles m1 10-15, m2 10-20 | pertussis complete `COMPLETE`. Measles: m2 is before 10-15 + 28d = 11-12, so it is invalid; its next dose has Du 2026-07-15 > T6m 2026-05-01, so the bucket would be `ON_TRACK`, which the invalid dose masks. Result: unknown `[DOSE_INVALID{checks: [minInterval], dose: m2, number: 2}, ON_TRACK]`, no nextDose |
| `repeat-dose-1-counted-from-min-age` | `minimal-single` | birth 2026-01-10, today 03-01; r1 02-20 (before 02-21, invalid), r2 02-21 (counted as dose 1) | unknown `[DOSE_INVALID{checks: [minAge], dose: r1, number: 1}]` only |
| `alt-plans-due-tie-latest-earliest` | new `minimal-alt-plans-tied` (plans `short` and `long`, dose 2 min interval P4W / P6W, both at P4M) | birth 2026-01-10, r1 03-10, today 04-15 | short E 04-07; long E 04-21; both Du 05-10, O 05-17. Merged E 04-21 is after today, so upcoming with `ALTERNATIVE_PLANS{series: tied-series}` |

**Fixtures.**

- The 9 fixture rule sets and the 3 `ruleset/testdata/validate` snapshots change mechanically and keep their values: spans `"6w"` → `"P6W"` (4a) and `"note": …` → `"illustrative": true` (4d).
- One new fixture (Q6).
- The READMEs to update are the golden README (notation, `-update`, contract migrations, new and re-derived lines) and the rulesets README (marker, new row).

## Follow-ups outside this PR (owner to approve before filing or editing issues)

- #11: reword the decision bullets on the illustrative marker and `-update`.
- #13 (S2): report a value given with precision `unknown`/`none`, and choose its scope.
- #14 (S3): `SAFEST_SERIES` params as a typed array of `{series, product?}`, and adjust the SAD §5.3 step 3 wording.
- First consumer: per-code `Reason` `oneOf`, and serialize `Problem.Scope` if guest import needs it.
- Editorial and server: refuse a snapshot that has `illustrative: true`.
- Delivery (first consumer): handle version skew, since an older engine fails closed on a snapshot that uses a newer member. Options: the snapshot declares the minimum contract or engine it needs and the bridge reports `unsupported` instead of `decode`; a cached client keeps the last snapshot that loaded; a binary rollback also rolls back a rule set that uses newer members.
