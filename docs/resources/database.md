---
page_title: "motherduck_database Resource"
description: |-
  A MotherDuck database, managed via SQL DDL.
---

# motherduck_database (Resource)

A MotherDuck database. MotherDuck exposes databases only over SQL (`CREATE DATABASE` /
`DROP DATABASE`), not the REST API, so this resource runs its DDL over a data-plane SQL
connection authenticated with a per-resource account `token` rather than the provider's
admin token. This lets one provider configuration manage databases across many accounts,
and lets the `token` reference a `motherduck_token` created in the same apply.

~> **Destroying this resource drops the database. `DROP DATABASE` uses the default
`RESTRICT`, so the delete fails if a share was created from this database.**

-> Refresh detects drift by scanning `SHOW ALL DATABASES` for the database `name` (its
`alias` column); a database absent from that listing is removed from state.

## Example Usage

```terraform
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}

resource "motherduck_token" "etl" {
  username = motherduck_service_account.etl.username
  name     = "etl-data-plane"
}

resource "motherduck_database" "warehouse" {
  name  = "warehouse"
  token = motherduck_token.etl.token
}
```

## Schema

### Required

- `name` (String) Name of the database. Changing it replaces the database.
- `token` (String, Sensitive) Data-plane token of the account that owns this database
  (e.g. `motherduck_token.x.token`). The `CREATE DATABASE` DDL runs as this account.

### Read-Only

- `id` (String) Same as `name`.

## Import

Import runs as the account whose `token` is set in configuration; the import ID is the
database name.

```shell
terraform import motherduck_database.warehouse warehouse
```
