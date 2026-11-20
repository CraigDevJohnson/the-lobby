#!/usr/bin/env bash
# Usage: inputs.sh pull
#        inputs.sh push [file ...]
#
# Copies the roots' private input files between this checkout and the state
# bucket (ADR 0016; "Private input files" in infra/README.md). They are the
# four gitignored tfvars files listed below, and each one's key is
# the-lobby/inputs/ followed by its path from the repository root.
#
# They hold the origin secrets and the family's addresses, so nothing here
# prints a file's content: each line names a file and its object version. A
# pulled file is readable by its owner alone.
#
# pull fetches every file the bucket holds. A local file that differs from
# the bucket's copy is left as it is unless FORCE=1. push uploads the files
# this checkout holds, or only the ones named. A pull is a read; a push
# changes AWS, so it needs Craig's go-ahead first.
set -euo pipefail
umask 077

bucket=craigdevjohnson-tofu-state-793680745829
region=us-west-2
prefix=the-lobby/inputs/
inputs=(
  infra/bootstrap/bootstrap.auto.tfvars
  infra/zero-trust/zero-trust.auto.tfvars
  infra/environments/dev/secrets.auto.tfvars
  infra/environments/prod/secrets.auto.tfvars
)

usage() {
  echo "usage: $0 pull | push [file ...]" >&2
  exit 2
}

mode=${1:-}
case "$mode" in
  pull) [ "$#" -eq 1 ] || usage ;;
  push) shift ;;
  *) usage ;;
esac

if [ -z "${AWS_PROFILE:-}" ]; then
  echo "Set AWS_PROFILE to the workloads account's admin profile, then: aws sso login" >&2
  exit 1
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

s3() { aws s3api "$@" --no-cli-pager --region "$region" --bucket "$bucket"; }

if [ "$mode" = pull ]; then
  # The download lands beside its target, under a name that ends in .tfvars
  # but not .auto.tfvars, so git ignores it and OpenTofu does not load it. It
  # is removed when the run ends, whether the run succeeds or fails.
  tmp=
  trap '[ -z "$tmp" ] || rm -f "$tmp"' EXIT

  held=$(s3 list-objects-v2 --prefix "$prefix" --query 'Contents[].[Key]' --output text)
  found=0
  differing=0
  for path in "${inputs[@]}"; do
    if ! grep -Fxq -- "$prefix$path" <<<"$held"; then
      echo "$path: not in the bucket"
      continue
    fi
    found=$((found + 1))
    tmp="$root/$path.pull.tfvars"
    version=$(s3 get-object --key "$prefix$path" --query VersionId --output text "$tmp")
    if [ -e "$root/$path" ] && [ "${FORCE:-}" != 1 ] && ! cmp -s "$tmp" "$root/$path"; then
      echo "$path: differs from version $version in the bucket, left as it is (FORCE=1 replaces it)"
      differing=$((differing + 1))
      rm -f "$tmp"
      continue
    fi
    chmod 600 "$tmp"
    mv -f "$tmp" "$root/$path"
    echo "$path: version $version"
  done
  if [ "$found" -eq 0 ]; then
    echo "The bucket holds no input file yet." >&2
    exit 1
  fi
  if [ "$differing" -gt 0 ]; then
    exit 1
  fi
  exit 0
fi

# upload FILE puts one input file in the bucket and prints its new version.
upload() {
  local version
  version=$(s3 put-object --key "$prefix$1" --body "$root/$1" --query VersionId --output text)
  echo "$1: version $version"
}

if [ "$#" -eq 0 ]; then
  pushed=0
  for path in "${inputs[@]}"; do
    if [ -f "$root/$path" ]; then
      upload "$path"
      pushed=$((pushed + 1))
    else
      echo "$path: not in this checkout"
    fi
  done
  if [ "$pushed" -eq 0 ]; then
    echo "This checkout holds no input file to push." >&2
    exit 1
  fi
  exit 0
fi

# Every name is checked before the first upload.
for path in "$@"; do
  known=0
  for input in "${inputs[@]}"; do
    [ "$path" != "$input" ] || known=1
  done
  if [ "$known" -eq 0 ]; then
    echo "$path is not one of the input files:" >&2
    printf '  %s\n' "${inputs[@]}" >&2
    exit 1
  fi
  if [ ! -f "$root/$path" ]; then
    echo "$path: not in this checkout" >&2
    exit 1
  fi
done
for path in "$@"; do
  upload "$path"
done
