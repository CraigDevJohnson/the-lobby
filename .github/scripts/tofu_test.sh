#!/usr/bin/env bash
# Tests .github/scripts/tofu.sh offline (task infra:check runs it). The script
# runs with a stand-in tofu on PATH that logs each call, prints a made-up plan
# and apply, and answers "show -json" with the plan the test chose, so nothing
# here reaches AWS or Cloudflare. jq is the real one.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/bin" "$work/dist" "$work/tmp"
: >"$work/dist/site.zip"

# The stand-in. plan writes STUB-PLAN-FILE to the file -out names; show -json
# and apply take only such a file, and show prints the JSON in STUB_JSON. The
# verb STUB_FAIL names fails. Each made-up value that the script must never
# print starts with STUB-.
cat >"$work/bin/tofu" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
echo "$*" >>"$STUB_LOG"
verb=$2
shift 2
if [ "${STUB_FAIL:-}" = "$verb" ]; then
  echo "stand-in tofu: $verb failed" >&2
  exit 1
fi
case "$verb" in
  init) ;;
  plan)
    for arg in "$@"; do
      case "$arg" in
        -out=*) echo STUB-PLAN-FILE >"${arg#-out=}" ;;
      esac
    done
    cat <<'PLAN'
module.site.aws_lambda_function.server: Refreshing state... [id=site-dev-server]

OpenTofu will perform the following actions:

  # module.site.aws_lambda_function.server will be updated in-place
  ~ resource "aws_lambda_function" "server" {
      ~ source_code_hash = "STUB-ATTRIBUTE" -> "STUB-ATTRIBUTE"
    }

Plan: 0 to add, 1 to change, 0 to destroy.
PLAN
    ;;
  show)
    if [ "$1" != -json ] || ! grep -q STUB-PLAN-FILE "$2"; then
      echo "stand-in tofu: show takes -json and a plan that plan saved" >&2
      exit 2
    fi
    cat "$STUB_JSON"
    ;;
  apply)
    if ! grep -q STUB-PLAN-FILE "${!#}"; then
      echo "stand-in tofu: apply takes a plan that plan saved" >&2
      exit 2
    fi
    echo 'module.site.aws_lambda_function.server: Modifying... [id=site-dev-server]'
    echo 'Apply complete! Resources: 0 added, 1 changed, 0 destroyed.'
    ;;
  *)
    echo "stand-in tofu: unexpected $verb" >&2
    exit 2
    ;;
esac
STUB
chmod +x "$work/bin/tofu"

export PATH="$work/bin:$PATH"
export STUB_LOG="$work/tofu.log" STUB_JSON="$work/plan.json"
export GITHUB_WORKSPACE="$work" GITHUB_STEP_SUMMARY="$work/summary" TMPDIR="$work/tmp"
unset STUB_FAIL

api=module.site.aws_apigatewayv2_api.http
domain=module.site.aws_apigatewayv2_domain_name.site
route=module.site.aws_apigatewayv2_route.default

fail() {
  echo "tofu_test: $1" >&2
  exit 1
}

# plan_json ADDRESS=ACTIONS... writes the JSON that show will print: one
# change per argument, with its actions separated by commas. A resource's
# type is the part of its address before its name.
plan_json() {
  printf '%s\n' "$@" | jq -R 'split("=") | {
    address: .[0],
    type: (.[0] | split(".") | .[-2]),
    change: {actions: (.[1] | split(",")), after: {value: "STUB-JSON-VALUE"}}
  }' | jq -s '{resource_changes: .}' >"$STUB_JSON"
}

# run MODE runs the script for dev, as a workflow step does. It sets out to
# everything the script printed and status to how it ended, starts the call
# log again, and keeps the output for the last check.
run() {
  : >"$STUB_LOG"
  status=0
  out=$(bash "$here/tofu.sh" "$1" dev 2>&1) || status=$?
  echo "$out" >>"$work/printed"
  [ -z "$(ls -A "$TMPDIR")" ] || fail "$1 left a temporary file behind"
}

says() { grep -Fq -- "$1" <<<"$out" || fail "expected \"$1\" in: $out"; }

# called VERB: did the script run tofu VERB for dev?
called() { grep -q -- "^-chdir=infra/environments/dev $1 " "$STUB_LOG"; }

# plan: one plan without the lock. Nothing is saved, read back or applied,
# whatever the plan holds.
plan_json "$api=delete,create"
run plan
[ "$status" -eq 0 ] || fail "plan failed: $out"
called 'plan -lock=false' || fail "plan did not run tofu plan -lock=false"
if grep -Eq -- ' -out=| show | apply ' "$STUB_LOG"; then
  fail "plan saved, read or applied a plan"
fi
says '# module.site.aws_lambda_function.server will be updated in-place'
says 'Plan: 0 to add, 1 to change, 0 to destroy.'

# apply: the plan that was saved is the one applied. The API and the domain
# may change in place, and what sits under them may be deleted or replaced.
plan_json "$api=update" "$domain=no-op" "$route=delete,create" \
  module.site.aws_apigatewayv2_integration.server=create,delete \
  module.site.aws_apigatewayv2_stage.default=delete \
  module.site.aws_apigatewayv2_api_mapping.site=delete
run apply
[ "$status" -eq 0 ] || fail "apply failed: $out"
saved=$(sed -n 's/.* plan -out=\([^ ]*\) .*/\1/p' "$STUB_LOG")
[ -n "$saved" ] || fail "apply did not save its plan"
grep -Fxq -- "-chdir=infra/environments/dev apply -input=false -no-color $saved" "$STUB_LOG" ||
  fail "apply did not apply exactly the plan it saved"
says '# module.site.aws_lambda_function.server will be updated in-place'
says 'Apply complete! Resources: 0 added, 1 changed, 0 destroyed.'
grep -Fq 'Apply complete!' "$GITHUB_STEP_SUMMARY" || fail "the job summary lacks the apply's totals"

# apply refuses a plan that deletes the API or the domain, or replaces one in
# either order, and says which and where the administrator's steps are.
for change in "$api=delete,create" "$api=delete" "$domain=create,delete" "$domain=delete"; do
  plan_json "$route=update" "$change"
  run apply
  [ "$status" -ne 0 ] || fail "apply went ahead with $change"
  ! called apply || fail "apply ran tofu apply with $change"
  says "::error::Nothing was applied to dev: the plan would delete or replace ${change%%=*}."
  says 'infra/README.md'
done

# Both at once are both named.
plan_json "$domain=delete,create" "$api=delete,create"
run apply
! called apply || fail "apply ran tofu apply with both replaced"
says "delete or replace $api, $domain."

# A saved plan that cannot be read is not applied: show failing, and show
# printing something that is not a plan.
plan_json "$api=update"
STUB_FAIL=show run apply
[ "$status" -ne 0 ] || fail "apply succeeded although show failed"
! called apply || fail "apply ran tofu apply although show failed"
echo '{"resource_changes": "STUB-JSON-VALUE"}' >"$STUB_JSON"
run apply
[ "$status" -ne 0 ] || fail "apply succeeded on JSON that is not a plan"
! called apply || fail "apply ran tofu apply on JSON that is not a plan"
says 'the saved plan could not be read'

# A plan that fails is an error, and nothing is read back or applied.
plan_json "$api=update"
STUB_FAIL=plan run apply
[ "$status" -ne 0 ] || fail "apply succeeded although the plan failed"
if called show || called apply; then
  fail "apply went on after a plan that failed"
fi

# An apply that fails is an error.
STUB_FAIL=apply run apply
[ "$status" -ne 0 ] || fail "apply succeeded although tofu apply failed"
says 'tofu apply failed for dev'

# Nothing the script printed or put in the job summary came from inside a
# plan: not an attribute's value, not the saved plan and not its JSON.
if grep -F 'STUB-' "$work/printed" "$GITHUB_STEP_SUMMARY"; then
  fail "the script printed what a plan holds"
fi

echo "tofu_test: ok"
