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

  # Where calendar apps fetch Session links (internal/soccer, LinkPrefix).
  calendar_link_path = "/soccer/link/"
}
