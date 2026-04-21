---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-04-21
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
- **System header has four states** (`checking`, `fixing`, `healthy`,
  `unhealthy`). It is cosmetic — `bootAssessed` guards real flow. Do not
  gate logic on the header state.
- **`FORGE-<verification>` code shown during login is the only cross-
  device binding** until OAuth PKCE lands (see `web/auth-flow.md`).
- **Password flows are a single `passwordInput` reused across 8 flows**
  (`passwordFlow` enum). Completion dispatches on the flow value — a new
  flow needs a switch arm in the finish handler, not a new screen.
- **Fixed body height of 19 rows** in `shell/layout.go`. Screens that
  overflow clip; add scrolling in the screen render, not the shell.
- **Runtime status and sensitive state come from two separate IPC calls**
  (`CmdStatus`, `CmdSensitiveProbe`); they refresh on a 2-second tick. A
  stale value lingers for up to 2s after a mutation.
- IF the daemon restarts THEN in-flight IPC calls fail and the TUI
  surfaces the error on the next tick — no retry loop.

## Decisions

- TUI-in-CLI, not TUI-in-daemon. Keeps daemon non-interactive and lets
  the TUI start without a running daemon (it drives `InstallService` and
  can render repair flows).
- Single god-model over child-per-screen Bubbletea models. Cross-screen
  state (sensitive auth, sync status, key list) is shared freely; a
  child-model split would require plumbing it back up.
