# Knowledge Base Index

Authoritative routing lives in root `AGENTS.md`. This file is only a browse aid.

## Architecture
- `architecture/overview.md` — top-level system shape and subsystem boundaries
- `architecture/security-model.md` — vault, session, and local trust security model
- `architecture/mono-repo.md` — repo layout, modules, and build/tooling boundaries

## CLI
- `cli/daemon.md` — daemon lifecycle, cold start, and live-session behavior
- `cli/ipc.md` — control socket model and IPC invariants
- `cli/sensitive-auth.md` — native auth, local unlock trust, and session policy
- `cli/ssh-agent.md` — SSH agent behavior and external-use rules
- `cli/tui.md` — TUI state model, auth wall, and shell rules
- `cli/vault.md` — local vault behavior and password/encryption boundaries

## Server
- `server/api.md` — HTTP API, auth endpoints, and protected/public boundaries
- `server/db-schema.md` — important tables and persistence assumptions
- `server/sync-protocol.md` — encrypted sync model and conflict rules

## Web
- `web/app-shell.md` — route protection and app-shell behavior
- `web/auth-flow.md` — browser auth, session cookie, and CLI handoff
- `web/dashboard.md` — worker-based crypto and dashboard vault state

## Ops
- `ops/release.md` — release flow, channels, and current signing gaps
- `ops/platform-packaging.md` — package formats and platform-specific caveats
