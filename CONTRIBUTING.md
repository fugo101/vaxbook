# Contributing to VaxBook

VaxBook is a neutral vaccination record for users in Vietnam, with recommendations that cite their sources. It is pre-release: the `main` branch is the only supported version. Issues and pull requests from outside are welcome. Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

VaxBook is not medical advice, and neither is any issue or review comment here.

## Ways to contribute

- **Bug reports.** Something in the app, API or engine behaves differently from the spec ([`docs/feature-spec-mvp.md`](docs/feature-spec-mvp.md)) or the SAD ([`docs/sad.md`](docs/sad.md)).
- **Catalog and rule data errors.** A product, series, dose rule or rule looks wrong. Use the "Catalog or rule data error" form. Name the Source document and give a Citation: the article, clause or page that says otherwise. A report without a source cannot be acted on, because every catalog item and rule must cite one. Catalog and rule data are editorial data, not code: they will live in the database and change only through the admin changeset flow ([SAD](docs/sad.md) §6.2, §7.3). The admin area is not built yet. Until it is, an accepted correction is recorded in the issue. Do not send a PR that edits test fixtures to change real catalog data.
- **Documentation.** Fixes and clarifications to the README, SAD, ADRs, glossary or research notes.
- **Code.** Fixes and features, after the issue-first step below where it applies.

Open an issue with one of the issue forms. For a security vulnerability, do not open an issue: follow [`SECURITY.md`](SECURITY.md). For questions, see [`SUPPORT.md`](SUPPORT.md).

The repository is public. Never paste a real vaccination record, the birth date of a real person, an email address or any other personal data into an issue, PR or comment. Use made-up values.

## Open an issue first for engine, rule set or catalog changes

A change to `engine/`, `ruleset/`, `contracts/ruleset.schema.json` or the engine Input and Output types starts as an issue, so that the expected behavior is agreed and written down before code is. Small fixes elsewhere (a typo, a broken link, an obvious bug with a test) can go straight to a PR.

New issues get the `needs-triage` label. The maintainer then moves them to one of these ([`docs/agents/triage-labels.md`](docs/agents/triage-labels.md)):

| Label | Meaning |
| --- | --- |
| `needs-info` | Waiting on the reporter for more information |
| `ready-for-agent` | Fully specified; an automated agent can implement it |
| `ready-for-human` | Fully specified; needs a human implementer |
| `wontfix` | Will not be actioned |

An issue labeled `ready-for-human`, `good first issue` or `help wanted` is open to outside contributors. Say in the issue that you are taking it, to avoid duplicate work.

## Development setup

Follow [README › Backend development](README.md#backend-development) for the tools (Go, Docker, Node) and the `task` commands.

Before you open a PR, run the same check as CI:

```sh
go tool -modfile=tools/go.mod task gen:check lint test
```

This works without Task installed. `test` needs a running Docker daemon (the tests start a throwaway Postgres with testcontainers) and Node 26 (the engine tests also run as WASM). A docs-only change needs no local Go check; CI still runs the full check. Check that links resolve and that the change has no personal identifiers. `go test ./internal/repocheck` is relevant when you touch workflows, the Dockerfile or the tool pins.

## Branches, commits and PR titles

- Work on a branch, not on `main`. Open the PR against `main`.
- Write commit messages and the PR title in [Conventional Commits](https://www.conventionalcommits.org/) form: `type(scope): summary`, for example `fix(engine): ...` or `docs: ...`. Allowed types: `feat`, `fix`, `perf`, `deps`, `revert`, `docs`, `style`, `chore`, `refactor`, `test`, `build`, `ci`. The `Conventional PR title` check enforces the PR title.
- PRs are squash-merged, so the PR title becomes the commit subject on `main`. Keep one topic per PR.
- Fill in the PR template. Link the issue with `Closes #<number>`.

## Engine and rule set rules

These hold for `engine/`, `ruleset/` and the engine contract (`contracts/ruleset.schema.json`, the Input and Output types). The reasons are in the ADRs and in [`CODING_STANDARDS.md`](CODING_STANDARDS.md).

- **Standard library only.** `engine/` and `ruleset/` import nothing outside the Go standard library, and give the same output natively and as WASM ([ADR-0002](docs/adr/0002-engine-go-native-and-wasm.md)). `task lint` also lints them as `js/wasm`.
- **Golden expectations are hand-written.** The expected `output` in a golden file is derived by hand from the spec and the SAD before the engine code exists. A changed expectation is re-derived by hand first. Never write an expectation from engine output: `go test ./engine -run TestGolden -update` only re-encodes a hand-written expectation, and refuses to write different values. See [`engine/testdata/golden/README.md`](engine/testdata/golden/README.md).
- **The contract grows additively.** Follow rules R1–R6 in [`docs/research/2026-11-21-engine-s1-contract-review.md`](docs/research/2026-11-21-engine-s1-contract-review.md) and the contract section of `CODING_STANDARDS.md`:
  - a member added after S1 is optional (`omitzero`), unless every stored document is migrated in the same change;
  - `null` is never valid;
  - requiredness comes from the struct tags;
  - the decoders keep rejecting unknown members;
  - a contract migration is its own commit, with no decision logic (one carve-out: a migration that adds an output member may add the code that emits it; see `CODING_STANDARDS.md`).
- **Test fixtures name their source.** A fixture modeled on a real rule names the Source document and Citation it follows. Real catalog and rule data will not be stored in git (see Ways to contribute).

## Documentation and wording

- Engineering docs, code comments, issues and PRs are in English. The English file is the source of truth.
- The product UI is Vietnamese-only. In English text, quote a UI or email string in Vietnamese exactly as users see it, followed by an English gloss, for example "chưa rõ" ("unknown").
- Use the terms in [`GLOSSARY.md`](GLOSSARY.md) and avoid the synonyms it lists. For example, write "Rule set version", not "release". A new domain term goes into the glossary in the same PR.
- Update the spec, SAD or an ADR in the same PR when a change alters what they describe.

## No personal identifiers

Do not commit or post operator domains, server IPs, host paths, usernames, email addresses or the names of other projects. Use placeholders such as `<operator-domain>`. Real values live only in deployment `.env` files, which are never committed. Logs carry internal UUIDs only (`CODING_STANDARDS.md`).

## Dependencies and pinning

- Dependabot opens update PRs every Monday (README › Dependency updates). A few values it cannot bump are kept in step by tests; the README lists them.
- GitHub Actions are pinned by full commit SHA with a version comment (`owner/repo@<sha> # vN`). Docker base images are pinned by digest. `go test ./internal/repocheck` checks the action pins. Follow the same form in any new workflow or Dockerfile stage.
- A new Go dependency needs a reason in the PR. `engine/` and `ruleset/` take none.

## Review

The maintainer reviews every PR, on a best-effort basis. A PR is merged only when the required checks are green (`Conventional PR title`, `Go (gen check, lint, test, WASM test)` and `Container image builds`) and review comments are resolved. Review follows [`CODING_STANDARDS.md`](CODING_STANDARDS.md).

## License

VaxBook is licensed under Apache-2.0 ([`LICENSE`](LICENSE), [`NOTICE`](NOTICE)). By submitting a contribution, you agree that it is licensed under the same terms (Apache-2.0, section 5). There is no CLA and no DCO sign-off; a `Signed-off-by` line is optional.
