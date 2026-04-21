---
title: Daemon IPC
applies_to:
  - cli/internal/ipc/**
last_verified: 2026-04-21
stable: yes
---

# Daemon IPC

Control channel between `forged` CLI/TUI and the daemon. One request per
connection, flat command-string dispatch, one response, close. No
streaming, no server push. Wire format in `proto/ipc.md`.

## Must know

- **`proto/ipc.md` is out of date.** It lists commands that no longer
  exist (`host`, `unhost`, `hosts`, `lock`, `unlock`, `sync-status`,
  `config-get`, `config-set`) and omits ones that do (`rename`, `view`,
  `generate`, `export-all`, `sensitive-*`, `ssh-route-*`, `sync-link`,
  `sync-unlink`). Code is the source of truth.
- **Windows IPC is not actually hosted.** Path helper returns a named-pipe
  string, but the server uses Go `unix` network + `chmod`. Treat Windows
  as unsupported until a platform shim lands.
- **Authentication is ambient** — owner-only `0600` perms on `ctl.sock`
  are the whole access control. No token. Sensitive ops still go through
  the sensitive-auth broker on top.
- **No client-side retry.** Transient daemon restart surfaces as an
  immediate error; TUI re-dials on the next tick.
- **10 MB response cap.** Bulk exports of large vaults can approach it;
  fix is streaming, not raising the cap.
- Request deadline is 60s default, 5 min for `sensitive-auth` /
  `sensitive-password` (biometric / password prompts).
- IF a handler's subsystem is not wired (sync bus, auth broker, SSH
  routing, link/unlink callbacks) THEN the handler returns "unavailable",
  not panic.
- IF the daemon is not running THEN every call fails fast with "daemon is
  not running" — no autostart from the IPC layer.
- Mutating key commands notify the sync bus and refresh SSH routing after
  the handler returns.

## Decisions

- Unix socket + length-prefixed JSON over gRPC/HTTP/protobuf. Surface is
  tiny, single-host, single-process-per-user. Do NOT add a codegen
  pipeline until the surface outgrows a flat command switch.
- Agent and control sockets stay split. See `architecture/overview.md`.
- No bearer token on top of owner-only perms — pure complexity, no
  threat-model gain (attacker with socket read has token read too).
