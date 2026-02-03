# Deployment notes

## systemd (Ubuntu/RHEL9)
- Build and install binaries to `/opt/keyspider/`.
- Place config at `/etc/keyspider/keyspider.yaml`.
- Create a service user:

```bash
sudo useradd --system --home /var/lib/keyspider --shell /usr/sbin/nologin keyspider
sudo install -d -o keyspider -g keyspider /var/lib/keyspider
```

- Install unit:

```bash
sudo install -m 0644 deploy/systemd/keyspiderd.service /etc/systemd/system/keyspiderd.service
sudo systemctl daemon-reload
sudo systemctl enable --now keyspiderd
```

## macOS
macOS does not natively use systemd. Use the provided launchd plist:

- Copy `deploy/launchd/com.keyspider.daemon.plist` to `~/Library/LaunchAgents/` (per-user) or `/Library/LaunchDaemons/` (system).
- Load it with `launchctl`.

## Podman

### Build image

```bash
podman build -t localhost/keyspider:latest -f deploy/podman/Containerfile .
```

### Run container (simple)

```bash
podman run --rm -p 8080:8080 \
  -e KEYSPIDER_DB_DSN='postgres://keyspider:keyspider@host.containers.internal:5432/keyspider?sslmode=disable' \
  localhost/keyspider:latest
```

Notes:
- `host.containers.internal` works on many Podman setups; adjust to your DB host.
- The image includes `openssh-client` so the container can SSH to targets.
- Mount SSH config/keys as needed for your environment.

### systemd Quadlet
Copy `deploy/podman/keyspiderd.container` to `/etc/containers/systemd/` (system) or `~/.config/containers/systemd/` (user), then:

```bash
systemctl daemon-reload
systemctl enable --now keyspiderd
```

If using the **user** location:

```bash
systemctl --user daemon-reload
systemctl --user enable --now keyspiderd
```
