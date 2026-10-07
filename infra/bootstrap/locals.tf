locals {
  # Every resource of this site carries this tag (default_tags in
  # providers.tf). The budget and the cost allocation tag key off it.
  site_tag_key   = "Site"
  site_tag_value = "craigdevjohnson.com"

  github_oidc_host = "token.actions.githubusercontent.com"

  # One entry per deployed environment. Both deployer roles are built from
  # this map, so they cannot drift apart. name_prefix must equal local.name
  # in modules/site ("site-<environment>"); state_prefix must equal the
  # directory part of that environment's backend key.
  environments = {
    dev = {
      github_environment = "dev"
      name_prefix        = "site-dev"
      state_prefix       = "the-lobby/dev"
    }
    prod = {
      github_environment = "production"
      name_prefix        = "site-prod"
      state_prefix       = "the-lobby/prod"
    }
  }
}
