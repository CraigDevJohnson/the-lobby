#!/usr/bin/env bash
# Usage: tofu.sh plan|apply <environment>
#
# Runs OpenTofu for infra/environments/<environment> with the Lambda zip at
# dist/site.zip. The full plan or apply output goes to a temporary file, not
# the log: this repository is public, so its logs are public, and the full
# output shows the family devices page's addresses (admin_emails). Only resource names,
# progress lines and the totals reach the log and the job summary. OpenTofu's
# own errors still go to the log, because they are written to stderr.
#
# apply saves its plan to a second temporary file and applies exactly that
# plan, unless the plan would delete or replace the environment's HTTP API or
# its custom domain name. A deployer role may delete those two but cannot
# create them (infra/bootstrap/github_actions.tf), and OpenTofu deletes
# before it creates, so that apply would take the site offline and then fail.
# It is refused before anything changes, and an administrator makes the change
# instead (infra/README.md, "Replacing or removing an API or its custom
# domain"). What sits under the API and the domain (routes, integrations,
# stages, mappings) may still be deleted. The saved plan holds secret values,
# so it is never printed or uploaded, and it is removed when the script ends.
set -euo pipefail

mode=${1:-}
env_name=${2:-}
case "$mode" in
  plan | apply) ;;
  *)
    echo "usage: $0 plan|apply <environment>" >&2
    exit 2
    ;;
esac
if [ -z "$env_name" ]; then
  echo "usage: $0 plan|apply <environment>" >&2
  exit 2
fi

root="infra/environments/$env_name"
zip="${GITHUB_WORKSPACE:-$PWD}/dist/site.zip"
if [ ! -f "$zip" ]; then
  echo "::error::$zip is missing; build it first (task lambda)" >&2
  exit 1
fi

tofu -chdir="$root" init -input=false -no-color

out=$(mktemp)
saved=$(mktemp)
trap 'rm -f "$out" "$saved"' EXIT

if [ "$mode" = plan ]; then
  args=(plan -lock=false) # read-only role: no lock, no state write
else
  args=(plan -out="$saved")
fi

status=0
tofu -chdir="$root" "${args[@]}" -input=false -no-color \
  -var-file="$env_name.tfvars" -var "site_zip_path=$zip" >"$out" || status=$?

# apply: read the saved plan before anything changes. Only the addresses of
# the API and the domain name leave the JSON, and jq's own messages are
# dropped because one can quote the value it stopped at.
refused=''
if [ "$mode" = apply ] && [ "$status" -eq 0 ]; then
  if ! guarded=$(tofu -chdir="$root" show -json "$saved" | jq -r '
    [
      (.resource_changes // [])[]
      | select(.type == "aws_apigatewayv2_api" or .type == "aws_apigatewayv2_domain_name")
      | select(any(.change.actions[]; . == "delete"))
      | .address
    ] | unique | join(", ")' 2>/dev/null); then
    refused="the saved plan could not be read (tofu show or jq failed), so it was not checked"
  elif [ -n "$guarded" ]; then
    refused="the plan would delete or replace $guarded. A deployer role cannot create an API or a custom domain, so an administrator makes this change: see \"Replacing or removing an API or its custom domain\" in infra/README.md"
  else
    tofu -chdir="$root" apply -input=false -no-color "$saved" >>"$out" || status=$?
  fi
fi

# Keep: the "# address will be ..." headers, apply progress lines, and the
# totals. Drop everything indented deeper, which is where attribute values are.
trimmed=$(grep -E '^(Plan:|Apply complete!|No changes\.|  # |[^ ]+: (Creating|Modifying|Destroying|Refreshing state|Reading|Read complete|Still |Creation complete|Modifications complete|Destruction complete))' "$out" || true)

{
  echo "## OpenTofu $mode: $env_name"
  echo
  echo '```text'
  echo "${trimmed:-(nothing to report)}"
  echo '```'
} | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"

if [ -n "$refused" ]; then
  echo "::error::Nothing was applied to $env_name: $refused" >&2
  exit 1
fi
if [ "$status" -ne 0 ]; then
  echo "::error::tofu $mode failed for $env_name (exit $status); see the messages above" >&2
fi
exit "$status"
