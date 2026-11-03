# DNS policies, each scoped to one location with the Location selector and
# ordered by precedence (lower runs first). The traffic expressions are
# written in the form the API returns, so a plan shows no change on every
# run. Each block, SafeSearch and Restricted Mode decision is logged under
# its location in Zero Trust > Insights > Logs > Gateway > DNS.

# Kids: no adult themes or gambling.
resource "cloudflare_zero_trust_gateway_policy" "kids_block_content" {
  account_id  = var.cloudflare_account_id
  name        = "Family kids: block adult themes and gambling"
  description = "The kids' iPads (Kids location): block the Adult Themes and Gambling content categories."
  enabled     = true
  action      = "block"
  filters     = ["dns"]
  precedence  = 1000
  traffic     = "any(dns.content_category[*] in {${join(" ", sort([for id in values(local.content_categories_kids) : tostring(id)]))}}) and ${local.location_filter["kids"]}"
}

# Kids: no anonymizers (the nearest thing to a VPN or proxy category),
# malware or phishing.
resource "cloudflare_zero_trust_gateway_policy" "kids_block_security" {
  account_id  = var.cloudflare_account_id
  name        = "Family kids: block anonymizers, malware and phishing"
  description = "The kids' iPads (Kids location): block the Anonymizer, Malware and Phishing security categories."
  enabled     = true
  action      = "block"
  filters     = ["dns"]
  precedence  = 2000
  traffic     = "any(dns.security_category[*] in {${join(" ", sort([for id in values(local.security_categories_kids) : tostring(id)]))}}) and ${local.location_filter["kids"]}"
}

# Kids: SafeSearch on every search engine Gateway knows.
resource "cloudflare_zero_trust_gateway_policy" "kids_safesearch" {
  account_id  = var.cloudflare_account_id
  name        = "Family kids: SafeSearch"
  description = "The kids' iPads (Kids location): enforce SafeSearch."
  enabled     = true
  action      = "safesearch"
  filters     = ["dns"]
  precedence  = 3000
  traffic     = local.location_filter["kids"]
}

# Kids: YouTube Restricted Mode.
resource "cloudflare_zero_trust_gateway_policy" "kids_youtube_restricted" {
  account_id  = var.cloudflare_account_id
  name        = "Family kids: YouTube Restricted Mode"
  description = "The kids' iPads (Kids location): enforce YouTube Restricted Mode."
  enabled     = true
  action      = "ytrestricted"
  filters     = ["dns"]
  precedence  = 4000
  traffic     = local.location_filter["kids"]
}

# Adults: malware and phishing only. The phones' real filter is the
# Cloudflare One Agent; this location is their fallback when it is off.
resource "cloudflare_zero_trust_gateway_policy" "adults_block_security" {
  account_id  = var.cloudflare_account_id
  name        = "Family adults: block malware and phishing"
  description = "The adults' iPhones with the agent off (Adults location): block the Malware and Phishing security categories."
  enabled     = true
  action      = "block"
  filters     = ["dns"]
  precedence  = 5000
  traffic     = "any(dns.security_category[*] in {${join(" ", sort([for id in values(local.security_categories_adults) : tostring(id)]))}}) and ${local.location_filter["adults"]}"
}
