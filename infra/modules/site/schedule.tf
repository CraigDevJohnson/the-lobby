# Both daily updates run at 16:00 UTC. Each target starts its own invocation
# so a slow provider does not use up the other provider's Lambda deadline.
# Keep the existing rule name for deployed schedules and the iCloud wizard.
resource "aws_cloudwatch_event_rule" "icloud_daily" {
  name                = "${local.name}-icloud-daily"
  description         = "Updates opted-in Members' Google and iCloud soccer calendars daily."
  schedule_expression = "cron(0 16 * * ? *)"
}

# The constant input is how the server tells this event from an HTTP request
# (cmd/site).
resource "aws_cloudwatch_event_target" "icloud_daily" {
  rule  = aws_cloudwatch_event_rule.icloud_daily.name
  arn   = aws_lambda_alias.live.arn
  input = jsonencode({ task = "icloud-daily" })
}

resource "aws_cloudwatch_event_target" "google_daily" {
  rule      = aws_cloudwatch_event_rule.icloud_daily.name
  target_id = "google-daily"
  arn       = aws_lambda_alias.live.arn
  input     = jsonencode({ task = "google-daily" })
}

# A rule needs no role to start a Lambda, only this permission, which lets
# in this one rule and nothing else of EventBridge's.
resource "aws_lambda_permission" "icloud_daily" {
  statement_id  = "AllowDailyICloudUpdate"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.server.function_name
  qualifier     = aws_lambda_alias.live.name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.icloud_daily.arn
}
