locals {
  # Every resource of this site carries this tag (default_tags in
  # providers.tf). The budget and the cost allocation tag key off it.
  site_tag_key   = "Site"
  site_tag_value = "craigdevjohnson.com"

  github_oidc_host = "token.actions.githubusercontent.com"

  # One entry per deployed environment. Both deployer roles and the
  # zone-wide Cloudflare rules are built from this map, so they cannot drift
  # apart. name_prefix must equal local.name in modules/site
  # ("site-<environment>"); state_prefix must equal the directory part of
  # that environment's backend key; hostname must equal the hostname that
  # environment's root gives the module, because the origin-secret header
  # is added for exactly that host (cloudflare.tf). Production is built at
  # next.craigdevjohnson.com and takes craigdevjohnson.com at the switch
  # (docs/baseline.md); both places change together then.
  environments = {
    dev = {
      github_environment = "dev"
      name_prefix        = "site-dev"
      state_prefix       = "the-lobby/dev"
      hostname           = "dev.craigdevjohnson.com"
    }
    prod = {
      github_environment = "production"
      name_prefix        = "site-prod"
      state_prefix       = "the-lobby/prod"
      hostname           = "next.craigdevjohnson.com"
    }
  }

  # Where calendar apps fetch Session links (internal/soccer, LinkPrefix).
  calendar_link_path = "/soccer/link/"
}
