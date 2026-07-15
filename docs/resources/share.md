---
page_title: "motherduck_share Resource"
description: |-
  A MotherDuck share of a database, managed via SQL DDL.
---

# motherduck_share (Resource)

A MotherDuck share of a database. MotherDuck exposes shares only over SQL
(`CREATE SHARE` / `DROP SHARE`), not the REST API, so this resource runs its DDL over a
data-plane SQL connection authenticated with the source-database owner's `token` — the
same account that owns the shared database, since `CREATE SHARE` runs as that owner.

~> **Changing `name`, `database`, or `access` replaces the share and rotates its
`share_url`, disconnecting existing consumers. MotherDuck has no `ALTER SHARE`; an access
change can only be applied by `CREATE OR REPLACE SHARE`, which mints a new URL. `access`
is therefore replace-triggering rather than in-place updatable.**

-> `grant_to` is authoritative from configuration only. MotherDuck exposes no system view
of a restricted share's grantees, so refresh does not read the granted user set back, and
out-of-band `GRANT`/`REVOKE` does not surface as drift. Grants apply only when `access` is
`restricted`.

## Example Usage

```terraform
resource "motherduck_service_account" "publisher" {
  username = "svc_publisher"
}

resource "motherduck_token" "publisher" {
  username = motherduck_service_account.publisher.username
  name     = "publisher-data-plane"
}

resource "motherduck_database" "analytics" {
  name  = "analytics"
  token = motherduck_token.publisher.token
}

resource "motherduck_share" "analytics" {
  name     = "analytics_share"
  database = motherduck_database.analytics.name
  token    = motherduck_token.publisher.token
  access   = "restricted"
  grant_to = ["consumer_1", "consumer_2@example-com"]
}

output "share_url" {
  value = motherduck_share.analytics.share_url
}
```

## Schema

### Required

- `name` (String) Name of the share. Changing it replaces the share.
- `database` (String) Name of the source database to share. Changing it replaces the share.
- `token` (String, Sensitive) Data-plane token of the account that owns the source
  database (e.g. `motherduck_token.x.token`). `CREATE SHARE` runs as this account.

### Optional

- `access` (String) `restricted` (default; owner only, extend with `grant_to`) or
  `unrestricted` (all MotherDuck users in the same cloud region). Changing it replaces the
  share because the only SQL path to change access is `CREATE OR REPLACE SHARE`, which
  rotates the URL.
- `grant_to` (List of String) Account usernames granted READ on this share. Applies only
  when `access` is `restricted`. Not read back from the server, so it never shows drift.

### Read-Only

- `share_url` (String) The `md:_share/<database>/<token>` URL consumers attach. Rotates on
  replace.

## Import

The import ID is `<token>,<share-name>`. The token is part of the ID because the read
that follows import must reach the owning account; it is the account's data-plane token,
the same value the resource's `token` attribute takes. `grant_to`, `database`, and
`access` are not recoverable from `OWNED_SHARES`, so verify them against configuration
after import.

```shell
terraform import motherduck_share.analytics "$MOTHERDUCK_TOKEN,analytics_share"
```
