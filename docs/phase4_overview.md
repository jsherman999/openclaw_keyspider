# Phase 4 (planned) overview

Phase 4 is about hardening, broader OS coverage (AIX), operational polish, and richer reporting.

This file documents the proposed changes; **only export formats are implemented now** (per project direction).

## Proposed Phase 4 changes (not implemented yet)

### 1) Hardening & security controls
- RBAC (read-only vs operator vs admin)
- Audit trail for all keyspider-initiated SSH commands
- Pluggable allow/deny lists for hosts, users, paths, and commands
- Safer key-hunt: explicit file pattern allowlist + per-root rate limits
- Encryption policy docs (no private key content stored; only paths + derived public key + fingerprint)

### 2) AIX support
- Parser support for AIX auth logs (e.g., `/var/adm/ras/authlog` depending on syslog config)
- Remote log discovery for AIX (where sshd logs land)
- OS detection logic and per-OS command templates

### 3) Watcher improvements
- True streaming tail:
  - `journalctl -f` cursor support for systemd hosts
  - `tail -F` file offset tracking for non-journald hosts
- Watchers table fully used (enable/disable, per-host mode, cursor)
- Server-sent events (SSE) endpoint for live web console updates

### 4) Reporting & exports (implemented: exports)
- Scheduled scans and scheduled exports
- Report templates per host / per key / per user
- Export formats:
  - JSON (graph + events)
  - CSV (events, keys)
  - GraphML (graph)
  - (future) STIX/TAXII-ish interchange

### 5) Web UI enhancements
- Graph exploration + filtering
- Key detail view with occurrences and locations
- Concern triage workflow (resolve, notes)

## Implemented now (Phase 4 subset)
- CLI export command:
  - graph export in **JSON**, **CSV**, and **GraphML**
- Minimal API export endpoint to download exports.
