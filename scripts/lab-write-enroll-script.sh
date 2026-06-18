#!/usr/bin/env bash
set -euo pipefail

login_server="${1:-}"
if [[ -z "${login_server}" ]]; then
  echo "LOGIN_SERVER is required, example: make lab-write-enroll-script LOGIN_SERVER=http://192.0.2.10:8081" >&2
  exit 1
fi

mkdir -p tmp

podman run --rm --network host \
  -v "${PWD}:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl node enroll-script \
    --os linux \
    --user lab \
    --ttl 1h \
    --tags tag:rmm-agent \
    --login-server "${login_server}" > tmp/enroll-linux.sh

chmod 700 tmp/enroll-linux.sh

echo "Wrote tmp/enroll-linux.sh"
echo "Copy it to the endpoint, then run: sudo bash /tmp/enroll-linux.sh"
