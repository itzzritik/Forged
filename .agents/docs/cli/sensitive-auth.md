---
title: Sensitive Auth
applies_to:
  - cli/internal/sensitiveauth/**
  - cli/cmd/forged-auth/**
depends_on:
  - architecture/security-model.md
  - cli/daemon.md
  - cli/ipc.md
last_verified: 2026-07-15
stable: partial
---

# Sensitive Auth

Sensitive auth is the gate for private-key use and live daemon-session hydrate. The broker is the single policy owner in the daemon; System Auth prompts come from `forged-auth`.

## Must know

- There is one shared active-use session for TUI access and external signing/auth. It lasts 4 hours unless the system locks/sleeps, the user locks Forged, or the daemon restarts.
- That session ends on:
  - expiry
  - system lock/sleep
  - explicit TUI idle lock
  - daemon restart
- An expired shared-session timestamp remains as cleanup evidence until the broker clears or replaces it; token pruning cannot hide session expiry.
- Lease expiry, daemon hydration, authorization grants, and session clearing share one broker transition lock; native prompts run outside it.
- Non-export `sensitive-auth` requests perform expiry cleanup before force can skip the initial active-session fast path, so it cannot revive a stale in-memory vault session.
- On desktop, TUI unlock first accepts a valid shared session; otherwise it tries System Auth and falls back to the universal master-password page on cancel, failure, unavailable System Auth, or missing device unlock.
- On desktop, external SSH/signing tries System Auth while the device-unlock window is usable, and denies on cancel/failure. When that window has lapsed (or was never enrolled), macOS fires a native master-password popup in the background and fails the triggering request immediately (ssh won't wait for a human); entering the password unlocks the shared session so the *next* connection succeeds. Single-flight + cooldown so a retry storm shows one popup and doesn't re-nag after dismissal. Windows/Linux still deny with an "open Forged" message pending native password prompts (broker gate is `runtime.GOOS == "darwin"`).
- Linux headless mode must be explicitly enabled by launching once with `forged --headless`. The daemon reads `security.headless_unlock` on each authorization, bypasses System Auth when no secure-store enrollment exists, and may then create or use file-backed local unlock trust. Both enable and disable lock the current session; `forged --headless=false` also removes that trust.
- Local-unlock metadata and its device key use one cross-process lock plus an A/B slot protocol. Refresh writes and durably commits the inactive key slot before publishing `local-unlock.json` version 2; recovery reads only its named slot. Invalidation durably records revocation before removing slots. Legacy single-key enrollment remains readable until the next password refresh. This prevents a crash or a concurrent renew/invalidate from pairing metadata with the wrong key.
- Windows stores those local-unlock device keys as CurrentUser-DPAPI blobs under `config.Paths.AuthDir()` (`local-unlock-a.dpapi` or `local-unlock-b.dpapi`, with the legacy `local-unlock.dpapi` retained for migration). The blobs remain install-ID-bound and protect at rest and across users, not against same-user malware or as a biometric ACL.
- A canceled Linux `pkexec` prompt remains a cancellation and can never fall through to headless enrollment.
- Linux `pkexec` authorization is capped at one minute so an unattended desktop prompt cannot hold SSH/signing authorization indefinitely.
- Export and change-password are always master-password-only. Export issues a short export token and does not rely on System Auth.
- Open TUI sessions relock after system lock/sleep and after 4 minutes of idle time.
- External System Auth prompts are single-flight with a short failure cooldown so parallel SSH/signing requests do not spam prompts.
- SSH route preparation normally uses public in-memory route data and does not prompt. If a cold daemon has no route cache, it may trigger external auth once to hydrate the vault before writing the route snippet.
- `broken` and `unavailable` are separate states. Unavailable means no System Auth path, usually headless; broken means the desktop System Auth path exists but failed unexpectedly.
- `Master Password Interval` is a local device policy because device unlock enrollment is per-device. It now bounds *inactivity*, not time-since-password: each successful biometric unlock slides the window forward (throttled), so an actively-used device never expires. A 90-day hard cap since the last master-password entry (`localEnrollmentHardCap`) still forces one periodic re-verification regardless of activity.
- Successful master-password fallback refreshes device unlock best-effort; successful biometric unlock slides the existing enrollment's expiry forward without a master password.
- Daemon shutdown stops new prompts and cancels broker-owned prompt contexts before waiting. It closes the helper after admitted auth work drains, while retaining the shared vault session until admitted agent and IPC work has finished.
- IPC cancellation sends the helper a one-way request-ID cancel. macOS invalidates the matching `LAContext` or password alert, Linux/Windows cancel the matching child process, helper lock monitors share the helper shutdown context, late results are suppressed, and one canceled broker waiter cannot abort a prompt still needed by another waiter.

## Decisions

- System Auth stays in a helper binary, not inside the daemon.
- One shared session is the UX/security balance; export is the intentional exception.
