# Terraform Provider for MotherDuck

Manage [MotherDuck](https://motherduck.com) organization resources with Terraform,
via the [MotherDuck REST API](https://motherduck.com/docs/api/rest-api/).

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
  username = "svc-etl"
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
configurations.

## Development

Requirements: Go 1.24+, Terraform 1.10+.

```bash
go build ./...

# Unit tests
go test ./...

# Acceptance tests (run against an in-memory mock of the MotherDuck API)
TF_ACC=1 go test ./... -v
```

To point acceptance tests at a specific Terraform binary, set `TF_ACC_TERRAFORM_PATH`.

To use a local build in Terraform, add a
[dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "jpig18/motherduck" = "/path/to/terraform-provider-motherduck"
  }
  direct {}
}
```

## Releasing

Tag a semver release (`v0.1.0`); the `release` GitHub Actions workflow builds, signs,
and publishes artifacts with GoReleaser in the layout the
[Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
expects. Requires `GPG_PRIVATE_KEY` and `PASSPHRASE` repo secrets.

## License

[MIT](LICENSE)
