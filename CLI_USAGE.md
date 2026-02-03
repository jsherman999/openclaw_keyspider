# Keyspider CLI usage

This file shows common operator workflows for **keyspider**.

Keyspider is designed to run on (or adjacent to) a **central jump server** that has passwordless SSH access to managed targets.

> Notes
> - Keyspider **never stores private key contents**. If it finds private keys on disk, it records the **path** and (when possible) a derived public key fingerprint.
> - “Spidering” expands from a destination host to its observed sources (DNS-based), but **all probing is done from the jump server**.

---

## 0) Prereqs

### Database (PostgreSQL 13)
Set the DB DSN (env or config):

```bash
export KEYSPIDER_DB_DSN='postgres://keyspider:keyspider@localhost:5432/keyspider?sslmode=disable'
```

### SSH
Keyspider shells out to `ssh`.

- Ensure the jump server user can run:
  - `ssh root@target` (or set `ssh.user` in config)
- Ensure host keys and SSH config are in place (ProxyJump, etc. if needed).

---

## 1) Apply DB migrations

```bash
go run ./cmd/keyspiderd migrate --config ./keyspider.example.yaml
```

(When installed as binaries: `keyspiderd migrate --config /etc/keyspider/keyspider.yaml`)

---

## 2) Scan a single host (historical)

Scan the last 7 days of sshd logs for **Accepted publickey** events, persist them, and ingest `authorized_keys` from the destination:

```bash
go run ./cmd/keyspider scan --host server1.example.com --since 168h
```

You’ll see a summary like:

- events inserted
- keys seen in `authorized_keys`
- edges created
- concerns raised (e.g. unreachable sources)

---

## 3) Spider outward from a host

Scan a host and then follow discovered sources up to N hops (DNS-based), probing each *reachable* source host from the jump server:

```bash
go run ./cmd/keyspider scan --host server1.example.com --since 72h --spider-depth 2
```

Use this to build the “spider web” graph from a starting point.

---

## 4) Watch hosts in near real-time (daemon)

### Configure watcher
Edit your config (example):

```yaml
watcher:
  enabled: true
  hosts:
    - server1.example.com
    - server2.example.com
  default_mode: auto    # auto|journal|tail
  host_modes:           # optional overrides
    aix1.example.com: tail
  dedupe_window: 256
```

### Start daemon

```bash
go run ./cmd/keyspiderd serve --config ./keyspider.example.yaml
```

The watcher will:
- stream logs using `journalctl -f` when available (with cursor resume)
- fall back to `tail -F` when journalctl is unavailable
- dedupe repeated log lines (in-memory window + restart-safe last-hash)

---

## 5) View data via API / Web UI

Start the daemon, then:

- Health:
  - `curl http://127.0.0.1:8080/healthz`
- Hosts:
  - `curl http://127.0.0.1:8080/hosts`
- Recent events for a host id:
  - `curl 'http://127.0.0.1:8080/events?host_id=1'`
- Live watcher stream (SSE):
  - `curl -N http://127.0.0.1:8080/watch/events`

Web UI:
- Open `http://127.0.0.1:8080/`
  - live event console
  - edge list from the graph export

---

## 6) Export graph data

### CLI export

- JSON:

```bash
go run ./cmd/keyspider export --format json --out graph.json
```

- CSV (edges):

```bash
go run ./cmd/keyspider export --format csv --out edges.csv
```

- GraphML:

```bash
go run ./cmd/keyspider export --format graphml --out graph.graphml
```

### API export

```bash
curl -o graph.json 'http://127.0.0.1:8080/export/graph?format=json&limit=10000'
curl -o edges.csv  'http://127.0.0.1:8080/export/graph?format=csv&limit=10000'
curl -o graph.graphml 'http://127.0.0.1:8080/export/graph?format=graphml&limit=10000'
```

---

## 7) Common analysis patterns

### A) “Who accessed this server recently?”
1) Run:

```bash
go run ./cmd/keyspider scan --host server1.example.com --since 24h
```

2) Use UI or:

```bash
curl 'http://127.0.0.1:8080/hosts'
# find host_id for server1
curl 'http://127.0.0.1:8080/events?host_id=<id>'
```

### B) “Which sources are suspicious/unreachable from jump?”
Run a spider scan and look at `concerns` (Phase 2/3 currently records concerns in DB). Exporting concerns is planned; for now query DB directly.

### C) “Where does this key exist on disk?”
Key locations are stored in `key_instances`.

- `instance_type=authorized_key` indicates the key was authorized on a destination account.
- `instance_type=private` indicates a private key file was found (path recorded, contents not stored).

(Adding a dedicated CLI/API query for this is planned.)
