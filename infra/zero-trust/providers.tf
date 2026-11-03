# The token comes from the CLOUDFLARE_API_TOKEN environment variable, as for
# every other root. This root needs a token with wider permissions than the
# deploy token (infra/README.md, The family Gateway configuration).
provider "cloudflare" {}
