# Offline checks on the bootstrap root. The provider is mocked, so no
# account is touched and no credentials are needed.

# Policies are built with jsonencode, so they are fully known at plan
# time. The only computed values they use are the OIDC provider ARN and
# the bucket ARN, which get realistic defaults here.
mock_provider "aws" {
  mock_data "aws_iam_openid_connect_provider" {
    defaults = {
      arn = "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
    }
  }

  mock_resource "aws_s3_bucket" {
    defaults = {
      arn = "arn:aws:s3:::mock-state-bucket"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/mock"
    }
  }
}

variables {
  budget_email = "budget@example.com"
}

run "roles_trust_exactly_one_github_subject_each" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role.pr_planner.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson/the-lobby:pull_request"
    error_message = "The planner must trust only pull requests of CraigDevJohnson/the-lobby."
  }

  assert {
    condition     = jsondecode(aws_iam_role.deployer["dev"].assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson/the-lobby:environment:dev"
    error_message = "The dev deployer must trust only the dev GitHub environment."
  }

  assert {
    condition     = jsondecode(aws_iam_role.deployer["prod"].assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson/the-lobby:environment:production"
    error_message = "The prod deployer must trust only the production GitHub environment."
  }

  assert {
    condition = alltrue([
      for role in [aws_iam_role.pr_planner, aws_iam_role.deployer["dev"], aws_iam_role.deployer["prod"]] :
      jsondecode(role.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:aud"] == "sts.amazonaws.com"
    ])
    error_message = "Every role must require aud = sts.amazonaws.com."
  }

  assert {
    condition = alltrue([
      for role in [aws_iam_role.pr_planner, aws_iam_role.deployer["dev"], aws_iam_role.deployer["prod"]] :
      length(jsondecode(role.assume_role_policy).Statement) == 1
      && jsondecode(role.assume_role_policy).Statement[0].Action == "sts:AssumeRoleWithWebIdentity"
      && !strcontains(role.assume_role_policy, "StringLike")
      && role.max_session_duration == 3600
    ])
    error_message = "Every role must have one web-identity statement, exact-match conditions and a one-hour session."
  }
}

run "planner_can_only_read" {
  command = plan

  assert {
    condition = alltrue([
      for word in ["Put", "Create", "Delete", "Update"] :
      !strcontains(aws_iam_role_policy.pr_planner_state.policy, word)
    ])
    error_message = "The planner's inline policy must contain no write action."
  }

  assert {
    condition     = strcontains(aws_iam_role_policy.pr_planner_state.policy, "/the-lobby/*")
    error_message = "The planner must be able to read state under the-lobby/."
  }

  assert {
    condition     = aws_iam_role_policy_attachment.pr_planner_read_only.policy_arn == "arn:aws:iam::aws:policy/ReadOnlyAccess"
    error_message = "The planner must carry ReadOnlyAccess and nothing broader."
  }
}

run "deployers_stay_inside_their_own_environment" {
  command = plan

  assert {
    condition = (
      strcontains(aws_iam_role_policy.deployer_apply["dev"].policy, "site-dev-")
      && !strcontains(aws_iam_role_policy.deployer_apply["dev"].policy, "site-prod-")
      && strcontains(aws_iam_role_policy.deployer_apply["dev"].policy, "/the-lobby/dev/*")
      && !strcontains(aws_iam_role_policy.deployer_apply["dev"].policy, "/the-lobby/prod/")
    )
    error_message = "The dev deployer must reference only site-dev-* resources and the-lobby/dev/* state."
  }

  assert {
    condition = (
      strcontains(aws_iam_role_policy.deployer_apply["prod"].policy, "site-prod-")
      && !strcontains(aws_iam_role_policy.deployer_apply["prod"].policy, "site-dev-")
      && strcontains(aws_iam_role_policy.deployer_apply["prod"].policy, "/the-lobby/prod/*")
      && !strcontains(aws_iam_role_policy.deployer_apply["prod"].policy, "/the-lobby/dev/")
    )
    error_message = "The prod deployer must reference only site-prod-* resources and the-lobby/prod/* state."
  }

  assert {
    condition = anytrue([
      for statement in jsondecode(aws_iam_role_policy.deployer_apply["dev"].policy).Statement :
      try(statement.Condition.StringEquals["iam:PassedToService"], "") == "lambda.amazonaws.com"
      && statement.Action == ["iam:PassRole"]
    ])
    error_message = "PassRole must be limited to Lambda."
  }

  assert {
    condition = alltrue([
      for statement in jsondecode(aws_iam_role_policy.deployer_apply["dev"].policy).Statement :
      statement.Resource != ["*"] || statement.Action == ["acm:RequestCertificate"]
    ])
    error_message = "Only acm:RequestCertificate may be granted on every resource."
  }
}

run "budget_is_five_dollars_on_the_site_tag" {
  command = plan

  assert {
    condition     = aws_budgets_budget.monthly.limit_amount == "5" && aws_budgets_budget.monthly.limit_unit == "USD"
    error_message = "The budget limit must be 5 USD."
  }

  assert {
    condition     = aws_budgets_budget.monthly.budget_type == "COST" && aws_budgets_budget.monthly.time_unit == "MONTHLY"
    error_message = "The budget must be a monthly cost budget."
  }

  assert {
    condition = anytrue([
      for filter in aws_budgets_budget.monthly.cost_filter :
      filter.name == "TagKeyValue" && contains(filter.values, "user:Site$craigdevjohnson.com")
    ])
    error_message = "The budget must filter on the Site = craigdevjohnson.com tag."
  }

  assert {
    condition     = toset([for n in aws_budgets_budget.monthly.notification : n.notification_type]) == toset(["ACTUAL", "FORECASTED"])
    error_message = "The budget must alert on both actual and forecasted spend."
  }

  assert {
    condition = alltrue([
      for n in aws_budgets_budget.monthly.notification :
      n.threshold == 100 && n.threshold_type == "PERCENTAGE" && n.comparison_operator == "GREATER_THAN" && n.subscriber_email_addresses == toset([var.budget_email])
    ])
    error_message = "Both alerts must fire above 100% and go to budget_email."
  }

}

run "deployers_can_only_create_bounded_roles" {
  command = plan

  assert {
    condition = alltrue([
      for env in keys(aws_iam_role_policy.deployer_apply) :
      strcontains(aws_iam_role_policy.deployer_apply[env].policy, "iam:PermissionsBoundary")
      && strcontains(aws_iam_role_policy.deployer_apply[env].policy, "iam:DeleteRolePermissionsBoundary")
    ])
    error_message = "Each deployer must create roles only with the site boundary, and must be denied removing it."
  }

  assert {
    condition     = aws_iam_policy.site_boundary.name == "the-lobby-site-boundary" && !strcontains(aws_iam_policy.site_boundary.policy, "\"*\"")
    error_message = "The site boundary must exist and never allow every action or every resource."
  }
}
