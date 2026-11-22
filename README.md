# VaxBook

A neutral vaccination record for users in Vietnam. Product spec: [`docs/feature-spec-mvp.md`](docs/feature-spec-mvp.md). Architecture: [`docs/sad.md`](docs/sad.md) and [`docs/adr/`](docs/adr/). Terms: [`GLOSSARY.md`](GLOSSARY.md).

## Backend development

You need Go 1.27 or later (`GOTOOLCHAIN=auto` fetches the exact version), Docker, and Node 26 (`task test` also runs the engine tests as WASM under Node). Other tools are pinned in `tools/go.mod`. If you have not installed [Task](https://taskfile.dev), run `go tool -modfile=tools/go.mod task <name>` instead of `task <name>`.

```sh
task test                     # Go tests against a throwaway Postgres (testcontainers), then the WASM engine run
task lint                     # golangci-lint, plus engine/ and ruleset/ as js/wasm
task gen                      # regenerate code from contracts/openapi.yaml and the ent schema
task migrate:diff NAME=foo    # write a goose migration for ent schema changes
task gen:check                # what CI runs: generated code and migrations are up to date

cp .env.example .env.dev      # then edit; never commit real values
task run                      # starts compose.dev.yaml Postgres and the server on :8080
```

### Dependency updates

Dependabot (`.github/dependabot.yml`) opens PRs every Monday for `go.mod`, `tools/go.mod`, GitHub Actions and the Docker images; each PR waits for owner review. Dependabot cannot bump a few values, so tests and one workflow keep them in step:

- A red `TestGoVersionAgrees` on a golang image PR: bump the `go` line in `go.mod` and `tools/go.mod` to the same version in that PR.
- A red `TestPostgresImageMatchesCompose` on a Postgres PR: bump `store.PostgresImage` to the same tag. A major version also needs a fresh dev volume (`task db:down -- -v`).
- golangci-lint: edit `.golangci-lint-version`; the Taskfile and CI both read it.
- Node: `TestNodeVersionAgrees` keeps `ci.yml` and the Taskfile on one major. The Freshness workflow opens an issue when golangci-lint or the Node LTS major falls behind.
- Tidy the tools module from its own directory (`cd tools && go mod tidy`), as Dependabot does. `task gen:check` fails if `go.mod` or `tools/go.mod` is not tidy.

In dev, `VAXBOOK_MAILER=console` writes sign-in codes to the server log instead of sending email.

The API contract is [`contracts/openapi.yaml`](contracts/openapi.yaml). Write requests need the header `X-VaxBook-CSRF: 1`.

## License

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
