---
page_title: "motherduck_active_accounts Data Source"
description: |-
  Active accounts in the organization and their active Ducklings.
---

# motherduck_active_accounts (Data Source)

Active accounts in the organization and their currently active Ducklings
(`GET /v1/active_accounts`). Requires the Admin role.

~> This endpoint is marked **Preview** by MotherDuck.

## Example Usage

```terraform
data "motherduck_active_accounts" "all" {}

output "active_usernames" {
  value = [for a in data.motherduck_active_accounts.all.accounts : a.username]
}
```

## Schema

### Read-Only

- `id` (String) Placeholder identifier (always `active_accounts`).
- `accounts` (List of Object)
  - `username` (String)
  - `ducklings` (List of Object) `id`, `type`, `status`.
