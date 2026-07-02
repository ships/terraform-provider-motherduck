terraform {
  required_providers {
    motherduck = {
      source = "jpig18/motherduck"
    }
  }
  # Ephemeral resources require Terraform 1.10+
  required_version = ">= 1.10"
}

provider "motherduck" {}

variable "dive_id" {
  type        = string
  description = "UUID of the Dive to embed"
}

resource "motherduck_service_account" "embed" {
  username = "svc-embed"
}

# The session token is ephemeral: never written to state or plan.
ephemeral "motherduck_embed_session" "dashboard" {
  dive_id  = var.dive_id
  username = motherduck_service_account.embed.username

  session_hint  = "tenant-42"
  initial_state = jsonencode({ filters = {} })
}
