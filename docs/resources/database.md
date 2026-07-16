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

~> **`deletion_policy` defaults to `prevent`: destroying is refused until you change it.**
Set `deletion_policy = "cascade"` to run `DROP DATABASE ... CASCADE` (drops the database and
its dependents), or `deletion_policy = "retain"` to remove it from Terraform state while leaving
the database in MotherDuck. `prevent` also blocks the replacement Terraform plans when you change
`name`.

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

### Optional

- `deletion_policy` (String) Behavior when this resource is destroyed: `prevent` (default),
  `retain`, or `cascade`. See the note above. Changing it is an in-place update, not a replacement.

### Read-Only

- `id` (String) Same as `name`.

## Import

The import ID is `<token>,<database-name>`. The token is part of the ID because the
managed database does not carry it and the initial read must reach the owning account
to confirm the database exists. The token is the account's data-plane token, the same
value the resource's `token` attribute takes.

```shell
terraform import motherduck_database.warehouse "$MOTHERDUCK_TOKEN,warehouse"
```
