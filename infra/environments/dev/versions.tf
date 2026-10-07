terraform {
  required_version = ">= 1.13.0"

  backend "s3" {
    bucket       = "craigdevjohnson-tofu-state-793680745829"
    key          = "the-lobby/dev/terraform.tfstate"
    region       = "us-west-2"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}
