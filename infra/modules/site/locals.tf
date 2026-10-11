locals {
  name     = "site-${var.environment}"
  base_url = "https://${var.hostname}"

  # The Tool backends, keyed by the name the Go server uses in its
  # environment (SOCCER_BACKEND_URL, MINECRAFT_BACKEND_SIGNING, ...). Only
  # the connected ones, those with a URL, reach the Lambda and its role.
  backends = {
    SOCCER = {
      url          = var.soccer_backend_url
      function_arn = var.soccer_backend_function_arn
      signing      = var.soccer_backend_signing
    }
    MINECRAFT = {
      url          = var.minecraft_backend_url
      function_arn = var.minecraft_backend_function_arn
      signing      = var.minecraft_backend_signing
    }
  }
  connected_backends = { for tool, backend in local.backends : tool => backend if backend.url != "" }

  # What the connected backends add to the Lambda's environment. The signing
  # override is only passed when set, so the server's own default holds.
  backend_environment = merge({}, [
    for tool, backend in local.connected_backends : merge(
      { "${tool}_BACKEND_URL" = backend.url },
      backend.signing == "" ? {} : { "${tool}_BACKEND_SIGNING" = backend.signing },
    )
  ]...)

  backend_function_arns = [for backend in values(local.connected_backends) : backend.function_arn]

  # What a backend's function ARN must look like: this account and region
  # (the boundary grants calls nowhere else, and the server signs for its
  # own region), named *-<environment>-* (the boundary's naming rule), with
  # an optional alias or version.
  backend_function_arn_pattern = "^arn:aws:lambda:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:function:[A-Za-z0-9_-]*-${var.environment}-[A-Za-z0-9_-]*(:[A-Za-z0-9_-]+)?$"

  # Where calendar apps fetch Session links (internal/soccer, LinkPrefix).
  calendar_link_path = "/soccer/link/"
}
