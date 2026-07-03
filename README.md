# Terraform Provider for MotherDuck

[![Terraform Registry](https://img.shields.io/badge/Terraform%20Registry-jpig18%2Fmotherduck-844FBA?logo=terraform)](https://registry.terraform.io/providers/jpig18/motherduck/latest)

Manage [MotherDuck](https://motherduck.com) organization resources with Terraform,
via the [MotherDuck REST API](https://motherduck.com/docs/api/rest-api/).

**Published on the Terraform Registry:**
[registry.terraform.io/providers/jpig18/motherduck](https://registry.terraform.io/providers/jpig18/motherduck/latest)

Covers the **entire REST API surface** as of mid-2026:

| API | Provider |
| --- | --- |
| `POST /v1/users`, `DELETE /v1/users/{u}` | `motherduck_service_account` resource |
| `POST/GET/DELETE /v1/users/{u}/tokens[/{id}]` | `motherduck_token` resource, `motherduck_tokens` data source |
| `GET/PUT /v1/users/{u}/instances` | `motherduck_duckling_config` resource + data source |
| `GET /v1/active_accounts` | `motherduck_active_accounts` data source |
| `POST /v1/dives/{id}/embed-session` | `motherduck_embed_session` ephemeral resource |

Databases and shares are not part of the REST API (they are SQL-only:
`CREATE DATABASE`, `CREATE SHARE`, `UPDATE SHARE`) and therefore are not manageable
by this provider today. When MotherDuck adds endpoints for them, they'll be added here.

> **Note:** MotherDuck marks the REST API as *Preview*; endpoints may change.

## Usage

```terraform
terraform {
  required_providers {
    motherduck = {
      source = "jpig18/motherduck"
    }
  }
}

# Reads MOTHERDUCK_API_TOKEN (or MOTHERDUCK_TOKEN); must be an Admin token
provider "motherduck" {}

resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_token" "etl" {
  username    = motherduck_service_account.etl.username
  name        = "etl-pipeline"
  ttl_seconds = 60 * 60 * 24 * 90
}

resource "motherduck_duckling_config" "etl" {
  username = motherduck_service_account.etl.username

  read_write {
    instance_size    = "standard"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size = "pulse"
    flock_size    = 2
  }
}
```

See [`docs/`](docs/) for the full reference and [`examples/`](examples/) for runnable
configurations:

- [`examples/complete`](examples/complete) — a service account, token, and Duckling config together.
- [`examples/embed-session`](examples/embed-session) — the ephemeral Dive embed session.
- [`examples/verify`](examples/verify) — an end-to-end smoke test that installs the provider
  from the public registry and asserts every resource/data-source round-trips (via `check {}` blocks).

## Testing

There are three layers, cheapest first. The first two need **no credentials and
no MotherDuck account** — they run against an in-memory mock of the REST API.

### 1. Unit tests (client)

Exercise the HTTP client directly (auth header, request/response shapes, error
parsing) against an `httptest` server:

```bash
go test ./internal/client/
```

### 2. Acceptance tests (provider)

Drive real Terraform plan/apply/import cycles through the provider against the
in-memory mock API. `TF_ACC=1` is required or they're skipped:

```bash
TF_ACC=1 go test ./internal/provider/ -v
```

- Terraform is auto-downloaded by the test framework. To use a specific binary
  instead, set `TF_ACC_TERRAFORM_PATH=/path/to/terraform`.
- Ephemeral-resource coverage needs Terraform **1.10+**.

Run everything (unit + acceptance) plus vet and formatting — the same gates CI runs:

```bash
go build ./... && go vet ./... && test -z "$(gofmt -l .)"
TF_ACC=1 go test ./...
```

### 3. Live end-to-end (against a real MotherDuck org)

[`examples/verify`](examples/verify) installs the provider **from the public
registry** and exercises every resource and data source, asserting each
round-trip with `check {}` blocks so a bad run fails loudly. This one hits the
real API and needs an **Admin** token:

```bash
cd examples/verify
export MOTHERDUCK_API_TOKEN=<admin token>
terraform init      # installs jpig18/motherduck from the registry
terraform apply     # creates a throwaway service account + token + config, runs checks
terraform destroy   # cleanup (permanently deletes the test service account)
```

A clean apply with all checks passing means every function works against the live
API. See [`examples/verify/README.md`](examples/verify/README.md) for the optional
embed-session test and name overrides.

## Development

Requirements: Go 1.24+, Terraform 1.10+.

```bash
go build ./...
```

To use a local build in Terraform without publishing, add a
[dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
to a `.terraformrc` (point `TF_CLI_CONFIG_FILE` at it, or use `~/.terraformrc`):

```hcl
provider_installation {
  dev_overrides {
    "jpig18/motherduck" = "/path/to/terraform-provider-motherduck"
  }
  direct {}
}
```

With a dev override in effect, skip `terraform init` — Terraform uses the local
binary directly. Rebuild (`go build -o terraform-provider-motherduck .`) after each
change.

## Releasing

Tag a semver release (`v0.1.0`); the `release` GitHub Actions workflow builds, signs,
and publishes artifacts with GoReleaser in the layout the
[Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
expects. Requires `GPG_PRIVATE_KEY` and `PASSPHRASE` repo secrets.

## License

[MIT](LICENSE)
