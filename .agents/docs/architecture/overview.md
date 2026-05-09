---
title: Architecture Overview
applies_to:
  - "**"
last_verified: 2026-05-09
stable: yes
---

# Architecture Overview

Forged is a local-first SSH key manager with a background daemon, a Bubble Tea TUI, optional cloud sync, and a web dashboard. Server stores encrypted blobs only.

## Must know

- `forged` hosts the CLI and TUI. The daemon renders nothing.
- `forged-sign` is the Git signer. `forged-auth` is the System Auth helper. `forged-server` is the HTTP API.
- TUI talks to the daemon over `ctl.sock`; SSH clients talk to `agent.sock`.
- Web `/api/*` is a thin proxy to `forged-server`; business logic stays in the server or CLI.
- Sync is event-bus driven. Callers mark state dirty; they do not run sync directly.
- If the daemon is down, signing and agent-backed auth fail. There is no fallback process.

## Decisions

- Agent and control sockets stay separate.
- System Auth stays in a helper binary so the daemon remains non-interactive.
