#!/usr/bin/env bash
set -euo pipefail

source ./scripts/env-file.sh
load_env_file .env

if [[ -z "${RMM_DOMAIN:-}" || -z "${HEADSCALE_DOMAIN:-}" ]]; then
  echo "RMM_DOMAIN and HEADSCALE_DOMAIN are required in .env" >&2
  exit 1
fi
if [[ -z "${RMM_BOOTSTRAP_TOKEN:-}" ]]; then
  echo "RMM_BOOTSTRAP_TOKEN is missing. Run: make deploy-bootstrap-token" >&2
  exit 1
fi

url="https://${RMM_DOMAIN}/bootstrap/linux?token=${RMM_BOOTSTRAP_TOKEN}&user=lab&ttl=1h&tags=tag:rmm-agent&login_server=https://${HEADSCALE_DOMAIN}"

echo "Run this on the endpoint:"
echo
printf "curl -fsSL %q | sudo bash\n" "${url}"
