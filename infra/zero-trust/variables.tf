variable "cloudflare_account_id" {
  description = "Cloudflare account that owns the Zero Trust organisation."
  type        = string
  default     = "ada2e308f4d17f259887ed31aa91cdf3"
}

variable "enrollment_emails" {
  description = "Who may enroll a device in the Zero Trust organisation with the Cloudflare One Agent: the two adults' addresses. Personal data, so it is set in the gitignored zero-trust.auto.tfvars (copy zero-trust.auto.tfvars.example) and never committed. Both addresses also go in the /admin page's list (modules/site, admin_emails)."
  type        = list(string)

  validation {
    condition     = length(var.enrollment_emails) > 0
    error_message = "enrollment_emails must list at least one address, or nobody can enroll a device."
  }

  validation {
    condition     = alltrue([for email in var.enrollment_emails : can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", email))])
    error_message = "Every entry of enrollment_emails must be an email address."
  }
}

variable "do_not_inspect_app_ids" {
  description = "Gateway application ids (Zero Trust > Traffic policies, the Application selector; or GET /accounts/{account_id}/gateway/app_types) of apps that must never be inspected beyond Cloudflare's own Do Not Inspect list: the household's banking apps. Which banks a household uses is personal data, so there is no default: set it, empty to start, in the gitignored zero-trust.auto.tfvars. The second Do Not Inspect policy exists only when the list is not empty."
  type        = list(number)

  validation {
    condition     = alltrue([for id in var.do_not_inspect_app_ids : id == floor(id) && id > 0])
    error_message = "Every entry of do_not_inspect_app_ids must be a positive whole number, an application id from the Application selector."
  }
}

variable "inspect_https" {
  description = "Turn on TLS decryption and the Gateway proxy (TCP and UDP), so HTTP policies and logs see inside HTTPS from the iPhones. Set to true only after the adult profile has been installed and the Gateway certificate trusted on both iPhones (docs/family-devices.md): decryption before that breaks HTTPS on the phones."
  type        = bool
  default     = false
}

variable "lock_client" {
  description = "Lock the Cloudflare One Agent on enrolled devices: the switch cannot be turned off and a device cannot leave the organisation. Off until the agent has run clean for a while (#40)."
  type        = bool
  default     = false
}
