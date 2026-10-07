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
