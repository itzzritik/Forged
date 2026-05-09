---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-05-04
stable: yes
---

# TUI

The TUI runs inside the `forged` CLI process. It owns UI state and talks to the daemon or local actions through injected dependencies.

## Must know

- `app.go` owns the single Bubble Tea model. Screen packages only render.
- Real work goes through launcher-built dependency closures. The TUI layer should not reach directly into daemon or vault packages.
- Navigation is route-stack based, with boundaries so back behavior can return to dashboard or exit.
- Body height is fixed. Pages that need more space must scroll or paginate themselves.
- Vault-backed launch repairs degraded machine state before showing the auth wall, so unlock happens against the daemon that will remain active.
- First setup or restore reuses the verified master password to hydrate the launch session and skips the duplicate startup auth wall when that succeeds.
- Header status must settle from explicit model messages; startup unlock finalizes health from the current snapshot, runtime sync polls daemon status, and signing load errors render as an issue instead of an endless spinner.
- The Agent tab includes SSH Routing diagnostics. The page reads and clears route memory through daemon IPC, refreshes an older running daemon once when IPC lacks the debug command, and then keeps route memory current with background polling while the page is open.
- SSH Routing diagnostics reuse the Commit Signing browser-table pattern: selected item summary on top, a compact table below, and no manual refresh footer action.
- Runtime sync errors must clear the in-memory syncing flag so stale status cannot leave the header spinner active forever.
- Startup unlock keeps master-password fallback available after native auth cancellation, with `A` as the footer retry for system auth while the password field is empty.
- While locked, the header uses the welcome product rail instead of live system status.
- Manage owns user-facing security settings. Doctor shows security capability state.
- `Master Password Interval` is local to the device, not synced through the vault.
- `External Use Policy` is only shown when native auth is truly unavailable on that machine.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One top-level model is still simpler here than child Bubble Tea models.
