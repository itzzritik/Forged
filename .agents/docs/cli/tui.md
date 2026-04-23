---
title: TUI
applies_to:
  - cli/internal/tui/**
depends_on:
  - cli/ipc.md
last_verified: 2026-04-23
stable: yes
---

# TUI

The TUI runs inside the `forged` CLI process. It owns UI state and talks to the daemon or local actions through injected dependencies.

## Must know

- `app.go` owns the single Bubble Tea model. Screen packages only render.
- Real work goes through launcher-built dependency closures. The TUI layer should not reach directly into daemon or vault packages.
- Navigation is route-stack based, with boundaries so back behavior can return to dashboard or exit.
- Body height is fixed. Pages that need more space must scroll or paginate themselves.
- Vault-backed launch starts in an auth wall before the dashboard.
- While locked, the header uses the welcome product rail instead of live system status.
- Manage owns user-facing security settings. Doctor shows security capability state.
- `Master Password Interval` is local to the device, not synced through the vault.
- `External Use Policy` is only shown when native auth is truly unavailable on that machine.

## Decisions

- TUI stays in the CLI process, not the daemon.
- One top-level model is still simpler here than child Bubble Tea models.
