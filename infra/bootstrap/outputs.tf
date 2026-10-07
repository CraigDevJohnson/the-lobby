output "state_bucket_name" {
  description = "Bucket name to use in every root's S3 backend."
  value       = aws_s3_bucket.state.bucket
}

output "state_bucket_arn" {
  value = aws_s3_bucket.state.arn
}

output "pr_planner_role_arn" {
  description = "Role the pull-request workflow assumes for read-only plans."
  value       = aws_iam_role.pr_planner.arn
}

output "dev_deployer_role_arn" {
  description = "Role the dev deploy job assumes."
  value       = aws_iam_role.deployer["dev"].arn
}

output "prod_deployer_role_arn" {
  description = "Role the production deploy job assumes."
  value       = aws_iam_role.deployer["prod"].arn
}

output "budget_name" {
  description = "The $5 monthly budget on this site's tagged resources."
  value       = aws_budgets_budget.monthly.name
}

output "site_permissions_boundary_arns" {
  description = "Boundary every site Lambda role must carry, per environment; the site module takes it as permissions_boundary_arn."
  value       = { for env, policy in aws_iam_policy.site_boundary : env => policy.arn }
}
