terraform {
  required_version = ">= 1.13.0"

  # This root creates the state bucket, then keeps its own state in it.
  # On a fresh account the bucket does not exist yet: comment this block
  # out, apply with local state, put it back and run
  # `tofu init -migrate-state`. See ../README.md.
  backend "s3" {
    bucket       = "craigdevjohnson-tofu-state-793680745829"
    key          = "the-lobby/bootstrap/terraform.tfstate"
    region       = "us-west-2"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
