<!--
Title: Conventional Commits, for example "fix(engine): ..." or "docs: ...".
It becomes the commit subject on main (squash merge). See CONTRIBUTING.md.
Never paste real personal or health data here; this repository is public.
-->

Closes #

## Summary

<!-- What changes and why, in a few lines. Engine, rule set or catalog changes need an issue first. -->

## Evidence

<!-- How you tested it. Paste the result of the check command, or say why it does not apply. -->

- [ ] `go tool -modfile=tools/go.mod task gen:check lint test` passes (docs-only: not needed locally; CI runs it)

## Checklist

<!-- Tick what applies; delete lines that do not. -->

- [ ] Golden expectations are hand-written from the spec, not taken from engine output (`-update` only re-encodes)
- [ ] Contract changes are additive (R1–R6 and `CODING_STANDARDS.md`): a new member is `omitzero` unless every stored document is migrated in the same change, no `null`
- [ ] A contract migration is its own commit, with no decision logic (`CODING_STANDARDS.md`; a migration that adds an output member may add the code that emits it)
- [ ] Test fixtures based on real rules name their Source document (Citation)
- [ ] Spec, SAD, ADRs or `GLOSSARY.md` updated where this change alters them
- [ ] Web UI changes meet WCAG 2.2 level AA, and email template changes follow it as far as it applies, per `ACCESSIBILITY.md` (keyboard, visible focus, contrast, color not the only signal)
- [ ] No personal identifiers (domains, IPs, host paths, usernames, emails, other project names) and no real health data

## Merge Danger

<!-- Door: one-way (hard to undo, for example a stored contract or a migration) or two-way. Blast radius: what breaks if this is wrong. -->
