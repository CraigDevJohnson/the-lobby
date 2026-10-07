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
}
