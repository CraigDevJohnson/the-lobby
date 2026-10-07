# What GitHub Actions may do in this account.
#
# GitHub signs a short-lived OpenID Connect token for each workflow run.
# AWS trusts that token through the provider below and hands out temporary
# credentials for one of three roles, so no AWS key is stored in GitHub
# (docs/baseline.md, "How it is run").

# The provider already exists in the account; another repo's stack owns
# it. IAM allows one per URL, so this root looks it up and never creates it.
data "aws_iam_openid_connect_provider" "github" {
  url = "https://${local.github_oidc_host}"
}

locals {
  # Which workflow run may assume which role, by the token's sub claim.
  # The pull_request form matches a pull request in this repository; the
  # environment form matches a job that targets that GitHub environment.
  github_subjects = merge(
    { pr_planner = "repo:${var.github_repository}:pull_request" },
    {
      for env, settings in local.environments :
      "deployer_${env}" => "repo:${var.github_repository}:environment:${settings.github_environment}"
    },
  )

  # Trust policies: StringEquals on both aud and sub, so only the exact
  # subject above gets in. No wildcards.
  github_trust_policies = {
    for role, subject in local.github_subjects : role => jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Sid       = "GitHubActionsOidc"
        Effect    = "Allow"
        Principal = { Federated = data.aws_iam_openid_connect_provider.github.arn }
        Action    = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${local.github_oidc_host}:aud" = "sts.amazonaws.com"
            "${local.github_oidc_host}:sub" = subject
          }
        }
      }]
    })
  }

  read_only_policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

# ---------------------------------------------------------------------------
# Pull-request planner: `tofu plan` on pull requests. Reads everything,
# writes nothing. Plans run with -lock=false, so it never touches the lock
# file and needs no PutObject.

resource "aws_iam_role" "pr_planner" {
  name                 = "the-lobby-pr-planner"
  description          = "GitHub Actions on pull requests of ${var.github_repository}: read-only plans."
  assume_role_policy   = local.github_trust_policies["pr_planner"]
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "pr_planner_read_only" {
  role       = aws_iam_role.pr_planner.name
  policy_arn = local.read_only_policy_arn
}

# ReadOnlyAccess already includes s3:Get* and s3:List*; this spells the
# state bucket out so the planner's one real need does not hang on a
# managed policy AWS may change.
resource "aws_iam_role_policy" "pr_planner_state" {
  name = "state-read"
  role = aws_iam_role.pr_planner.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ListStateBucket"
        Effect   = "Allow"
        Action   = ["s3:ListBucket"]
        Resource = [aws_s3_bucket.state.arn]
      },
      {
        Sid      = "ReadState"
        Effect   = "Allow"
        Action   = ["s3:GetObject"]
        Resource = ["${aws_s3_bucket.state.arn}/the-lobby/*"]
      },
    ]
  })
}

# ---------------------------------------------------------------------------
# Deployers: one per environment, from the map in locals.tf. ReadOnlyAccess
# covers the refresh and the plan; the inline policy adds the writes that
# applying modules/site needs, each pinned to that environment's names.
# Reads (Describe*, Get*, List*) are left to ReadOnlyAccess on purpose: the
# provider adds read calls between versions, the writes are what matter.

resource "aws_iam_role" "deployer" {
  for_each = local.environments

  name                 = "the-lobby-${each.key}-deployer"
  description          = "GitHub Actions deploys of ${var.github_repository} to ${each.key} (GitHub environment ${each.value.github_environment})."
  assume_role_policy   = local.github_trust_policies["deployer_${each.key}"]
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "deployer_read_only" {
  for_each = local.environments

  role       = aws_iam_role.deployer[each.key].name
  policy_arn = local.read_only_policy_arn
}

resource "aws_iam_role_policy" "deployer_apply" {
  for_each = local.environments

  name = "apply-${each.key}"
  role = aws_iam_role.deployer[each.key].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # OpenTofu state: this environment's key and its lock file
      # (<key>.tflock), nothing under another environment's prefix.
      {
        Sid      = "StateList"
        Effect   = "Allow"
        Action   = ["s3:ListBucket"]
        Resource = [aws_s3_bucket.state.arn]
      },
      {
        Sid      = "StateReadWrite"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
        Resource = ["${aws_s3_bucket.state.arn}/${each.value.state_prefix}/*"]
      },

      # Lambda: the function, its published versions, the live alias, the
      # invoke permission for API Gateway and the concurrency cap.
      {
        Sid    = "Lambda"
        Effect = "Allow"
        Action = [
          "lambda:CreateFunction",
          "lambda:UpdateFunctionCode",
          "lambda:UpdateFunctionConfiguration",
          "lambda:DeleteFunction",
          "lambda:PublishVersion",
          "lambda:CreateAlias",
          "lambda:UpdateAlias",
          "lambda:DeleteAlias",
          "lambda:AddPermission",
          "lambda:RemovePermission",
          "lambda:PutFunctionConcurrency",
          "lambda:DeleteFunctionConcurrency",
          "lambda:TagResource",
          "lambda:UntagResource",
        ]
        Resource = [
          "arn:aws:lambda:${var.aws_region}:${var.aws_account_id}:function:${each.value.name_prefix}-*",
          "arn:aws:lambda:${var.aws_region}:${var.aws_account_id}:function:${each.value.name_prefix}-*:*",
        ]
      },

      # CloudWatch Logs: the Lambda's and the API's log groups.
      {
        Sid    = "Logs"
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:DeleteLogGroup",
          "logs:PutRetentionPolicy",
          "logs:DeleteRetentionPolicy",
          "logs:TagResource",
          "logs:UntagResource",
        ]
        Resource = [
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/lambda/${each.value.name_prefix}-*",
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/apigateway/${each.value.name_prefix}-*",
        ]
      },

      # DynamoDB: the Sign-in sessions table, its TTL and its backup setting.
      {
        Sid    = "DynamoDB"
        Effect = "Allow"
        Action = [
          "dynamodb:CreateTable",
          "dynamodb:UpdateTable",
          "dynamodb:DeleteTable",
          "dynamodb:UpdateTimeToLive",
          "dynamodb:UpdateContinuousBackups",
          "dynamodb:TagResource",
          "dynamodb:UntagResource",
        ]
        Resource = ["arn:aws:dynamodb:${var.aws_region}:${var.aws_account_id}:table/${each.value.name_prefix}-*"]
      },

      # ACM: the hostname's certificate. A certificate has no name of its
      # own, so RequestCertificate cannot be narrowed; the rest is limited
      # to certificates in this account and region.
      {
        Sid      = "AcmRequest"
        Effect   = "Allow"
        Action   = ["acm:RequestCertificate"]
        Resource = ["*"]
      },
      {
        Sid    = "AcmManage"
        Effect = "Allow"
        Action = [
          "acm:DeleteCertificate",
          "acm:AddTagsToCertificate",
          "acm:RemoveTagsFromCertificate",
        ]
        Resource = ["arn:aws:acm:${var.aws_region}:${var.aws_account_id}:certificate/*"]
      },

      # IAM: the Lambda's execution role and its inline policy, and handing
      # that role to Lambda and to nothing else. A role may only be created
      # or given a policy while it carries the site permissions boundary, so
      # a deploy can never mint a role with more power than the boundary
      # allows, and the boundary cannot be taken off again.
      {
        Sid      = "IamRolesBounded"
        Effect   = "Allow"
        Action   = ["iam:CreateRole", "iam:PutRolePolicy"]
        Resource = ["arn:aws:iam::${var.aws_account_id}:role/${each.value.name_prefix}-*"]
        Condition = {
          StringEquals = { "iam:PermissionsBoundary" = aws_iam_policy.site_boundary.arn }
        }
      },
      {
        Sid    = "IamRoles"
        Effect = "Allow"
        Action = [
          "iam:UpdateRole",
          "iam:UpdateRoleDescription",
          "iam:UpdateAssumeRolePolicy",
          "iam:DeleteRole",
          "iam:DeleteRolePolicy",
          "iam:TagRole",
          "iam:UntagRole",
        ]
        Resource = ["arn:aws:iam::${var.aws_account_id}:role/${each.value.name_prefix}-*"]
      },
      {
        Sid      = "IamBoundaryStays"
        Effect   = "Deny"
        Action   = ["iam:DeleteRolePermissionsBoundary", "iam:PutRolePermissionsBoundary"]
        Resource = ["arn:aws:iam::${var.aws_account_id}:role/${each.value.name_prefix}-*"]
      },
      {
        Sid      = "IamPassRoleToLambda"
        Effect   = "Allow"
        Action   = ["iam:PassRole"]
        Resource = ["arn:aws:iam::${var.aws_account_id}:role/${each.value.name_prefix}-*"]
        Condition = {
          StringEquals = { "iam:PassedToService" = "lambda.amazonaws.com" }
        }
      },

      # API Gateway: broader than the rest on purpose. Its ARNs carry ids,
      # not names (/apis/<id>, /domainnames/<hostname>), so there is no
      # site-dev-* pattern to pin to. apigateway:* here means GET, POST,
      # PUT, PATCH and DELETE on APIs, custom domains and their tags in this
      # region; it reaches nothing outside API Gateway.
      {
        Sid    = "ApiGateway"
        Effect = "Allow"
        Action = ["apigateway:*"]
        Resource = [
          "arn:aws:apigateway:${var.aws_region}::/apis*",
          "arn:aws:apigateway:${var.aws_region}::/domainnames*",
          "arn:aws:apigateway:${var.aws_region}::/tags/*",
        ]
      },
    ]
  })
}

# The most any site Lambda role may ever do, whatever its own policy says.
# Deploys can only create roles that carry this boundary (see above).
resource "aws_iam_policy" "site_boundary" {
  name        = "the-lobby-site-boundary"
  description = "Permissions boundary for the site's Lambda roles (site-<env>-*)."

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = ["arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/lambda/site-*:*"]
      },
      {
        Sid    = "SiteTables"
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem",
        ]
        Resource = ["arn:aws:dynamodb:${var.aws_region}:${var.aws_account_id}:table/site-*"]
      },
      {
        Sid      = "SiteParameters"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = ["arn:aws:ssm:${var.aws_region}:${var.aws_account_id}:parameter/the-lobby/*"]
      },
      # Calling Tool backends with signed requests (ADR 0006).
      {
        Sid      = "CallToolBackends"
        Effect   = "Allow"
        Action   = ["lambda:InvokeFunctionUrl", "lambda:InvokeFunction"]
        Resource = ["arn:aws:lambda:${var.aws_region}:${var.aws_account_id}:function:*"]
      },
    ]
  })
}
