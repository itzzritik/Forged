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

- Socket ownership and `0600` perms are the main access control. On Linux and macOS, the daemon also requires the kernel-reported peer UID and clients compare the kernel peer UID/PID with a stable `daemon.pid` record before marshaling or sending a request. A verified endpoint mismatch blocks readiness repair, service freshening, and account-service restart instead of restarting against a live untrusted socket. Sensitive operations still add broker checks on top. Windows control-pipe server identity still needs a trusted executable-identity design.
- Mutable JSON and framing buffers are cleared after use to shorten sensitive-data lifetime; the compatible wire format still uses transient JSON strings.
- Vault-backed handlers can be called while the daemon is cold; they should return a locked error, not panic.
- Ordinary vault handlers require an active broker session. A fresh export authorization establishes that session and issues its separate one-use export token.
- The listener retries temporary accept failures with bounded backoff, exits quietly when closed, and logs terminal failures. Shutdown first closes admission and every tracked connection, then waits for admitted handlers.
- Client calls have a context-aware API. Closing or timing out the one-request connection cancels the matching server request; dispatch stays synchronous so shutdown still waits for admitted work.
- Hydration-based authorization carries the request context through verification and vault recovery. A disconnect, timeout, or write failure rolls back a newly hydrated session and scoped token before it can persist; SSH route preparation keeps that delivery fence through its retry response.
- `proto/ipc.md` is not current. Code is the source of truth for the command set.
- `sensitive-auth` takes an `action` and optional `force`. Its shared-session actions expire stale state first; `force=true` then skips the initial active-session fast path and follows normal reauthorization policy. `private-key` and `export` are password-only. Normal TUI launch sends `view` with `false` and reuses a valid shared session.
- Full private-key views require a separate one-use, short-lived token issued only after verified master-password authorization. A shared SSH/signing session alone never authorizes PEM delivery.
- `status` exposes sensitive session state and daemon build id so the TUI/readiness layer can detect cold, active, and stale daemon states.
- Key list, single-key view/export, and full-vault export handlers ask the sync bus for a lightweight foreground refresh before reading local vault data.
- Key removal uses the exact reviewed name and can bind the request to its reviewed fingerprint so a stale confirmation cannot remove a different key.
- Manual sync captures the encrypted blob, KDF parameters, and protected key from one vault snapshot.
- Manual sync uses a two-minute IPC deadline on both client and daemon, so its bounded 30-second push, pull, and retry-push conflict path can complete.
- The Manual Sync caller does not refresh account credentials; the daemon-owned sync token source refreshes them so a save-failed rotation remains available to the process that performs the sync.
- A refresh-only account sends a fixed non-secret placeholder in Manual Sync's legacy-required `token` field; current daemons ignore it and use the daemon-owned token source.
- Manual sync fails closed when the stateful sync bus is unavailable; it never performs a stateless push over unknown remote data.
- A missing remote vault after prior linked history is a retryable status error; local sync state retains that condition across restart and the client never recreates the remote automatically.
- When sync history is quarantined, `status` still exposes its safe recovery error without a sync bus and manual sync returns that error instead of suggesting a restart.
- When saved account credentials cannot be read, `status.sync.last_error` and manual sync return fixed safe credential-store guidance without a sync bus, including if a credential fault detaches the active bus during Manual Sync. Sync-state recovery guidance takes priority.
- `status` advertises `account_change_protocol`. Account actions repair or restart the managed service until it supports the required protocol, then use versioned replace and clear commands with no direct-write fallback. The commands serialize credential and sync publication state; remote reconciliation runs afterward as bounded background work.
- Account replace v4 and clear v4 can return successful cleanup-pending data after credentials commit. `sync_cleanup_pending` and `credential_secret_cleanup_pending` are independent; the latter covers credential secrets and related local artifacts. Clients treat either as committed, show guidance, and do not retry retained state or reusable credential slots. Versioned commands prevent older clients from silently ignoring a new result.
- Hidden SSH route IPC prepares per-attempt snippets from `%C`, `%h`, `%p`, `%r`, and `%n`; prepare and success require a positive client PID and success matches it with `%C`. Prepare failures are quiet so the managed SSH config fails closed with no default identities. Preparation expires a stale broker session at entry, then route service rechecks the session immediately before each private probe, direct-probe signature, or learned-proof write; a locked session falls back to public cached candidates without prompting. Authorization and either probe strategy share one 45-second server work context, the hook waits 50 seconds, and the server connection deadline leaves a final response margin.
- If route prepare finds no public route cache because the daemon is cold, IPC runs external auth once and retries prepare after hydration.
- TUI SSH-route diagnostics require an active broker session; clearing still calls the route service so vault tombstones and sync mutation handling stay correct. The public route cache remains available only to normal route preparation after lock.
- On Windows, a failed current-token pipe identity lookup is returned directly to IPC callers; it is not reported as a stopped daemon.
- On Windows, the daemon does not register a route handler. Stale hidden routing helpers fail before IPC and direct route requests return unavailable rather than accepting an untrusted client PID.
- Windows IPC support is still incomplete.

## Decisions

- Keep the flat JSON-over-socket model. The surface is small enough that gRPC/codegen is not worth it.
