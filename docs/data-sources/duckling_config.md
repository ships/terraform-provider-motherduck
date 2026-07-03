---
page_title: "motherduck_duckling_config Data Source"
description: |-
  Duckling (compute instance) configuration of a MotherDuck user.
---

# motherduck_duckling_config (Data Source)

Duckling (compute instance) configuration of a user
(`GET /v1/users/{username}/instances`). Requires the Admin role.

## Example Usage

```terraform
data "motherduck_duckling_config" "etl" {
  username = "svc_etl"
}

output "etl_instance_size" {
  value = data.motherduck_duckling_config.etl.read_write.instance_size
}
```

## Schema

### Required

- `username` (String) User whose Duckling configuration to read.

### Read-Only

- `id` (String) Same as `username`.
- `read_write` (Object) `instance_size`, `cooldown_seconds`.
- `read_scaling` (Object) `instance_size`, `flock_size`, `cooldown_seconds`.
