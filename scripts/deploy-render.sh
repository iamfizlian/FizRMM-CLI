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

sed \
  -e "s#__HEADSCALE_DOMAIN__#${HEADSCALE_DOMAIN}#g" \
  deploy/headscale-config.yaml.template > deploy/generated/headscale-config.yaml
