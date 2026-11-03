# One DNS location per device group, reached over DNS over HTTPS only. The
# IPv4 endpoint stays off: Gateway tells IPv4 queries apart by source
# network, which a phone on cellular never has, and a location with it on
# would answer queries from anywhere. Neither is the default location; if
# the account already has one (Gateway makes one on first use), it is left
# alone (infra/README.md, The family Gateway configuration).
#
# Dashboard: Zero Trust > Traffic controls > Resolver endpoints.
resource "cloudflare_zero_trust_dns_location" "location" {
  for_each = local.locations

  account_id     = var.cloudflare_account_id
  name           = each.value
  client_default = false
  ecs_support    = false

  # Once endpoints is set, the provider wants all four.
  endpoints = {
    doh = {
      enabled       = true
      require_token = false
    }
    dot  = { enabled = false }
    ipv4 = { enabled = false }
    ipv6 = { enabled = false }
  }
}
