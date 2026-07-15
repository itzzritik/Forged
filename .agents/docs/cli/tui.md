---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-07-15
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
- If a local vault appears during linked restore, recovery preserves it instead of replacing it and tells the user to reopen Forged.
- Vault-backed launch gives an installed running service a bounded boot grace, then repairs degraded, stale, or wrong-owner state before showing the auth wall so unlock uses the daemon that will remain active.
- Maintenance stays on the current route: dashboard repairs run in the background with header status, and a completed Doctor repair refreshes health without reloading the active dashboard route; password screens still clear sensitive input and restore their underlying route. Password setup and recovery use the existing busy state.
- A failed Doctor repair leaves a compact global header warning and retains its detailed, width-wrapped error on Doctor until the next Doctor repair starts or a later health refresh is ready; the repair error adds no body row to active Key, Manage, or Agent views.
- TUI maintenance is single-flight: while a repair is active, Manage and unauthenticated Sync cannot start login or another repair; read-only assessment refreshes remain independent except while the active repair owns the authoritative snapshot.
- SSH-agent integration and Doctor repair are mutually exclusive: while a toggle runs, Agent actions are inert and Doctor cannot start; while Doctor repairs, only its SSH-integration action is unavailable and navigation and read-only Agent views remain available.
- First setup or restore reuses the verified master password to hydrate the launch session and skips the duplicate startup auth wall when that succeeds.
- Header status must settle from explicit model messages; startup unlock finalizes health from the current snapshot, runtime sync polls daemon status, and signing load errors render as an issue instead of an endless spinner.
- Runtime and snapshot health checks accept only their newest generation; daemon transport loss and recovery each trigger one readiness refresh, including for local-only vaults.
- Doctor marks a daemon as outdated when its IPC build id does not match the current CLI build; Fix Issues restarts the managed service through readiness.
- Doctor validates `config.toml`, reports parse errors as blocked user action, and does not overwrite an invalid file during repair.
- If any service-repair attempt cannot prove that a live daemon is service-owned, Doctor marks the service non-repairable for that snapshot and tells the user to stop the running daemon or service, then refresh; it does not retry or take over the process.
- Doctor treats disabled SSH integration as an intentional setting: it repairs unrelated device issues without restoring the managed SSH include, reconstructs that opt-out from Forged's exact commented include if `config.toml` must be recreated, and directs explicit re-enablement to the Agent tab. If effective SSH configuration still selects Forged after that integration is disabled, Doctor and the header warn that the external setting needs manual update; Forged does not auto-repair it.
- A daemon endpoint-identity failure blocks daemon and IPC work without blocking TUI navigation or triggering repair retries; recovery mode exposes only Agent SSH-integration removal and Doctor, where it is non-repairable. Once removed, integration stays disabled until the endpoint identity is resolved.
- Runtime-path recovery preserves a direct `forged doctor` route and labels dependent checks unavailable rather than inferring health from zero, stale, or default values.
- Runtime-path recovery invalidates in-flight device-login and credential-save results, so a late account commit cannot escape its Agent/Doctor-only safe surface.
- Doctor distinguishes unsupported System Auth from a prompt that is unavailable in the current environment.
- Doctor shows explicit Linux headless mode as file-backed trust and labels desktop System Auth detection as terminal-local rather than daemon readiness.
- Doctor security refreshes accept only their newest response and render inspection failures instead of staying on Checking or showing stale capability rows.
- The standalone `forged doctor` route remains available when no vault exists; it reports device-level health instead of replacing diagnostics with onboarding.
- Doctor checks stay in dependency order while their status icon and tone show changing health.
- Doctor check rows page within the available body height and remain reachable with Up/Down; narrow layouts stack each check so its status stays readable.
- Tables and import-review rows size against the real body width, collapse secondary columns before names or status, and truncate by terminal display cells so wide Unicode values cannot overflow.
- Account profile fields hard-wrap unbroken names and email addresses; compact header greetings truncate by display cells, while post-login identity lines wrap instead of overflowing.
- Key names are rendered with terminal control characters replaced, including legacy or synced data that predates storage validation.
- External error text, runtime sync messages, and learned-route labels are rendered with terminal control characters replaced before they reach the terminal.
- On Linux and macOS, the Agent tab includes SSH Routing diagnostics. The page reads and clears route memory through daemon IPC and keeps route memory current with background polling while the page is open.
- On Windows, the Agent tab omits SSH Routing and Doctor explains that automatic routing is unavailable while the normal SSH agent remains active.
- SSH Routing diagnostics reuse the Commit Signing browser-table pattern: selected item summary on top, a compact table below, and no manual refresh footer action.
- SSH Routing row pages shrink with available height; when the table cannot fit, the selected-route summary remains navigable instead of showing clipped table chrome.
- SSH Routing keeps its route count and polling/clear feedback in the two-line summary so errors and progress remain visible without adding table chrome.
- Key mutations and commit-signing changes hide route navigation once submitted; delayed completions may update cached data but cannot dismiss a newer authentication wall.
- Vault export warns before and after writing plaintext private keys, writes through a private same-directory temporary file before replacing the destination, hides route navigation during the write, and waits for explicit dismissal on success.
- Import preview, picker, and completion UI only apply to the active dashboard import operation; leaving it or locking drops the TUI's preview-key references and invalidates pending previews/pickers. A completed off-route import still refreshes cached keys and signing state, but cannot schedule a return or dismiss a lock screen.
- Post-preview import failures retain sanitized name/fingerprint/reason details, with each untrusted error bounded before redaction. A read-only result view never resubmits successful inputs, pages the selected reason on short terminals, and writes one bounded aggregate diagnostic event.
- Change-password success returns only while its dashboard success route is still visible, so a stale timer cannot dismiss a new authentication wall.
- Key-browser refreshes reuse the live search input, treat a successful empty list as loaded, clear transient refresh failures after success without erasing route guidance, and become cache-only after navigation; normal list reads already apply the daemon's freshness policy.
- Browser-origin key success and fallback transitions collapse their transient child before returning to the Browser; direct routes replace only their own entry.
- Key-browser row pages shrink with the available body height so the selected row and bottom-docked search controls remain visible on short terminals.
- Commit Signing uses the same dynamic row paging and drops its redundant status card on short terminals, where signing state already remains visible in the header.
- Signing-status refreshes are single-flight and identify the configured Forged key by matching one parsed SSH fingerprint against one key list.
- Runtime sync errors must clear the in-memory syncing flag so stale status cannot leave the header spinner active forever.
- Manual Sync is single-flight; a current trigger failure remains visible with a retry action in both Manage and the direct Sync page, while stale replies cannot clear a newer request's state.
- An active non-credential runtime sync error takes precedence over credential repair and normal sync actions: Manage and direct Sync surface it, direct Sync opens Doctor, and a latent credential warning returns when it clears.
- A saved-account credential-store failure is distinct from logged out: Doctor, Manage, direct Sync, browser approval, and the header retain fixed repair guidance from either readiness or a recognized daemon runtime diagnostic, while no-vault startup offers account repair instead of a new vault. A successful deliberate startup or maintenance unlock queues one readiness refresh after the next successful runtime-status poll only when that guidance was visible; ordinary clean polls do not clear it.
- Idle locking keeps one coalesced deadline timer; keyboard activity moves the deadline instead of spawning another timer, and non-quit input pauses while the daemon lock request is in flight.
- Accepted TUI action failures append throttled route/action/version context to a size-capped TUI log, available through `forged logs tui`; expected cancellations and auth prompts are skipped, and diagnostics redact credentials, URLs, quoted values, emails, and file paths.
- TUI log writers coordinate through the persistent `.lock` sidecar; do not unlink it while a CLI or TUI process may still be writing.
- Doctor's copied report contains only version/platform/build metadata and check statuses; it omits row details, logs, account identity, paths, and key data.
- Doctor report copy completion uses the shared clipboard generation, so runtime-path recovery cancels a pending copy and a late result cannot replace its recovery guidance.
- Startup unlock uses the shared auth broker: it first accepts a valid shared session; otherwise desktop TUI tries System Auth and falls back to the universal master-password page. Esc, a replacement attempt, and TUI exit cancel the live IPC/native prompt before exposing or leaving that fallback, while headless TUI hydrates enrolled device unlock without prompting.
- Master-password screens share one component for create, restore, unlock fallback, export, repair, and change-password flows.
- Password-form error, success, and progress feedback uses the active field width and wraps instead of clipping on narrow terminals.
- Login, password, key-detail, and key-form bodies use the actual shell body width; key metadata stacks into labeled blocks before a narrow table would make values unreadable, and key details plus delete review scroll so lower fields remain reachable. Below 28 body columns, key forms dock feedback and ellipsize variable text to keep the active field visible.
- Valid password submissions reset every field immediately; leaving a password screen discards the component, and background commands clear their owned byte copies when done.
- Printable keys always reach focused inputs. Ctrl-C is the only global quit shortcut, except while master-password rotation is changing local and remote state; startup System Auth retry uses Ctrl-A while the password field is empty.
- Single-letter footer actions accept either letter case. Separate actions use separate letters instead of Shift-only variants.
- Footers stay on one line and compact to key badges when labels do not fit; extreme widths retain the final escape action and mark omitted actions.
- UI chrome falls back to ASCII for non-UTF-8 or legacy terminals; `FORGED_ASCII=1` forces it without rewriting user-provided text.
- Foreground colors adapt to the terminal background; `FORGED_COLOR_SCHEME=light|dark` overrides incorrect detection.
- On supported terminals, the TUI uses the alternate screen so account and key data do not remain in shell scrollback after exit.
- Browser login remains cancelable from session creation through approval polling; canceling stops retries and in-flight HTTP before it can open a late browser. While the daemon commits the account, screen actions are hidden and ignored; Ctrl-C remains the global force-quit shortcut.
- Browser-login start progress is accepted only while its matching start channel is active, so a buffered retry update cannot overwrite a terminal start result or approval state.
- Device-login links hard-wrap at the active body width and stay bottom-docked, so a narrow terminal keeps the full URL readable while retaining Open Link and Copy URL actions.
- A browser-open or URL-copy failure preserves the active device-login code, link, and polling session; Open Link and Copy URL stay available with nonfatal retry guidance. Copy feedback is fenced to that login, so a late helper result cannot overwrite account commit state; Esc still cancels login while a copy runs. Only failures that end the login flow use its terminal error state.
- When account persistence succeeds but sync-state or credential cleanup is pending, or delivery cannot confirm cleanup after commit, the TUI completes login, returns to the dashboard root, and shows a recovery warning instead of falsely presenting login as failed.
- When logout commits but sync or credential cleanup is pending or unconfirmed, the TUI completes logout, returns to the dashboard root with Manage selected, and shows a warning before another account can be selected.
- Private-key clipboard copies use sensitive platform hints when available, show a 45-second countdown, and clear when the TUI begins an idle lock or observes a sensitive-session lock. While a direct copy is pending, Esc invalidates it and returns safely while other input stays blocked. Clearing is best-effort and only proceeds when the copied value still matches at the check; delayed copy results are fenced to the active key-detail operation, with stale leases cleared instead of installed. Normal TUI exit also clears an active copy.
- While locked, the header uses the welcome product rail instead of live system status.
- Manage owns user-facing security settings. Doctor shows security capability state.
- Master-password interval loading, retries, and saves have explicit single-flight states; failed loads never expose a fake default selection.
- `Master Password Interval` is local to the device, not synced through the vault.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One top-level model is still simpler here than child Bubble Tea models.
