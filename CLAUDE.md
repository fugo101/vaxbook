# vaxbook

## Documentation language

- Engineering docs are written in **English**, and the English file is the source of truth. This covers the SAD, ADRs, glossary, research notes, READMEs, code comments and issues.
- A doc that also needs a Vietnamese version gets a separate `<name>.vi.md` file next to it, as a reference copy. If the two disagree, the English file wins. Today only `docs/feature-spec-mvp.vi.md` exists.
- Product UI is Vietnamese-only. Inside English docs, quote UI and email strings in Vietnamese exactly as users see them, followed by an English gloss.
- Never commit personal identifiers: the operator's domain, server IPs, host paths, usernames or the names of other projects. In docs, use placeholders such as `<operator-domain>`, `<stacks-dir>` and `<proxy-network>`. Real values live only in deployment `.env` files, which are never committed.

## Backend checks

Run `go tool -modfile=tools/go.mod task gen:check lint test` (works without Task installed; `task test` also finds the Docker socket). Commands and setup: README › Backend development. Review rules: `CODING_STANDARDS.md`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues on `fugo101/vaxbook` (public), via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`, applied as GitHub labels. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
