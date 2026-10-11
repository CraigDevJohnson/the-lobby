# The Cloudflare pieces every environment of the site shares. Cloudflare
# allows one zone ruleset per phase and one One-time PIN identity provider
# per account, so none of these can belong to an environment root: this
# root declares each once, with one rule per environment (ADR 0008). What is
# specific to one hostname (its DNS records and its Access application)
# stays in modules/site.

data "cloudflare_zone" "site" {
  filter = {
    name = var.cloudflare_zone_name
    account = {
      id = var.cloudflare_account_id
    }
  }
}

# Cloudflare adds a secret header on every request to each environment's
# hostname. That environment's Go server refuses requests without it, so
# nobody can reach API Gateway around Cloudflare (ADR 0005). Each
# environment has its own secret, given to its Lambda by its own root.
resource "cloudflare_ruleset" "origin_secret" {
  zone_id     = data.cloudflare_zone.site.id
  name        = "the-lobby-origin-secret"
  description = "Adds X-Origin-Secret on requests to the site's hostnames"
  kind        = "zone"
  phase       = "http_request_late_transform"

  rules = [
    for env in sort(keys(local.environments)) : {
      ref         = "origin_secret_${env}"
      description = "Add X-Origin-Secret for ${local.environments[env].hostname} (${env})"
      expression  = "(http.host eq \"${local.environments[env].hostname}\")"
      action      = "rewrite"
      enabled     = true
      action_parameters = {
        headers = {
          "X-Origin-Secret" = {
            operation = "set"
            value     = var.origin_secrets[env]
          }
        }
      }
    }
  ]
}

# Calendar apps fetch Session links at /soccer/link/<token>.ics with their
# own headers, not a browser's, so Browser Integrity Check is off on that
# path of every hostname (ADR 0005). A configuration rule is a zone ruleset
# too, and this resource declares the whole phase: importing a ruleset the
# zone already has hands every rule in it to this list, so anything not
# declared here is deleted on the next apply (../README.md).
resource "cloudflare_ruleset" "calendar_links" {
  zone_id     = data.cloudflare_zone.site.id
  name        = "the-lobby-calendar-links"
  description = "Settings for calendar apps fetching the site's Session links"
  kind        = "zone"
  phase       = "http_config_settings"

  rules = [
    for env in sort(keys(local.environments)) : {
      ref         = "calendar_links_bic_off_${env}"
      description = "Browser Integrity Check off for ${local.environments[env].hostname}${local.calendar_link_path}"
      expression  = "(http.host eq \"${local.environments[env].hostname}\" and starts_with(http.request.uri.path, \"${local.calendar_link_path}\"))"
      action      = "set_config"
      enabled     = true
      action_parameters = {
        bic = false
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
