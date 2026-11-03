#!/usr/bin/env bash
# Usage: smoke.sh <hostname> <api_endpoint> [admin_aud]
#
# Three checks after a deploy. Through Cloudflare, /healthz must answer 200
# with the body "ok". Straight at API Gateway, without the secret header, it
# must answer 403: nothing reaches the site around Cloudflare (ADR 0005). And
# the family devices page (ADR 0010): while its Access application exists
# (the root's access_admin_aud output is not empty), /admin and a profile
# route must send a browser without a token to Cloudflare Access, proving
# Access covers both paths; without one, the server itself must refuse the
# page with 503.
set -euo pipefail

host=${1:?hostname}
api=${2:?api_endpoint}
admin_aud=${3:-}
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

for path in /admin /admin/profiles/kids-ipad.mobileconfig; do
  answer=$(curl -sS --max-time 20 -o /dev/null -w '%{http_code} %{redirect_url}' "https://$host$path" || echo "000")
  code=${answer%% *}
  redirect=${answer#* }
  if [ -n "$admin_aud" ]; then
    case "$code $redirect" in
      30[1278]\ https://*.cloudflareaccess.com/*)
        echo "https://$host$path: $code to Cloudflare Access, as it should be without a token"
        ;;
      *)
        echo "::error::https://$host$path answered $code (${redirect:-no redirect}), expected a redirect to Cloudflare Access" >&2
        exit 1
        ;;
    esac
  else
    if [ "$code" != 503 ]; then
      echo "::error::https://$host$path answered $code, expected 503 while the page has no Access application" >&2
      exit 1
    fi
    echo "https://$host$path: 503, as it should be with no Access application yet"
  fi
done
