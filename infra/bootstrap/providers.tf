# Budgets and Cost Explorer are global services. The provider sends their
# calls to us-east-1 on its own, so one provider in us-west-2 is enough.
provider "aws" {
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]

  default_tags {
    tags = {
      (local.site_tag_key) = local.site_tag_value
      Environment          = "shared"
      ManagedBy            = "opentofu"
      Repo                 = "the-lobby"
      project              = "the-lobby" # the cost allocation tag every workload carries (aws-design G10)
    }
  }
}

# The zone-wide Cloudflare pieces (cloudflare.tf). The token comes from the
# CLOUDFLARE_API_TOKEN environment variable, as for the environment roots.
provider "cloudflare" {}
