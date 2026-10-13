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
  foundry_backend_url            = var.foundry_backend_url
  foundry_backend_function_arn   = var.foundry_backend_function_arn
  foundry_backend_signing        = var.foundry_backend_signing
}
