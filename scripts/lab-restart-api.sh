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

podman compose up -d --force-recreate rmm-api

echo "rmm-api restarted with Headscale API access"
echo "Next: make lab-enroll-script LOGIN_SERVER=http://<this-pc-ip>:8081"
