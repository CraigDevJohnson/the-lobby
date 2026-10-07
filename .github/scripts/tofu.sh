#!/usr/bin/env bash
# Usage: tofu.sh plan|apply <environment>
#
# Runs OpenTofu for infra/environments/<environment> with the Lambda zip at
# dist/site.zip. The full plan or apply output goes to a temporary file, not
# the log: this repository is public, so its logs are public, and the full
# output shows the Access invite list (allowed_emails). Only resource names,
# progress lines and the totals reach the log and the job summary. OpenTofu's
# own errors still go to the log, because they are written to stderr.
set -euo pipefail

mode=${1:-}
env_name=${2:-}
case "$mode" in
  plan) args=(plan -lock=false) ;; # read-only role: no lock, no state write
  apply) args=(apply -auto-approve) ;;
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
trap 'rm -f "$out"' EXIT

status=0
tofu -chdir="$root" "${args[@]}" -input=false -no-color \
  -var-file="$env_name.tfvars" -var "site_zip_path=$zip" >"$out" || status=$?

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

if [ "$status" -ne 0 ]; then
  echo "::error::tofu $mode failed for $env_name (exit $status); see the messages above" >&2
fi
exit "$status"
