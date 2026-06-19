# FizRMM CLI

Terminal-first open source RMM prototype.

This repository is starting from the MVP path in [terminal_rmm_open_source_plan.md](terminal_rmm_open_source_plan.md):

```text
Enroll node -> see node -> SSH to node -> run job -> collect inventory -> show metrics -> audit everything
```

## Current Status

Scaffold only. The first useful target is a local lab with:

- `rmmctl --help`
- `rmm-api --version`
- PostgreSQL
- Headscale
- Prometheus
- Alertmanager
- Loki

Go is required to build the binaries. Rootless Podman with Compose support is the preferred lab runtime.

## Repository Layout

```text
cmd/rmmctl/                 CLI entrypoint
cmd/rmm-api/                API entrypoint
configs/headscale/          Headscale lab configuration placeholder
configs/prometheus/         Prometheus lab configuration
docs/                       Operator and lab notes
internal/version/           Shared version metadata
migrations/                 PostgreSQL schema migrations
terminal_rmm_open_source_plan.md
```

## Build

```bash
go build ./cmd/rmmctl
go build ./cmd/rmm-api
```

## Run

```bash
go run ./cmd/rmmctl --help
go run ./cmd/rmm-api --version
```

Local API with PostgreSQL:

```bash
RMM_DATABASE_URL='postgres://fizrmm:fizrmm-lab-password@localhost:5432/fizrmm?sslmode=disable' \
RMM_AUTO_MIGRATE=true \
go run ./cmd/rmm-api
```

List nodes through the API:

```bash
go run ./cmd/rmmctl node list
go run ./cmd/rmmctl --json node list
```

Lab shortcut commands:

```bash
make compose-up
podman exec fizrmm-cli_headscale_1 headscale users create lab
make lab-headscale-key
make lab-bootstrap-token
make lab-restart-api
make lab-bootstrap-command CONTROL_PLANE_URL=http://<this-pc-ip>:8080
make lab-node-list
```

Start the lab:

```bash
podman compose up -d
```

See [docs/lab.md](docs/lab.md) for the current lab sequence and validation notes.

Validate Compose:

```bash
podman compose config
```

## Next Engineering Steps

1. Run `gofmt`, `go mod tidy`, and `go test ./...` on a machine with Go installed.
2. Validate the lab with `podman compose config` and `podman compose up -d`.
3. Expand migrations for operators, RBAC, enrollment keys, policies, and artifacts.
4. Enroll a disposable test node and verify `node list` shows it.
5. Add audited SSH command execution.
