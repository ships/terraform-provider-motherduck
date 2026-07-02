---
page_title: "motherduck_embed_session Ephemeral Resource"
description: |-
  A short-lived embed session for a MotherDuck Dive.
---

# motherduck_embed_session (Ephemeral Resource)

An embed session for a MotherDuck Dive, created on behalf of a service account
(`POST /v1/dives/{dive_id}/embed-session`).

Sessions are short-lived credentials, so this is an **ephemeral resource**
(requires Terraform 1.10+): the session token is never persisted to state or plan.

## Example Usage

```terraform
ephemeral "motherduck_embed_session" "dashboard" {
  dive_id  = "8b6c8b1e-..."
  username = motherduck_service_account.embed.username

  session_hint = "tenant-42"

  required_resources {
    url   = "md:_share/datalake/..."
    alias = "datalake"
  }

  initial_state = jsonencode({ filters = {} })
}
```

## Schema

### Required

- `dive_id` (String) UUID of the Dive to embed.
- `username` (String) Service account the session is created for.

### Optional

- `session_hint` (String) Opaque hint used to partition sessions (for example, an
  end-user or tenant ID).
- `required_resources` (List of Object) Database resources the session needs access to:
  `url` (Required), `alias` (Optional).
- `initial_state` (String) JSON-encoded initial state for the embedded Dive.
- `version` (Number) Embed session protocol version.

### Read-Only

- `session` (String, Sensitive) The embed session token.
