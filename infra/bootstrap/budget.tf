# Cost control: a $5 monthly ceiling on everything tagged
# Site = craigdevjohnson.com (docs/baseline.md, "How it is run").
#
# Budgets and Cost Explorer are global services; the provider sends these
# calls to us-east-1 on its own, so no second provider is needed.

# Budgets can only filter on a tag that has been activated as a cost
# allocation tag. This account is a member of an AWS Organization, so the
# activation is done once from the management account, not here:
#   aws ce update-cost-allocation-tags-status --profile mgmt-admin \
#     --cost-allocation-tags-status TagKey=Site,Status=Active
# AWS lets a tag be activated only after it has appeared in billing data,
# up to 24 hours after the first tagged resource exists. Until then the
# budget exists but matches no cost.

resource "aws_budgets_budget" "monthly" {
  name         = "the-lobby-monthly"
  budget_type  = "COST"
  limit_amount = "5"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  # Budgets write a tag filter as "user:<Key>$<Value>".
  cost_filter {
    name   = "TagKeyValue"
    values = [format("user:%s$%s", local.site_tag_key, local.site_tag_value)]
  }

  # Spent more than the limit this month.
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.budget_email]
  }

  # On course to spend more than the limit by month end.
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.budget_email]
  }

  # The filter is meaningless until the tag is active, so if the tag
  # resource has to wait a day, the budget waits with it.
}
