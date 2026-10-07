provider "aws" {
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]

  default_tags {
    tags = {
      Site        = "craigdevjohnson.com"
      Environment = var.environment
      ManagedBy   = "opentofu"
      Repo        = "website"
    }
  }
}

# The token comes from the CLOUDFLARE_API_TOKEN environment variable.
provider "cloudflare" {}
