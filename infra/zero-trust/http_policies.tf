# HTTP policies, which apply to the iPhones through the Cloudflare One
# Agent once inspect_https is on. Gateway evaluates Do Not Inspect policies
# before everything else whatever their precedence, so these are written
# before any inspection is turned on. Allow or block policies for the
# adults, when Craig names them, go below these (#40).

# Apps known to pin their certificates, which break under inspection:
# Cloudflare's own "Do Not Inspect" application type, kept current by them.
resource "cloudflare_zero_trust_gateway_policy" "do_not_inspect_known" {
  account_id  = var.cloudflare_account_id
  name        = "Family: do not inspect apps with pinned certificates"
  description = "Cloudflare's Do Not Inspect application type: apps known to use embedded certificates, which would break under TLS decryption."
  enabled     = true
  action      = "off"
  filters     = ["http"]
  precedence  = 1000
  traffic     = "any(app.type.ids[*] in {${local.do_not_inspect_app_type}})"
}

# The household's own apps that must not be inspected, named in the
# gitignored tfvars; the policy exists only while the list is not empty.
# Cloudflare's docs: a Do Not Inspect policy for named applications written
# through Terraform must use the app.hosts_ids and app.supports_ids
# selectors (an app's own hostnames and the shared ones it depends on), not
# app.ids. The two are joined with or so a request to either kind of
# hostname escapes inspection.
resource "cloudflare_zero_trust_gateway_policy" "do_not_inspect_named" {
  count = length(var.do_not_inspect_app_ids) > 0 ? 1 : 0

  account_id  = var.cloudflare_account_id
  name        = "Family: do not inspect the household's own apps"
  description = "Apps named in do_not_inspect_app_ids, such as banking apps, which must not be inspected."
  enabled     = true
  action      = "off"
  filters     = ["http"]
  precedence  = 2000
  traffic     = "any(app.hosts_ids[*] in {${local.do_not_inspect_app_id_set}}) or any(app.supports_ids[*] in {${local.do_not_inspect_app_id_set}})"
}
