package enrollment

import (
	"fmt"
	"strings"
)

func LinuxScript(loginServer string, authKey string, hostname string, sshUser string, sshPublicKey string) string {
	hostnameLine := `HOSTNAME="$(hostname -f 2>/dev/null || hostname)"`
	if strings.TrimSpace(hostname) != "" {
		hostnameLine = fmt.Sprintf("HOSTNAME=%q", hostname)
	}
	sshSetup := ""
	if strings.TrimSpace(sshPublicKey) != "" {
		if strings.TrimSpace(sshUser) == "" {
			sshUser = "rmm"
		}
		sshSetup = fmt.Sprintf(`
SSH_USER=%q
SSH_PUBLIC_KEY=%q

if ! command -v sshd >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y openssh-server
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y openssh-server
  elif command -v yum >/dev/null 2>&1; then
    sudo yum install -y openssh-server
  elif command -v apk >/dev/null 2>&1; then
    sudo apk add --no-cache openssh-server
  fi
fi

if ! id "${SSH_USER}" >/dev/null 2>&1; then
  sudo useradd --create-home --shell /bin/bash "${SSH_USER}"
fi
sudo install -d -m 700 -o "${SSH_USER}" -g "${SSH_USER}" "/home/${SSH_USER}/.ssh"
printf '%%s\n' "${SSH_PUBLIC_KEY}" | sudo tee "/home/${SSH_USER}/.ssh/authorized_keys" >/dev/null
sudo chown "${SSH_USER}:${SSH_USER}" "/home/${SSH_USER}/.ssh/authorized_keys"
sudo chmod 600 "/home/${SSH_USER}/.ssh/authorized_keys"

if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl enable --now ssh || sudo systemctl enable --now sshd
fi
`, sshUser, sshPublicKey)
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
%s

sudo tailscale up \
  --reset \
  --force-reauth \
  --login-server "${LOGIN_SERVER}" \
  --authkey "${AUTH_KEY}" \
  --hostname "${HOSTNAME}" \
  --ssh=false

tailscale status
`, loginServer, authKey, hostnameLine, sshSetup)
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
