#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo ".env is missing. Run: cp deploy/.env.example .env, then edit it." >&2
  exit 1
fi

source ./scripts/env-file.sh
load_env_file .env

compose_cmd="${COMPOSE_CMD:-podman-compose}"
compose_file="deploy/generated/compose.yml"
headscale_user="${DEPLOY_HEADSCALE_USER:-lab}"

if [[ -z "${RMM_BOOTSTRAP_TOKEN:-}" ]]; then
  ./scripts/deploy-bootstrap-token.sh
  load_env_file .env
fi

./scripts/deploy-render.sh
"${compose_cmd}" -f "${compose_file}" up -d

echo "Waiting for Headscale to accept commands..."
for _ in $(seq 1 30); do
  if "${compose_cmd}" -f "${compose_file}" exec headscale headscale users list >/tmp/fizrmm-headscale-users.txt 2>/tmp/fizrmm-headscale-users.err; then
    break
  fi
  sleep 2
done

if ! "${compose_cmd}" -f "${compose_file}" exec headscale headscale users list >/tmp/fizrmm-headscale-users.txt 2>/tmp/fizrmm-headscale-users.err; then
  cat /tmp/fizrmm-headscale-users.err >&2
  echo "Headscale did not become ready." >&2
  exit 1
fi

if ! grep -Eq "(^|[[:space:]])${headscale_user}($|[[:space:]])" /tmp/fizrmm-headscale-users.txt; then
  if ! "${compose_cmd}" -f "${compose_file}" exec headscale headscale users create "${headscale_user}"; then
    "${compose_cmd}" -f "${compose_file}" exec headscale headscale users list >/tmp/fizrmm-headscale-users.txt
    if ! grep -Eq "(^|[[:space:]])${headscale_user}($|[[:space:]])" /tmp/fizrmm-headscale-users.txt; then
      echo "Failed to create Headscale user: ${headscale_user}" >&2
      exit 1
    fi
  fi
fi

./scripts/deploy-headscale-key.sh
./scripts/deploy-render.sh
"${compose_cmd}" -f "${compose_file}" up -d --force-recreate rmm-api

echo
echo "Server setup complete."
./scripts/deploy-bootstrap-command.sh
