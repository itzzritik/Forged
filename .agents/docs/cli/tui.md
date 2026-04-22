---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-04-22
stable: yes
---

# TUI

Bubbletea program hosted in the `forged` CLI process (not the daemon).
Single top-level `model` in `app.go` that composes per-area sub-states
(keys, agent, manage, doctor) and route-per-screen navigation via a push-
down `Session` stack.

## Must know

- **TUI runs in `forged`, not the daemon.** Every runtime action goes
  through the `Dependencies` struct — closures built in
  `cmd/forged/cmd/launcher.go` that call `actions.*` (vault-local) or
  `ipc.NewClient(paths.CtlSocket()).Call(...)` (daemon). The TUI layer
  itself never imports `ipc` or `vault` directly.
- **All deps are required.** `tui.Run` returns an error if any closure is
  nil. Tests must stub every one.
- **Navigation is a stack of `Route{ID, Params}` plus a `boundary`
  index.** `Session.Back()` pops until the boundary. Entry intents set the
  boundary so "back from Keys Detail" can land in Dashboard or exit.
- **Screens are presentational.** Packages under `screens/` (keys,
  dashboard, doctor, repair, account, common, agent) expose `Render*`
  functions that take a flat state struct; `app.go` owns *all* state and
  owns which screen renders. Sub-screen directories do NOT hold their own
  bubbletea models.
- **One giant `model` struct.** `app.go` is ~2900 lines; `keys.go` ~2400.
  Sub-states (`keyBrowserState`, `agentState`, `manageState`, etc.) are
  fields, not child models. Adding a new screen means adding fields and
  switch cases, not a child tea.Model.
- **Manage now owns the user-facing security settings.** The Manage tab
  includes direct rows for:
  - `Master Password Interval`
  - `External Use Policy`
  Pressing `Enter` updates the setting in `config.toml` immediately and
  refreshes local security state in place.
- **Doctor now surfaces security capability state.** The Doctor table
  includes:
  - `System Auth`
  - `Secure Store`
  - `External Use`
  These rows are driven by local capability probing plus the saved
  security policy, not by daemon runtime status.
- **System header has four states** (`checking`, `fixing`, `healthy`,
  `unhealthy`). It is cosmetic — `bootAssessed` guards real flow. Do not
  gate logic on the header state.
- **`FORGE-<verification>` code shown during login is the only cross-
  device binding** until OAuth PKCE lands (see `web/auth-flow.md`).
- **Password flows are a single `passwordInput` reused across 8 flows**
  (`passwordFlow` enum). Completion dispatches on the flow value — a new
  flow needs a switch arm in the finish handler, not a new screen.
- **Vault-backed TUI launch now forces a startup unlock step.** After
  `Assess`, the TUI either:
  - asks for native auth immediately when IPC is already ready, then runs
    startup repair
  - or runs startup repair first and asks for native auth right after if
    the daemon was cold
  - or skips native auth entirely and goes straight to the master-
    password screen when local unlock trust is clearly missing
  Touch ID / Hello failure or cancel falls back to the master-password
  screen for this launch only.
- **Startup unlock screens reuse the welcome product rail.** While
  `passwordStartupUnlock` is on screen, the header keeps the
  brand/version frame and shows the same product rail used by the
  welcome state instead of live health, signing, and sync status.
- **Open TUI sessions re-lock in place after daemon session loss.**
  When runtime status flips from unlocked to locked after system
  lock/sleep or other shared-session invalidation, the foreground TUI
  switches back to `passwordStartupUnlock`. If local unlock trust exists,
  the screen stays visibly locked and waits for explicit `Enter` before
  retrying native auth. If local unlock trust is missing, it goes
  straight to the master-password prompt.
- **Open TUI sessions also idle-lock after 4 minutes with no key input.**
  The TUI owns this timer. When it fires, it calls the same
  `sensitive-lock` path used by shared-session invalidation, so external
  SSH/signing trust is cleared too. The timer resets only on real
  `tea.KeyMsg` input, not on resize, spinner ticks, or background status
  polling.
- **Fixed body height of 19 rows** in `shell/layout.go`. Screens that
  overflow clip; add scrolling in the screen render, not the shell.
- **Runtime status and sensitive state come from two separate IPC calls**
  (`CmdStatus`, `CmdSensitiveProbe`); runtime status polling now
  continues across all vault-backed screens so an already-open TUI can
  observe shared-session loss after lock/sleep. A stale value can still
  linger for up to ~2s after a mutation.
- IF the daemon restarts THEN in-flight IPC calls fail and the TUI
  surfaces the error on the next tick — no retry loop.

## Decisions

- TUI-in-CLI, not TUI-in-daemon. Keeps daemon non-interactive and lets
  the TUI start without a running daemon (it drives `InstallService` and
  can render repair flows).
- Single god-model over child-per-screen Bubbletea models. Cross-screen
  state (sensitive auth, sync status, key list) is shared freely; a
  child-model split would require plumbing it back up.
