---
page_title: "motherduck_tokens Data Source"
description: |-
  Access tokens of a MotherDuck user.
---

# motherduck_tokens (Data Source)

Access tokens of a MotherDuck user (`GET /v1/users/{username}/tokens`).
Secret token values are never returned by this endpoint.

## Example Usage

```terraform
data "motherduck_tokens" "etl" {
  username = "svc-etl"
}

output "token_names" {
  value = [for t in data.motherduck_tokens.etl.tokens : t.name]
}
```

## Schema

### Required

- `username` (String) User whose tokens to list.

### Read-Only

- `id` (String) Same as `username`.
- `tokens` (List of Object)
  - `id` (String) Token ID (UUID).
  - `name` (String)
  - `token_type` (String) `read_write` or `read_scaling`.
  - `created_at` (String)
  - `expires_at` (String) Empty for non-expiring tokens.
  - `read_only` (Boolean)
