# The daily iCloud update (ADR 0015): once a day EventBridge starts the
# site's Lambda, which adds new soccer games to the iCloud calendar of each
# Member who turned the daily update on.

# The rule's clock is UTC: 16:00 is 9 or 10 in the morning in Boise, an hour
# at which a shared calendar telling its people about each game added wakes
# nobody. The name stays site-<environment>-*, the only rules the deploy role
# may manage (infra/bootstrap).
resource "aws_cloudwatch_event_rule" "icloud_daily" {
  name                = "${local.name}-icloud-daily"
  description         = "Once a day, starts the site's daily iCloud update, which adds new soccer games to Members' iCloud calendars."
  schedule_expression = "cron(0 16 * * ? *)"
}

# The constant input is how the server tells this event from an HTTP request
# (cmd/site).
resource "aws_cloudwatch_event_target" "icloud_daily" {
  rule  = aws_cloudwatch_event_rule.icloud_daily.name
  arn   = aws_lambda_alias.live.arn
  input = jsonencode({ task = "icloud-daily" })
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
