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
  default = "prod"
}

# Production is built at next.craigdevjohnson.com and takes
# craigdevjohnson.com at the switch (docs/baseline.md). The hostname is also
# in infra/bootstrap (locals.environments), where the origin-secret header
# is added for it; change both together.
variable "hostname" {
  type    = string
  default = "next.craigdevjohnson.com"
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
  description = "The Access invite list for production."
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

# The Tool backends the production site calls (ADR 0006): each backend's Function
# URL and the ARN of the Lambda behind it, from that repo's own OpenTofu
# outputs. Empty means unset, and the Tool's page says its backend is
# unavailable. The values go in prod.tfvars; see infra/README.md.

variable "soccer_backend_url" {
  description = "The Let's Plan Soccer backend's Function URL (SOCCER_BACKEND_URL); empty leaves the Tool unconnected."
  type        = string
  default     = ""
}

variable "soccer_backend_function_arn" {
  description = "The Lambda behind soccer_backend_url, named soccer-prod-*; set together with the URL."
  type        = string
  default     = ""
}

variable "soccer_backend_signing" {
  description = "SOCCER_BACKEND_SIGNING override (lambda or off); empty lets the server sign as it does by default."
  type        = string
  default     = ""
}

variable "minecraft_backend_url" {
  description = "The Minecraft Launcher backend's Function URL (MINECRAFT_BACKEND_URL); empty leaves the Tool unconnected."
  type        = string
  default     = ""
}

variable "minecraft_backend_function_arn" {
  description = "The Lambda behind minecraft_backend_url, named <prefix>-prod-launcher; set together with the URL."
  type        = string
  default     = ""
}

variable "minecraft_backend_signing" {
  description = "MINECRAFT_BACKEND_SIGNING override (lambda or off); empty lets the server sign as it does by default."
  type        = string
  default     = ""
}

variable "foundry_backend_url" {
  description = "The Foundry backend's Function URL (FOUNDRY_BACKEND_URL); empty leaves the Tool unconnected."
  type        = string
  default     = ""
}

variable "foundry_backend_function_arn" {
  description = "The Lambda behind foundry_backend_url, named foundry-prod-controller; set together with the URL."
  type        = string
  default     = ""
}

variable "foundry_join_url" {
  description = "Foundry's join page (FOUNDRY_JOIN_URL), offered to Members with the foundry grant when its status can't be checked; empty offers none."
  type        = string
  default     = ""
}

variable "foundry_backend_signing" {
  description = "FOUNDRY_BACKEND_SIGNING override (lambda or off); empty lets the server sign as it does by default."
  type        = string
  default     = ""
}
