#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo ".env is missing. Run deploy from the repository root." >&2
  exit 1
fi

source ./scripts/env-file.sh
load_env_file .env

if [[ -z "${HEADSCALE_DOMAIN:-}" ]]; then
  echo "HEADSCALE_DOMAIN is required in .env" >&2
  exit 1
fi
if [[ -z "${RMM_DOMAIN:-}" ]]; then
  echo "RMM_DOMAIN is required in .env" >&2
  exit 1
fi

compose_cmd="${COMPOSE_CMD:-podman-compose}"
compose_file="deploy/generated/compose.yml"
headscale_user="${DEPLOY_HEADSCALE_USER:-lab}"
hostname="${RMM_CONTROLLER_HOSTNAME:-fizrmm-controller}"

wait_for_apt_locks() {
  if ! command -v fuser >/dev/null 2>&1; then
    return
  fi

  locks=(
    /var/lib/apt/lists/lock
    /var/lib/dpkg/lock
    /var/lib/dpkg/lock-frontend
    /var/cache/apt/archives/lock
  )

  for _ in $(seq 1 120); do
    if ! sudo fuser "${locks[@]}" >/dev/null 2>&1; then
      return
    fi
    echo "Waiting for apt/dpkg lock to clear..."
    sleep 5
  done

  echo "Timed out waiting for apt/dpkg lock to clear." >&2
  sudo fuser -v "${locks[@]}" || true
  exit 1
}

if ! command -v tailscale >/dev/null 2>&1; then
  wait_for_apt_locks
  curl -fsSL https://tailscale.com/install.sh | sh
fi

if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now tailscaled
fi

./scripts/deploy-render.sh
"${compose_cmd}" -f "${compose_file}" up -d --force-recreate headscale

echo "Requesting controller enrollment key from RMM API..."
payload="$(printf '{"user":%s,"ttl":"24h","tags":[]}' "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "${headscale_user}")")"
if ! curl -fsSL \
  -H "Content-Type: application/json" \
  -H "X-RMM-Actor: deploy-controller-enroll" \
  -d "${payload}" \
  "https://${RMM_DOMAIN}/v1/overlay/preauthkeys" \
  -o /tmp/fizrmm-controller-preauth.json; then
  echo "Failed to create controller enrollment key through RMM API." >&2
  exit 1
fi
key="$(python3 -c 'import json; print(json.load(open("/tmp/fizrmm-controller-preauth.json"))["data"]["key"])')"

sudo tailscale up \
  --reset \
  --force-reauth \
  --login-server "https://${HEADSCALE_DOMAIN}" \
  --auth-key "${key}" \
  --hostname "${hostname}" \
  --ssh=false

tailscale status
