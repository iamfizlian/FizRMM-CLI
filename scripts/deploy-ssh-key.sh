#!/usr/bin/env bash
set -euo pipefail

key_dir="deploy/generated/ssh"
key_file="${key_dir}/id_ed25519"

mkdir -p "${key_dir}"
chmod 700 "${key_dir}"

if [[ -f "${key_file}" && -f "${key_file}.pub" ]]; then
  exit 0
fi

if ! command -v ssh-keygen >/dev/null 2>&1; then
  echo "ssh-keygen is required to create the RMM endpoint SSH key." >&2
  exit 1
fi

ssh-keygen -t ed25519 -N "" -C "fizrmm-endpoint-access" -f "${key_file}" >/dev/null
chmod 600 "${key_file}"
chmod 644 "${key_file}.pub"

echo "Generated RMM endpoint SSH key at ${key_file}"
