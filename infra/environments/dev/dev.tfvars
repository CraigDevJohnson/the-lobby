# Non-secret settings for dev. Committed; CI and developers both pass it with
# -var-file=dev.tfvars.
#
# The secrets and personal data this root also needs, allowed_emails and
# origin_secret, are not here. On a developer's computer they live in
# secrets.auto.tfvars beside this file (gitignored; copy
# secrets.auto.tfvars.example). In CI they come from GitHub, as the variable
# ALLOWED_EMAILS_DEV and the secret ORIGIN_SECRET_DEV, passed as TF_VAR_
# environment variables. The Cloudflare API token is always read from the
# CLOUDFLARE_API_TOKEN environment variable.

cloudflare_account_id = "ada2e308f4d17f259887ed31aa91cdf3"
access_team_domain    = "https://craigdevjohnson.cloudflareaccess.com"

# From infra/bootstrap (output site_permissions_boundary_arn).
permissions_boundary_arn = "arn:aws:iam::793680745829:policy/the-lobby-site-boundary"
