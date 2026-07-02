---
page_title: "motherduck_duckling_config Resource"
description: |-
  Duckling (compute instance) configuration for a MotherDuck user.
---

# motherduck_duckling_config (Resource)

Duckling (compute instance) configuration for a user
(`PUT /v1/users/{username}/instances`). Requires the Admin role.

-> This is a settings-style resource: the configuration exists for every user, so
destroying it only removes it from Terraform state — it does not reset the user's
configuration in MotherDuck.

## Example Usage

```terraform
resource "motherduck_duckling_config" "etl" {
  username = motherduck_service_account.etl.username

  read_write {
    instance_size    = "jumbo"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size    = "standard"
    flock_size       = 4
    cooldown_seconds = 600
  }
}
```

## Schema

### Required

- `username` (String) User whose Duckling configuration is managed. Forces replacement.
- `read_write` (Block) Configuration of the user's read/write Duckling.
  - `instance_size` (String, Required) `pulse`, `standard`, `jumbo`, `mega`, or `giga`.
  - `cooldown_seconds` (Number, Optional) Idle seconds before spin-down (60 to 86400).
- `read_scaling` (Block) Configuration of the user's read-scaling Ducklings.
  - `instance_size` (String, Required) `pulse`, `standard`, `jumbo`, `mega`, or `giga`.
  - `flock_size` (Number, Required) Number of read-scaling instances (0 to 64).
  - `cooldown_seconds` (Number, Optional) Idle seconds before spin-down (60 to 86400).

### Read-Only

- `id` (String) Same as `username`.

## Import

```shell
terraform import motherduck_duckling_config.etl svc-etl
```
