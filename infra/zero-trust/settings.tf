# Inspection, behind inspect_https. Turning TLS decryption on before the
# certificate is trusted on the phones breaks their HTTPS, so the variable
# flips only after the runbook's certificate-trust step (docs/family-
# devices.md). The certificate stays selected either way: Gateway already
# uses the imported one, so clearing the selection while inspection is off
# would only change a live setting for nothing (and Cloudflare refuses to
# unbind a certificate in use, code 2118). With decryption off, the
# selection decrypts nothing.
#
# Dashboard: Zero Trust > Traffic policies > Traffic settings > Proxy and
# inspection.
resource "cloudflare_zero_trust_gateway_settings" "account" {
  account_id = var.cloudflare_account_id

  settings = {
    activity_log = { enabled = true }
    tls_decrypt  = { enabled = var.inspect_https }
    certificate  = { id = cloudflare_zero_trust_gateway_certificate.gateway.id }

    # Declared only so its computed version stays out of the plan: Cloudflare
    # bumps it on every write, and an ignored object would carry the old
    # number into the plan ("inconsistent result after apply"). Its one
    # setting stays as the dashboard has it (ignored below).
    extended_email_matching = { enabled = false }
  }

  # The provider reads the whole settings object back from the API, so a
  # setting this root does not declare would otherwise plan as a removal
  # on every run. Everything but the three above stays as the dashboard has
  # it.
  lifecycle {
    ignore_changes = [
      settings.antivirus,
      settings.block_page,
      settings.body_scanning,
      settings.browser_isolation,
      settings.custom_certificate,
      settings.extended_email_matching.enabled,
      settings.fips,
      settings.host_selector,
      settings.inspection,
      settings.max_ttl_secs,
      settings.protocol_detection,
      settings.sandbox,
    ]
  }
}

# The Gateway proxy toggles live with the device settings, not the gateway
# settings. HTTP policies see nothing until both are on. This resource
# cannot be imported; its first apply writes the two toggles.
resource "cloudflare_zero_trust_device_settings" "account" {
  account_id = var.cloudflare_account_id

  gateway_proxy_enabled     = var.inspect_https
  gateway_udp_proxy_enabled = var.inspect_https

  # Cloudflare answers with the account's other device settings too. Left
  # undeclared they would plan as removals, which the provider sends as
  # nulls and Cloudflare refuses (code 2024), so they stay as they are.
  lifecycle {
    ignore_changes = [
      disable_for_time,
      external_emergency_signal_enabled,
      external_emergency_signal_fingerprint,
      external_emergency_signal_interval,
      external_emergency_signal_url,
      root_certificate_installation_enabled,
      use_zt_virtual_ip,
    ]
  }
}

# Optional, Craig's call (#40, item 8): lock the agent's switch and keep
# enrolled devices from leaving the organisation. Off by default, so the
# account's default device profile is left as the dashboard has it until
# the agent has run clean for a while. Once on, the root owns the profile:
# the provider gives every switch it does not declare its own default
# (allow_mode_switch false, captive_portal 180, tunnel_protocol "" and so
# on) and sends them all, so read the plan against the dashboard's Default
# profile before applying.
resource "cloudflare_zero_trust_device_default_profile" "lock" {
  count = var.lock_client ? 1 : 0

  account_id       = var.cloudflare_account_id
  switch_locked    = true
  allowed_to_leave = false
}
