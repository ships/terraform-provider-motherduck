---
page_title: "motherduck_service_account Resource"
description: |-
  A MotherDuck service-account user.
---

# motherduck_service_account (Resource)

A MotherDuck service-account user (`POST /v1/users`). The API currently creates users
with the **Member** role only.

~> **Destroying this resource permanently deletes the user and all of their data
(databases, tokens, shares they own). This cannot be undone.**

-> The REST API has no per-user read endpoint, so existence is verified via the
user's token list during refresh.

## Example Usage

```terraform
resource "motherduck_service_account" "etl" {
  username = "svc_etl"
}
```

## Schema

### Required

- `username` (String) Username for the service account. Changing it replaces the account.

### Read-Only

- `id` (String) Same as `username`.

## Import

```shell
terraform import motherduck_service_account.etl svc_etl
```
