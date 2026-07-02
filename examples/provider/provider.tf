terraform {
  required_providers {
    motherduck = {
      source = "jpig18/motherduck"
    }
  }
}

# Token from MOTHERDUCK_API_TOKEN (or MOTHERDUCK_TOKEN) env var.
# Must belong to a user with the Admin role.
provider "motherduck" {}
