#!/usr/bin/env bash
set -euo pipefail

key="$(podman exec fizrmm-cli_headscale_1 headscale apikeys create --expiration 24h | tail -n 1)"

if [[ -z "${key}" ]]; then
  echo "failed to create Headscale API key" >&2
  exit 1
fi

tmp="$(mktemp)"
if [[ -f .env ]]; then
  grep -v '^RMM_HEADSCALE_API_KEY=' .env > "${tmp}" || true
fi
printf 'RMM_HEADSCALE_API_KEY=%s\n' "${key}" >> "${tmp}"
mv "${tmp}" .env

echo "Wrote a temporary 24h Headscale API key to .env"
echo "Next: make lab-restart-api"
