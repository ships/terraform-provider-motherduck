---
page_title: "motherduck_token Resource"
description: |-
  An access token for a MotherDuck user.
---

# motherduck_token (Resource)

An access token for a MotherDuck user (`POST /v1/users/{username}/tokens`).
The API has no token-update operation, so **any change forces a new token**.

-> The secret `token` value is only returned by the API at creation time and is stored
in Terraform state. Protect your state, or hand the value straight to a secret manager
(e.g. an `aws_secretsmanager_secret_version`).

## Example Usage

```terraform
resource "motherduck_token" "ci" {
  username    = motherduck_service_account.etl.username
  name        = "ci-token"
  ttl_seconds = 2592000 # 30 days
  token_type  = "read_write"
}

# Store the secret out-of-band instead of consuming it from state everywhere
resource "aws_secretsmanager_secret_version" "motherduck" {
  secret_id     = aws_secretsmanager_secret.motherduck.id
  secret_string = motherduck_token.ci.token
}
```

## Schema

### Required

- `username` (String) User the token belongs to. Forces replacement.
- `name` (String) Display name for the token. Forces replacement.

### Optional

- `ttl_seconds` (Number) Token lifetime in seconds (300 to 31536000). Omit for a
  non-expiring token. Forces replacement.
- `token_type` (String) `read_write` (default) or `read_scaling`. Forces replacement.

### Read-Only

- `id` (String) Token ID (UUID).
- `token` (String, Sensitive) The secret token value. Only returned at creation.
- `created_at` (String) Creation timestamp.
- `expires_at` (String) Expiry timestamp; empty for non-expiring tokens.
- `read_only` (Boolean) Whether the token is read-only.

## Import

Tokens are imported as `<username>/<token_id>`:

```shell
terraform import motherduck_token.ci svc_etl/9a1b2c3d-...
```

Two caveats, both because the API does not return them:

- The secret `token` value cannot be recovered, so it remains null after import.
- `ttl_seconds` cannot be recovered either (the API exposes `expires_at`, not the
  original lifetime). If your config sets `ttl_seconds`, the next plan will show the
  token being **replaced** (`ttl_seconds` forces replacement). Import is therefore
  most useful for non-expiring tokens; for a token created with a TTL, expect a
  one-time recreate on the first apply after import.
