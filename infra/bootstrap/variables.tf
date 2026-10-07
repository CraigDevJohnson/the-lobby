variable "aws_region" {
  description = "AWS region for the state bucket."
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
