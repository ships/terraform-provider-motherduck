# MotherDuck provider verification stack

Proves the published `jpig18/motherduck` provider works end-to-end against a real
MotherDuck org, pulling the provider from the **public Terraform Registry**.

## What it exercises

| Function | Type | Verified by |
| --- | --- | --- |
| `motherduck_service_account` | resource | created; used by everything below |
| `motherduck_token` | resource | `check "token_created"` (non-empty secret) |
| `motherduck_duckling_config` | resource | `check "duckling_roundtrip"` |
| `motherduck_tokens` | data source | `check "token_roundtrip"` (sees the created token) |
| `motherduck_duckling_config` | data source | `check "duckling_*_roundtrip"` |
| `motherduck_active_accounts` | data source | `active_account_count` output |
| `motherduck_embed_session` | ephemeral | optional (needs a Dive UUID) — see below |

`check {}` blocks make `apply` **fail loudly** if any round-trip is wrong.

## Run

```bash
export MOTHERDUCK_API_TOKEN=<admin token>   # must be an Admin-role token

terraform init      # installs jpig18/motherduck v0.1.0 from the public registry
terraform apply     # creates the test account + token + config, runs the checks
terraform output    # inspect the round-tripped values

terraform destroy   # removes the test service account (and its token)
```

> ⚠️ `terraform destroy` permanently deletes the test service account — fine for
> this throwaway `tfverify-svc`, but note the delete is irreversible.

Override names if `tfverify-svc` collides:

```bash
terraform apply -var 'username=tfverify-2' -var 'token_name=tfverify-2-tok'
```

## Optional: embed session (ephemeral resource)

Needs a real Dive UUID:

```bash
mv embed_session.tf.optional embed_session.tf
terraform apply -var 'dive_id=<your-dive-uuid>'
```

The session token is ephemeral (never written to state/plan), so a clean apply =
the API accepted it; a bad `dive_id` fails the apply.

## Notes

- `active_account_count` may be `0`: the endpoint only lists accounts with a
  *currently active* Duckling, and a brand-new test account hasn't run a query.
- No `dev_overrides` here — this is the real registry install path a consumer gets.
