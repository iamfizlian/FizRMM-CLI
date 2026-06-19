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

compose_cmd="${COMPOSE_CMD:-podman-compose}"
compose_file="deploy/generated/compose.yml"
headscale_container="${HEADSCALE_CONTAINER:-generated_headscale_1}"
headscale_user="${DEPLOY_HEADSCALE_USER:-lab}"
hostname="${RMM_CONTROLLER_HOSTNAME:-fizrmm-controller}"

headscale_exec() {
  timeout 10s podman exec "${headscale_container}" headscale "$@"
}

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

echo "Waiting for Headscale to accept commands..."
headscale_ready=false
for _ in $(seq 1 30); do
  if headscale_exec users list >/tmp/fizrmm-headscale-users.txt 2>/tmp/fizrmm-headscale-users.err; then
    headscale_ready=true
    break
  fi
  sleep 2
done
if [[ "${headscale_ready}" != "true" ]]; then
  cat /tmp/fizrmm-headscale-users.err >&2
  podman logs --tail 80 "${headscale_container}" >&2 || true
  echo "Headscale did not become ready." >&2
  exit 1
fi

if ! headscale_exec users create "${headscale_user}" >/tmp/fizrmm-headscale-user-create.out 2>&1; then
  if ! grep -qi "already exists" /tmp/fizrmm-headscale-user-create.out; then
    cat /tmp/fizrmm-headscale-user-create.out >&2
    exit 1
  fi
fi

if ! headscale_exec preauthkeys create \
  --user "${headscale_user}" \
  --expiration 24h >/tmp/fizrmm-controller-preauth.out 2>&1; then
  cat /tmp/fizrmm-controller-preauth.out >&2
  exit 1
fi
key="$(tail -n 1 /tmp/fizrmm-controller-preauth.out)"

sudo tailscale up \
  --reset \
  --force-reauth \
  --login-server "https://${HEADSCALE_DOMAIN}" \
  --auth-key "${key}" \
  --hostname "${hostname}" \
  --ssh=false

tailscale status
