---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-10-09
stable: yes
---

# TUI

The TUI runs in the `forged` CLI process and is keyboard-only.

## Must know

- Screens and modals only render and emit messages; real work goes through injected `core.Deps` closures, never direct daemon or vault calls. View does no I/O and never mutates shared state.
- Every async result that can race carries an id from `core.NextID()`, minted before the Cmd closure; owners drop stale results (newest wins). Boot id is checked before newest-wins.
- Overlays close themselves with `core.Close(self)`; the root removes that exact overlay, so a late result never closes another one.
- The root broadcasts non-key messages to all screens and overlays. Screens, not modals, handle results that must outlive the modal and emit `core.KeysChangedMsg`.
- While an overlay is open, keys and paste go only to the top overlay, except `m`: when that overlay is not a `core.Capturer` that is capturing, the root opens its actions menu and replays the chosen key only if that overlay is still on top. Every overlay that takes typed text must implement `core.Capturer`. Otherwise keys go to the gate or active screen behind the same guards as typed text.
- On lock or recovery, `LockedMsg` is broadcast to every screen and overlay before any overlay is removed, so secrets are wiped first.
- No screen keeps a private copy of the body-size rule; use `core.BodySize` / `core.BodyOrigin` (breakpoints may read `st.Width`/`st.Height`).
- Below 40×12 only ctrl+c and q work, so no blind destructive keys.
- The sweep gradient is only for thin elements, never fills or text blocks.
- Icons come only from `ui.G.Icon`, which is empty in ASCII mode; place them with `ui.Icon`/`ui.IconWidth` so rows still align without them.
- Launch draws nothing until a quick status probe answers: an already unlocked session stays blank and opens straight to the dashboard (the splash shows only if startup passes 1.5 s); locked, no daemon, or no answer within 0.3 s shows the splash at once. Once visible the splash never blanks again.
- Flame frames tick only while a gate screen shows at 40×16 or larger; the dashboard never animates. `FORGED_ASCII=1` or `NO_COLOR` draws no fire.
- Gate messages use only the card's error row; toasts and queued notices never draw on the gate. Notices arriving there wait in one queue (newest 5, dropped only on quit) and show when the dashboard returns, after a replay of any toast the gate covered. Long warnings and errors always open an info modal; other live notices wait behind a playing queue.
- Dim and modal fills are applied per cell on the composited canvas, so ANSI inside content survives; inner style resets must not break a selection band or banner fill.
- External text (errors, key names, routes, runtime messages) is control-character sanitized and truncated by display cells.
- Never import `lipgloss/v2/compat` (probes the terminal at init). Light/dark follows `BackgroundColorMsg`; `FORGED_COLOR_SCHEME`, `FORGED_ASCII=1` and `FORGED_ANIMATIONS=0` override.
- `ui.Secret` owns a preallocated rune buffer, never grows it, never creates a string, and is wiped on submit, lock and discard. Passwords never go through Huh or `textinput`.
- Paste-to-import keeps the key in a paste target and never renders it.
- Over SSH with a 256-color `TERM` and no `COLORTERM`, the program assumes truecolor (SSH drops `COLORTERM`). Links open only with `$BROWSER` set or locally with a display; otherwise the login card copies on enter. The approval URL is an OSC 8 hyperlink so wrapped lines stay clickable.
- Over SSH or on Linux without a display, plain copies go through the terminal (OSC 52) and private-key copy is refused: a secret never travels through the terminal, and a remote clipboard cannot be verified or cleared.
- Private-key clipboard copy: 45-second countdown; clear only if the clipboard still holds the copy; failed clears toast once and retry (5s, 30s after 3 tries); esc invalidates a pending copy; a copy started while locked is cleared immediately; exit clears it. A normal copy drops the lease only if it started after that private copy (`CancelClipMsg.Since`). Input is blocked except esc while a clipboard write runs.
- Idle lock (4 minutes) is one coalesced timer, moved by activity and re-armed on leaving the gate. It locks the view only; esc on the wall never reveals the previous screen.
- The lock wall starts System Auth only on enter-with-empty-field or tab, never automatically while the user may be away.
- Over SSH (`Deps.Remote`) the TUI never starts System Auth on any OS: no trust hint, the startup check shows "Unlocking Forged", and unlock requests carry `remote`.
- The headless-unlock offer opens only right after a password unlock, after the gate closes and under any queued notice modal, and once per device (`security.headless_offered`).
- Leaving recovery or finishing a login before the first unlock never reveals the dashboard: recovery exit restarts boot; a login runs post-login maintenance on the gate, then shows Unlock; a sync issue during a gate-started login cancels it and shows Unlock (or Welcome when there is no vault). Maintenance results while view-locked update state only.
- Maintenance is single-flight; runtime and snapshot checks accept only their newest generation.
- Quit is blocked while a password change runs, even on the lock wall. Ctrl+C otherwise always quits.
- Signing status loads are single-flight with a reload fence: an invalidation during a load queues one reload instead of accepting the stale result.
- Manual sync is single-flight; a sync issue redirects to Health and a credential issue starts the login repair. A credential-store failure is distinct from logged out.
- A daemon endpoint-identity failure blocks daemon and IPC work but not navigation; recovery exposes only SSH-integration removal and Health.
- Key mutations and signing changes hide route navigation once submitted; late results may refresh caches but never dismiss a newer auth wall.
- Import never resubmits successful inputs. Export warns before and after writing plaintext private keys and writes via a private same-directory temp file.
- `forged doctor` opens straight to Health and works with no vault. Disabled SSH integration is intentional and repair never restores it; `config.toml` parse errors are never overwritten; a live daemon not provably service-owned is never taken over. The copied report holds only version/platform/build metadata and check statuses.
- Failed actions append throttled, redacted context to a size-capped TUI log (`forged logs tui`); never unlink its `.lock` sidecar.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One root model plus screen packages sharing `core.State`; screens do not import each other.
