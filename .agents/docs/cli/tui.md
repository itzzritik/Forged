---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-07-14
stable: yes
---

# TUI

The TUI runs inside the `forged` CLI process. It owns UI state and talks to the daemon or local actions through injected dependencies.

## Must know

- `app.go` owns the single Bubble Tea model. Screen packages only render.
- Real work goes through launcher-built dependency closures. The TUI layer should not reach directly into daemon or vault packages.
- Navigation is route-stack based, with boundaries so back behavior can return to dashboard or exit.
- Body height is fixed. Pages that need more space must scroll or paginate themselves; import review sizes its window to the available body and key feedback stays bottom-docked.
- Readiness establishes or restores a usable vault before it creates config files or enables the managed SSH include.
- Vault-backed launch repairs degraded or stale machine state before showing the auth wall, so unlock happens against the daemon that will remain active.
- Maintenance stays on the current route: dashboard repairs run in the background with header status, while password setup and recovery use the existing busy state.
- First setup or restore reuses the verified master password to hydrate the launch session and skips the duplicate startup auth wall when that succeeds.
- Header status must settle from explicit model messages; startup unlock finalizes health from the current snapshot, runtime sync polls daemon status, and signing load errors render as an issue instead of an endless spinner.
- Doctor marks a daemon as outdated when its IPC build id does not match the current CLI build; Fix Issues restarts the managed service through readiness.
- Doctor distinguishes unsupported System Auth from a prompt that is unavailable in the current environment.
- The Agent tab includes SSH Routing diagnostics. The page reads and clears route memory through daemon IPC and keeps route memory current with background polling while the page is open.
- SSH Routing diagnostics reuse the Commit Signing browser-table pattern: selected item summary on top, a compact table below, and no manual refresh footer action.
- Key deletion and commit-signing disable use an explicit review-and-confirm step. Once submitted, their non-cancelable work hides route navigation until it finishes.
- Vault export warns before and after writing plaintext private keys, hides route navigation during the write, and waits for explicit dismissal on success.
- Signing-status refreshes are single-flight and identify the configured Forged key by matching one parsed SSH fingerprint against one key list.
- Runtime sync errors must clear the in-memory syncing flag so stale status cannot leave the header spinner active forever.
- Idle locking keeps one coalesced deadline timer; keyboard activity moves the deadline instead of spawning another timer.
- Accepted TUI action failures append throttled route/action/version context to a size-capped TUI log, available through `forged logs tui`; expected cancellations and auth prompts are skipped, and diagnostics redact credentials, URLs, quoted values, emails, and file paths.
- TUI log writers coordinate through the persistent `.lock` sidecar; do not unlink it while a CLI or TUI process may still be writing.
- Doctor's copied report contains only version/platform/build metadata and check statuses; it omits row details, logs, account identity, paths, and key data.
- Startup unlock uses the shared auth broker: desktop TUI tries System Auth and falls back to the universal master-password page; Esc stops waiting and exposes that fallback, while headless TUI hydrates enrolled device unlock without prompting.
- Master-password screens share one component for create, restore, unlock fallback, export, repair, and change-password flows.
- Valid password submissions reset every field immediately; leaving a password screen discards the component, and background commands clear their owned byte copies when done.
- Printable keys always reach focused inputs. Ctrl-C is the only global quit shortcut, except while master-password rotation is changing local and remote state; startup System Auth retry uses Ctrl-A while the password field is empty.
- Single-letter footer actions accept either letter case. Separate actions use separate letters instead of Shift-only variants.
- UI chrome falls back to ASCII for non-UTF-8 or legacy terminals; `FORGED_ASCII=1` forces it without rewriting user-provided text.
- Foreground colors adapt to the terminal background; `FORGED_COLOR_SCHEME=light|dark` overrides incorrect detection.
- Browser login remains cancelable until approval arrives. While the daemon commits the account, screen actions are hidden and ignored; Ctrl-C remains the global force-quit shortcut.
- Private-key clipboard copies use sensitive platform hints when available, show a 45-second countdown, and clear only while the copied value is still current; normal TUI exit also clears an active copy.
- While locked, the header uses the welcome product rail instead of live system status.
- Manage owns user-facing security settings. Doctor shows security capability state.
- `Master Password Interval` is local to the device, not synced through the vault.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One top-level model is still simpler here than child Bubble Tea models.
