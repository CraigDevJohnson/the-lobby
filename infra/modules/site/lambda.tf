# The site's Go server. One function, one published version, one alias.

resource "aws_cloudwatch_log_group" "server" {
  name              = "/aws/lambda/${local.name}-server"
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "server" {
  name                 = "${local.name}-server"
  assume_role_policy   = data.aws_iam_policy_document.lambda_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

# The account and region this environment deploys into, which a Tool
# backend must share: the site permissions boundary grants calls nowhere
# else, and the Go server signs requests for its own region.
data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

# Least privilege: write to its own log group, read/write the Sign-in
# sessions table, and call the connected Tool backends. Nothing else.
data "aws_iam_policy_document" "server" {
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.server.arn}:*"]
  }

  statement {
    sid = "Sessions"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query",
    ]
    resources = [aws_dynamodb_table.sessions.arn]
  }

  # Calling the connected Tool backends' Function URLs with signed requests
  # (ADR 0006), within what the site permissions boundary allows. AWS needs
  # lambda:InvokeFunction alongside InvokeFunctionUrl for a Function URL,
  # and the condition keeps it to calls through the URL: without it this
  # same-account grant would also let the site invoke the backend directly.
  dynamic "statement" {
    for_each = length(local.backend_function_arns) > 0 ? { backends = local.backend_function_arns } : {}

    content {
      sid       = "CallToolBackendUrls"
      actions   = ["lambda:InvokeFunctionUrl"]
      resources = statement.value
    }
  }

  dynamic "statement" {
    for_each = length(local.backend_function_arns) > 0 ? { backends = local.backend_function_arns } : {}

    content {
      sid       = "CallToolBackendsThroughUrlsOnly"
      actions   = ["lambda:InvokeFunction"]
      resources = statement.value

      condition {
        test     = "Bool"
        variable = "lambda:InvokedViaFunctionUrl"
        values   = ["true"]
      }
    }
  }
}

resource "aws_iam_role_policy" "server" {
  name   = "${local.name}-server"
  role   = aws_iam_role.server.id
  policy = data.aws_iam_policy_document.server.json
}

resource "aws_lambda_function" "server" {
  function_name = "${local.name}-server"
  role          = aws_iam_role.server.arn

  filename         = var.zip_path
  source_code_hash = filebase64sha256(var.zip_path)
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]

  memory_size                    = 256
  timeout                        = 10
  reserved_concurrent_executions = var.reserved_concurrent_executions
  publish                        = true

  # The Tool backends' addresses and signing overrides join the site's own
  # settings when a backend is connected (locals.tf).
  environment {
    variables = merge({
      SITE_ENV           = var.environment
      SITE_BASE_URL      = local.base_url
      ACCESS_TEAM_DOMAIN = var.access_team_domain
      ACCESS_AUD         = cloudflare_zero_trust_access_application.signin.aud
      ORIGIN_SECRET      = var.origin_secret
      SESSIONS_TABLE     = aws_dynamodb_table.sessions.name
    }, local.backend_environment)
  }

  depends_on = [
    aws_cloudwatch_log_group.server,
    aws_iam_role_policy.server,
  ]
}

resource "aws_lambda_alias" "live" {
  name             = "live"
  function_name    = aws_lambda_function.server.function_name
  function_version = aws_lambda_function.server.version
}
