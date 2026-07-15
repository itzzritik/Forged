---
title: Daemon
applies_to:
  - cli/internal/daemon/**
  - cli/cmd/forged/cmd/daemon.go
  - cli/internal/platform/mlock_*.go
  - cli/internal/platform/socket_unix.go
  - cli/internal/platform/peer*.go
depends_on:
  - architecture/security-model.md
  - cli/ipc.md
last_verified: 2026-07-15
stable: partial
---

# Daemon

The daemon is the long-running per-user process behind SSH agent access, IPC, sync, and sensitive-auth session state.

## Must know

- The daemon hosts two sockets:
  - `agent.sock` for SSH clients
  - `ctl.sock` for CLI/TUI control
- It now boots cold by default. Installed services and foreground `forged daemon` no longer depend on a stored plaintext master password.
- A foreground daemon accepts an optional startup password only from an explicit non-terminal stdin stream, never an environment variable; it trims one LF or CRLF delimiter and clears the buffer after startup hydration.
- A live vault session exists only after sensitive auth or password fallback hydrates it.
- A foreground startup password hydrates through the broker, so it establishes the same active session as an interactive password unlock.
- A delivered master-password authorization refreshes local unlock enrollment from the active vault session; a canceled or failed response does not change that durable trust.
- When the shared session is cleared, the daemon drops back to cold state.
- Session clear immediately cancels and detaches initial-link and sync-bus work before closing the vault. Stopped buses discard completion-state writes; old work drains separately, and a generation plus vault-identity fence prevents stale link or bus work from applying a vault merge or publishing a bus or sync-state metadata after its ownership is revoked. Revocation waits only for an already admitted bounded local commit.
- Account and sync-state transitions revoke detached local work outside the live-session lock. Pre-link credential work drains independently after its initialization run is canceled; once network reconciliation starts, transitions cancel, gate, and join that link before changing account state. Their credential and state-file commits use a validated, fingerprinted state snapshot without holding the live-session lock; only the bounded pre-detach dirty transition is persisted while it is held so a crash cannot lose it.
- Sync initialization reserves a generation- and vault-fenced run while holding the live-session lock. Credential-store reads deliberately occur outside transition ownership; the run is revalidated under that ownership before state work, errors, retries, or publication. Initial-link credential refresh and identity validation complete before an atomic promotion to a static-token, gate-fenced link run. Recovery/candidate preparation and activation saves are run-gated outside the live-session lock; a final dirty-marker check and bus publication remain brief session-owned steps. Clear, account changes, and unlink detach stale runs; stale workers cannot publish a bus or alter the current run's pending state.
- Sync only exists while account credentials are present and a live vault session is available.
- Missing account metadata is the only logged-out state. Unreadable or unavailable saved credentials stop an active bus and block automatic sync initialization with fixed safe guidance; a later vault hydration or verified account replacement gets one new read attempt, and only a successful read or durable logout clears the diagnostic. Sync-state recovery guidance takes priority.
- Local sync state preserves the three-way merge base and server version. Corrupt history is quarantined with a recovery marker, blocks sync and account-change state replacement, and is never auto-relinked or deleted as stale state.
- Account actions require the daemon's account-change protocol, repairing or restarting the managed service before sending versioned replace and clear commands. They never write credentials directly. Replacement and clear hold the credential lock through their sync-state transaction, so restore cannot publish state for the previous account mid-switch or logout. A manual or system lock does not wait for that transaction; an already admitted account transaction keeps sync paused until it completes, then initializes only from the current session and durable account state.
- A replacement re-reads the committed credential record under that lock before clearing a credential-sync block. A failed verification remains sync-blocked and returns fixed safe guidance even when the credential save itself committed.
- If account replacement saves credentials but cannot remove staged prior sync state, it preserves that state, marks sync recovery, and reports committed cleanup-pending success. It never guesses that a later staged file is safe to delete.
- If post-replacement retirement cannot remove an old credential secret or related local artifact, the daemon preserves the committed new account, reports a separate cleanup-pending result, and never auto-retries against reusable credential slots.
- If logout deletes credentials but cannot remove sync state or its dirty marker, it remains locally logged out and reports committed cleanup-pending success. Retained sync state marks recovery; a dirty-marker-only failure is visible without a false recovery state.
- If post-logout retirement cannot remove an old credential secret or related local artifact, the daemon preserves the committed local logout, reports a separate cleanup-pending result, and never auto-retries against reusable credential slots.
- Refreshing credentials for the same account and server retains its validated sync history; only a different-account transition stages that history away before linking. Windows stages or restores that same-directory state with a no-replace write-through move; native power-loss validation remains required because Windows cannot flush the parent directory portably.
- Logout commits local credentials and sync-state removal outside the live-session lock, then releases the credential lock before the bounded remote token revocation.
- While sync is active, learned SSH route proofs mark the vault dirty and the sync bus also runs low-frequency status checks. Route preparation expires a stale broker session before work and rechecks it before any private probe, direct-probe signature, or learned-proof write; locked sessions retain only public cached candidates.
- Every service-repair attempt restarts a live daemon only when a platform-reported service PID matches the daemon PID file. It never takes over an unowned daemon automatically; the user must stop the running daemon or service first. Windows has no task-to-pipe PID binding, so a live control pipe blocks automatic restart until the scheduled task or foreground daemon is stopped manually.
- Unix command inspection combines `ps`'s executable-name field with raw command text, so it can identify the `daemon` argument without whitespace-tokenizing a managed executable path containing spaces.
- On Linux and macOS, installed-service freshening waits for the matching-build IPC daemon to publish its managed-service PID ownership before it reports success; a persistent foreground takeover fails without another restart. Windows retains its pipe-only freshness result because Task Scheduler cannot prove task-to-pipe ownership.
- Service callers that need a usable daemon wait for installed, valid, running service state, both sockets, a responsive status endpoint, and (when known) the expected build. Linux and macOS also wait for service/PID ownership; Windows cannot prove that last relationship but still requires both pipes.
- Daemon status exposes a build id. Readiness treats a running daemon with a different or missing build id as degraded and repairs it by reinstalling/restarting the managed service.
- Linux user-service commands derive `XDG_RUNTIME_DIR` and `DBUS_SESSION_BUS_ADDRESS` when shells omit them, which is common in headless SSH or remote-editor sessions. Service installation also returns `daemon-reload` and enable failures to repair callers.
- Linux service `ExecStart` operands escape literal percent signs for systemd. Inspection reads the effective `[Service]` command across the managed unit and direct `.d` fragments, honors resets, decodes literal `%%` escapes, accepts basic single- and double-quoted operands, and requires exactly an executable followed by `daemon`; unsupported systemd syntax fails closed.
- Service-definition inspection fails closed when a current or legacy Linux/macOS/Windows command is unreadable, empty, malformed, or does not invoke `daemon`. Linux and macOS still retain platform PID evidence after that config failure so a safely owned live daemon can be repaired; Windows has no task-to-pipe ownership proof.
- Linux writes a replacement unit beside the active unit and renames it only after rendering and closing succeeds, so a write failure or interruption leaves either the prior or the new complete service definition, never a truncated one.
- Managed macOS and Windows service installation creates the daemon log directory before replacing service configuration, so a directory error cannot install a known-unbootable service.
- macOS service start and removal boot out current and legacy launchd labels before deleting legacy plists, so an old KeepAlive job cannot respawn; a fresh bootstrap is not force-killed and relaunched.
- macOS launchd stderr uses a separate private, unrotated `forged-stderr.log`, so fatal output does not remain attached to a rotated daemon log; `forged logs stderr` follows it.
- Persistent Forged state lives under `~/.config/forged` on every OS. Auth/device trust lives under `~/.config/forged/auth`. Linux keeps runtime sockets under `/run/user/<uid>/forged`; macOS and Windows use `~/.config/forged/runtime` for runtime metadata, with Windows sockets using named pipes.
- On Unix, the runtime directory must be a real current-user-owned directory and is set to `0700` before daemon-lock or socket work; same-user runtime-file control remains outside that boundary.
- Windows pipe names are opaque, domain-separated hashes of the current process token SID; startup fails before binding if that identity cannot be resolved.
- Windows deliberately starts no SSH route service. Startup migrates a uniquely marked Forged routing section in the private managed SSH config to agent-only form and removes local route artifacts; it accepts the exact legacy one-line or current five-directive routing prefix, while duplicate route markers or a changed prefix block migration instead of being overwritten. Normal SSH-agent and commit-signing operations remain available.
- Windows support is still partial around socket transport and platform helpers.
- Windows service health reads numeric Task Scheduler state through COM instead of localized command output, and stop/restart/removal propagate control failures. Windows ignores advisory PID records and uses reachable control-pipe liveness to avoid stale-PID reuse; that probe does not authenticate the pipe server. Unix uses signal-zero and rejects a non-Forged command when inspection is available.
- Shutdown is idempotent and two-phase: both listeners and their active connections close before any handler wait; native-auth waits are released without clearing the session; the vault closes only after admitted work finishes.
- A daemon publishes its PID only after both servers start and removes the PID file only while it still owns that record.
- On routing platforms, startup rewrites the managed SSH config before exposing either IPC or the agent listener, so an upgraded config cannot briefly pair new rules with an old live route service.
- Unix listeners never unlink a path before binding and remove it on close only while it is still the inode they created; stale cleanup refuses non-socket paths and inode replacements.
- Unix startup holds a persistent `daemon.lock` sidecar through shutdown and checks a live legacy PID before probing or removing socket paths; the lock file is never unlinked.
- Unix control IPC admits only a kernel-reported peer with the daemon user's effective UID; clients verify the peer UID/PID against a stable daemon PID record before sending a request. This narrows socket replacement but cannot attest a process that controls the same user's runtime files or distinguish PID reuse after an unclean exit; stronger attestation needs process-start or code identity.
- An endpoint identity mismatch blocks readiness repair, service freshening, and account-service restart rather than stopping a managed daemon while an untrusted endpoint is live.

## Decisions

- The daemon stays long-running even in cold state.
- Service install and repair stay inside the CLI, not a separate installer.
