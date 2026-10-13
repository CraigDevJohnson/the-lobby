# Non-secret settings for production. Committed; CI and developers both
# pass it with -var-file=prod.tfvars.
#
# The secrets and personal data this root also needs, allowed_emails and
# origin_secret, are not here. On a developer's computer they live in
# secrets.auto.tfvars beside this file (gitignored; copy
# secrets.auto.tfvars.example). In CI they come from the production
# GitHub environment, as the variable ALLOWED_EMAILS_PROD and the secret
# ORIGIN_SECRET_PROD, passed as TF_VAR_ environment variables. The
# Cloudflare API token is always read from the CLOUDFLARE_API_TOKEN
# environment variable.

cloudflare_account_id = "ada2e308f4d17f259887ed31aa91cdf3"
access_team_domain    = "https://craigdevjohnson.cloudflareaccess.com"

# From infra/bootstrap (output site_permissions_boundary_arns, key prod).
permissions_boundary_arn = "arn:aws:iam::793680745829:policy/the-lobby-site-boundary-prod"

# The Tool backends the site calls with signed requests (ADR 0006), as in
# dev.tfvars: each backend's Function URL and the ARN of the Lambda behind
# it, named *-prod-* because the production site's permissions boundary
# lets it call nothing else. Until a production backend is deployed its
# lines stay commented out and its page says the backend is unavailable.
# The soccer and Minecraft production backends were deployed on 2026-10-12:
# soccer from soccer main 6a21823 (`task infra:apply ENV=prod`), the
# Launcher from minecraft main 267afd4 (`launcher.sites`, runbook step 4).

soccer_backend_url          = "https://rqhfmqtc6kqdpb3kf2op2nu6ha0mkddl.lambda-url.us-west-2.on.aws/"
soccer_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:soccer-prod-server"

minecraft_backend_url          = "https://6h27nvacivfdvfb7fgsfnkxtka0zetmn.lambda-url.us-west-2.on.aws/"
minecraft_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:minecraft-family-prod-launcher"

# The Foundry backend, the foundry repo's foundry-prod-controller, is not
# deployed yet. Once it is, the prod entries of that repo's controller_urls
# and controller_function_arns outputs go in these two lines.

# foundry_backend_url          = "https://<id>.lambda-url.us-west-2.on.aws/"
# foundry_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:foundry-prod-controller"
