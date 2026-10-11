# The Deploy workflow's production job reads hostname: next.craigdevjohnson.com
# until the switch, then craigdevjohnson.com (docs/pipeline.md).
output "hostname" {
  description = "The hostname this environment serves."
  value       = module.site.hostname
}

output "api_endpoint" {
  description = "API Gateway's own URL."
  value       = module.site.api_endpoint
}

output "custom_domain_target" {
  description = "The API Gateway domain the Cloudflare CNAME points at."
  value       = module.site.custom_domain_target
}

output "access_aud" {
  description = "Access application AUD tag."
  value       = module.site.access_aud
}

output "lambda_function_name" {
  value = module.site.lambda_function_name
}

output "sessions_table_name" {
  value = module.site.sessions_table_name
}
