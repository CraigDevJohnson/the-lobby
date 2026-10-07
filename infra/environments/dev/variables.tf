variable "aws_region" {
  type    = string
  default = "us-west-2"
}

variable "aws_account_id" {
  description = "The workloads account. Applies against any other account are refused."
  type        = string
  default     = "793680745829"
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "hostname" {
  type    = string
  default = "dev.craigdevjohnson.com"
}

variable "cloudflare_zone_name" {
  type    = string
  default = "craigdevjohnson.com"
}

variable "cloudflare_account_id" {
  description = "Cloudflare account id (Dashboard, zone overview, right-hand column)."
  type        = string
}

variable "access_team_domain" {
  description = "Zero Trust team domain, for example https://<team>.cloudflareaccess.com."
  type        = string
}

variable "allowed_emails" {
  description = "The Access invite list for dev."
  type        = list(string)
}

variable "origin_secret" {
  description = "Secret header value shared by Cloudflare and the Go server. Generate with: openssl rand -hex 32"
  type        = string
  sensitive   = true
}

variable "site_zip_path" {
  description = "Zip holding the built Go server (bootstrap), relative to this directory unless absolute."
  type        = string
  default     = "../../../dist/site.zip"
}

variable "permissions_boundary_arn" {
  description = "From infra/bootstrap: site_permissions_boundary_arn."
  type        = string
}
