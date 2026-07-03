# CLAUDE.md

Guidance for Claude Code working in this repository.

## What this is

A Terraform provider for [MotherDuck](https://motherduck.com), wrapping the
[MotherDuck REST API](https://motherduck.com/docs/api/rest-api/). Built with
`terraform-plugin-framework`. Published on the public registry as
[`jpig18/motherduck`](https://registry.terraform.io/providers/jpig18/motherduck/latest).

- `internal/client/` — thin Go client for the REST API (one method per endpoint).
- `internal/provider/` — provider, resources, data sources, ephemeral resource.
- `docs/`, `examples/` — registry docs and runnable configs.

**Scope:** the provider covers exactly what the REST API exposes — service
accounts, tokens, Duckling config, active accounts, Dive embed sessions.
Databases and shares are **SQL-only** (`CREATE DATABASE`, `CREATE SHARE`) and are
deliberately **out of scope** — do not try to add them as REST resources.

## How to test

Three layers, cheapest first. Layers 1–2 need **no credentials** (they run against
an in-memory mock of the API in `internal/provider/testserver_test.go`).

```bash
# 1. Unit tests — HTTP client against an httptest server
go test ./internal/client/

# 2. Acceptance tests — real Terraform cycles through the provider against the
#    mock API. TF_ACC=1 is REQUIRED or they're skipped.
TF_ACC=1 go test ./internal/provider/ -v

# Full CI-equivalent gate (what .github/workflows/test.yml runs):
go build ./... && go vet ./... && test -z "$(gofmt -l .)"
TF_ACC=1 go test ./...
```

Notes:
- Acceptance tests auto-download Terraform. On this machine the `terraform` shell
  command is an AWS-account guard alias — pass the real binary explicitly if a test
  needs a fixed one: `TF_ACC_TERRAFORM_PATH=/Users/pignato/bin/terraform`.
- Ephemeral-resource tests need Terraform **1.10+**.
- After changing any resource schema, re-run the acceptance tests — they catch
  plan/apply/import regressions the unit tests don't.

### Live end-to-end (real MotherDuck org, needs an Admin token)

`examples/verify/` installs the provider from the public registry and asserts every
resource/data-source round-trips via `check {}` blocks:

```bash
cd examples/verify
export MOTHERDUCK_API_TOKEN=<admin token>
terraform init && terraform apply    # green checks = all functions work live
terraform destroy                    # deletes the throwaway test service account
```

Do not run the live test without an explicit ask — it creates and deletes real
users in the org.

## Local iteration (without publishing)

Build a local binary and use a `dev_overrides` block (see README "Development").
With an override in effect, **skip `terraform init`**; rebuild after each change:

```bash
go build -o terraform-provider-motherduck .
```

## Conventions

- One resource/data-source per file in `internal/provider/`, named
  `resource_*.go` / `data_source_*.go` / `ephemeral_*.go`.
- Every new endpoint gets: a client method + a mock branch in `testserver_test.go`
  + an acceptance test + a `docs/` page + an `examples/` snippet.
- Keep the client pure-Go (no CGO) — it preserves the static cross-compiled release.
- Run `gofmt -w .` and `terraform fmt -recursive examples/` before committing.

## Releasing

Push a semver tag (`vX.Y.Z`); `.github/workflows/release.yml` runs GoReleaser to
build, GPG-sign, and publish registry artifacts. Requires `GPG_PRIVATE_KEY` and
`PASSPHRASE` repo secrets. The registry ingests the signed release automatically.
