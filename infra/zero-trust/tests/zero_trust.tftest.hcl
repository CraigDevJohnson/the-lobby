# Offline checks on the family Gateway configuration. The provider is
# mocked, so no account is touched and no credentials are needed.

mock_provider "cloudflare" {
  # The account's identity providers, which infra/bootstrap owns: one
  # One-time PIN provider, as the enrollment application requires.
  mock_data "cloudflare_zero_trust_access_identity_providers" {
    defaults = {
      # Every attribute of the element type has to be present in a mock.
      result = [
        {
          id                      = "11111111-2222-3333-4444-555555555555"
          name                    = "One-time PIN"
          type                    = "onetimepin"
          config                  = null
          read_only               = null
          saml_certificate_set    = null
          saml_certificate_set_id = null
          scim_config             = null
        }
      ]
    }
  }

  # Both locations get the same mock id and subdomain; the tests check
  # shapes, not which of the two a value came from.
  mock_resource "cloudflare_zero_trust_dns_location" {
    defaults = {
      id            = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
      doh_subdomain = "mocksubdomain"
    }
  }

  mock_resource "cloudflare_zero_trust_gateway_certificate" {
    defaults = {
      id             = "99999999-8888-7777-6666-555555555555"
      certificate    = "-----BEGIN CERTIFICATE-----\nMOCK\n-----END CERTIFICATE-----\n"
      binding_status = "available"
      in_use         = false
    }
  }
}

variables {
  enrollment_emails      = ["one@example.com", "two@example.com"]
  do_not_inspect_app_ids = []
}

run "two_dns_locations_over_https_only" {
  command = plan

  assert {
    condition     = toset([for l in cloudflare_zero_trust_dns_location.location : l.name]) == toset(["Kids", "Adults"])
    error_message = "There must be exactly the Kids and Adults locations."
  }

  assert {
    condition     = alltrue([for l in cloudflare_zero_trust_dns_location.location : l.client_default == false])
    error_message = "Neither location may be the account's default: an existing default is left alone."
  }

  assert {
    condition     = alltrue([for l in cloudflare_zero_trust_dns_location.location : l.endpoints.doh.enabled == true && l.endpoints.doh.require_token == false])
    error_message = "Each location must answer DNS over HTTPS without a token, which is what a device profile can use."
  }

  assert {
    condition     = alltrue([for l in cloudflare_zero_trust_dns_location.location : l.endpoints.dot.enabled == false && l.endpoints.ipv4.enabled == false && l.endpoints.ipv6.enabled == false])
    error_message = "DoT, IPv4 and IPv6 endpoints stay off: a phone on cellular has no fixed source network."
  }

  assert {
    condition     = alltrue([for l in cloudflare_zero_trust_dns_location.location : l.max_ttl.mode == "inherit" && length(l.networks) == 0 && length(l.endpoints.doh.networks) == 0 && length(l.endpoints.dot.networks) == 0 && length(l.endpoints.ipv6.networks) == 0])
    error_message = "networks, max_ttl and each endpoint's networks must be set in the config: provider 5.x cannot create a location while any of them is unknown (Value Conversion Error)."
  }

  assert {
    condition     = output.kids_doh_url == "https://mocksubdomain.cloudflare-gateway.com/dns-query" && output.adults_doh_url == "https://mocksubdomain.cloudflare-gateway.com/dns-query"
    error_message = "The DoH outputs must be https://<doh_subdomain>.cloudflare-gateway.com/dns-query, the form internal/profile accepts."
  }
}

run "dns_policies_are_scoped_to_their_location_and_ordered" {
  command = plan

  assert {
    condition = alltrue([
      for p in [
        cloudflare_zero_trust_gateway_policy.kids_block_content,
        cloudflare_zero_trust_gateway_policy.kids_block_security,
        cloudflare_zero_trust_gateway_policy.kids_safesearch,
        cloudflare_zero_trust_gateway_policy.kids_youtube_restricted,
        cloudflare_zero_trust_gateway_policy.adults_block_security,
      ] : p.filters == tolist(["dns"]) && p.enabled == true && strcontains(p.traffic, "dns.location in {\"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee\"}")
    ])
    error_message = "Every DNS policy must be a dns filter scoped to a location with the Location selector."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.kids_block_content.action == "block" && startswith(cloudflare_zero_trust_gateway_policy.kids_block_content.traffic, "any(dns.content_category[*] in {2 8}) and ")
    error_message = "The kids content policy must block Adult Themes (2) and Gambling (8)."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.kids_block_security.action == "block" && startswith(cloudflare_zero_trust_gateway_policy.kids_block_security.traffic, "any(dns.security_category[*] in {117 131 68}) and ")
    error_message = "The kids security policy must block Anonymizer (68), Malware (117) and Phishing (131)."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.kids_safesearch.action == "safesearch" && cloudflare_zero_trust_gateway_policy.kids_youtube_restricted.action == "ytrestricted"
    error_message = "The kids location must enforce SafeSearch and YouTube Restricted Mode."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.adults_block_security.action == "block" && startswith(cloudflare_zero_trust_gateway_policy.adults_block_security.traffic, "any(dns.security_category[*] in {117 131}) and ")
    error_message = "The adults policy must block Malware (117) and Phishing (131) only."
  }

  assert {
    condition = length(distinct([
      cloudflare_zero_trust_gateway_policy.kids_block_content.precedence,
      cloudflare_zero_trust_gateway_policy.kids_block_security.precedence,
      cloudflare_zero_trust_gateway_policy.kids_safesearch.precedence,
      cloudflare_zero_trust_gateway_policy.kids_youtube_restricted.precedence,
      cloudflare_zero_trust_gateway_policy.adults_block_security.precedence,
    ])) == 5
    error_message = "Every DNS policy needs its own explicit precedence."
  }
}

run "do_not_inspect_covers_cloudflares_list_and_nothing_else_by_default" {
  command = plan

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.do_not_inspect_known.action == "off" && cloudflare_zero_trust_gateway_policy.do_not_inspect_known.filters == tolist(["http"]) && cloudflare_zero_trust_gateway_policy.do_not_inspect_known.traffic == "any(app.type.ids[*] in {16})"
    error_message = "The first Do Not Inspect policy must cover Cloudflare's Do Not Inspect application type (16)."
  }

  assert {
    condition     = length(cloudflare_zero_trust_gateway_policy.do_not_inspect_named) == 0
    error_message = "With no apps named, there must be no second Do Not Inspect policy."
  }
}

run "named_apps_get_their_own_do_not_inspect_policy" {
  command = plan

  variables {
    do_not_inspect_app_ids = [1245, 505]
  }

  assert {
    condition     = length(cloudflare_zero_trust_gateway_policy.do_not_inspect_named) == 1 && cloudflare_zero_trust_gateway_policy.do_not_inspect_named[0].action == "off" && cloudflare_zero_trust_gateway_policy.do_not_inspect_named[0].filters == tolist(["http"])
    error_message = "Named apps must get one Do Not Inspect http policy."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.do_not_inspect_named[0].traffic == "any(app.hosts_ids[*] in {1245 505}) or any(app.supports_ids[*] in {1245 505})"
    error_message = "The named policy must match the apps' hostnames and support hostnames by id, as Cloudflare's docs require for Terraform-written Do Not Inspect policies."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_policy.do_not_inspect_named[0].precedence > cloudflare_zero_trust_gateway_policy.do_not_inspect_known.precedence
    error_message = "Cloudflare's own list runs first."
  }
}

run "enrollment_allows_the_adults_by_one_time_code" {
  command = plan

  assert {
    condition     = cloudflare_zero_trust_access_application.enrollment.type == "warp"
    error_message = "Device enrollment permissions are an Access application of type warp."
  }

  assert {
    condition     = length(cloudflare_zero_trust_access_application.enrollment.policies) == 1 && cloudflare_zero_trust_access_application.enrollment.policies[0].decision == "allow"
    error_message = "The enrollment application must carry exactly one allow policy."
  }

  assert {
    condition     = length(cloudflare_zero_trust_access_application.enrollment.policies[0].include) == length(var.enrollment_emails)
    error_message = "Every enrollment address must appear in the policy."
  }

  assert {
    condition     = cloudflare_zero_trust_access_application.enrollment.allowed_idps == toset(["11111111-2222-3333-4444-555555555555"])
    error_message = "Enrollment must allow exactly the account's One-time PIN identity provider, found through the data source (ADR 0008)."
  }
}

run "inspection_is_off_until_asked" {
  command = plan

  assert {
    condition     = cloudflare_zero_trust_gateway_certificate.gateway.activate == null
    error_message = "activate stays unset: true makes every update ask Cloudflare to activate a certificate Gateway is using, which it refuses (code 2118), and false would deactivate it."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_settings.account.settings.tls_decrypt.enabled == false && cloudflare_zero_trust_gateway_settings.account.settings.certificate.id == "99999999-8888-7777-6666-555555555555"
    error_message = "By default TLS decryption is off, and the Gateway certificate Gateway already uses stays selected."
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_settings.account.settings.activity_log.enabled == true
    error_message = "The Gateway activity log stays on: it is the one log every device writes to."
  }

  assert {
    condition     = cloudflare_zero_trust_device_settings.account.gateway_proxy_enabled == false && cloudflare_zero_trust_device_settings.account.gateway_udp_proxy_enabled == false
    error_message = "By default the Gateway proxy is off on TCP and UDP."
  }

  assert {
    condition     = length(cloudflare_zero_trust_device_default_profile.lock) == 0
    error_message = "By default the device profile is left alone."
  }
}

run "inspection_follows_the_variable" {
  command = plan

  variables {
    inspect_https = true
  }

  assert {
    condition     = cloudflare_zero_trust_gateway_settings.account.settings.tls_decrypt.enabled == true && cloudflare_zero_trust_gateway_settings.account.settings.certificate.id == "99999999-8888-7777-6666-555555555555"
    error_message = "With inspect_https, TLS decryption is on and the Gateway certificate is selected for it."
  }

  assert {
    condition     = cloudflare_zero_trust_device_settings.account.gateway_proxy_enabled == true && cloudflare_zero_trust_device_settings.account.gateway_udp_proxy_enabled == true
    error_message = "With inspect_https, the Gateway proxy is on for TCP and UDP."
  }
}

run "the_client_lock_is_optional" {
  command = plan

  variables {
    lock_client = true
  }

  assert {
    condition     = length(cloudflare_zero_trust_device_default_profile.lock) == 1 && cloudflare_zero_trust_device_default_profile.lock[0].switch_locked == true && cloudflare_zero_trust_device_default_profile.lock[0].allowed_to_leave == false
    error_message = "With lock_client, the default device profile locks the switch and keeps devices in the organisation."
  }
}

run "enrollment_needs_at_least_one_address" {
  command = plan

  variables {
    enrollment_emails = []
  }

  expect_failures = [var.enrollment_emails]
}

run "enrollment_addresses_are_email_addresses" {
  command = plan

  variables {
    enrollment_emails = ["not an address"]
  }

  expect_failures = [var.enrollment_emails]
}

run "app_ids_are_positive_whole_numbers" {
  command = plan

  variables {
    do_not_inspect_app_ids = [0, 12.5]
  }

  expect_failures = [var.do_not_inspect_app_ids]
}
