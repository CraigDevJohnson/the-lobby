# The Gateway root certificate: what Gateway re-signs HTTPS with when it
# inspects, and what the adult profile installs on the iPhones
# (internal/profile). Accounts created since 2024-10-17 already have one,
# deployed at the edge and in use by Gateway, so it is imported, not
# generated (three generations a day, ten available at once;
# infra/README.md). Selecting it for inspection is settings.tf's job.
#
# activate is left unset on purpose. It is a Terraform-only switch: true
# makes every update POST .../activate, which Cloudflare refuses for a
# certificate Gateway is using (400, code 2118, "Certificate cannot be
# unbound or deleted while in use"), and false would deactivate it. Unset,
# the provider makes neither call.
#
# Dashboard: Zero Trust > Traffic policies > Traffic settings > Certificates.
resource "cloudflare_zero_trust_gateway_certificate" "gateway" {
  account_id = var.cloudflare_account_id
}
