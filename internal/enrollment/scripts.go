package enrollment

import (
	"fmt"
	"strings"
)

func LinuxScript(loginServer string, authKey string, hostname string) string {
	hostnameLine := `HOSTNAME="$(hostname -f 2>/dev/null || hostname)"`
	if strings.TrimSpace(hostname) != "" {
		hostnameLine = fmt.Sprintf("HOSTNAME=%q", hostname)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

LOGIN_SERVER=%q
AUTH_KEY=%q
%s

if ! command -v tailscale >/dev/null 2>&1; then
  curl -fsSL https://tailscale.com/install.sh | sh
fi

if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now tailscaled
fi

sudo tailscale up \
  --reset \
  --force-reauth \
  --login-server "${LOGIN_SERVER}" \
  --authkey "${AUTH_KEY}" \
  --hostname "${HOSTNAME}" \
  --ssh=false

tailscale status
`, loginServer, authKey, hostnameLine)
}

func WindowsScript(loginServer string, authKey string, hostname string) string {
	hostnameLine := `$Hostname = $env:COMPUTERNAME`
	if strings.TrimSpace(hostname) != "" {
		hostnameLine = fmt.Sprintf("$Hostname = %q", hostname)
	}

	return fmt.Sprintf(`$ErrorActionPreference = "Stop"

$LoginServer = %q
$AuthKey = %q
%s

tailscale up --login-server $LoginServer --authkey $AuthKey --hostname $Hostname
tailscale status
`, loginServer, authKey, hostnameLine)
}
