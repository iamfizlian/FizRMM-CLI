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
- `podman-compose` on Ubuntu is supported and is the default used by this Makefile.
- Ports `80` and `443` free on the server.

## DNS And TLS

Use two DNS names:

```text
rmm.example.com        A/AAAA -> your server public IP
headscale.example.com  A/AAAA -> your server public IP
```

Both names can point to the same server. Caddy routes traffic by hostname:

```text
https://rmm.example.com        -> rmm-api
https://headscale.example.com  -> Headscale
```

TLS is handled automatically by Caddy through Let's Encrypt. For that to work:

- `RMM_DOMAIN` and `HEADSCALE_DOMAIN` must resolve publicly to this server.
- TCP `80` must be reachable for ACME HTTP-01 validation.
- TCP `443` must be reachable for HTTPS.
- No other service can already be bound to ports `80` or `443`.

If the server is behind a cloud firewall or VPS security group, open:

```text
80/tcp    Caddy HTTP and Let's Encrypt validation
443/tcp   Caddy HTTPS for rmm-api and Headscale
3478/udp  Headscale STUN for NAT traversal
```

Do not expose PostgreSQL.

## Headscale Domain

`HEADSCALE_DOMAIN` is the public URL that endpoint Tailscale clients use as their login server.

Example `.env`:

```env
RMM_DOMAIN=rmm.example.com
HEADSCALE_DOMAIN=headscale.example.com
RMM_PUBLIC_BASE_URL=https://rmm.example.com
```

The generated Headscale config sets:

```yaml
server_url: https://headscale.example.com
```

Endpoint bootstrap scripts then run Tailscale with:

```bash
tailscale up --login-server https://headscale.example.com ...
```

Changing `HEADSCALE_DOMAIN` after nodes are enrolled may require re-enrolling or reconfiguring those nodes, so choose it intentionally.

## First Deploy

```bash
cp deploy/.env.example .env
```

Edit `.env`:

- `RMM_DOMAIN`
- `HEADSCALE_DOMAIN`
- `POSTGRES_PASSWORD`
- `RMM_PUBLIC_BASE_URL`

Use a long random PostgreSQL password. The deploy renderer URL-encodes it for `RMM_DATABASE_URL`, but avoid newlines.

Generate a bootstrap token:

```bash
make deploy-bootstrap-token
```

Start the server and finish the initial Headscale/RMM wiring:

```bash
make deploy-up
make deploy-init
```

Enroll the RMM server itself as a Headscale controller so it can reach endpoint tailnet IPs for SSH actions:

```bash
make deploy-controller-enroll
```

Run the printed `curl -fsSL ... | sudo bash` command on an endpoint.

After the endpoint enrolls, listing nodes refreshes Headscale state automatically:

```bash
podman run --rm --network host \
  -v "$PWD:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl --api-url "https://${RMM_DOMAIN}" node list
```

Run an audited SSH command against an enrolled node:

```bash
podman run --rm --network host \
  -v "$PWD:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl --api-url "https://${RMM_DOMAIN}" exec --node fiz-ubu-acdx -- hostname
```

Run a built-in diagnostic:

```bash
podman run --rm --network host \
  -v "$PWD:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl --api-url "https://${RMM_DOMAIN}" node check --node fiz-ubu-acdx summary
```

Open the terminal operator UI:

```bash
podman run --rm -it --network host \
  -v "$PWD:/src:Z" \
  -w /src docker.io/library/golang:1.22 \
  go run ./cmd/rmmctl --api-url "https://${RMM_DOMAIN}" tui
```
