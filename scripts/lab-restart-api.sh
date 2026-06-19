#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo ".env is missing. Run: make lab-headscale-key" >&2
  exit 1
fi

set -a
source .env
set +a

if [[ -z "${RMM_HEADSCALE_API_KEY:-}" ]]; then
  echo "RMM_HEADSCALE_API_KEY is missing in .env. Run: make lab-headscale-key" >&2
  exit 1
fi
if [[ -z "${RMM_BOOTSTRAP_TOKEN:-}" ]]; then
  echo "RMM_BOOTSTRAP_TOKEN is missing in .env. Run: make lab-bootstrap-token" >&2
  exit 1
fi

compose_cmd="${COMPOSE_CMD:-podman-compose}"
"${compose_cmd}" up -d --force-recreate rmm-api

echo "rmm-api restarted with Headscale API access"
echo "Next: make lab-bootstrap-command CONTROL_PLANE_URL=http://<this-pc-ip>:8080"
