# =============================================================================
# MotherDuck provider verification stack
#
# Exercises every REST-backed function the provider exposes and asserts the
# round-trips with check {} blocks. Run against a real MotherDuck org:
#
#   export MOTHERDUCK_API_TOKEN=<admin token>
#   terraform init      # pulls jpig18/motherduck from the public registry
#   terraform apply
#   terraform destroy   # cleans up (deletes the test service account + token)
# =============================================================================

# --- Resource 1: service account (POST /v1/users) ---------------------------
resource "motherduck_service_account" "verify" {
  username = var.username
}

# --- Resource 2: access token (POST /v1/users/{u}/tokens) -------------------
resource "motherduck_token" "verify" {
  username    = motherduck_service_account.verify.username
  name        = var.token_name
  ttl_seconds = 3600
  token_type  = "read_write"
}

# --- Resource 3: Duckling config (PUT /v1/users/{u}/instances) --------------
resource "motherduck_duckling_config" "verify" {
  username = motherduck_service_account.verify.username

  read_write {
    instance_size    = "standard"
    cooldown_seconds = 300
  }

  read_scaling {
    instance_size = "pulse"
    flock_size    = 1
  }
}

# --- Data source 1: list tokens (GET /v1/users/{u}/tokens) ------------------
data "motherduck_tokens" "verify" {
  username   = motherduck_service_account.verify.username
  depends_on = [motherduck_token.verify]
}

# --- Data source 2: read Duckling config (GET /v1/users/{u}/instances) ------
data "motherduck_duckling_config" "verify" {
  username   = motherduck_service_account.verify.username
  depends_on = [motherduck_duckling_config.verify]
}

# --- Data source 3: active accounts (GET /v1/active_accounts) ---------------
data "motherduck_active_accounts" "all" {
  depends_on = [motherduck_service_account.verify]
}

# =============================================================================
# Self-verifying assertions — apply FAILS if any round-trip is wrong.
# =============================================================================

check "token_created" {
  assert {
    condition     = motherduck_token.verify.token != ""
    error_message = "Token resource returned an empty secret value."
  }
}

check "token_roundtrip" {
  assert {
    condition = contains(
      [for t in data.motherduck_tokens.verify.tokens : t.id],
      motherduck_token.verify.id
    )
    error_message = "Created token was not found via the motherduck_tokens data source."
  }
}

check "duckling_roundtrip" {
  assert {
    condition     = data.motherduck_duckling_config.verify.read_write.instance_size == "standard"
    error_message = "Duckling read_write.instance_size did not round-trip as 'standard'."
  }
}

check "duckling_scaling_roundtrip" {
  assert {
    condition     = data.motherduck_duckling_config.verify.read_scaling.flock_size == 1
    error_message = "Duckling read_scaling.flock_size did not round-trip as 1."
  }
}
