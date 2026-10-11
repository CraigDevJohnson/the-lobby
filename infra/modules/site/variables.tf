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

variable "permissions_boundary_arn" {
  description = "Permissions boundary for the Lambda role, created by infra/bootstrap. Deploys from CI can only create roles that carry it."
  type        = string
  default     = null
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

# The Tool backends the site calls with signed requests (ADR 0006). Each
# Tool takes three settings, named after the environment variables the Go
# server reads. Empty means unset: the Lambda then gets no *_BACKEND_URL
# and the page says the Tool is unavailable. The URL is the backend's
# Function URL and the ARN the Lambda behind it, which the site role is
# allowed to invoke; the site permissions boundary only lets the role call
# functions named *-<environment>-*, so the ARN must carry this
# environment's name. The signing override is "lambda" (always sign) or
# "off" (never); left empty, the server signs everywhere but a developer's
# computer, which is what a deployed site wants.

variable "soccer_backend_url" {
  description = "The Schedule Downloader backend's Function URL (the soccer repo), passed to the Lambda as SOCCER_BACKEND_URL. Empty leaves the Tool unconnected."
  type        = string
  default     = ""

  validation {
    condition     = var.soccer_backend_url == "" || startswith(var.soccer_backend_url, "https://")
    error_message = "soccer_backend_url must be the backend's https:// Function URL, or empty."
  }
}

variable "soccer_backend_function_arn" {
  description = "The Lambda behind soccer_backend_url, which the site role may invoke (lambda:InvokeFunctionUrl and InvokeFunction). Named soccer-<environment>-*, as the site permissions boundary requires. Set together with soccer_backend_url."
  type        = string
  default     = ""

  validation {
    condition     = (var.soccer_backend_url == "") == (var.soccer_backend_function_arn == "")
    error_message = "soccer_backend_function_arn and soccer_backend_url are set together or not at all: the site must be allowed to call the backend it is given."
  }

  validation {
    condition     = var.soccer_backend_function_arn == "" || can(regex("^arn:aws:lambda:[a-z0-9-]+:[0-9]{12}:function:[A-Za-z0-9_-]*-${var.environment}-[A-Za-z0-9_-]*(:[A-Za-z0-9_-]+)?$", var.soccer_backend_function_arn))
    error_message = "soccer_backend_function_arn must be a Lambda function ARN named <tool>-${var.environment}-*; the site permissions boundary lets the site call nothing else."
  }
}

variable "soccer_backend_signing" {
  description = "SOCCER_BACKEND_SIGNING override: lambda always signs requests to the backend, off never does. Empty passes nothing, and the server signs everywhere but a developer's computer."
  type        = string
  default     = ""

  validation {
    condition     = contains(["", "lambda", "off"], var.soccer_backend_signing)
    error_message = "soccer_backend_signing must be \"lambda\", \"off\", or empty."
  }
}

variable "minecraft_backend_url" {
  description = "The Minecraft Launcher backend's Function URL (the minecraft repo's launcher), passed to the Lambda as MINECRAFT_BACKEND_URL. Empty leaves the Tool unconnected."
  type        = string
  default     = ""

  validation {
    condition     = var.minecraft_backend_url == "" || startswith(var.minecraft_backend_url, "https://")
    error_message = "minecraft_backend_url must be the backend's https:// Function URL, or empty."
  }
}

variable "minecraft_backend_function_arn" {
  description = "The Lambda behind minecraft_backend_url, which the site role may invoke (lambda:InvokeFunctionUrl and InvokeFunction). Named <prefix>-<environment>-launcher, as the site permissions boundary requires. Set together with minecraft_backend_url."
  type        = string
  default     = ""

  validation {
    condition     = (var.minecraft_backend_url == "") == (var.minecraft_backend_function_arn == "")
    error_message = "minecraft_backend_function_arn and minecraft_backend_url are set together or not at all: the site must be allowed to call the backend it is given."
  }

  validation {
    condition     = var.minecraft_backend_function_arn == "" || can(regex("^arn:aws:lambda:[a-z0-9-]+:[0-9]{12}:function:[A-Za-z0-9_-]*-${var.environment}-[A-Za-z0-9_-]*(:[A-Za-z0-9_-]+)?$", var.minecraft_backend_function_arn))
    error_message = "minecraft_backend_function_arn must be a Lambda function ARN named <tool>-${var.environment}-*; the site permissions boundary lets the site call nothing else."
  }
}

variable "minecraft_backend_signing" {
  description = "MINECRAFT_BACKEND_SIGNING override: lambda always signs requests to the backend, off never does. Empty passes nothing, and the server signs everywhere but a developer's computer."
  type        = string
  default     = ""

  validation {
    condition     = contains(["", "lambda", "off"], var.minecraft_backend_signing)
    error_message = "minecraft_backend_signing must be \"lambda\", \"off\", or empty."
  }
}
