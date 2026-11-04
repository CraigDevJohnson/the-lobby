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

  # Provider 5.27 (and every 5.x release up to 5.28.1) reads networks,
  # max_ttl and each endpoint's networks into Go types that cannot hold an
  # unknown value, although the schema marks them Computed. Left unset,
  # they plan as unknown and the create fails with "Value Conversion
  # Error ... Received unknown value". Each is set to what Cloudflare
  # defaults to: no source networks (an empty list lets every source in,
  # which a phone on cellular needs) and the account's own TTL cap.
  networks = []
  max_ttl  = { mode = "inherit" }

  # Once endpoints is set, the provider wants all four.
  endpoints = {
    doh = {
      enabled       = true
      require_token = false
      networks      = []
    }
    dot = {
      enabled  = false
      networks = []
    }
    ipv4 = { enabled = false }
    ipv6 = {
      enabled  = false
      networks = []
    }
  }

  # Cloudflare's schema marks these lists nullable. If the API answers an
  # empty one as null, every plan shows an in-place update setting it back
  # to [], which changes nothing. Do not hide that with ignore_changes: the
  # provider would then plan the ignored fields as unknown on any other
  # update and fail with the same Value Conversion Error.
}
