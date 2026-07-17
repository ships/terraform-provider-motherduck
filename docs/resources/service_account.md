---
page_title: "motherduck_service_account Resource"
description: |-
  A MotherDuck service-account user.
---

# motherduck_service_account (Resource)

A MotherDuck service-account user (`POST /v1/users`). The API currently creates users
with the **Member** role only.

~> **`deletion_policy` defaults to `prevent`: destroying is refused until you change it.**
Set `deletion_policy = "cascade"` to permanently delete the user and all of their data
(databases, tokens, shares they own) — this cannot be undone — or `deletion_policy = "retain"`
to remove the account from Terraform state while leaving the user in MotherDuck.

-> `prevent` blocks every destroy path, including the replacement Terraform plans when you
change `username`. Relax the policy in one apply, then rename or destroy in the next. Removing
the `resource` block and relaxing the policy in the *same* apply is refused, because Terraform
plans the destroy against the prior (`prevent`) state. `retain` is the attribute form of
Terraform's `removed { lifecycle { destroy = false } }` block.

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

### Optional

- `deletion_policy` (String) Behavior when this resource is destroyed: `prevent` (default),
  `retain`, or `cascade`. See the note above. Changing it is an in-place update, not a replacement.

### Read-Only

- `id` (String) Same as `username`.

## Import

```shell
terraform import motherduck_service_account.etl svc_etl
```
