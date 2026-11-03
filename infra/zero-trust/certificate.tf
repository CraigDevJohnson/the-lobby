# The Gateway root certificate: what Gateway re-signs HTTPS with when it
# inspects, and what the adult profile installs on the iPhones
# (internal/profile). Accounts created since 2024-10-17 already have one in
# the "available" state, so it is imported, not generated (three generations
# a day, ten available at once; infra/README.md). activate keeps it
# deployed at the edge; selecting it for inspection is settings.tf's job,
# done when inspect_https is on.
#
# Dashboard: Zero Trust > Traffic policies > Traffic settings > Certificates.
resource "cloudflare_zero_trust_gateway_certificate" "gateway" {
  account_id = var.cloudflare_account_id
  activate   = true
}
