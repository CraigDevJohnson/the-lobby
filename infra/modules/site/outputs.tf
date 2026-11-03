output "hostname" {
  value = var.hostname
}

output "api_endpoint" {
  description = "API Gateway's own URL, for checking the Lambda without Cloudflare in front."
  value       = aws_apigatewayv2_api.http.api_endpoint
}

output "custom_domain_target" {
  description = "The API Gateway domain that the Cloudflare CNAME points at."
  value       = aws_apigatewayv2_domain_name.site.domain_name_configuration[0].target_domain_name
}

output "access_aud" {
  description = "The Access application's audience (AUD) tag; the Go server checks it in the cf-access JWT."
  value       = cloudflare_zero_trust_access_application.signin.aud
}

output "access_admin_aud" {
  description = "The family devices page's Access application audience (ADR 0010), which the Go server checks for /admin; empty while admin_emails is empty and there is no application."
  value       = local.admin_aud
}

output "lambda_function_name" {
  value = aws_lambda_function.server.function_name
}

output "lambda_alias_arn" {
  value = aws_lambda_alias.live.arn
}

output "sessions_table_name" {
  value = aws_dynamodb_table.sessions.name
}
