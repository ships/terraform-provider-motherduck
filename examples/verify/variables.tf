variable "username" {
  type        = string
  description = "Service account to create for the verification run. Must be unique in the org. Letters, numbers, and underscores only."
  default     = "tfverify_svc"
}

variable "token_name" {
  type        = string
  description = "Name for the access token created during verification."
  default     = "tfverify_token"
}

variable "dive_id" {
  type        = string
  description = "Optional Dive UUID to exercise the ephemeral embed-session resource. Leave empty to skip."
  default     = ""
}
