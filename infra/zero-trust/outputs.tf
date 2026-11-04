# What the /admin page and the profiles need (#41, #42): Craig puts the two
# endpoints and the certificate into the GitHub variables KIDS_DOH_URL,
# ADULTS_DOH_URL and GATEWAY_ROOT_CA_PEM and each environment's
# secrets.auto.tfvars (docs/pipeline.md).

output "kids_doh_url" {
  description = "The Kids location's DNS over HTTPS endpoint, KIDS_DOH_URL."
  value       = "https://${cloudflare_zero_trust_dns_location.location["kids"].doh_subdomain}.cloudflare-gateway.com/dns-query"
}

output "adults_doh_url" {
  description = "The Adults location's DNS over HTTPS endpoint, ADULTS_DOH_URL."
  value       = "https://${cloudflare_zero_trust_dns_location.location["adults"].doh_subdomain}.cloudflare-gateway.com/dns-query"
}

output "gateway_root_ca_pem" {
  description = "The Gateway root certificate as PEM, GATEWAY_ROOT_CA_PEM: what the adult profile installs."
  value       = cloudflare_zero_trust_gateway_certificate.gateway.certificate
}

output "gateway_certificate_status" {
  description = "Whether the certificate is deployed at the edge (binding_status available) and used by Gateway (in_use)."
  value = {
    id             = cloudflare_zero_trust_gateway_certificate.gateway.id
    binding_status = cloudflare_zero_trust_gateway_certificate.gateway.binding_status
    in_use         = cloudflare_zero_trust_gateway_certificate.gateway.in_use
    expires_on     = cloudflare_zero_trust_gateway_certificate.gateway.expires_on
  }
}

output "location_ids" {
  description = "The two DNS locations' ids, for reading the Gateway log by location."
  value       = { for key, location in cloudflare_zero_trust_dns_location.location : key => location.id }
}
