# Lab Notes

The lab is intentionally small and should prove the MVP loop before broader RMM features are added.

## Prerequisites

- Go 1.22 or newer.
- Rootless Podman with Compose support.
- A VM or test host that can install the Tailscale client.

## Validate The Scaffold

```bash
go test ./...
podman compose config
```

## Start Control Plane Services

```bash
make compose-up
```

## Check API

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/v1/version
curl http://localhost:8080/metrics
```

## Headscale

The current Headscale config is a lab placeholder pinned to the Compose image. Validate it before treating the lab as working:

```bash
podman compose logs headscale
```

Create the lab Headscale user once:

```bash
podman exec fizrmm-cli_headscale_1 headscale users create lab
```

Prepare API access for `rmm-api`:

```bash
make lab-headscale-key
make lab-restart-api
```

## Enroll One Test Node

The next real milestone is enrolling one disposable endpoint. Use a Linux VM, spare laptop, or test host. Do not enroll your main workstation unless you intentionally want to change its Tailscale state.

Generate the enrollment script:

```bash
make lab-enroll-script LOGIN_SERVER=http://<this-pc-ip>:8081
```

Run the generated script on the disposable Linux endpoint.

Or write the script to a file:

```bash
make lab-write-enroll-script LOGIN_SERVER=http://<this-pc-ip>:8081
```

Copy `tmp/enroll-linux.sh` to the endpoint. For example:

```bash
scp tmp/enroll-linux.sh user@<endpoint-ip>:/tmp/enroll-linux.sh
ssh user@<endpoint-ip> 'sudo bash /tmp/enroll-linux.sh'
```

After the endpoint enrolls, sync Headscale into the RMM database:

```bash
make lab-sync-nodes
make lab-node-list
```

Expected result: `make lab-node-list` shows the enrolled endpoint with hostname, status, tenant id, and tailnet IP.

If the endpoint is running on the same machine as the lab, `localhost` may work:

```bash
make lab-enroll-script LOGIN_SERVER=http://localhost:8081
```

For a VM or another machine on your LAN, use this PC's LAN IP instead.
