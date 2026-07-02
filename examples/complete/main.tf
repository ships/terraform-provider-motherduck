terraform {
  required_providers {
    motherduck = {
      source = "jpig18/motherduck"
    }
  }
}

provider "motherduck" {}

# A service account for a data pipeline.
resource "motherduck_service_account" "etl" {
  username = "svc-etl"
}

# A 90-day read/write token for it. The secret is in state; ship it to a
# secret manager rather than passing it around as an output.
resource "motherduck_token" "etl" {
  username    = motherduck_service_account.etl.username
  name        = "etl-pipeline"
  ttl_seconds = 60 * 60 * 24 * 90
}

# Right-size its compute.
resource "motherduck_duckling_config" "etl" {
  username = motherduck_service_account.etl.username

  read_write {
    instance_size    = "standard"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size = "pulse"
    flock_size    = 2
  }
}

# Introspection.
data "motherduck_tokens" "etl" {
  username   = motherduck_service_account.etl.username
  depends_on = [motherduck_token.etl]
}

data "motherduck_active_accounts" "all" {}

output "etl_token_ids" {
  value = [for t in data.motherduck_tokens.etl.tokens : t.id]
}

output "active_accounts" {
  value = [for a in data.motherduck_active_accounts.all.accounts : a.username]
}
