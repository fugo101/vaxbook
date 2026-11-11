# vaxbook

## Documentation language

- Engineering docs are written in **English**, and the English file is the source of truth. This covers the SAD, ADRs, glossary, research notes, READMEs, code comments and issues.
- A doc that also needs a Vietnamese version gets a separate `<name>.vi.md` file next to it, as a reference copy. If the two disagree, the English file wins. Today only `docs/feature-spec-mvp.vi.md` exists.
- Product UI is Vietnamese-only. Inside English docs, quote UI and email strings in Vietnamese exactly as users see them, followed by an English gloss.
- Never commit personal identifiers: the operator's domain, server IPs, host paths, usernames or the names of other projects. In docs, use placeholders such as `<operator-domain>`, `<stacks-dir>` and `<proxy-network>`. Real values live only in deployment `.env` files, which are never committed.

## Agent skills

### Issue tracker

Issues are markdown files under `.scratch/<feature-slug>/` in this repo. See `docs/agents/issue-tracker.md`.

### Triage labels

Default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. They're stored as a `Status:` line in each issue file. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
