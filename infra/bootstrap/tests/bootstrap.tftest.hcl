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

mock_provider "cloudflare" {
  mock_data "cloudflare_zone" {
    defaults = {
      id = "0123456789abcdef0123456789abcdef"
    }
  }
}

variables {
  budget_email = "budget@example.com"
  origin_secrets = {
    dev  = "test-only-not-a-real-secret-dev-0123456789abcdef"
    prod = "test-only-not-a-real-secret-prod-0123456789abcdef"
  }
}

run "origin_secret_header_is_added_for_every_hostname" {
  command = plan

  assert {
    condition     = cloudflare_ruleset.origin_secret.phase == "http_request_late_transform" && cloudflare_ruleset.origin_secret.kind == "zone"
    error_message = "The header transform must be a zone ruleset in http_request_late_transform."
  }

  assert {
    condition     = length(cloudflare_ruleset.origin_secret.rules) == 2
    error_message = "There must be one origin-secret rule per environment (dev and prod)."
  }

  assert {
    condition     = cloudflare_ruleset.origin_secret.rules[0].expression == "(http.host eq \"dev.craigdevjohnson.com\")" && cloudflare_ruleset.origin_secret.rules[1].expression == "(http.host eq \"next.craigdevjohnson.com\")"
    error_message = "Each rule must add the header on requests to its own environment's hostname only."
  }

  assert {
    condition     = alltrue([for rule in cloudflare_ruleset.origin_secret.rules : rule.action_parameters.headers["X-Origin-Secret"].operation == "set"])
    error_message = "Every rule must set X-Origin-Secret."
  }
}

run "browser_integrity_check_is_off_on_every_hostnames_calendar_links" {
  command = plan

  assert {
    condition     = cloudflare_ruleset.calendar_links.phase == "http_config_settings" && cloudflare_ruleset.calendar_links.kind == "zone"
    error_message = "The calendar link settings must be a zone configuration ruleset (http_config_settings)."
  }

  assert {
    condition     = length(cloudflare_ruleset.calendar_links.rules) == 2 && alltrue([for rule in cloudflare_ruleset.calendar_links.rules : rule.action == "set_config" && rule.action_parameters.bic == false])
    error_message = "One rule per environment must turn Browser Integrity Check off (ADR 0005)."
  }

  assert {
    condition     = cloudflare_ruleset.calendar_links.rules[1].expression == "(http.host eq \"next.craigdevjohnson.com\" and starts_with(http.request.uri.path, \"/soccer/link/\"))"
    error_message = "Each rule must match only its hostname's /soccer/link/ addresses."
  }
}

run "bot_fight_mode_is_off_and_members_sign_in_by_one_time_code" {
  command = plan

  assert {
    condition     = cloudflare_bot_management.zone.fight_mode == false
    error_message = "Bot Fight Mode must be off for the zone (ADR 0005)."
  }

  assert {
    condition     = cloudflare_zero_trust_access_identity_provider.otp.type == "onetimepin"
    error_message = "The one identity provider must be One-time PIN."
  }
}

run "roles_trust_exactly_one_github_subject_each" {
  command = plan

  assert {
    condition     = jsondecode(aws_iam_role.pr_planner.assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson@42454849/the-lobby@1408231725:pull_request"
    error_message = "The planner must trust only pull requests of CraigDevJohnson/the-lobby."
  }

  assert {
    condition     = jsondecode(aws_iam_role.deployer["dev"].assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson@42454849/the-lobby@1408231725:environment:dev"
    error_message = "The dev deployer must trust only the dev GitHub environment."
  }

  assert {
    condition     = jsondecode(aws_iam_role.deployer["prod"].assume_role_policy).Statement[0].Condition.StringEquals["token.actions.githubusercontent.com:sub"] == "repo:CraigDevJohnson@42454849/the-lobby@1408231725:environment:production"
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
      statement.Resource != ["*"]
      || statement.Action == ["acm:RequestCertificate"]
      || (statement.Sid == "LogDelivery" && alltrue([
        for action in statement.Action :
        contains([
          "logs:CreateLogDelivery", "logs:GetLogDelivery", "logs:UpdateLogDelivery",
          "logs:DeleteLogDelivery", "logs:ListLogDeliveries", "logs:PutResourcePolicy",
          "logs:DescribeResourcePolicies", "logs:DescribeLogGroups",
        ], action)
      ]))
    ])
    error_message = "Only acm:RequestCertificate and the CloudWatch Logs delivery actions may be granted on every resource."
  }

  assert {
    condition = alltrue([
      for env in keys(aws_iam_role_policy.deployer_apply) :
      alltrue([
        for action in ["logs:CreateLogDelivery", "logs:PutResourcePolicy", "logs:DescribeResourcePolicies", "logs:DescribeLogGroups"] :
        contains(flatten([
          for statement in jsondecode(aws_iam_role_policy.deployer_apply[env].policy).Statement :
          statement.Action if statement.Effect == "Allow"
        ]), action)
      ])
    ])
    error_message = "Each deployer must carry the four actions AWS needs to set up a CloudWatch Logs delivery the first time, or creating an API stage with access logging fails."
  }

  assert {
    condition = alltrue([
      for env in keys(aws_iam_role_policy.deployer_apply) :
      anytrue([
        for statement in jsondecode(aws_iam_role_policy.deployer_apply[env].policy).Statement :
        statement.Sid == "LogDelivery"
        && try(statement.Condition.StringEquals["aws:RequestedRegion"], "") == var.aws_region
      ])
    ])
    error_message = "The log-delivery grant on every resource must be limited to the deployment region."
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
    condition = alltrue([
      for env, policy in aws_iam_policy.site_boundary :
      policy.name == "the-lobby-site-boundary-${env}"
      && !strcontains(policy.policy, "\"*\"")
      && strcontains(policy.policy, "site-${env}-")
      && !strcontains(policy.policy, env == "dev" ? "site-prod-" : "site-dev-")
    ])
    error_message = "Each environment's boundary must name only that environment's resources and never allow everything."
  }
}
