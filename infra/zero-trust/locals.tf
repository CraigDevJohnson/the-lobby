locals {
  # The two Gateway DNS locations. Each device profile points at one of
  # their DNS over HTTPS endpoints (internal/profile); every policy below is
  # scoped to one of them, so the iPads and the iPhones get different rules
  # and the Gateway log says which location a query came from.
  locations = {
    kids   = "Kids"
    adults = "Adults"
  }

  # The Location selector for each location, in the form the API stores.
  location_filter = {
    for key, location in cloudflare_zero_trust_dns_location.location :
    key => "dns.location in {\"${location.id}\"}"
  }

  # Cloudflare's category ids (Zero Trust > Traffic policies, the Content
  # Categories and Security Categories selectors; checked 2026-10-31). There
  # is no "VPN" or "proxy" category: Anonymizer is Cloudflare's category for
  # sites that let people browse anonymously.
  content_categories_kids    = { adult_themes = 2, gambling = 8 }
  security_categories_kids   = { anonymizer = 68, malware = 117, phishing = 131 }
  security_categories_adults = { malware = 117, phishing = 131 }

  # The Application selector's "Do Not Inspect" application type, which
  # Cloudflare keeps up to date with apps known to pin their certificates.
  # Cloudflare's own HTTP policy examples write it as
  # any(app.type.ids[*] in {16}).
  do_not_inspect_app_type = 16

  # The household's named apps as a selector set: space-separated ids.
  do_not_inspect_app_id_set = join(" ", [for id in var.do_not_inspect_app_ids : tostring(id)])

  # Members and the two adults sign in by emailed one-time code through the
  # account's One-time PIN identity provider, which infra/bootstrap owns;
  # this root only finds it, as modules/site does.
  otp_identity_provider_ids = [
    for provider in data.cloudflare_zero_trust_access_identity_providers.account.result : provider.id
    if provider.type == "onetimepin"
  ]
}

data "cloudflare_zero_trust_access_identity_providers" "account" {
  account_id = var.cloudflare_account_id
}
