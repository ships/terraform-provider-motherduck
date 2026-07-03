output "service_account" {
  description = "The created service account username."
  value       = motherduck_service_account.verify.username
}

output "token_id" {
  description = "ID of the created token (secret itself is sensitive)."
  value       = motherduck_token.verify.id
}

output "token_expires_at" {
  value = motherduck_token.verify.expires_at
}

output "tokens_seen_by_data_source" {
  description = "Token names the data source reports for the service account."
  value       = [for t in data.motherduck_tokens.verify.tokens : t.name]
}

output "duckling_config_readback" {
  description = "Duckling config as read back through the data source."
  value = {
    read_write   = data.motherduck_duckling_config.verify.read_write
    read_scaling = data.motherduck_duckling_config.verify.read_scaling
  }
}

output "active_account_count" {
  description = "Number of accounts with active Ducklings (a fresh test account may be 0)."
  value       = length(data.motherduck_active_accounts.all.accounts)
}
