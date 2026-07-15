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
- Helpers emit that lock event on macOS screen lock/screensaver and will-sleep, Linux ScreenSaver lock and logind `PrepareForSleep(true)`, and Windows session lock and WMI entering suspend. Linux system-bus access and Windows Modern Standby behavior still require native validation.
- An expired shared-session timestamp remains as cleanup evidence until the broker clears or replaces it; token pruning cannot hide session expiry.
- Lease expiry, daemon hydration, authorization grants, and session clearing share one broker transition lock; native prompts run outside it.
- Non-export `sensitive-auth` requests perform expiry cleanup before force can skip the initial active-session fast path, so it cannot revive a stale in-memory vault session.
- On desktop, TUI unlock first accepts a valid shared session; otherwise it tries System Auth and falls back to the universal master-password page on cancel, failure, unavailable System Auth, or missing device unlock.
- On desktop, external SSH/signing tries System Auth while the device-unlock window is usable, and denies on cancel/failure. When that window has lapsed (or was never enrolled), macOS fires a native master-password popup in the background and fails the triggering request immediately (ssh won't wait for a human); entering the password unlocks the shared session so the *next* connection succeeds. Single-flight + cooldown so a retry storm shows one popup and doesn't re-nag after dismissal. Windows/Linux still deny with an "open Forged" message pending native password prompts (broker gate is `runtime.GOOS == "darwin"`).
- Linux headless mode must be explicitly enabled by launching once with `forged --headless`. The daemon reads `security.headless_unlock` on each authorization, bypasses System Auth when no secure-store enrollment exists, and may then create or use file-backed local unlock trust. Both enable and disable lock the current session; `forged --headless=false` also removes that trust.
- Local-unlock metadata and its device key use one cross-process lock plus an A/B slot protocol. Refresh writes and durably commits the inactive key slot before publishing `local-unlock.json` version 2; recovery reads only its named slot. Invalidation durably records revocation before removing slots. Legacy single-key enrollment remains readable until the next password refresh. This prevents a crash or a concurrent renew/invalidate from pairing metadata with the wrong key.
- Windows stores those local-unlock device keys as CurrentUser-DPAPI blobs under `config.Paths.AuthDir()` (`local-unlock-a.dpapi` or `local-unlock-b.dpapi`, with the legacy `local-unlock.dpapi` retained for migration). The blobs remain install-ID-bound and protect at rest and across users, not against same-user malware or as a biometric ACL.
- macOS Keychain writes pass account and local-unlock secret material through `security -i` standard input, never command-line arguments; malformed or oversized command input fails safely.
- A canceled Linux `pkexec` prompt remains a cancellation and can never fall through to headless enrollment.
- Linux `pkexec` authorization is capped at one minute so an unattended desktop prompt cannot hold SSH/signing authorization indefinitely.
- Export, private-key clipboard views, and change-password are always master-password-only. Export and private-key views each use a scoped short-lived one-use token and do not rely on System Auth. A successful password authorization also establishes the normal bounded shared session, so the daemon never retains an untracked hydrated vault.
- Open TUI sessions relock after system lock/sleep and after 4 minutes of idle time.
- External System Auth prompts are single-flight with a short failure cooldown so parallel SSH/signing requests do not spam prompts.
- SSH route preparation normally uses public in-memory route data and does not prompt. If a cold daemon has no route cache, it may trigger external auth once to hydrate the vault before writing the route snippet.
- `broken` and `unavailable` are separate states. Unavailable means no System Auth path, usually headless; broken means the desktop System Auth path exists but failed unexpectedly.
- `Master Password Interval` is a local device policy because device unlock enrollment is per-device. It now bounds *inactivity*, not time-since-password: each successful biometric unlock slides the window forward (throttled), so an actively-used device never expires. A 90-day hard cap since the last master-password entry (`localEnrollmentHardCap`) still forces one periodic re-verification regardless of activity.
- A successfully delivered master-password authorization refreshes device unlock best-effort; canceled or failed IPC delivery does not modify local enrollment. Successful biometric unlock slides the existing enrollment's expiry forward without a master password.
- Manual and system locks advance a broker authorization generation before clearing the session and cancel active System Auth and master-password prompts. A result from an older native prompt or password popup cannot hydrate or grant a new shared session after that lock.
- Daemon shutdown stops new prompts and cancels broker-owned prompt contexts before waiting. It closes the helper after admitted auth work drains, while retaining the shared vault session until admitted agent and IPC work has finished.
- An unexpected System Auth helper exit marks native auth broken and immediately clears the shared session; an intentional daemon shutdown is handled by its normal lock path.
- IPC cancellation sends the helper a one-way request-ID cancel. macOS invalidates the matching `LAContext` or password alert, Linux/Windows cancel the matching child process, helper lock monitors share the helper shutdown context, late results are suppressed, and one canceled broker waiter cannot abort a prompt still needed by another waiter.
- Password verification and vault hydration cannot be interrupted mid-derivation. Password verification only proves the password; for IPC, enrollment refresh waits for successful response delivery, while cancellation or a write failure clears a newly hydrated session and revokes scoped tokens before the authorization can persist.
- Password paths preserve only an unsupported vault-version error, so a newer or legacy vault directs the user to a compatible Forged release. Wrong-password, corrupted-vault, and other verification failures remain generic.
- A direct password or System Auth hydration grant stays single-flight until its delivery result resolves, so one canceled submission cannot leave another provisional submission behind. Concurrent callers retry after that short delivery window.

## Decisions

- System Auth stays in a helper binary, not inside the daemon.
- One shared session is the UX/security balance; export is the intentional exception.
