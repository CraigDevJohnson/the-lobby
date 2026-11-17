#!/usr/bin/env bash
# Tests infra/inputs.sh offline (task infra:check runs it). A copy of the
# script runs in a scratch checkout, with a stand-in aws on PATH that keeps
# "the bucket" in a folder, so nothing here reaches AWS or this checkout's own
# input files.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

repo=$work/repo
mkdir -p "$work/bin" "$work/bucket" \
  "$repo/infra/bootstrap" "$repo/infra/zero-trust" \
  "$repo/infra/environments/dev" "$repo/infra/environments/prod"
cp "$here/inputs.sh" "$repo/infra/inputs.sh"

# The stand-in: s3api list-objects-v2, get-object and put-object against the
# folder STUB_BUCKET, one file per key. It logs every call to STUB_LOG, and
# the verb STUB_FAIL names fails the way a refused or expired session does.
cat >"$work/bin/aws" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
echo "$*" >>"$STUB_LOG"
if [ "$1" != s3api ]; then
  echo "stand-in aws: unexpected $*" >&2
  exit 2
fi
verb=$2
shift 2
key='' body='' prefix='' out=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --key) key=$2 && shift 2 ;;
    --body) body=$2 && shift 2 ;;
    --prefix) prefix=$2 && shift 2 ;;
    --bucket | --region | --query | --output) shift 2 ;;
    --no-cli-pager) shift ;;
    --*)
      echo "stand-in aws: unexpected option $1" >&2
      exit 2
      ;;
    *) out=$1 && shift ;;
  esac
done
refuse() {
  if [ "${STUB_FAIL:-}" = "$verb" ]; then
    echo "stand-in aws: An error occurred (AccessDenied)" >&2
    exit 254
  fi
}
cd "$STUB_BUCKET"
case "$verb" in
  list-objects-v2)
    refuse
    keys=$(find "${prefix%/}" -type f 2>/dev/null | sort || true)
    echo "${keys:-None}"
    ;;
  get-object)
    cp "$key" "$out"
    refuse
    echo "version-of-$(basename "$key")"
    ;;
  put-object)
    refuse
    mkdir -p "$(dirname "$key")"
    cp "$body" "$key"
    echo "version-of-$(basename "$key")"
    ;;
  *)
    echo "stand-in aws: unexpected s3api $verb" >&2
    exit 2
    ;;
esac
STUB
chmod +x "$work/bin/aws"

export PATH="$work/bin:$PATH"
export STUB_BUCKET="$work/bucket" STUB_LOG="$work/aws.log"
export AWS_PROFILE=workloads-admin
unset FORCE STUB_FAIL
: >"$STUB_LOG"

bootstrap=infra/bootstrap/bootstrap.auto.tfvars
zero_trust=infra/zero-trust/zero-trust.auto.tfvars
dev=infra/environments/dev/secrets.auto.tfvars
held=$STUB_BUCKET/the-lobby/inputs

fail() {
  echo "inputs_test: $1" >&2
  exit 1
}

# run ARGS... runs the script the way a person's shell would, with files it
# creates open to all unless the script says otherwise. It sets out to
# everything the script printed and status to how it ended, and keeps the
# output for the last check.
run() {
  status=0
  out=$(umask 022 && bash "$repo/infra/inputs.sh" "$@" 2>&1) || status=$?
  echo "$out" >>"$work/printed"
}

says() { grep -Fq -- "$1" <<<"$out" || fail "expected \"$1\" in: $out"; }

# No profile: stops before any call to AWS.
AWS_PROFILE='' run pull
[ "$status" -ne 0 ] || fail "pull ran without AWS_PROFILE"
says "Set AWS_PROFILE"
[ ! -s "$STUB_LOG" ] || fail "aws was called without AWS_PROFILE"

# An empty bucket: an error that says so.
run pull
[ "$status" -ne 0 ] || fail "pull of an empty bucket succeeded"
says "The bucket holds no input file yet."

# Two of the four in the bucket: both arrive, readable by the owner alone,
# and the other two are named without failing the pull.
mkdir -p "$held/infra/bootstrap" "$held/infra/zero-trust"
echo 'origin_secrets = "BUCKET-CONTENT-ONE"' >"$held/$bootstrap"
echo 'enrollment_emails = "BUCKET-CONTENT-TWO"' >"$held/$zero_trust"
run pull
[ "$status" -eq 0 ] || fail "pull failed: $out"
says "$bootstrap: version version-of-bootstrap.auto.tfvars"
says "$zero_trust: version version-of-zero-trust.auto.tfvars"
says "$dev: not in the bucket"
for path in "$bootstrap" "$zero_trust"; do
  cmp -s "$held/$path" "$repo/$path" || fail "$path is not the bucket's copy"
  [ -n "$(find "$repo/$path" -perm 600)" ] || fail "$path is not mode 600"
done
[ ! -e "$repo/$dev" ] || fail "pull made a file the bucket does not hold"

# A local file that differs is left alone and fails the pull; FORCE=1
# replaces it. The file that still matches is pulled either way.
echo 'origin_secrets = "LOCAL-CONTENT"' >"$repo/$bootstrap"
run pull
[ "$status" -ne 0 ] || fail "pull succeeded over a local file that differs"
says "$bootstrap: differs from version version-of-bootstrap.auto.tfvars"
says "$zero_trust: version version-of-zero-trust.auto.tfvars"
grep -q LOCAL-CONTENT "$repo/$bootstrap" || fail "pull replaced a local file that differs"
[ -z "$(find "$repo" -name '*.pull.tfvars')" ] || fail "pull left a temporary file behind"
FORCE=1 run pull
[ "$status" -eq 0 ] || fail "FORCE=1 pull failed: $out"
cmp -s "$held/$bootstrap" "$repo/$bootstrap" || fail "FORCE=1 did not replace the local file"
[ -n "$(find "$repo/$bootstrap" -perm 600)" ] || fail "the replaced file is not mode 600"

# aws failing is an error, never "not in the bucket". A listing that fails
# stops the pull, and a download that fails leaves no temporary file and the
# local file as it was.
STUB_FAIL=list-objects-v2 run pull
[ "$status" -ne 0 ] || fail "pull succeeded although the listing failed"
if grep -Eq 'not in the bucket|holds no input file' <<<"$out"; then
  fail "a failed listing was taken for an empty bucket: $out"
fi
STUB_FAIL=get-object run pull
[ "$status" -ne 0 ] || fail "pull succeeded although a download failed"
[ -z "$(find "$repo" -name '*.pull.tfvars')" ] || fail "a failed download left a temporary file behind"
cmp -s "$held/$bootstrap" "$repo/$bootstrap" || fail "a failed download changed the local file"

# push refuses a path that is not one of the four, before any upload.
echo 'site_hostname = "dev"' >"$repo/infra/environments/dev/dev.tfvars"
run push "$zero_trust" infra/environments/dev/dev.tfvars
[ "$status" -ne 0 ] || fail "push took a path outside the four"
says "infra/environments/dev/dev.tfvars is not one of the input files"
! grep -q put-object "$STUB_LOG" || fail "push uploaded before refusing a path"

# push with no names uploads the files this checkout holds, and only those.
echo 'enrollment_emails = "LOCAL-CONTENT-TWO"' >"$repo/$zero_trust"
run push
[ "$status" -eq 0 ] || fail "push failed: $out"
says "$bootstrap: version version-of-bootstrap.auto.tfvars"
says "$zero_trust: version version-of-zero-trust.auto.tfvars"
says "$dev: not in this checkout"
cmp -s "$repo/$zero_trust" "$held/$zero_trust" || fail "push did not upload $zero_trust"
[ ! -e "$held/$dev" ] || fail "push uploaded a file this checkout does not hold"
[ ! -e "$held/infra/environments/dev/dev.tfvars" ] || fail "push uploaded dev.tfvars"

# push with a name uploads that file alone: one more upload after the two above.
run push "$bootstrap"
[ "$status" -eq 0 ] || fail "push of one file failed: $out"
[ "$(grep -c put-object "$STUB_LOG")" -eq 3 ] || fail "push of one file made more than one upload"

# An upload that fails is an error.
STUB_FAIL=put-object run push
[ "$status" -ne 0 ] || fail "push succeeded although an upload failed"

# Every call names the bucket's region and turns the pager off, and nothing
# the script printed came from inside a file.
if grep -v -- '--no-cli-pager --region us-west-2 --bucket craigdevjohnson-tofu-state-793680745829' "$STUB_LOG"; then
  fail "an aws call lacked --no-cli-pager, the region or the bucket"
fi
if grep -E 'CONTENT|origin_secrets|enrollment_emails' "$work/printed"; then
  fail "the script printed a file's content"
fi

echo "inputs_test: ok"
