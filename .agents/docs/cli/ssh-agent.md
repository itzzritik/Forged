---
title: SSH Agent
applies_to:
  - cli/internal/agent/**
  - cli/internal/hostmatch/**
  - cli/internal/sshrouting/**
  - cli/internal/platform/pipe_windows.go
depends_on:
  - cli/daemon.md
last_verified: 2026-05-09
stable: yes
---

# SSH Agent

Forged implements the OpenSSH agent protocol from the vault keystore. Listing and signing are supported; agent-side key mutation is not.

## Must know

- OpenSSH routing is primarily config-driven: managed `Match exec` prepares a short-lived `%C.conf` snippet with public-key hint files and `IdentitiesOnly yes`.
- `%C` is connection-scope, so concurrent same-host routes share the snippet name. The service tracks attempts by client PID and writes the union of active candidates for that `%C`; agent signing still filters by PID.
- The routing service keeps an in-memory public route/key cache after vault lock. This lets route prepare emit candidate public-key hints after system lock so OpenSSH reaches the agent and external System Auth can run at signing time.
- Routed OpenSSH clients are scoped by peer PID as a fallback. If a route exists with zero candidates, the agent exposes zero keys instead of falling back to the full vault.
- GitHub/GitLab repo routes are considered proven only after a provider repo probe. Exact proven repo routes emit only the proven key; same-owner and same-host history only rank candidates.
- Explicit `ssh` client commands resolve as plain SSH targets even when launched from inside a Git working tree.
- Cold daemon sessions can hydrate on first agent use if policy allows it.
- External agent use goes through `ActionExternal`, not the TUI-style view path.
- `forged-sign` now does an auth preflight so Git commit signing can show cleaner auth errors.
- Raw SSH agent protocol is still limited in how much error detail it can surface back to callers.

## Decisions

- Agent mutation operations stay unsupported; the TUI is the write surface.
- Agent and control traffic stay on separate sockets.
- Private keys are never written for routing. Stable hint files under the managed SSH config contain public keys only; runtime snippets are short-lived.
