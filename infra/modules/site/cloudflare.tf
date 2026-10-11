# What Cloudflare does for this hostname: DNS, and Access on the sign-in
# path. The zone-wide pieces (the X-Origin-Secret header, the calendar-link
# settings, Bot Fight Mode and the One-time PIN identity provider) belong
# to infra/bootstrap, because Cloudflare allows one of each per zone or
# account and every environment shares them (ADR 0008).

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

# Members sign in by emailed one-time code through the account's One-time
# PIN identity provider, which infra/bootstrap owns; this root only finds
# it. Google comes later, as a second provider there.
data "cloudflare_zero_trust_access_identity_providers" "account" {
  account_id = var.cloudflare_account_id
}

locals {
  otp_identity_provider_ids = [
    for provider in data.cloudflare_zero_trust_access_identity_providers.account.result : provider.id
    if provider.type == "onetimepin"
  ]
}

# Access guards only /signin. Everything else is guarded by the site's own
# Sign-in session, so Visitors and calendar apps never meet Access (ADR 0004).
resource "cloudflare_zero_trust_access_application" "signin" {
  account_id       = var.cloudflare_account_id
  name             = "${local.name}-signin"
  type             = "self_hosted"
  domain           = "${var.hostname}/signin"
  session_duration = "24h"
  allowed_idps     = [one(local.otp_identity_provider_ids)]

  app_launcher_visible = false

  lifecycle {
    precondition {
      condition     = length(local.otp_identity_provider_ids) == 1
      error_message = "The Cloudflare account must have exactly one One-time PIN identity provider, which infra/bootstrap creates (ADR 0008)."
    }
  }

  policies = [
    {
      name       = "Invited Members"
      decision   = "allow"
      precedence = 1
      include    = [for email in var.allowed_emails : { email = { email = email } }]
    }
  ]
}
