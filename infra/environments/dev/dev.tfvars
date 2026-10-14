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

# From infra/bootstrap (output site_permissions_boundary_arns, key dev).
permissions_boundary_arn = "arn:aws:iam::793680745829:policy/the-lobby-site-boundary-dev"

# The Tool backends the site calls with signed requests (ADR 0006). Each is
# the backend's Function URL, which AWS lets only the site's role call, and
# the ARN of the Lambda behind it, both from that repo's own OpenTofu
# outputs. They are not secret: without the site's signature a Function URL
# answers 403. They belong here, not only on a computer, because the Deploy
# workflow applies this file on every push to main and would undo values it
# cannot see. Until a backend is deployed its lines stay commented out and
# its page says the backend is unavailable. The soccer and Minecraft dev
# backends were deployed on 2026-10-11 (soccer main 586a461, minecraft main
# f738b2c).

soccer_backend_url          = "https://fwbmn726osd2u3ctu5oqjhu3hm0ryqic.lambda-url.us-west-2.on.aws/"
soccer_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:soccer-dev-server"

minecraft_backend_url          = "https://doedzzndd54qlqchcrywrz24jm0uhahj.lambda-url.us-west-2.on.aws/"
minecraft_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:minecraft-family-dev-launcher"

# The Foundry backend, the foundry repo's foundry-dev-controller, deployed on
# 2026-10-14 by that repo's Deploy workflow (foundry main 33e7518): the dev
# entries of its controller_urls and controller_function_arns outputs.

foundry_backend_url          = "https://4midqrnm6h53i37w2yminvltcu0ndslr.lambda-url.us-west-2.on.aws/"
foundry_backend_function_arn = "arn:aws:lambda:us-west-2:793680745829:function:foundry-dev-controller"

# Foundry's join page, the foundry repo's local.foundry_join_url. A Member
# with the foundry grant is given it when Foundry's status can't be checked,
# so it is offered only once the backend above is connected.
foundry_join_url = "https://foundry.craigdevjohnson.com:30000/join"
