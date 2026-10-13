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

# The Tool backends the dev site calls (ADR 0006): each backend's Function
# URL and the ARN of the Lambda behind it, from that repo's own OpenTofu
# outputs. Empty means unset, and the Tool's page says its backend is
# unavailable. The values go in dev.tfvars; see infra/README.md.

variable "soccer_backend_url" {
  description = "The Let's Plan Soccer backend's Function URL (SOCCER_BACKEND_URL); empty leaves the Tool unconnected."
  type        = string
  default     = ""
}

variable "soccer_backend_function_arn" {
  description = "The Lambda behind soccer_backend_url, named soccer-dev-*; set together with the URL."
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
  description = "The Lambda behind minecraft_backend_url, named <prefix>-dev-launcher; set together with the URL."
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
  description = "The Lambda behind foundry_backend_url, named foundry-dev-controller; set together with the URL."
  type        = string
  default     = ""
}

variable "foundry_backend_signing" {
  description = "FOUNDRY_BACKEND_SIGNING override (lambda or off); empty lets the server sign as it does by default."
  type        = string
  default     = ""
}
