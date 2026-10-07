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
  name               = "${local.name}-server"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

# Least privilege: write to its own log group, and read/write the Sign-in
# sessions table. Nothing else.
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

  environment {
    variables = {
      SITE_ENV           = var.environment
      SITE_BASE_URL      = local.base_url
      ACCESS_TEAM_DOMAIN = var.access_team_domain
      ACCESS_AUD         = cloudflare_zero_trust_access_application.signin.aud
      ORIGIN_SECRET      = var.origin_secret
      SESSIONS_TABLE     = aws_dynamodb_table.sessions.name
    }
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
