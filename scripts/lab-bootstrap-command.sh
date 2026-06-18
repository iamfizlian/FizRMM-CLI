#!/usr/bin/env bash
set -euo pipefail

control_plane_url="${1:-}"
if [[ -z "${control_plane_url}" ]]; then
  echo "CONTROL_PLANE_URL is required, example: make lab-bootstrap-command CONTROL_PLANE_URL=http://192.0.2.10:8080" >&2
  exit 1
fi
if [[ ! -f .env ]]; then
  echo ".env is missing. Run: make lab-bootstrap-token" >&2
  exit 1
fi

set -a
source .env
set +a

if [[ -z "${RMM_BOOTSTRAP_TOKEN:-}" ]]; then
  echo "RMM_BOOTSTRAP_TOKEN is missing. Run: make lab-bootstrap-token" >&2
  exit 1
fi

headscale_url="${control_plane_url%:8080}:8081"
url="${control_plane_url%/}/bootstrap/linux?token=${RMM_BOOTSTRAP_TOKEN}&user=lab&ttl=1h&tags=tag:rmm-agent&login_server=${headscale_url}"

echo "Run this on the endpoint:"
echo
printf "curl -fsSL %q | sudo bash\n" "${url}"
