#!/usr/bin/env bash
set -euo pipefail

token="$(openssl rand -hex 24)"

tmp="$(mktemp)"
if [[ -f .env ]]; then
  grep -v '^RMM_BOOTSTRAP_TOKEN=' .env > "${tmp}" || true
fi
printf 'RMM_BOOTSTRAP_TOKEN=%s\n' "${token}" >> "${tmp}"
mv "${tmp}" .env

echo "Wrote a bootstrap token to .env"
echo "Next: make deploy-up"
