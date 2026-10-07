variable "environment" {
  description = "Which environment this is: dev or prod."
  type        = string

  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be \"dev\" or \"prod\"."
  }
}

variable "hostname" {
  description = "The site's hostname for this environment, for example dev.craigdevjohnson.com."
  type        = string
}

variable "cloudflare_account_id" {
  description = "Cloudflare account that owns the zone and the Zero Trust organisation."
  type        = string
}

variable "cloudflare_zone_name" {
  description = "The Cloudflare zone the hostname lives in, for example craigdevjohnson.com."
  type        = string
}

variable "access_team_domain" {
  description = "The Zero Trust team domain, for example https://<team>.cloudflareaccess.com. The Go server uses it to verify Access tokens."
  type        = string

  validation {
    condition     = startswith(var.access_team_domain, "https://") && endswith(var.access_team_domain, ".cloudflareaccess.com")
    error_message = "access_team_domain must look like https://<team>.cloudflareaccess.com."
  }
}

variable "allowed_emails" {
  description = "The Access invite list: email addresses that may reach /signin. This is the first of the two steps for adding a Member (ADR 0004)."
  type        = list(string)

  validation {
    condition     = length(var.allowed_emails) > 0
    error_message = "allowed_emails must list at least one address, or nobody can sign in."
  }
}

variable "origin_secret" {
  description = "Secret that Cloudflare adds in the X-Origin-Secret header and the Go server checks, so nothing reaches the site around Cloudflare (ADR 0005)."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.origin_secret) >= 32
    error_message = "origin_secret must be at least 32 characters; generate one with: openssl rand -hex 32"
  }
}

variable "zip_path" {
  description = "Path to the zip holding the Go server binary, named bootstrap."
  type        = string
}

variable "reserved_concurrent_executions" {
  description = "Concurrency cap on the site's Lambda, the cost brake from ADR 0002."
  type        = number
  default     = 5
}

variable "log_retention_days" {
  description = "How long CloudWatch keeps Lambda and API Gateway logs."
  type        = number
  default     = 30
}

variable "sessions_table_deletion_protection" {
  description = "Whether the Sign-in sessions table refuses to be deleted."
  type        = bool
  default     = false
}
