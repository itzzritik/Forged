# Knowledge Base Index

Human browse aid. The authoritative path-to-shard mapping lives in root `AGENTS.md` under `## Docs Routing`.

See `.agents/docs/meta.md` for format and update rules.

## Architecture
- `architecture/overview.md` — system-level view, components, data flow
- `architecture/security-model.md` — crypto primitives, key hierarchy, threat model
- `architecture/mono-repo.md` — repo structure rationale, CI, `just`

## CLI (`cli/`)
- `cli/daemon.md` — daemon lifecycle, PID/socket, session state
- `cli/vault.md` — vault format, crypto operations, password change
- `cli/ipc.md` — IPC protocol, command registry
- `cli/ssh-agent.md` — SSH agent protocol, signing, host matching
- `cli/tui.md` — TUI architecture, screens, patterns
- `cli/sensitive-auth.md` — lease/broker model, lock/unlock semantics

## Server (`server/`)
- `server/api.md` — HTTP routes, auth middleware, rate limiting
- `server/db-schema.md` — tables, migrations, indexes
- `server/sync-protocol.md` — push/pull, conflict resolution

## Web (`web/`)
- `web/auth-flow.md` — login, OAuth, sessions
- `web/dashboard.md` — dashboard crypto, VaultContext, worker
- `web/app-shell.md` — Next.js routing, proxy, public pages

## Ops (`ops/`)
- `ops/release.md` — GoReleaser, CI, signing
- `ops/platform-packaging.md` — Homebrew, Scoop, APT, installers
