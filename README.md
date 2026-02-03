# keyspider

Keyspider maps SSH key usage across an environment from a central jump server.

- Parses SSHD auth logs on Linux (and later AIX) to extract key fingerprints and access metadata.
- Correlates fingerprints to public keys found in `authorized_keys`.
- Optionally hunts for key files on reachable *source* systems (from the jump server only) and records the on-disk location (never stores private key contents).
- Provides CLI, API, and Web UI.

## Phased implementation

### Phase 1 (MVP): inventory + historical report
- CLI: scan a host for recent SSH access events (log pull + parse)
- CLI/API: scan `authorized_keys` and map fingerprints -> keys
- DB schema + migrations (PostgreSQL 13)

### Phase 2: spider graph expansion (from jump only)
- Discover source hosts from events (DNS)
- Attempt enrichment (reachable/unreachable flag)
- Build edges and basic graph queries
- Persist authorized_keys -> fingerprints -> keys
- Best-effort key hunt (private key *locations* only) under allowlisted roots

### Phase 3: watcher (near real-time)
- Watcher service that ingests new access events continuously
- Scroll/search events via API/UI

### Phase 4: hardening + AIX + reporting
- Documented in `docs/phase4_overview.md`
- Implemented now: export formats

## Development

### Requirements
- Go 1.22+
- PostgreSQL 13

### Quick start (dev)
1. Create database and user.
2. Set env:

```bash
export KEYSPIDER_DB_DSN='postgres://keyspider:keyspider@localhost:5432/keyspider?sslmode=disable'
```

3. Run migrations (phase 1 uses the built-in migrator):

```bash
go run ./cmd/keyspiderd migrate
```

4. Start API:

```bash
go run ./cmd/keyspiderd serve
```

5. Run a scan (placeholder in phase 1; will evolve):

```bash
go run ./cmd/keyspider scan --host myserver.example.com --since 168h
```

## Deployment
See `deploy/` for systemd and podman artifacts.
