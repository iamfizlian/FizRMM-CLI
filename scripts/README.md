# Lab Scripts

These scripts wrap the current lab flow so you do not need to remember long Podman commands.

Run from the repository root.

```bash
make compose-up
make lab-headscale-key
make lab-restart-api
make lab-enroll-script LOGIN_SERVER=http://<this-pc-ip>:8081
```

Run the generated enrollment script on a disposable Linux VM or test machine.

After the endpoint enrolls:

```bash
make lab-sync-nodes
make lab-node-list
```
