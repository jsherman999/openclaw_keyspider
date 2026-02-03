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
- Build image:

```bash
podman build -t localhost/keyspider:latest -f deploy/podman/Containerfile .
```

- If using systemd quadlet: copy `deploy/podman/keyspiderd.container` to `/etc/containers/systemd/` and run `systemctl --user daemon-reload` or system-level reload depending on location.
