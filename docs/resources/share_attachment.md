---
page_title: "motherduck_share_attachment Resource"
description: |-
  A consumer-side attachment of a MotherDuck share, managed via SQL DDL.
---

# motherduck_share_attachment (Resource)

A consumer-side attachment of a MotherDuck share. MotherDuck exposes attachments only over
SQL (`ATTACH` / `DETACH`), not the REST API, so this resource runs its DDL over a
data-plane SQL connection authenticated with the **consumer** account's `token` — the
account that will hold the attached database, since `ATTACH` binds the share into that
account under a local `alias`.

~> **All attributes are replace-triggering. `ATTACH`/`DETACH` have no in-place mutation, so
changing `share_url`, `alias`, or `token` detaches the old attachment and re-attaches.**

## Example Usage

```terraform
resource "motherduck_service_account" "consumer" {
  username = "svc_consumer"
}

resource "motherduck_token" "consumer" {
  username = motherduck_service_account.consumer.username
  name     = "consumer-data-plane"
}

# motherduck_share.analytics is created and granted to the consumer elsewhere.
resource "motherduck_share_attachment" "analytics" {
  share_url = motherduck_share.analytics.share_url
  alias     = "analytics"
  token     = motherduck_token.consumer.token
}
```

## Schema

### Required

- `share_url` (String) The `md:_share/...` URL to attach (e.g.
  `motherduck_share.x.share_url`). Changing it replaces the attachment.
- `alias` (String) Local database name the share is attached as. Changing it replaces the
  attachment.
- `token` (String, Sensitive) Data-plane token of the consumer account that will hold the
  attached database (e.g. `motherduck_token.x.token`). `ATTACH` runs as this account.
  Changing it replaces the attachment.

### Read-Only

- `id` (String) Same as `alias`.

## Import

Import runs as the account whose `token` is set in configuration; the import ID is the
attach `alias`. `token` and `share_url` are not recoverable from the attached database
alone, so verify them against configuration after import.

```shell
terraform import motherduck_share_attachment.analytics analytics
```
