#!/usr/bin/env bash
# Usage: smoke.sh <hostname> <api_endpoint>
#
# Two checks after a deploy. Through Cloudflare, /healthz must answer 200 with
# the body "ok". Straight at API Gateway, without the secret header, it must
# answer 403: nothing reaches the site around Cloudflare (ADR 0005).
set -euo pipefail

host=${1:?hostname}
api=${2:?api_endpoint}
api=${api%/}

body=$(mktemp)
trap 'rm -f "$body"' EXIT

# The alias may take a moment to point at the new version; allow a short wait.
attempt=0
until
  attempt=$((attempt + 1))
  code=$(curl -sS --max-time 20 -o "$body" -w '%{http_code}' "https://$host/healthz" || true)
  [ "$code" = 200 ] && [ "$(cat "$body")" = ok ]
do
  if [ "$attempt" -ge 6 ]; then
    echo "::error::https://$host/healthz answered $code, expected 200 ok" >&2
    head -c 500 "$body" >&2
    echo >&2
    exit 1
  fi
  sleep 10
done
echo "https://$host/healthz: 200 ok"

code=$(curl -sS --max-time 20 -o /dev/null -w '%{http_code}' "$api/healthz" || true)
if [ "$code" != 403 ]; then
  echo "::error::$api/healthz answered $code, expected 403 (Cloudflare only)" >&2
  exit 1
fi
echo "$api/healthz: 403, as it should be without Cloudflare"
