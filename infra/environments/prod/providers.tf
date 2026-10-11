provider "aws" {
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]

  default_tags {
    tags = {
      Site        = "craigdevjohnson.com"
      Environment = var.environment
      ManagedBy   = "opentofu"
      Repo        = "the-lobby"
      project     = "the-lobby" # the cost allocation tag every workload carries (aws-design G10)
    }
  }
}

# The token comes from the CLOUDFLARE_API_TOKEN environment variable.
provider "cloudflare" {}
