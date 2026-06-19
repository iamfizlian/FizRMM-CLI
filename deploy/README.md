# FizRMM Server Deployment

This profile is for a reachable server, usually a small VPS.

## Requirements

- DNS records:
  - `RMM_DOMAIN` points to the server.
  - `HEADSCALE_DOMAIN` points to the server.
- Inbound firewall:
  - TCP `80`
  - TCP `443`
  - UDP `3478`
- Podman with Compose support.
- Ports `80` and `443` free on the server.

## First Deploy

```bash
cp deploy/.env.example .env
```

Edit `.env`:

- `RMM_DOMAIN`
- `HEADSCALE_DOMAIN`
- `POSTGRES_PASSWORD`
- `RMM_PUBLIC_BASE_URL`

Generate a bootstrap token:

```bash
make deploy-bootstrap-token
```

Start the server:

```bash
make deploy-up
```

Create the Headscale user once:

```bash
podman compose -f deploy/compose.yml --env-file .env exec headscale headscale users create lab
```

Create the Headscale API key for `rmm-api`:

```bash
make deploy-headscale-key
make deploy-restart-api
```

Print the endpoint bootstrap command:

```bash
make deploy-bootstrap-command
```

Run the printed `curl -fsSL ... | sudo bash` command on an endpoint.

After the endpoint enrolls:

```bash
podman run --rm --network host \
  -v "$PWD:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl --api-url "https://${RMM_DOMAIN}" overlay nodes sync
```
