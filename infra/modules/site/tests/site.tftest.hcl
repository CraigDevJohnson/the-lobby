# Offline checks on the module. Both providers are mocked, so no account is
# touched and no credentials are needed.

# The providers validate some values at plan time (ARNs, hex ids, policy
# JSON), so those computed attributes get realistic defaults.
mock_provider "aws" {
  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}"
    }
  }

  mock_data "aws_caller_identity" {
    defaults = {
      account_id = "123456789012"
    }
  }

  mock_data "aws_region" {
    defaults = {
      region = "us-west-2"
      name   = "us-west-2"
    }
  }

  mock_resource "aws_iam_role" {
    defaults = {
      arn = "arn:aws:iam::123456789012:role/mock"
    }
  }

  mock_resource "aws_cloudwatch_log_group" {
    defaults = {
      arn = "arn:aws:logs:us-west-2:123456789012:log-group:mock"
    }
  }

  mock_resource "aws_dynamodb_table" {
    defaults = {
      arn = "arn:aws:dynamodb:us-west-2:123456789012:table/mock"
    }
  }

  mock_resource "aws_lambda_function" {
    defaults = {
      arn        = "arn:aws:lambda:us-west-2:123456789012:function:mock"
      invoke_arn = "arn:aws:apigateway:us-west-2:lambda:path/2015-03-31/functions/arn:aws:lambda:us-west-2:123456789012:function:mock/invocations"
      version    = "1"
    }
  }

  mock_resource "aws_lambda_alias" {
    defaults = {
      arn        = "arn:aws:lambda:us-west-2:123456789012:function:mock:live"
      invoke_arn = "arn:aws:apigateway:us-west-2:lambda:path/2015-03-31/functions/arn:aws:lambda:us-west-2:123456789012:function:mock:live/invocations"
    }
  }

  mock_resource "aws_apigatewayv2_api" {
    defaults = {
      execution_arn = "arn:aws:execute-api:us-west-2:123456789012:mockapi"
      api_endpoint  = "https://mockapi.execute-api.us-west-2.amazonaws.com"
    }
  }

  mock_resource "aws_acm_certificate" {
    defaults = {
      arn = "arn:aws:acm:us-west-2:123456789012:certificate/mock"
      domain_validation_options = [{
        domain_name           = "dev.craigdevjohnson.com"
        resource_record_name  = "_mock.dev.craigdevjohnson.com."
        resource_record_type  = "CNAME"
        resource_record_value = "_mock.acm-validations.aws."
      }]
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
  environment           = "dev"
  hostname              = "dev.craigdevjohnson.com"
  cloudflare_account_id = "0123456789abcdef0123456789abcdef"
  cloudflare_zone_name  = "craigdevjohnson.com"
  access_team_domain    = "https://example.cloudflareaccess.com"
  allowed_emails        = ["member@example.com", "another@example.com"]
  origin_secret         = "test-only-not-a-real-secret-0123456789abcdef"
  zip_path              = "tests/fixtures/site.zip"
}

run "access_app_guards_only_the_signin_path" {
  command = plan

  assert {
    condition     = cloudflare_zero_trust_access_application.signin.domain == "dev.craigdevjohnson.com/signin"
    error_message = "The Access application must be scoped to <hostname>/signin, nothing wider."
  }

  assert {
    condition     = cloudflare_zero_trust_access_application.signin.session_duration == "24h"
    error_message = "Access session duration must be 24h."
  }

  assert {
    condition     = length(cloudflare_zero_trust_access_application.signin.policies) == 1 && cloudflare_zero_trust_access_application.signin.policies[0].decision == "allow"
    error_message = "The Access application must carry exactly one allow policy."
  }

  assert {
    condition     = length(cloudflare_zero_trust_access_application.signin.policies[0].include) == length(var.allowed_emails)
    error_message = "Every allowed email must appear in the Access policy."
  }
}

run "bot_fight_mode_is_off" {
  command = plan

  assert {
    condition     = cloudflare_bot_management.zone.fight_mode == false
    error_message = "Bot Fight Mode must be off for the zone (ADR 0005)."
  }
}

run "lambda_has_a_concurrency_cap" {
  command = plan

  assert {
    condition     = aws_lambda_function.server.reserved_concurrent_executions == 5
    error_message = "The Lambda must carry the default concurrency cap of 5 (ADR 0002)."
  }

  assert {
    condition     = aws_lambda_function.server.runtime == "provided.al2023" && contains(aws_lambda_function.server.architectures, "arm64")
    error_message = "The Lambda must run the Go binary on provided.al2023, arm64."
  }

  assert {
    condition     = aws_lambda_function.server.environment[0].variables["SITE_BASE_URL"] == "https://dev.craigdevjohnson.com"
    error_message = "SITE_BASE_URL must be https://<hostname>."
  }
}

run "lambda_concurrency_cap_is_a_variable" {
  command = plan

  variables {
    reserved_concurrent_executions = 2
  }

  assert {
    condition     = aws_lambda_function.server.reserved_concurrent_executions == 2
    error_message = "reserved_concurrent_executions must come from the variable."
  }
}

run "cloudflare_adds_the_origin_secret_header" {
  command = plan

  assert {
    condition     = cloudflare_ruleset.origin_secret.phase == "http_request_late_transform" && cloudflare_ruleset.origin_secret.kind == "zone"
    error_message = "The header transform must be a zone ruleset in http_request_late_transform."
  }

  assert {
    condition     = cloudflare_ruleset.origin_secret.rules[0].expression == "(http.host eq \"dev.craigdevjohnson.com\")"
    error_message = "The header must be added on requests to the hostname only."
  }

  assert {
    condition     = cloudflare_ruleset.origin_secret.rules[0].action_parameters.headers["X-Origin-Secret"].operation == "set"
    error_message = "The rule must set X-Origin-Secret."
  }

  assert {
    condition     = cloudflare_dns_record.site.proxied == true && cloudflare_dns_record.site.type == "CNAME"
    error_message = "The hostname must be a proxied CNAME."
  }

  assert {
    condition     = cloudflare_dns_record.acm_validation["dev.craigdevjohnson.com"].proxied == false && cloudflare_dns_record.acm_validation["dev.craigdevjohnson.com"].name == "_mock.dev.craigdevjohnson.com"
    error_message = "ACM validation records must be DNS only, with the trailing dot trimmed."
  }
}

run "sessions_table_expires_by_ttl" {
  command = plan

  assert {
    condition     = aws_dynamodb_table.sessions.ttl[0].attribute_name == "expires_at" && aws_dynamodb_table.sessions.ttl[0].enabled == true
    error_message = "Sign-in sessions must expire through the expires_at TTL attribute."
  }

  assert {
    condition     = aws_dynamodb_table.sessions.billing_mode == "PAY_PER_REQUEST" && aws_dynamodb_table.sessions.hash_key == "id"
    error_message = "Sessions table must be on-demand with hash key id."
  }
}

run "tool_backends_are_unconnected_by_default" {
  command = plan

  assert {
    condition     = !contains(keys(aws_lambda_function.server.environment[0].variables), "SOCCER_BACKEND_URL") && !contains(keys(aws_lambda_function.server.environment[0].variables), "MINECRAFT_BACKEND_URL")
    error_message = "With no backend set, the Lambda must get no *_BACKEND_URL; the page then says the Tool is unavailable."
  }

  assert {
    condition     = !contains(keys(aws_lambda_function.server.environment[0].variables), "SOCCER_BACKEND_SIGNING") && !contains(keys(aws_lambda_function.server.environment[0].variables), "MINECRAFT_BACKEND_SIGNING")
    error_message = "With no backend set, the Lambda must get no *_BACKEND_SIGNING either."
  }

  assert {
    condition     = length([for s in data.aws_iam_policy_document.server.statement : s if startswith(s.sid, "CallToolBackend")]) == 0
    error_message = "With no backend set, the site role must get no lambda:Invoke* grant."
  }
}

run "connected_backends_reach_the_lambda_and_its_role" {
  command = plan

  variables {
    soccer_backend_url             = "https://soccer0123456789abcdef.lambda-url.us-west-2.on.aws/"
    soccer_backend_function_arn    = "arn:aws:lambda:us-west-2:123456789012:function:soccer-dev-server"
    minecraft_backend_url          = "https://minecraft0123456789abcd.lambda-url.us-west-2.on.aws/"
    minecraft_backend_function_arn = "arn:aws:lambda:us-west-2:123456789012:function:minecraft-family-dev-launcher"
    minecraft_backend_signing      = "lambda"
  }

  assert {
    condition     = aws_lambda_function.server.environment[0].variables["SOCCER_BACKEND_URL"] == "https://soccer0123456789abcdef.lambda-url.us-west-2.on.aws/" && aws_lambda_function.server.environment[0].variables["MINECRAFT_BACKEND_URL"] == "https://minecraft0123456789abcd.lambda-url.us-west-2.on.aws/"
    error_message = "Each connected backend's URL must reach the Lambda as <TOOL>_BACKEND_URL."
  }

  assert {
    condition     = !contains(keys(aws_lambda_function.server.environment[0].variables), "SOCCER_BACKEND_SIGNING") && aws_lambda_function.server.environment[0].variables["MINECRAFT_BACKEND_SIGNING"] == "lambda"
    error_message = "A signing override must reach the Lambda only when set."
  }

  assert {
    condition     = aws_lambda_function.server.environment[0].variables["SITE_BASE_URL"] == "https://dev.craigdevjohnson.com"
    error_message = "The site's own settings must survive the merge with the backends'."
  }

  assert {
    condition     = one([for s in data.aws_iam_policy_document.server.statement : s if s.sid == "CallToolBackendUrls"]).actions == toset(["lambda:InvokeFunctionUrl"])
    error_message = "The site role must be allowed lambda:InvokeFunctionUrl on the backends (ADR 0006)."
  }

  assert {
    condition     = one([for s in data.aws_iam_policy_document.server.statement : s if s.sid == "CallToolBackendsThroughUrlsOnly"]).actions == toset(["lambda:InvokeFunction"])
    error_message = "The site role must be allowed lambda:InvokeFunction on the backends, which a Function URL also needs."
  }

  assert {
    condition     = anytrue([for c in one([for s in data.aws_iam_policy_document.server.statement : s if s.sid == "CallToolBackendsThroughUrlsOnly"]).condition : c.test == "Bool" && c.variable == "lambda:InvokedViaFunctionUrl" && length(c.values) == 1 && contains(c.values, "true")])
    error_message = "lambda:InvokeFunction must be limited to calls through the Function URL, or the site could invoke a backend directly."
  }

  assert {
    condition     = alltrue([for s in data.aws_iam_policy_document.server.statement : s.resources == toset(["arn:aws:lambda:us-west-2:123456789012:function:soccer-dev-server", "arn:aws:lambda:us-west-2:123456789012:function:minecraft-family-dev-launcher"]) if startswith(s.sid, "CallToolBackend")])
    error_message = "Both grants must name exactly the connected backends' function ARNs."
  }
}

run "a_backend_url_needs_its_function_arn" {
  command = plan

  variables {
    soccer_backend_url = "https://soccer0123456789abcdef.lambda-url.us-west-2.on.aws/"
  }

  expect_failures = [var.soccer_backend_function_arn]
}

run "a_backend_must_belong_to_this_environment" {
  command = plan

  variables {
    minecraft_backend_url          = "https://minecraft0123456789abcd.lambda-url.us-west-2.on.aws/"
    minecraft_backend_function_arn = "arn:aws:lambda:us-west-2:123456789012:function:minecraft-family-prod-launcher"
  }

  expect_failures = [var.minecraft_backend_function_arn]
}

run "a_backend_must_be_in_this_account_and_region" {
  command = plan

  variables {
    soccer_backend_url          = "https://soccer0123456789abcdef.lambda-url.us-west-2.on.aws/"
    soccer_backend_function_arn = "arn:aws:lambda:us-east-1:123456789012:function:soccer-dev-server"
  }

  expect_failures = [var.soccer_backend_function_arn]
}

run "a_backend_url_must_be_a_function_url_in_this_region" {
  command = plan

  variables {
    minecraft_backend_url          = "https://minecraft0123456789abcd.lambda-url.us-east-1.on.aws/"
    minecraft_backend_function_arn = "arn:aws:lambda:us-west-2:123456789012:function:minecraft-family-dev-launcher"
  }

  expect_failures = [var.minecraft_backend_url]
}

run "a_backend_url_must_be_a_function_url_at_all" {
  command = plan

  variables {
    soccer_backend_url          = "https://soccer.example.com/"
    soccer_backend_function_arn = "arn:aws:lambda:us-west-2:123456789012:function:soccer-dev-server"
  }

  expect_failures = [var.soccer_backend_url]
}

run "a_signing_override_is_lambda_or_off" {
  command = plan

  variables {
    soccer_backend_signing = "always"
  }

  expect_failures = [var.soccer_backend_signing]
}

run "browser_integrity_check_is_off_on_calendar_links" {
  command = plan

  assert {
    condition     = cloudflare_ruleset.calendar_links.phase == "http_config_settings" && cloudflare_ruleset.calendar_links.kind == "zone"
    error_message = "The calendar link settings must be a zone configuration ruleset (http_config_settings)."
  }

  assert {
    condition     = length(cloudflare_ruleset.calendar_links.rules) == 1 && cloudflare_ruleset.calendar_links.rules[0].action == "set_config" && cloudflare_ruleset.calendar_links.rules[0].action_parameters.bic == false
    error_message = "The one rule must turn Browser Integrity Check off (ADR 0005)."
  }

  assert {
    condition     = cloudflare_ruleset.calendar_links.rules[0].expression == "(http.host eq \"dev.craigdevjohnson.com\" and starts_with(http.request.uri.path, \"/soccer/link/\"))"
    error_message = "The rule must match only the hostname's /soccer/link/ addresses."
  }
}
