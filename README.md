# VaxBook

A neutral vaccination record for users in Vietnam. Product spec: [`docs/feature-spec-mvp.md`](docs/feature-spec-mvp.md). Architecture: [`docs/sad.md`](docs/sad.md) and [`docs/adr/`](docs/adr/). Terms: [`GLOSSARY.md`](GLOSSARY.md).

## Backend development

You need Go 1.27 or later (`GOTOOLCHAIN=auto` fetches the exact version), Docker, and Node 26 (`task test` also runs the engine tests as WASM under Node). Other tools are pinned in `go.tool.mod`. If you have not installed [Task](https://taskfile.dev), run `go tool -modfile=go.tool.mod task <name>` instead of `task <name>`.

```sh
task test                     # Go tests against a throwaway Postgres (testcontainers), then the WASM engine run
task lint                     # golangci-lint, plus engine/ and ruleset/ as js/wasm
task gen                      # regenerate code from contracts/openapi.yaml and the ent schema
task migrate:diff NAME=foo    # write a goose migration for ent schema changes
task gen:check                # what CI runs: generated code and migrations are up to date

cp .env.example .env.dev      # then edit; never commit real values
task run                      # starts compose.dev.yaml Postgres and the server on :8080
```

In dev, `VAXBOOK_MAILER=console` writes sign-in codes to the server log instead of sending email.

The API contract is [`contracts/openapi.yaml`](contracts/openapi.yaml). Write requests need the header `X-VaxBook-CSRF: 1`.

## License

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
