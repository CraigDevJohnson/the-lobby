variable "aws_region" {
  description = "AWS region for the state bucket and everything modules/site creates."
  type        = string
  default     = "us-west-2"
}

variable "aws_account_id" {
  description = "The workloads account. Applies against any other account are refused."
  type        = string
  default     = "793680745829"
}

variable "state_bucket_name" {
  description = "Name of the private S3 bucket that holds OpenTofu state for this site."
  type        = string
  default     = "craigdevjohnson-tofu-state-793680745829"
}

variable "github_subject_prefix" {
  description = "Start of the sub claim in this repository's GitHub OIDC tokens. GitHub now uses the immutable form with owner and repository ids, so a renamed or re-created repository can never match. Read it with: gh api repos/<owner>/<repo>/actions/oidc/customization/sub"
  type        = string
  default     = "repo:CraigDevJohnson@42454849/the-lobby@1408231725"
}

variable "github_repository" {
  description = "The GitHub repository whose Actions may assume the roles here, as owner/name."
  type        = string
  default     = "CraigDevJohnson/the-lobby"
}

variable "budget_email" {
  description = "Where the monthly budget alerts go. No default on purpose: set it in bootstrap.auto.tfvars (gitignored)."
  type        = string
  sensitive   = false

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.budget_email))
    error_message = "budget_email must be an email address."
  }
}

variable "cloudflare_account_id" {
  description = "Cloudflare account that owns the zone and the Zero Trust organisation."
  type        = string
  default     = "ada2e308f4d17f259887ed31aa91cdf3"
}

variable "cloudflare_zone_name" {
  description = "The Cloudflare zone every environment's hostname lives in."
  type        = string
  default     = "craigdevjohnson.com"
}

variable "origin_secrets" {
  description = "Per environment (dev, prod), the secret Cloudflare adds in the X-Origin-Secret header on requests to that environment's hostname. Each must equal the origin_secret that environment's own root gives its Lambda, or the site refuses every request (ADR 0005). Set in bootstrap.auto.tfvars (gitignored); generate each with: openssl rand -hex 32"
  type        = map(string)
  sensitive   = true

  validation {
    condition     = alltrue([for env in ["dev", "prod"] : contains(keys(var.origin_secrets), env)])
    error_message = "origin_secrets needs a value for every environment: dev and prod."
  }

  validation {
    condition     = alltrue([for secret in values(var.origin_secrets) : length(secret) >= 32])
    error_message = "Every origin secret must be at least 32 characters; generate one with: openssl rand -hex 32"
  }
}
