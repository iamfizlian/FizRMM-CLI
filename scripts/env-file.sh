#!/usr/bin/env bash

load_env_file() {
  local env_file="${1:-.env}"

  if [[ ! -f "${env_file}" ]]; then
    echo "${env_file} is missing. Run: cp deploy/.env.example .env, then edit it." >&2
    return 1
  fi

  local line key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" != *=* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"

    if [[ ! "${key}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      echo "invalid env key in ${env_file}: ${key}" >&2
      return 1
    fi

    export "${key}=${value}"
  done < "${env_file}"
}
