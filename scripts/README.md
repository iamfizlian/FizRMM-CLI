# Lab Scripts

These scripts wrap the current lab flow so you do not need to remember long Podman commands.

Run from the repository root.

```bash
make compose-up
make lab-headscale-key
make lab-bootstrap-token
make lab-restart-api
make lab-bootstrap-command CONTROL_PLANE_URL=http://<this-pc-ip>:8080
```

Run the printed bootstrap command on a disposable Linux endpoint. The endpoint does not need this repository.

After the endpoint enrolls, `node list` refreshes Headscale state automatically:

```bash
make lab-node-list
```
