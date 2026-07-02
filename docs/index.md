---
page_title: "MotherDuck Provider"
description: |-
  Manage MotherDuck service accounts, access tokens, Duckling instance configuration, and Dive embed sessions via the MotherDuck REST API.
---

# MotherDuck Provider

The MotherDuck provider manages organization-level MotherDuck resources through the
[MotherDuck REST API](https://motherduck.com/docs/api/rest-api/):

- **Service accounts** (`motherduck_service_account`)
- **Access tokens** (`motherduck_token`)
- **Duckling (instance) configuration** (`motherduck_duckling_config`)
- **Dive embed sessions** (ephemeral `motherduck_embed_session`)
- Data sources for tokens, Duckling configuration, and active accounts

It covers the full surface of the REST API as of mid-2026. Databases and shares are
**not** manageable through the REST API (they are SQL-only, e.g. `CREATE DATABASE`,
`CREATE SHARE`); when MotherDuck adds endpoints for them, this provider will follow.

~> The REST API is marked **Preview** by MotherDuck; endpoints may change.

## Authentication

All operations require an access token belonging to a MotherDuck user with the
**Admin** role (read/write scope). Get one from the MotherDuck UI under
*Settings → Integrations → Access tokens*.

## Example Usage

```terraform
terraform {
  required_providers {
    motherduck = {
      source = "jpig18/motherduck"
    }
  }
}

# Token from MOTHERDUCK_API_TOKEN (or MOTHERDUCK_TOKEN) env var
provider "motherduck" {}

resource "motherduck_service_account" "etl" {
  username = "svc-etl"
}

resource "motherduck_token" "etl" {
  username    = motherduck_service_account.etl.username
  name        = "etl-pipeline"
  ttl_seconds = 60 * 60 * 24 * 90 # 90 days
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

## Schema

### Optional

- `token` (String, Sensitive) MotherDuck access token of an **Admin** user. May also be
  set via the `MOTHERDUCK_API_TOKEN` or `MOTHERDUCK_TOKEN` environment variable.
- `base_url` (String) Base URL of the MotherDuck REST API. Defaults to
  `https://api.motherduck.com`. May also be set via `MOTHERDUCK_API_BASE_URL`.
