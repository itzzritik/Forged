---
title: Architecture Overview
applies_to:
  - "**"
last_verified: 2026-04-21
stable: yes
---

# Architecture Overview

SSH agent replacement with encrypted local vault, cloud sync, TUI, and web
dashboard. Keys stay encrypted at rest; decrypted only in daemon memory
while unlocked. Server never sees plaintext keys or the master password.

Four binaries: `forged` (CLI+TUI), `forged-sign` (git signer),
`forged-auth` (platform biometric helper — real on macOS, stub elsewhere),
`forged-server` (Go HTTP API on Postgres).

Protocol specs live in `proto/` — `ipc.md`, `sync-api.md`,
`vault-format.md`. Shards link; do not duplicate.

## Must know

- **TUI runs in the `forged` process, not the daemon.** It drives the
  daemon over `ctl.sock`; the daemon renders nothing.
- **Web `/api/*` is a transparent proxy** to `forged-server`. No business
  logic on the web side.
- **`forged-auth` is two programs behind one name** — Swift target on
  macOS, Go stub elsewhere. Build flows differ per OS.
- **Sync fans out via a debounced event bus.** Callers post events; they
  do not call the sync engine directly.
- Legacy top-level commands (`agent`, `key`, `login`, etc.) still parse
  and print TUI-redirect hints. Do not reuse those names.
- IF the daemon is not running THEN `forged-sign` cannot sign — no fallback.
- IF `credentials.json` is missing/invalid THEN sync is off but the local
  agent and IPC still serve.

## Decisions

- Agent and control sockets stay split. Merging leaks internal IPC to any
  SSH client.
- Biometric auth lives in a separate helper so the daemon stays
  non-interactive and the macOS `LocalAuthentication` flow stays inside a
  signed Swift target.
