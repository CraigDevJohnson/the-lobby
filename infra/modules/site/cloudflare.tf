# Everything Cloudflare does for this hostname: DNS, the secret header,
# bot settings, and Access on the sign-in path.

data "cloudflare_zone" "site" {
  filter = {
    name = var.cloudflare_zone_name
    account = {
      id = var.cloudflare_account_id
    }
  }
}

# DNS-only records that let ACM check we own the hostname.
resource "cloudflare_dns_record" "acm_validation" {
  for_each = {
    for option in aws_acm_certificate.site.domain_validation_options : option.domain_name => {
      name   = option.resource_record_name
      type   = option.resource_record_type
      record = option.resource_record_value
    }
  }

  zone_id = data.cloudflare_zone.site.id
  name    = trimsuffix(each.value.name, ".")
  type    = each.value.type
  content = trimsuffix(each.value.record, ".")
  ttl     = 60
  proxied = false
  comment = "ACM validation for ${local.name}"
}

# The hostname itself: proxied through Cloudflare to the API Gateway domain.
resource "cloudflare_dns_record" "site" {
  zone_id = data.cloudflare_zone.site.id
  name    = var.hostname
  type    = "CNAME"
  content = aws_apigatewayv2_domain_name.site.domain_name_configuration[0].target_domain_name
  ttl     = 1 # automatic; required for proxied records
  proxied = true
  comment = "${local.name}: API Gateway custom domain"
}

# Cloudflare adds a secret header on every request to the hostname. The Go
# server refuses requests without it, so nobody can reach API Gateway around
# Cloudflare (ADR 0005). Cloudflare allows one zone ruleset per phase.
resource "cloudflare_ruleset" "origin_secret" {
  zone_id     = data.cloudflare_zone.site.id
  name        = "${local.name}-origin-secret"
  description = "Adds X-Origin-Secret on requests to ${var.hostname}"
  kind        = "zone"
  phase       = "http_request_late_transform"

  rules = [
    {
      ref         = "origin_secret"
      description = "Add X-Origin-Secret for ${var.hostname}"
      expression  = "(http.host eq \"${var.hostname}\")"
      action      = "rewrite"
      enabled     = true
      action_parameters = {
        headers = {
          "X-Origin-Secret" = {
            operation = "set"
            value     = var.origin_secret
          }
        }
      }
    }
  ]
}

# Zone-wide and deliberate: calendar apps are not browsers, and on the free
# plan Bot Fight Mode cannot be skipped for one path (ADR 0005).
resource "cloudflare_bot_management" "zone" {
  zone_id    = data.cloudflare_zone.site.id
  fight_mode = false
}

# Members sign in by emailed one-time code. Google comes later.
resource "cloudflare_zero_trust_access_identity_provider" "otp" {
  account_id = var.cloudflare_account_id
  name       = "One-time PIN"
  type       = "onetimepin"
  config     = {}
}

# resource "cloudflare_zero_trust_access_identity_provider" "google" {
#   account_id = var.cloudflare_account_id
#   name       = "Google"
#   type       = "google"
#   config = {
#     client_id     = var.google_client_id
#     client_secret = var.google_client_secret
#   }
# }

# Access guards only /signin. Everything else is guarded by the site's own
# Sign-in session, so Visitors and calendar apps never meet Access (ADR 0004).
resource "cloudflare_zero_trust_access_application" "signin" {
  account_id       = var.cloudflare_account_id
  name             = "${local.name}-signin"
  type             = "self_hosted"
  domain           = "${var.hostname}/signin"
  session_duration = "24h"
  allowed_idps     = [cloudflare_zero_trust_access_identity_provider.otp.id]

  app_launcher_visible = false

  policies = [
    {
      name       = "Invited Members"
      decision   = "allow"
      precedence = 1
      include    = [for email in var.allowed_emails : { email = { email = email } }]
    }
  ]
}
