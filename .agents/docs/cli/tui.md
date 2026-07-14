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
- Vault-backed launch gives an installed running service a bounded boot grace, then repairs degraded, stale, or wrong-owner state before showing the auth wall so unlock uses the daemon that will remain active.
- Maintenance stays on the current route: dashboard repairs run in the background with header status, while password setup and recovery use the existing busy state.
- First setup or restore reuses the verified master password to hydrate the launch session and skips the duplicate startup auth wall when that succeeds.
- Header status must settle from explicit model messages; startup unlock finalizes health from the current snapshot, runtime sync polls daemon status, and signing load errors render as an issue instead of an endless spinner.
- Runtime and snapshot health checks accept only their newest generation; daemon transport loss and recovery each trigger one readiness refresh, including for local-only vaults.
- Doctor marks a daemon as outdated when its IPC build id does not match the current CLI build; Fix Issues restarts the managed service through readiness.
- Doctor validates `config.toml`, reports parse errors as blocked user action, and does not overwrite an invalid file during repair.
- Doctor distinguishes unsupported System Auth from a prompt that is unavailable in the current environment.
- Doctor shows explicit Linux headless mode as file-backed trust and labels desktop System Auth detection as terminal-local rather than daemon readiness.
- Doctor security refreshes accept only their newest response and render inspection failures instead of staying on Checking or showing stale capability rows.
- The standalone `forged doctor` route remains available when no vault exists; it reports device-level health instead of replacing diagnostics with onboarding.
- Doctor checks stay in dependency order while their status icon and tone show changing health.
- Doctor check rows page within the available body height and remain reachable with Up/Down; narrow layouts stack each check so its status stays readable.
- Tables size against the real body width, collapse secondary columns before names or status, and truncate by terminal display cells so wide Unicode values cannot overflow.
- The Agent tab includes SSH Routing diagnostics. The page reads and clears route memory through daemon IPC and keeps route memory current with background polling while the page is open.
- SSH Routing diagnostics reuse the Commit Signing browser-table pattern: selected item summary on top, a compact table below, and no manual refresh footer action.
- SSH Routing row pages shrink with available height; when the table cannot fit, the selected-route summary remains navigable instead of showing clipped table chrome.
- SSH Routing keeps its route count and polling/clear feedback in the two-line summary so errors and progress remain visible without adding table chrome.
- Key mutations and commit-signing changes hide route navigation once submitted; delayed completions may update cached data but cannot dismiss a newer authentication wall.
- Vault export warns before and after writing plaintext private keys, writes through a private same-directory temporary file before replacing the destination, hides route navigation during the write, and waits for explicit dismissal on success.
- Import's delayed success return only navigates while the dashboard import route is still active, so it cannot dismiss a lock screen.
- Change-password success returns only while its dashboard success route is still visible, so a stale timer cannot dismiss a new authentication wall.
- Key-browser refreshes reuse the live search input, treat a successful empty list as loaded, clear transient refresh failures after success without erasing route guidance, and become cache-only after navigation; normal list reads already apply the daemon's freshness policy.
- Key-browser row pages shrink with the available body height so the selected row and bottom-docked search controls remain visible on short terminals.
- Commit Signing uses the same dynamic row paging and drops its redundant status card on short terminals, where signing state already remains visible in the header.
- Signing-status refreshes are single-flight and identify the configured Forged key by matching one parsed SSH fingerprint against one key list.
- Runtime sync errors must clear the in-memory syncing flag so stale status cannot leave the header spinner active forever.
- Idle locking keeps one coalesced deadline timer; keyboard activity moves the deadline instead of spawning another timer, and non-quit input pauses while the daemon lock request is in flight.
- Accepted TUI action failures append throttled route/action/version context to a size-capped TUI log, available through `forged logs tui`; expected cancellations and auth prompts are skipped, and diagnostics redact credentials, URLs, quoted values, emails, and file paths.
- TUI log writers coordinate through the persistent `.lock` sidecar; do not unlink it while a CLI or TUI process may still be writing.
- Doctor's copied report contains only version/platform/build metadata and check statuses; it omits row details, logs, account identity, paths, and key data.
- Startup unlock uses the shared auth broker: desktop TUI tries System Auth and falls back to the universal master-password page; Esc, a replacement attempt, and TUI exit cancel the live IPC/native prompt before exposing or leaving that fallback, while headless TUI hydrates enrolled device unlock without prompting.
- Master-password screens share one component for create, restore, unlock fallback, export, repair, and change-password flows.
- Valid password submissions reset every field immediately; leaving a password screen discards the component, and background commands clear their owned byte copies when done.
- Printable keys always reach focused inputs. Ctrl-C is the only global quit shortcut, except while master-password rotation is changing local and remote state; startup System Auth retry uses Ctrl-A while the password field is empty.
- Single-letter footer actions accept either letter case. Separate actions use separate letters instead of Shift-only variants.
- Footers stay on one line and compact to key badges when labels do not fit; extreme widths retain the final escape action and mark omitted actions.
- UI chrome falls back to ASCII for non-UTF-8 or legacy terminals; `FORGED_ASCII=1` forces it without rewriting user-provided text.
- Foreground colors adapt to the terminal background; `FORGED_COLOR_SCHEME=light|dark` overrides incorrect detection.
- On supported terminals, the TUI uses the alternate screen so account and key data do not remain in shell scrollback after exit.
- Browser login remains cancelable from session creation through approval polling; canceling stops retries and in-flight HTTP before it can open a late browser. While the daemon commits the account, screen actions are hidden and ignored; Ctrl-C remains the global force-quit shortcut.
- Private-key clipboard copies use sensitive platform hints when available, show a 45-second countdown, and clear only while the copied value is still current; normal TUI exit also clears an active copy.
- While locked, the header uses the welcome product rail instead of live system status.
- Manage owns user-facing security settings. Doctor shows security capability state.
- Master-password interval loading, retries, and saves have explicit single-flight states; failed loads never expose a fake default selection.
- `Master Password Interval` is local to the device, not synced through the vault.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One top-level model is still simpler here than child Bubble Tea models.
