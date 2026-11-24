# Security policy

## Supported versions

VaxBook is pre-release and has no tagged releases. Only the `main` branch is supported. Fixes land on `main`; nothing is backported.

## Reporting a vulnerability

Do not open a public issue or pull request for a security problem.

Use GitHub's private vulnerability reporting instead: open <https://github.com/fugo101/vaxbook/security/advisories/new> (Security tab, "Report a vulnerability"). Only the maintainers can read the report. The project publishes no email address for security reports.

The same private form is also used for Code of Conduct reports, which are not vulnerabilities: start the title with "Conduct:" (see [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)). The rest of this policy is about vulnerabilities.

Please include:

- what is affected (the engine, the API, or the rule set data) and the commit or branch you tested;
- the steps to reproduce, with the smallest example that shows the problem;
- the impact as you understand it, for example what an attacker could read, change or bypass;
- how you would like to be credited, if at all.

Do not include real personal data or real vaccination records in a report. Use made-up data.

## Scope

In scope:

- the engine (`engine/`) and the rule set (`ruleset/`), including differences between native and WASM output that could change a recommendation;
- the API server (`apps/api/`, `contracts/openapi.yaml`): passwordless email sign-in with a one-time code, server-side sessions, the `X-VaxBook-CSRF: 1` header on write requests, and rate limits and lockouts. The admin area is not wired yet; once it is, it is in scope too ([ADR-0008](docs/adr/0008-admin-behind-cloudflare-access.md));
- the build and delivery setup: the `Dockerfile`, the CI workflows and pinned dependencies;
- privacy of health data: any way for one person to read or change another person's vaccination data, or for personal data (emails, codes, vaccination records) to leak into logs, URLs or third parties.

Out of scope:

- disagreements about medical content, for example a series you think is wrong or a missing recommendation. Use the "Catalog or rule data error" issue form instead; see [`SUPPORT.md`](SUPPORT.md);
- attacks that need physical access to a device, a compromised machine on the reporter's side, or social engineering of maintainers;
- denial of service by sheer traffic volume;
- vulnerabilities in a third-party dependency that have no effect on VaxBook. Report those upstream. If VaxBook is affected, report it here.

## Known limitations of the beta

These are documented decisions, not vulnerabilities. A report that only restates them will be closed:

- the closed beta has no encryption at rest and no backups, so data can be lost if the server fails ([ADR-0007](docs/adr/0007-beta-without-encryption-and-backups.md));
- the beta uses third-party processors outside Vietnam ([ADR-0005](docs/adr/0005-vn-vps-with-cross-border-processors.md)).

## What to expect

VaxBook is maintained on a best-effort basis. There is no response-time commitment. The maintainers will read the report, may ask follow-up questions in the private advisory, and will say whether they accept it as a vulnerability (or, for a "Conduct:" report, handle it under the Code of Conduct).

When a fix is on `main`, the maintainers may publish a GitHub security advisory and credit the reporter if they want that. Please keep the details private until then.
