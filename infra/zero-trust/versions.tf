terraform {
  required_version = ">= 1.13.0"

  # State beside the other roots' (infra/README.md, Where state lives). The
  # bucket is infra/bootstrap's, so applying this root needs an AWS session
  # for the state as well as the Cloudflare token for the resources.
  backend "s3" {
    bucket       = "craigdevjohnson-tofu-state-793680745829"
    key          = "the-lobby/zero-trust/terraform.tfstate"
    region       = "us-west-2"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}
