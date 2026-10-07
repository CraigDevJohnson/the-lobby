terraform {
  required_version = ">= 1.13.0"

  # No backend block on purpose: this root keeps local state, because it
  # creates the bucket that every other root stores its state in.

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
