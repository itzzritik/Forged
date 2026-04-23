---
title: SSH Agent
applies_to:
  - cli/internal/agent/**
  - cli/internal/hostmatch/**
  - cli/internal/sshrouting/**
  - cli/internal/platform/pipe_windows.go
depends_on:
  - cli/daemon.md
last_verified: 2026-04-23
stable: yes
---

# SSH Agent

Forged implements the OpenSSH agent protocol from the vault keystore. Listing and signing are supported; agent-side key mutation is not.

## Must know

- Each connection is scoped by peer PID when possible, so SSH routing can narrow which keys are visible for that client.
- Cold daemon sessions can hydrate on first agent use if policy allows it.
- External agent use goes through `ActionExternal`, not the TUI-style view path.
- `forged-sign` now does an auth preflight so Git commit signing can show cleaner auth errors.
- Raw SSH agent protocol is still limited in how much error detail it can surface back to callers.

## Decisions

- Agent mutation operations stay unsupported; the TUI is the write surface.
- Agent and control traffic stay on separate sockets.
