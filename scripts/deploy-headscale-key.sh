#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo ".env is missing. Run: cp deploy/.env.example .env, then edit it." >&2
  exit 1
fi

compose_cmd="${COMPOSE_CMD:-podman-compose}"
key="$("${compose_cmd}" -f deploy/compose.yml --env-file .env exec headscale headscale apikeys create --expiration 24h | tail -n 1)"

if [[ -z "${key}" ]]; then
  echo "failed to create Headscale API key" >&2
  exit 1
fi

tmp="$(mktemp)"
grep -v '^RMM_HEADSCALE_API_KEY=' .env > "${tmp}" || true
printf 'RMM_HEADSCALE_API_KEY=%s\n' "${key}" >> "${tmp}"
mv "${tmp}" .env

echo "Wrote a temporary 24h Headscale API key to .env"
echo "Next: make deploy-restart-api"
