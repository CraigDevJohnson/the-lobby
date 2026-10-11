module "site" {
  source = "../../modules/site"

  environment              = var.environment
  hostname                 = var.hostname
  cloudflare_account_id    = var.cloudflare_account_id
  cloudflare_zone_name     = var.cloudflare_zone_name
  access_team_domain       = var.access_team_domain
  allowed_emails           = var.allowed_emails
  origin_secret            = var.origin_secret
  zip_path                 = var.site_zip_path
  permissions_boundary_arn = var.permissions_boundary_arn

  soccer_backend_url             = var.soccer_backend_url
  soccer_backend_function_arn    = var.soccer_backend_function_arn
  soccer_backend_signing         = var.soccer_backend_signing
  minecraft_backend_url          = var.minecraft_backend_url
  minecraft_backend_function_arn = var.minecraft_backend_function_arn
  minecraft_backend_signing      = var.minecraft_backend_signing
}

# The zone-wide Cloudflare pieces moved to infra/bootstrap (ADR 0008). These
# blocks drop them from this root's state without deleting them from
# Cloudflare, where bootstrap imports the same objects (infra/README.md).
# Delete the blocks once dev has been applied with them.
removed {
  from = module.site.cloudflare_ruleset.origin_secret

  lifecycle {
    destroy = false
  }
}

removed {
  from = module.site.cloudflare_ruleset.calendar_links

  lifecycle {
    destroy = false
  }
}

removed {
  from = module.site.cloudflare_bot_management.zone

  lifecycle {
    destroy = false
  }
}

removed {
  from = module.site.cloudflare_zero_trust_access_identity_provider.otp

  lifecycle {
    destroy = false
  }
}
