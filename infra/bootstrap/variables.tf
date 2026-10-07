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
