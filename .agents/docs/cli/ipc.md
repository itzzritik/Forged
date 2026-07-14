---
title: Daemon IPC
applies_to:
  - cli/internal/ipc/**
last_verified: 2026-07-15
stable: yes
---

# Daemon IPC

`ctl.sock` is a one-request control channel between `forged` and the daemon. One connection carries one request and one response.

## Must know

- Socket ownership and `0600` perms are the main access control. Sensitive operations still add broker checks on top.
- Mutable JSON and framing buffers are cleared after use to shorten sensitive-data lifetime; the compatible wire format still uses transient JSON strings.
- Vault-backed handlers can be called while the daemon is cold; they should return a locked error, not panic.
- The listener retries temporary accept failures with bounded backoff, exits quietly when closed, and logs terminal failures. Shutdown first closes admission and every tracked connection, then waits for admitted handlers.
- Client calls have a context-aware API. Closing or timing out the one-request connection cancels the matching server request; dispatch stays synchronous so shutdown still waits for admitted work.
- `proto/ipc.md` is not current. Code is the source of truth for the command set.
- `sensitive-auth` takes an `action` and optional `force`. Its non-export requests expire stale shared state first; `force=true` then skips the initial active-session fast path and follows normal reauthorization policy. Normal TUI launch sends `false` and reuses a valid shared session.
- `status` exposes sensitive session state and daemon build id so the TUI/readiness layer can detect cold, active, and stale daemon states.
- Key list/view/export handlers ask the sync bus for a lightweight foreground refresh before reading local vault data.
- Key removal uses the exact reviewed name and can bind the request to its reviewed fingerprint so a stale confirmation cannot remove a different key.
- Manual sync captures the encrypted blob, KDF parameters, and protected key from one vault snapshot.
- Manual sync fails closed when the stateful sync bus is unavailable; it never performs a stateless push over unknown remote data.
- `status` advertises `account_change_protocol`. Account actions repair or restart the managed service until it supports the required protocol, then use versioned replace and clear commands with no direct-write fallback. The commands serialize credential and sync publication state; remote reconciliation runs afterward as bounded background work.
- Hidden SSH route IPC prepares per-attempt snippets from `%C`, `%h`, `%p`, `%r`, and `%n`; prepare failures are quiet so the managed SSH config fails closed with no default identities. Authorization and either probe strategy share one 45-second server work context, the hook waits 50 seconds, and the server connection deadline leaves a final response margin.
- If route prepare finds no public route cache because the daemon is cold, IPC runs external auth once and retries prepare after hydration.
- TUI diagnostics use SSH route list/clear IPC. Clearing routes must call the route service so vault tombstones and sync mutation handling stay correct.
- On Windows, a failed current-token pipe identity lookup is returned directly to IPC callers; it is not reported as a stopped daemon.
- On Windows, the daemon does not register a route handler. Stale hidden routing helpers fail before IPC and direct route requests return unavailable rather than accepting an untrusted client PID.
- Windows IPC support is still incomplete.

## Decisions

- Keep the flat JSON-over-socket model. The surface is small enough that gRPC/codegen is not worth it.
