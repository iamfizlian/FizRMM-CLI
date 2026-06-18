# Terminal-Based Open Source RMM Plan

_Last updated: 2026-06-18_

## Executive Summary

The best architecture for a terminal-based open source RMM is **not one monolithic RMM product**. It is a controlled combination of focused open source components:

- **Headscale + Tailscale clients** for the private management overlay network.
- **SSH / WinRM / PowerShell** for direct administrative access.
- **Ansible**, with **Salt** for always-on agent behavior.
- **Prometheus or VictoriaMetrics** for metrics.
- **Alertmanager** for alert routing.
- **Loki or VictoriaLogs** for logs.
- **Vector** for log and telemetry shipping.
- **osquery** for endpoint inventory and security-oriented state collection.
- **PostgreSQL** as the main RMM state database.
- **NetBox** as an optional network source of truth for sites, devices, prefixes, VLANs, and tenant metadata.
- **OpenBao or SOPS + age** for secrets.
- **Bubble Tea, Ratatui, or Textual** for the terminal UI.

The key update from the WireGuard discussion is:

> **Use Headscale instead of manually managed WireGuard for the RMM overlay. WireGuard remains the underlying secure tunnel technology.**

Headscale should be treated as the fleet coordination/control-plane layer: enrollment, node identity, peer discovery, DNS, tags, routes, access policy, and API integration. Raw WireGuard should remain available for static site-to-site tunnels, routers, break-glass access, or simple fixed topology deployments.

---

## Design Goals

The RMM should be:

1. **Terminal-first**
   - Fast keyboard-driven TUI.
   - Scriptable CLI.
   - JSON output mode for automation.
   - No hard dependency on a web UI.

2. **Self-hosted and open source**
   - Prefer permissive or well-understood open source licenses.
   - Avoid SaaS-only control planes.
   - Avoid proprietary agent lock-in.

3. **Secure by default**
   - No inbound ports on managed endpoints.
   - Private overlay access only.
   - Deny-by-default network policy.
   - Per-tenant segmentation.
   - Short-lived enrollment keys.
   - Full command/audit logging.

4. **Linux-first, mixed-environment capable**
   - Linux and BSD managed well from day one.
   - Windows via WinRM, PowerShell, WMI-compatible tooling, and optional GUI fallback.
   - macOS support through SSH, MDM-adjacent scripts, and osquery where appropriate.

5. **Composable**
   - No attempt to rewrite Prometheus, Ansible, Headscale, NetBox, or Loki.
   - The RMM should orchestrate and unify them behind a terminal workflow.

## Non-Goals

The first implementation should stay intentionally narrow.

- Do not build a full EDR/XDR product.
- Do not replace a PSA, ticketing system, or billing platform.
- Do not make a web UI required for core operations.
- Do not expose SSH, WinRM, Prometheus exporters, Loki, or PostgreSQL publicly.
- Do not build a custom endpoint agent until SSH/WinRM/Ansible/osquery limits are proven.
- Do not support every operating system and package manager in the MVP.
- Do not treat GUI remote control as part of the trusted core path.
- Do not rely on one flat shared tenant network for MSP production use.

---

## Recommended Architecture

```text
                           Operator Workstation
                         rmmctl / TUI / SSH client
                                    |
                                    | Headscale/Tailscale overlay
                                    v
+-----------------------------------------------------------------------+
|                           RMM Control Plane                            |
|                                                                       |
|  rmm-api / rmmctl backend                                             |
|  - Inventory service                                                  |
|  - Job scheduler                                                      |
|  - Command broker                                                     |
|  - Tenant/site model                                                  |
|  - Audit log                                                          |
|  - Headscale API integration                                          |
|  - Prometheus/VictoriaMetrics integration                             |
|  - Loki/VictoriaLogs integration                                      |
|                                                                       |
|  Data stores:                                                         |
|  - PostgreSQL: inventory, jobs, audit, tenants                         |
|  - Object/file storage: scripts, artifacts, reports                    |
|  - OpenBao or SOPS/age: secrets                                       |
+-----------------------------------------------------------------------+
                                    |
                                    | Headscale/Tailscale private network
                                    v
+-----------------------------------------------------------------------+
|                           Managed Endpoints                            |
|                                                                       |
|  Linux/BSD:                                                           |
|  - tailscaled                                                         |
|  - sshd                                                               |
|  - node_exporter or vmagent                                           |
|  - Vector or Fluent Bit                                               |
|  - osquery                                                            |
|  - optional Salt minion                                               |
|                                                                       |
|  Windows:                                                             |
|  - Tailscale client                                                   |
|  - WinRM / PowerShell Remoting                                        |
|  - windows_exporter                                                   |
|  - Vector Windows Event Log source                                    |
|  - osquery                                                            |
|  - optional MeshCentral/RustDesk GUI fallback                         |
+-----------------------------------------------------------------------+
```

---

## Headscale vs Raw WireGuard Update

### Decision

Use **Headscale + Tailscale client** as the primary RMM overlay network.

Do **not** replace WireGuard conceptually. Headscale does not replace WireGuard as a tunnel protocol. Instead, it replaces the manual operational burden around WireGuard:

- Key distribution
- Peer configuration
- IP assignment
- Node discovery
- DNS naming
- ACL/grant policy
- NAT traversal coordination
- Route publication
- Node onboarding and revocation

### Why Headscale Makes Sense for RMM

RMM endpoints are often behind NAT, dynamic ISP links, CGNAT, LTE/5G, home routers, hotel networks, customer firewalls, and changing Wi-Fi networks. Raw WireGuard can work, but it leaves too much operational work to the RMM platform.

Headscale gives the RMM an overlay layer with:

- API-driven device and user management.
- Pre-authenticated keys for unattended enrollment.
- Tags for non-human/service devices.
- DNS/MagicDNS-style naming.
- Subnet router and exit node support.
- Policy via grants/ACLs.
- DERP relay support when direct peer-to-peer connection fails.

### Raw WireGuard Still Has a Place

Keep raw WireGuard for:

- Static site-to-site links.
- Router/firewall deployments where Tailscale clients are undesirable.
- Very small fixed networks.
- Break-glass tunnels.
- Environments requiring the smallest possible dependency chain.

### Decision Matrix

| Use case | Prefer Headscale | Prefer Raw WireGuard |
|---|---:|---:|
| Many moving endpoints | Yes | No |
| Customer laptops behind NAT | Yes | No |
| Unattended enrollment | Yes | Possible, but custom |
| DNS names per node | Yes | Custom |
| API-driven node lifecycle | Yes | Custom |
| Static site-to-site | Optional | Yes |
| Router-to-router tunnel | Optional | Yes |
| Minimal attack surface | Maybe | Yes |
| Multi-tenant MSP isolation | Yes, but carefully | Per-tunnel possible |

---

## Headscale Deployment Model

### Recommended Tenant Strategy

For an RMM/MSP-style model, avoid one flat shared tailnet unless this is strictly a homelab or internal-only deployment.

Preferred options:

#### Option A: One Headscale Instance Per Tenant

```text
headscale-customer-a.example.com
headscale-customer-b.example.com
headscale-customer-c.example.com
```

**Pros**

- Strongest tenant isolation.
- Cleaner blast-radius boundaries.
- Easier customer offboarding.
- Easier policy reasoning.

**Cons**

- More instances to operate.
- More DERP/routing/cert automation.
- More backup/upgrade coordination.

#### Option B: One Shared Headscale Instance With Strict Policy

```text
headscale.example.com
  tag:tenant-a
  tag:tenant-b
  tag:tenant-c
  tag:rmm-controller
  tag:monitoring
```

**Pros**

- Operationally simpler.
- Easier centralized monitoring.
- Fewer public services.

**Cons**

- Policy mistakes can create cross-tenant exposure.
- Harder to prove isolation.
- Higher blast radius.

#### Recommendation

For a serious RMM design:

- Use **one Headscale instance per tenant** for MSP/customer environments.
- Use **one shared Headscale instance** only for internal IT, lab, or single-organization deployments.
- For early MVP, start with one instance but design the database schema and policy model so tenant-per-Headscale is easy later.

---

## Headscale Policy Model

Headscale defaults are not sufficient for RMM. The RMM should install and manage a deny-by-default policy from the beginning.

### Tag Model

Suggested tags:

```text
tag:rmm-controller       # RMM control plane nodes
tag:rmm-operator         # Admin/operator workstations
tag:rmm-agent            # Managed endpoints
tag:monitoring           # Prometheus/VictoriaMetrics scrapers
tag:logging              # Log collectors or aggregators
tag:tenant-<name>        # Tenant/customer identity
tag:site-<name>          # Site/location identity
tag:subnet-router        # Nodes advertising site routes
tag:breakglass           # Emergency access nodes
```

### Network Policy Principles

1. Operators should not automatically reach every endpoint directly.
2. The RMM controller should reach endpoints only on required management ports.
3. Monitoring should scrape metrics ports only.
4. Logging should receive/pull logs only where required.
5. Endpoints should not laterally communicate with each other by default.
6. Tenant tags must never be allowed to cross by accident.

### Example Directional Policy

The following block is intent-oriented pseudocode. Validate exact syntax against the Headscale/Tailscale policy format and version in use before applying it. Production policy examples should include the tested Headscale version, parser/linter result, and CI test output.

```jsonc
{
  "grants": [
    {
      "src": ["tag:rmm-controller"],
      "dst": ["tag:rmm-agent"],
      "ip": ["22", "5985", "5986"]
    },
    {
      "src": ["tag:monitoring"],
      "dst": ["tag:rmm-agent"],
      "ip": ["9100", "9182", "9104"]
    },
    {
      "src": ["tag:rmm-operator"],
      "dst": ["tag:rmm-controller"],
      "ip": ["22", "443"]
    }
  ],
  "ssh": [
    {
      "action": "accept",
      "src": ["tag:rmm-operator"],
      "dst": ["tag:rmm-controller"],
      "users": ["rmm-admin"]
    }
  ]
}
```

### RMM Policy Guardrails

The RMM should lint every policy before applying it:

- Reject wildcard `*:*` rules except in explicitly marked lab mode.
- Reject tenant-to-tenant grants.
- Reject endpoint-to-endpoint grants unless explicitly scoped.
- Require a change reason and operator identity.
- Store every policy version in Git.
- Run policy tests in CI before reload.

---

## DERP / Relay Planning

Headscale/Tailscale-style overlays try direct connections first and use DERP relay when direct peer-to-peer connectivity fails.

For production RMM, plan DERP intentionally.

### MVP DERP

```text
headscale.example.com
  - Headscale control server
  - Embedded DERP enabled
  - TLS via Caddy/Traefik/Nginx
  - UDP/3478 for STUN
```

### Production DERP

```text
headscale.example.com
  - Headscale API/control server

derp-east.example.com
  - DERP relay
  - STUN UDP/3478

derp-west.example.com
  - DERP relay
  - STUN UDP/3478
```

### DERP Recommendations

- Do not rely on a single DERP relay for production.
- Keep public Tailscale DERP fallback only if the privacy/security model allows it.
- Monitor relay latency, throughput, and failed direct-connection rates.
- Keep DERP separate from the RMM API if scale or availability matters.
- Alert when a large percentage of sessions are relayed instead of direct.

---

## Recommended Tool Stack

### Core Connectivity

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Overlay network | Headscale + Tailscale client | NetBird, Firezone | Headscale is preferred because it is narrow, scriptable, and pairs well with terminal tooling. |
| Static tunnels | WireGuard | OpenVPN | Use for site-to-site and break-glass, not broad endpoint fleet management. |
| Relay/NAT fallback | DERP | TURN-like relay models | Run at least two relays for production. |
| DNS inside overlay | Headscale DNS/MagicDNS-style naming | CoreDNS | Keep names predictable for terminal workflows. |

### Command Execution and Automation

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Linux/BSD remote shell | OpenSSH | Tailscale SSH | Use SSH over the overlay; keep OS-level audit logs. |
| Windows command execution | WinRM + PowerShell | OpenSSH on Windows | WinRM is the practical baseline for Windows automation. |
| Config management | Ansible | Salt | Ansible is simpler for MVP. Salt is better if always-on event-driven execution becomes necessary. |
| Fleet jobs | RMM job runner wrapping Ansible/SSH/WinRM | Salt reactor | Keep job state in PostgreSQL. |
| Network automation | Ansible network collections, Nornir | Netmiko, Scrapli | Useful for routers/switches/firewalls, especially for a network-engineer-focused RMM. |

### Monitoring and Metrics

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Metrics server | Prometheus | VictoriaMetrics | Prometheus is simplest; VictoriaMetrics is attractive for larger retention/cardinality. |
| Linux metrics | node_exporter | vmagent host metrics | Standard baseline. |
| Windows metrics | windows_exporter | Telegraf | Needed for CPU, memory, disk, service, and process metrics. |
| Blackbox checks | blackbox_exporter | Gatus | Useful for ping, TCP, HTTP, TLS checks. |
| Alert routing | Alertmanager | Grafana Alerting, vmalert | Alertmanager is the standard pairing with Prometheus. |

### Logs and Events

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Log pipeline agent | Vector | Fluent Bit, Grafana Alloy | Vector is strong for cross-platform pipelines and transformations. |
| Log store | Loki | VictoriaLogs, OpenSearch | Loki is operationally lighter than OpenSearch for many RMM use cases. |
| Linux logs | journald/file sources | rsyslog | Ship auth, sudo, system, package, and service logs. |
| Windows logs | Windows Event Log source | Winlogbeat | Capture security, system, application, PowerShell, and WinRM logs. |

### Inventory and Endpoint State

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Endpoint inventory | osquery | Ansible facts, Salt grains | osquery is useful for software, users, processes, interfaces, and security posture. |
| Network source of truth | NetBox | Nautobot | NetBox is excellent for sites, devices, prefixes, VLANs, tenants, and automation source-of-truth use. |
| RMM operational database | PostgreSQL | SQLite for MVP | PostgreSQL should be used once there are multiple operators or tenants. |
| Search | PostgreSQL full text | Meilisearch/OpenSearch | Start simple. |

### Secrets and Identity

| Function | Primary | Alternative | Notes |
|---|---|---|---|
| Static secrets in Git | SOPS + age | git-crypt | Great for bootstrap and small deployments. |
| Runtime secrets | OpenBao | Infisical, Bitwarden Secrets Manager | Prefer OpenBao over HashiCorp Vault for open-source purity. |
| Operator auth | OIDC | Local users for lab only | Use Keycloak, Authentik, Zitadel, or existing IdP. |
| SSH certificates | Smallstep CA | OpenSSH CA scripts | Better than long-lived SSH keys. |

### Terminal UI

| Language | TUI Framework | Best fit |
|---|---|---|
| Go | Bubble Tea | Strong CLI ecosystem, easy static binaries. Recommended default. |
| Rust | Ratatui | Excellent performance and safety, more implementation overhead. |
| Python | Textual | Fastest prototyping, heavier runtime footprint. |

Recommended TUI approach:

```text
rmmctl                # scriptable CLI
rmmctl tui            # terminal UI
rmmctl node list
rmmctl node ssh <node>
rmmctl job run <playbook> --target tag:site-nyc
rmmctl metrics top --tenant customer-a
rmmctl logs tail <node>
rmmctl headscale node list
```

---

## MVP Architecture

Start with a focused MVP that proves secure terminal-based operations without trying to build every RMM feature.

### MVP Components

```text
Core:
  - Headscale
  - Tailscale clients
  - PostgreSQL
  - rmm-api
  - rmmctl CLI/TUI

Automation:
  - SSH for Linux/BSD
  - WinRM for Windows
  - Ansible for jobs

Observability:
  - Prometheus
  - node_exporter
  - windows_exporter
  - Alertmanager

Inventory:
  - osquery
  - Ansible facts

Secrets:
  - SOPS + age initially
  - OpenBao later
```

### MVP Features

1. Register tenant/site.
2. Generate Headscale pre-auth key.
3. Enroll endpoint.
4. Show endpoint online/offline status.
5. Open SSH/WinRM session through overlay.
6. Run ad hoc command.
7. Run Ansible playbook.
8. Collect inventory.
9. Scrape metrics.
10. Show alerts in terminal.
11. Tail logs for a node.
12. Record all jobs and operator actions.

### MVP Acceptance Criteria

The MVP should not be considered complete until these checks pass in a repeatable lab:

1. A fresh lab can be started from documented commands.
2. One Linux node can enroll with a short-lived Headscale pre-auth key.
3. The node appears in `rmmctl node list` with tenant, site, hostname, tailnet IP, and online status.
4. `rmmctl node ssh <node>` opens an audited management session over the overlay.
5. `rmmctl exec` can run a command and records stdout, stderr, exit code, actor, target, and timestamps.
6. `rmmctl job run` can execute one Ansible playbook against a tag selector.
7. Inventory collection records OS, packages, interfaces, disks, users, and services.
8. Prometheus scrapes node metrics through the overlay.
9. One basic alert appears in `rmmctl alerts list`.
10. Every operator action creates an audit event.
11. A backup and restore test preserves tenants, nodes, jobs, audit events, and Headscale state.
12. A policy test proves that endpoint-to-endpoint and tenant-to-tenant traffic is denied by default.

---

## Production Architecture

Once the MVP is stable, move toward a more durable deployment.

```text
Public edge:
  - Caddy/Traefik/Nginx
  - TLS automation
  - WAF/rate limits where appropriate

Control plane:
  - rmm-api replicas
  - rmm-worker pool
  - PostgreSQL HA/backups
  - Redis/NATS for job queue if needed
  - Headscale per tenant or segmented shared Headscale

Overlay:
  - Dedicated Headscale server(s)
  - DERP east/west
  - DNS integration
  - Policy-as-code repo

Observability:
  - Prometheus or VictoriaMetrics
  - Alertmanager
  - Loki or VictoriaLogs
  - Grafana optional, not required for terminal workflows

Secrets:
  - OpenBao
  - Short-lived credentials
  - SSH CA

Audit:
  - Append-only audit table
  - Export to logs
  - Signed job artifacts
```

---

## Endpoint Enrollment Flow

### Linux Enrollment

This example is a bootstrap sketch, not a production installer. A production installer should detect distro/package manager, verify package signatures, retry transient failures, be idempotent, write logs, support rollback/offboarding, and report enrollment status back to the RMM API.

```bash
#!/usr/bin/env bash
set -euo pipefail

HEADSCALE_URL="https://headscale.example.com"
AUTH_KEY="${RMM_AUTH_KEY:?missing auth key}"

curl -fsSL https://tailscale.com/install.sh | sh

tailscale up \
  --login-server "${HEADSCALE_URL}" \
  --authkey "${AUTH_KEY}" \
  --hostname "$(hostname -f 2>/dev/null || hostname)" \
  --ssh=false

# Baseline packages
apt-get update || true
apt-get install -y openssh-server prometheus-node-exporter osquery vector || true

systemctl enable --now ssh || true
systemctl enable --now prometheus-node-exporter || true
systemctl enable --now osqueryd || true
systemctl enable --now vector || true
```

### Windows Enrollment

This example assumes Tailscale is already installed or installed by an external software deployment tool. A production enrollment path should provide MSI/winget/Chocolatey options, event logging, WinRM hardening, firewall profile checks, rollback, and offboarding.

```powershell
$HeadscaleUrl = "https://headscale.example.com"
$AuthKey = $env:RMM_AUTH_KEY

# Install Tailscale client using your preferred package source.
# Example package managers: winget, Chocolatey, Intune, GPO, PDQ, or manual MSI deployment.

tailscale up `
  --login-server $HeadscaleUrl `
  --authkey $AuthKey `
  --hostname $env:COMPUTERNAME

Enable-PSRemoting -Force
Set-Service WinRM -StartupType Automatic
Start-Service WinRM
```

### Enrollment Security Requirements

- Use short-lived pre-auth keys.
- Use reusable keys only for tightly controlled imaging/provisioning workflows.
- Assign tags at enrollment where possible.
- Rotate keys regularly.
- Expire unused keys automatically.
- Record key creator, purpose, tenant, and expiration.
- Do not embed long-lived keys in public scripts.

---

## RMM Data Model

Minimum PostgreSQL entities:

```text
tenants
  id
  name
  slug
  headscale_instance_id
  created_at

sites
  id
  tenant_id
  name
  timezone
  tags

nodes
  id
  tenant_id
  site_id
  hostname
  fqdn
  os_family
  os_version
  architecture
  headscale_node_id
  tailnet_ip
  last_seen_at
  status
  tags

node_interfaces
  id
  node_id
  name
  mac
  ip_addresses
  mtu

software_packages
  id
  node_id
  name
  version
  source
  installed_at

jobs
  id
  tenant_id
  created_by
  type
  status
  target_selector
  command_or_playbook
  created_at
  started_at
  finished_at

job_results
  id
  job_id
  node_id
  exit_code
  stdout_ref
  stderr_ref
  started_at
  finished_at

alerts
  id
  tenant_id
  source
  labels
  severity
  status
  starts_at
  ends_at

audit_events
  id
  tenant_id
  actor
  action
  target_type
  target_id
  metadata
  created_at
```

Additional entities should be added before multi-operator or multi-tenant production use:

```text
operators
  id
  external_subject
  display_name
  email
  status
  created_at

roles
  id
  name
  permissions

operator_tenant_roles
  operator_id
  tenant_id
  role_id

api_tokens
  id
  tenant_id
  name
  token_hash
  scopes
  expires_at
  created_by

headscale_instances
  id
  tenant_id
  base_url
  status
  version
  created_at

enrollment_keys
  id
  tenant_id
  site_id
  headscale_key_id
  tags
  reusable
  expires_at
  created_by
  revoked_at

policy_versions
  id
  tenant_id
  version
  content_ref
  lint_status
  applied_at
  applied_by
  change_reason

scripts
  id
  tenant_id
  name
  content_ref
  checksum
  signed_by
  created_at

maintenance_windows
  id
  tenant_id
  name
  target_selector
  starts_at
  ends_at
  timezone

alert_silences
  id
  tenant_id
  alert_id
  reason
  starts_at
  ends_at
  created_by

job_artifacts
  id
  job_id
  node_id
  kind
  object_ref
  checksum
  created_at
```

---

## Terminal UX Plan

The TUI should optimize for triage and action.

### Primary Screens

```text
Dashboard
  - Active alerts
  - Offline nodes
  - Recent failed jobs
  - DERP/overlay health
  - Patch/inventory drift summary

Tenants
  - Tenant list
  - Tenant status
  - Headscale instance health

Nodes
  - Filter by tenant/site/tag/status
  - Online/offline
  - OS/version
  - Last seen
  - Tailnet IP
  - Active alerts

Node Detail
  - Identity
  - Interfaces
  - Metrics snapshot
  - Recent logs
  - Installed packages
  - Running services
  - Open shell
  - Run job

Jobs
  - Running jobs
  - Scheduled jobs
  - History
  - Per-node results

Alerts
  - Grouped by tenant/site/severity
  - Acknowledge/silence
  - Jump to node/logs/job

Logs
  - Tail by node
  - Search by tenant/site/tag
  - Filter auth/sudo/WinRM/PowerShell/service logs

Overlay
  - Headscale nodes
  - DERP status
  - Policy status
  - Pre-auth keys
```

### Example Commands

```bash
rmmctl tenant list
rmmctl site list --tenant acme
rmmctl node list --tenant acme --status offline
rmmctl node ssh acme-web-01
rmmctl node winrm acme-win-01
rmmctl node inventory acme-web-01
rmmctl job run patch-linux.yml --target 'tenant=acme,tag=linux'
rmmctl logs tail acme-web-01 --unit ssh
rmmctl alerts list --severity critical
rmmctl overlay nodes --tenant acme
rmmctl overlay preauth create --tenant acme --ttl 1h --tags tag:rmm-agent,tag:tenant-acme
```

---

## Patch Management Strategy

### Linux

Use Ansible modules or distro-native tools:

- Debian/Ubuntu: `apt`, `unattended-upgrades` optional.
- RHEL/Rocky/Alma/Fedora: `dnf`/`yum`.
- SUSE: `zypper`.
- Arch: `pacman`, with more caution.

Patch workflow:

1. Inventory packages.
2. Classify security updates.
3. Create patch plan.
4. Run dry-run/check mode where supported.
5. Patch canary group.
6. Patch production group.
7. Reboot if required.
8. Verify services and metrics.
9. Store job output and package delta.

### Windows

Options:

- PowerShell `PSWindowsUpdate` module.
- Windows Update Agent API.
- WSUS integration for larger environments.
- Ansible Windows modules for orchestration.

Patch workflow:

1. Check pending updates.
2. Approve or filter update classes.
3. Install on canary group.
4. Reboot if needed.
5. Validate service health.
6. Store KB/update history.

### Network Devices

For a network-engineer-friendly RMM, add network device support separately from endpoint patching:

- Inventory via NetBox/NAPALM/Nornir.
- Backup configs before changes.
- Validate candidate config.
- Push during maintenance windows.
- Confirm post-change reachability.
- Diff startup/running config.

---

## Security Model

### Threat Model

Primary assets:

- Tenant boundary and overlay policy.
- Operator identity and session tokens.
- Headscale API keys and enrollment keys.
- SSH/WinRM credentials and certificates.
- Job output, scripts, playbooks, and audit records.
- Inventory, package, service, and user data.

Important trust boundaries:

- Operator workstation to RMM API.
- RMM API to Headscale API.
- RMM worker to managed endpoint.
- Managed endpoint to metrics/log pipeline.
- Tenant A resources to tenant B resources.
- Public internet to Headscale/DERP/API edge.

Abuse cases to design against:

- A stolen pre-auth key enrolls an attacker-controlled node.
- A compromised operator workstation runs jobs across many tenants.
- A malicious endpoint attempts lateral movement over the overlay.
- A bad policy change exposes one tenant to another tenant.
- A compromised RMM API mints enrollment keys or schedules jobs.
- A compromised package or script runs through the trusted job runner.
- A DERP relay becomes overloaded or reveals sensitive metadata.

Required mitigations:

- Short-lived scoped enrollment keys.
- Per-tenant RBAC and just-in-time elevation for destructive actions.
- Deny-by-default overlay policy with CI tests.
- Immutable or append-only audit export.
- Signed scripts/playbooks for high-risk actions.
- Alerting for unusual enrollment, policy, job, and auth behavior.
- Backup/restore testing and key rotation procedures.

### Network Security

- No public SSH/WinRM exposure.
- All management traffic uses the Headscale/Tailscale overlay.
- Deny endpoint-to-endpoint communication by default.
- Use tenant/site tags in policy.
- Use separate Headscale instances for high-risk tenant boundaries.
- Monitor DERP use and unusual node-to-node flows.

### Identity and Access

- OIDC for operators.
- MFA enforced by identity provider.
- Per-tenant role-based access.
- Just-in-time elevation for destructive actions.
- Separate break-glass account path.
- SSH certificates instead of static SSH keys where possible.

### Endpoint Security

- Least-privilege service accounts.
- Do not run all RMM jobs as root/Administrator by default.
- Use signed scripts/playbooks.
- Store command output securely.
- Log every remote shell open and job execution.
- Detect suspicious use of RMM tools.

### Secrets

- No secrets in plaintext config files.
- SOPS + age for Git-managed bootstrap secrets.
- OpenBao for runtime secrets as the system matures.
- Rotate Headscale API keys.
- Rotate enrollment keys.
- Separate tenant secrets.

### Supply Chain

- Pin package versions for agents where possible.
- Verify checksums/signatures.
- Build custom agent packages in CI.
- Keep SBOMs for internally built binaries.
- Record exact versions deployed per node.

### RBAC Model

Minimum roles:

| Role | Scope | Capabilities |
|---|---|---|
| Owner | System or tenant | Manage tenants, roles, policies, secrets, and destructive actions. |
| Admin | Tenant | Manage sites, nodes, jobs, enrollment keys, alerts, and maintenance windows. |
| Operator | Tenant/site | Run approved jobs, open shells where allowed, view inventory/logs/alerts. |
| Read-only | Tenant/site | View nodes, inventory, metrics, logs, jobs, and audit records. |
| Break-glass | Time-limited | Emergency access with mandatory reason, alerting, and audit export. |

RBAC requirements:

- Every API request must carry actor, tenant, role, and request id.
- Destructive actions require explicit reason and elevated permission.
- Cross-tenant queries are system-admin only.
- Break-glass access must expire automatically.
- Local lab users are acceptable only for non-production.

---

## Build vs Integrate Boundary

The RMM should **build glue, workflow, policy, and terminal UX**. It should **not** rebuild mature infrastructure components.

### Build

- `rmmctl` CLI/TUI.
- RMM API.
- Tenant/site/node/job model.
- Job scheduler and audit trail.
- Headscale integration layer.
- Inventory normalization.
- Alert/log/metric terminal views.
- Policy linter and guardrails.
- Enrollment orchestration.

### Integrate

- Headscale for overlay control.
- Tailscale client for endpoint overlay networking.
- OpenSSH/WinRM for execution.
- Ansible/Salt for automation.
- Prometheus/VictoriaMetrics for metrics.
- Loki/VictoriaLogs for logs.
- Vector/Fluent Bit for collection.
- osquery for endpoint state.
- NetBox for network source of truth.
- OpenBao/SOPS for secrets.

---

## Agentless vs Agent Decision

Default to agentless execution for the MVP:

- SSH for Linux/BSD.
- WinRM/PowerShell for Windows.
- Ansible for orchestration.
- osquery/exporters/Vector for state, metrics, and logs.

Consider a lightweight custom agent only when at least one of these becomes a real blocker:

- Endpoints are frequently offline and need queued local execution.
- NAT or identity constraints prevent reliable SSH/WinRM.
- The platform needs real-time event streaming beyond osquery/log shippers.
- Local privilege separation is required for safer self-service actions.
- Windows automation over WinRM is too fragile for target environments.

If an agent is added later, keep it narrow: enrollment heartbeat, local job queue, script execution with signed payloads, artifact upload, and health reporting.

---

## API Contract Sketch

Initial API resources:

```text
GET    /healthz
GET    /v1/tenants
POST   /v1/tenants
GET    /v1/sites
POST   /v1/sites
GET    /v1/nodes
GET    /v1/nodes/{id}
POST   /v1/enrollment-keys
POST   /v1/jobs
GET    /v1/jobs
GET    /v1/jobs/{id}
GET    /v1/jobs/{id}/results
GET    /v1/inventory/nodes/{id}
GET    /v1/alerts
POST   /v1/alerts/{id}/silence
GET    /v1/audit-events
GET    /v1/overlay/nodes
POST   /v1/policies/lint
POST   /v1/policies/apply
```

API requirements:

- JSON request/response bodies.
- Stable error envelope with code, message, request id, and details.
- Tenant id required for tenant-scoped writes.
- Pagination for list endpoints.
- Audit event emitted for every write and shell/job action.
- OIDC/JWT validation in production.

---

## Tool Alternatives Worth Tracking

### NetBird

NetBird is a strong alternative to Headscale if you want an integrated open source zero-trust overlay with more built-in management UI concepts. It may be more complete out of the box, but Headscale is cleaner if the goal is a narrow control plane that the terminal RMM owns.

### Firezone

Firezone is strong for least-privilege remote access to resources and user/application access workflows. It is less directly aligned with a fleet-device RMM model than Headscale, but worth watching for secure access patterns.

### MeshCentral

MeshCentral can provide remote desktop, remote terminal, file management, and device management. It is useful as an optional GUI/remote-control fallback, especially for helpdesk-like functions. Do not expose it broadly; treat it as a high-risk administrative tool.

### RustDesk

RustDesk can be used as a self-hosted remote desktop fallback. Keep it optional and separate from the core terminal RMM. Prefer SSH/WinRM for routine administration.

---

## Implementation Roadmap

### Phase 0 — Repo and Lab Foundation

- Create mono-repo or coordinated repos.
- Choose language: Go recommended for static `rmmctl` and API binaries.
- Define a Podman Compose lab.
- Deploy PostgreSQL.
- Deploy Headscale.
- Deploy Prometheus and Alertmanager.
- Add one Linux node and one Windows node.
- Add initial API/CLI skeleton.
- Add lint/test commands.
- Add backup/restore notes.

Deliverable:

```text
rmmctl node list
rmmctl overlay nodes
rmmctl node ssh <linux-node>
```

Done means:

- `podman compose config` validates.
- `rmmctl --help` runs.
- `rmm-api --version` runs.
- Lab config is documented.
- Version choices are pinned or explicitly marked as placeholders.

### Phase 1 — Overlay Enrollment

- Headscale API integration.
- Tenant/site model.
- Pre-auth key creation.
- Linux enrollment script.
- Windows enrollment script.
- Node sync from Headscale into RMM database.
- Basic deny-by-default policy template.
- Enrollment key audit events.
- Enrollment/offboarding documentation.

Deliverable:

```text
rmmctl overlay preauth create --tenant lab --ttl 1h
rmmctl node enroll-script --os linux
rmmctl node list --online
```

Done means:

- Pre-auth keys expire automatically.
- Enrollment tags are tenant/site scoped.
- Revoked keys cannot enroll nodes.
- Offboarding removes overlay access and marks the node inactive.

### Phase 2 — Command and Job Execution

- SSH command runner.
- WinRM command runner.
- Ansible playbook runner.
- Job status tracking.
- Per-node output capture.
- Audit events.
- Job timeout and cancellation.
- Artifact storage references.

Deliverable:

```text
rmmctl exec --target node-01 -- uname -a
rmmctl job run baseline.yml --target tag:linux
rmmctl job logs <job-id>
```

Done means:

- Job output is stored per node.
- Failed and partial jobs are represented clearly.
- Every command records actor, tenant, target, reason, timestamps, and exit code.

### Phase 3 — Inventory

- osquery deployment.
- Ansible fact collection.
- Normalize OS, package, interface, disk, service, and user data.
- Store inventory snapshots.
- Add inventory diff view.
- Store snapshot metadata and source freshness.

Deliverable:

```text
rmmctl node inventory node-01
rmmctl node packages node-01
rmmctl inventory diff node-01 --since 7d
```

Done means:

- Inventory collection is repeatable.
- Diffs identify package, interface, service, user, and disk changes.
- Stale inventory is visible in the terminal UI.

### Phase 4 — Monitoring

- node_exporter/windows_exporter deployment.
- Prometheus scrape config generation from RMM inventory.
- Alertmanager integration.
- Terminal alert screen.
- Basic alert rules.
- Overlay scrape target generation.

Deliverable:

```text
rmmctl metrics top --cpu
rmmctl alerts list
rmmctl alerts silence <alert-id> --duration 1h
```

Done means:

- Metrics flow only across the overlay.
- Alerts include tenant/site/node labels.
- Silences are audited and expire automatically.

### Phase 5 — Logging

- Vector deployment.
- Loki or VictoriaLogs deployment.
- Linux journald collection.
- Windows Event Log collection.
- Terminal log search/tail.
- Retention and tenant label strategy.

Deliverable:

```text
rmmctl logs tail node-01
rmmctl logs search --tenant acme 'sudo OR sshd'
```

Done means:

- Logs include tenant/site/node labels.
- Sensitive log fields are reviewed before storage.
- Log retention is documented.

### Phase 6 — Patch Management

- Linux package update workflow.
- Windows update workflow.
- Canary groups.
- Maintenance windows.
- Reboot detection.
- Post-patch verification.
- Rollback notes where realistic.

Deliverable:

```text
rmmctl patch plan --tenant acme --os linux
rmmctl patch apply --plan <plan-id> --canary
rmmctl patch status <plan-id>
```

Done means:

- Patch actions require maintenance window or explicit override.
- Canary failures stop broader rollout.
- Reboot-required state is detected and reported.

### Phase 7 — Multi-Tenant Hardening

- Headscale per tenant support.
- Policy-as-code tests.
- Tenant isolation checks.
- OpenBao integration.
- SSH CA integration.
- Immutable audit export.
- Backup/restore drill.
- Cross-tenant isolation test suite.

Deliverable:

```text
rmmctl tenant create acme --headscale dedicated
rmmctl policy test --tenant acme
rmmctl audit export --tenant acme --since 30d
```

Done means:

- Tenant-to-tenant overlay access tests fail closed.
- Shared infrastructure access is documented.
- Audit export can be verified for integrity.

---

## Initial Podman Compose Lab

Recommended lab services:

```yaml
services:
  postgres:
    image: postgres:16

  headscale:
    image: headscale/headscale:latest

  prometheus:
    image: prom/prometheus:latest

  alertmanager:
    image: prom/alertmanager:latest

  loki:
    image: grafana/loki:latest

  grafana:
    image: grafana/grafana:latest
    profiles: ["optional"]

  rmm-api:
    build: ./rmm-api

  rmm-worker:
    build: ./rmm-worker
```

For production, pin versions instead of using `latest`.

Use rootless Podman as the preferred local lab runtime. Keep the Compose file compatible enough that Docker remains a fallback where necessary, but validate the primary path with Podman.

The lab should eventually include:

- Named volumes for PostgreSQL, Headscale, Prometheus, Loki, and Grafana.
- Healthchecks for all long-running services.
- Config mounts for Headscale, Prometheus, Alertmanager, Loki, and Vector.
- A private Podman network for control-plane services.
- A documented way to join an external VM or container as a managed node.
- Backup and restore commands for PostgreSQL and Headscale state.
- No default production secrets.

---

## Failure Modes

Design explicit behavior for these cases:

| Failure | Expected behavior |
|---|---|
| Headscale unavailable | Existing overlay sessions may continue, but enrollment and node sync fail with clear errors. |
| DERP relay unavailable | Direct connections continue where possible; relayed connection failures are surfaced in overlay health. |
| PostgreSQL unavailable | API rejects writes, workers pause new jobs, and no audit events are silently dropped. |
| RMM worker crashes mid-job | Job is marked interrupted or timed out and can be retried safely. |
| Endpoint offline | Job result remains pending until timeout; operator can filter offline targets. |
| Bad policy candidate | Linter/test failure blocks apply. |
| Bad policy already applied | Roll back to previous stored policy version. |
| Secrets backend unavailable | New secret reads fail closed; cached secrets must have short TTLs. |
| Metrics/log backend unavailable | Command execution continues, but observability degradation is visible. |

---

## Backup and Restore

Minimum backup scope:

- PostgreSQL database.
- Headscale database/configuration.
- Policy-as-code repository.
- Object storage for job artifacts, reports, and scripts.
- SOPS/OpenBao bootstrap material and recovery keys.
- Prometheus/Loki data if historical observability is required.

Minimum restore drill:

1. Restore PostgreSQL into a clean lab.
2. Restore Headscale state/configuration.
3. Start `rmm-api` and `rmm-worker`.
4. Verify tenant, node, job, inventory, and audit data.
5. Verify existing nodes can reconnect or are clearly marked for re-enrollment.
6. Run one read-only command and one inventory collection.
7. Record restore time and any manual steps.

---

## Testing Strategy

- Unit tests for policy linting, target selectors, RBAC checks, and API validation.
- Integration tests for PostgreSQL migrations and repository queries.
- CLI golden-output tests for JSON mode.
- Lab tests for Headscale enrollment and node sync.
- Job runner tests for success, failure, timeout, cancellation, and partial target failure.
- Security tests for tenant isolation, denied endpoint-to-endpoint access, and destructive action authorization.
- Backup/restore test before any production deployment.

---

## Licensing Review

Before packaging or distributing the project, verify license compatibility for:

- Headscale.
- Tailscale clients.
- Prometheus, Alertmanager, node_exporter, and windows_exporter.
- Loki or VictoriaLogs.
- Vector or Fluent Bit.
- osquery.
- Ansible and optional Salt.
- OpenBao, SOPS, and age.
- Bubble Tea, Ratatui, or Textual.

Record license, source URL, version, redistribution constraints, and whether the component is a runtime dependency, build dependency, or optional integration.

---

## Open Questions

1. Should the first implementation target internal IT only or MSP multi-tenancy?
2. Should the agentless model remain the default, or should a lightweight custom agent be added later?
3. Should Salt be introduced early for event-driven execution, or deferred until Ansible/SSH shows limits?
4. Should NetBox be mandatory or optional?
5. How much Windows support is required in the first release?
6. Should GUI remote control be included, or kept as an external integration?

---

## Recommended Next Build Step

Build the MVP around this minimal path:

```text
Headscale + Tailscale client
PostgreSQL
Go rmmctl CLI/TUI
SSH executor
Ansible runner
Prometheus + node_exporter
osquery inventory collection
```

Do this before adding logs, Windows patching, GUI fallback, or Salt. The first proof should be:

```text
Enroll node -> see node -> SSH to node -> run job -> collect inventory -> show metrics -> audit everything
```

---

## References

- Headscale features: https://headscale.net/stable/about/features/
- Headscale clients: https://headscale.net/stable/about/clients/
- Headscale registration: https://headscale.net/stable/ref/registration/
- Headscale policy: https://headscale.net/stable/ref/policy/
- Headscale DERP: https://headscale.net/stable/ref/derp/
- Headscale API: https://headscale.net/stable/ref/api/
- Tailscale technical overview: https://tailscale.com/docs/concepts/what-is-tailscale
- WireGuard conceptual overview: https://www.wireguard.com/
- Prometheus overview: https://prometheus.io/docs/introduction/overview/
- Alertmanager: https://prometheus.io/docs/alerting/latest/alertmanager/
- Ansible introduction: https://docs.ansible.com/projects/ansible/latest/getting_started/introduction.html
- Salt introduction: https://docs.saltproject.io/en/latest/topics/index.html
- Grafana Loki: https://grafana.com/docs/loki/latest/
- Vector documentation: https://vector.dev/docs/
- VictoriaMetrics documentation: https://docs.victoriametrics.com/
- NetBox documentation: https://netboxlabs.com/docs/netbox/
- OpenBao documentation: https://openbao.org/docs/
- NetBird documentation: https://docs.netbird.io/
- Firezone documentation: https://www.firezone.dev/kb
