# Who may enroll a device with the Cloudflare One Agent. In Cloudflare this
# is an Access application of type warp carrying the enrollment policy; the
# dashboard's "Device enrollment permissions" page is this application, so
# if the dashboard already made one it is imported rather than created
# (infra/README.md). Cloudflare names it ("Warp Login App") and gives it its
# domain (<team>.cloudflareaccess.com/warp); neither is set here. The adults
# sign in by emailed one-time code, like Members do on the site.
#
# Dashboard: Zero Trust > Team & Resources > Devices > Device profiles >
# Management > Device enrollment permissions.
resource "cloudflare_zero_trust_access_application" "enrollment" {
  account_id       = var.cloudflare_account_id
  type             = "warp"
  session_duration = "24h"
  allowed_idps     = [one(local.otp_identity_provider_ids)]

  lifecycle {
    precondition {
      condition     = length(local.otp_identity_provider_ids) == 1
      error_message = "The Cloudflare account must have exactly one One-time PIN identity provider, which infra/bootstrap creates (ADR 0008)."
    }
  }

  policies = [
    {
      name       = "Family adults"
      decision   = "allow"
      precedence = 1
      include    = [for email in var.enrollment_emails : { email = { email = email } }]
    }
  ]
}
