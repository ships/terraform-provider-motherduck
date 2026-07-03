terraform {
  # Ephemeral resources (motherduck_embed_session) need >= 1.10;
  # check {} blocks need >= 1.5. Pin to 1.10 to cover both.
  required_version = ">= 1.10"

  required_providers {
    motherduck = {
      # Pulled from the PUBLIC Terraform Registry (no dev_overrides).
      source  = "jpig18/motherduck"
      version = ">= 0.1.0"
    }
  }
}

provider "motherduck" {
  # Reads MOTHERDUCK_API_TOKEN (or MOTHERDUCK_TOKEN) from the environment.
  # Token must belong to a user with the Admin role.
}
