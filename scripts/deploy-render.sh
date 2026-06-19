#!/usr/bin/env bash
set -euo pipefail

source ./scripts/env-file.sh
load_env_file .env

required=(
  RMM_DOMAIN
  HEADSCALE_DOMAIN
  POSTGRES_PASSWORD
  RMM_PUBLIC_BASE_URL
)

for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "${name} is required in .env" >&2
    exit 1
  fi
done

mkdir -p deploy/generated
./scripts/deploy-ssh-key.sh

RMM_API_ADDR="${RMM_API_ADDR:-:8080}"
RMM_AUTO_MIGRATE="${RMM_AUTO_MIGRATE:-true}"
RMM_BOOTSTRAP_TOKEN="${RMM_BOOTSTRAP_TOKEN:-}"
RMM_HEADSCALE_API_KEY="${RMM_HEADSCALE_API_KEY:-}"
RMM_MIGRATIONS_DIR="${RMM_MIGRATIONS_DIR:-/migrations}"
RMM_SSH_USER="${RMM_SSH_USER:-rmm}"
RMM_SSH_PORT="${RMM_SSH_PORT:-22}"
RMM_SSH_PUBLIC_KEY="${RMM_SSH_PUBLIC_KEY:-$(cat deploy/generated/ssh/id_ed25519.pub)}"

urlencode() {
  python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

sed_escape() {
  printf '%s' "$1" | sed -e 's/[#&\]/\\&/g'
}

repo_root="$(pwd)"
POSTGRES_PASSWORD_URLENCODED="$(urlencode "${POSTGRES_PASSWORD}")"

sed \
  -e "s#__HEADSCALE_DOMAIN__#$(sed_escape "${HEADSCALE_DOMAIN}")#g" \
  deploy/headscale-config.yaml.template > deploy/generated/headscale-config.yaml

sed \
  -e "s#__REPO_ROOT__#$(sed_escape "${repo_root}")#g" \
  -e "s#__RMM_DOMAIN__#$(sed_escape "${RMM_DOMAIN}")#g" \
  -e "s#__HEADSCALE_DOMAIN__#$(sed_escape "${HEADSCALE_DOMAIN}")#g" \
  -e "s#__POSTGRES_PASSWORD__#$(sed_escape "${POSTGRES_PASSWORD}")#g" \
  -e "s#__POSTGRES_PASSWORD_URLENCODED__#$(sed_escape "${POSTGRES_PASSWORD_URLENCODED}")#g" \
  -e "s#__RMM_API_ADDR__#$(sed_escape "${RMM_API_ADDR}")#g" \
  -e "s#__RMM_AUTO_MIGRATE__#$(sed_escape "${RMM_AUTO_MIGRATE}")#g" \
  -e "s#__RMM_BOOTSTRAP_TOKEN__#$(sed_escape "${RMM_BOOTSTRAP_TOKEN}")#g" \
  -e "s#__RMM_HEADSCALE_API_KEY__#$(sed_escape "${RMM_HEADSCALE_API_KEY}")#g" \
  -e "s#__RMM_MIGRATIONS_DIR__#$(sed_escape "${RMM_MIGRATIONS_DIR}")#g" \
  -e "s#__RMM_PUBLIC_BASE_URL__#$(sed_escape "${RMM_PUBLIC_BASE_URL}")#g" \
  -e "s#__RMM_SSH_USER__#$(sed_escape "${RMM_SSH_USER}")#g" \
  -e "s#__RMM_SSH_PORT__#$(sed_escape "${RMM_SSH_PORT}")#g" \
  -e "s#__RMM_SSH_PUBLIC_KEY__#$(sed_escape "${RMM_SSH_PUBLIC_KEY}")#g" \
  deploy/compose.yml.template > deploy/generated/compose.yml
