# Repo Rules

## Planning

- Put all new repo plans in `.agents/plan/`.
- Do not create repo-root planning Markdown files unless the user asks.
- Do not recreate `TUI-*.md` files.
- Use one folder per feature or workstream.
- Reuse an existing feature folder when it already exists.
- Use date-stamped filenames: `YYYY-MM-DD-<topic>.md`.
- Keep `.agents/plan/INDEX.md` updated when adding or changing a plan.

## Plan Format

- Start every plan with:
  - `Status: planned | in_progress | blocked | done`
  - `Priority: high | medium | low`
  - `Last updated: YYYY-MM-DD`
  - `Scope: <one-line scope>`
- Use this section order:
  - `Context`
  - `What Exists Today`
  - `Goal`
  - `Pending`
  - `In Progress`
  - `Done`
  - `Open Questions / Risks`
  - `Next Recommended Step`

## Plan Writing

- Write for a new agent with zero context.
- Be concise, direct, and operational.
- Use real file paths, commands, and current behavior when useful.

## Plan Lifecycle

- Keep status current.
- Do not delete completed plan files automatically.
- When a plan is completed, tell the user which files were completed and on what date, then ask whether they should be deleted.

## Code Rules

- Keep code minimal, clean, and modular. Avoid bloat and redundancy.
- Prefer stdlib over third-party code when it is adequate.
- Add comments only when they explain why, not what.
- Wrap errors with context, for example: `fmt.Errorf("doing x: %w", err)`.

## Docs Rules

- Keep `.agents/docs/` minimal.
- Only document non-obvious invariants, decisions, risks, and traps.
- If code makes something obvious, do not document it.
- Cut stale or obvious text instead of appending new prose.
- Avoid history, changelog text, exhaustive route lists, field lists, and code-location notes.

## Docs Routing

Before editing files under a path below, read the listed docs. Load nothing else.

| Touching... | Read |
| --- | --- |
| `cli/internal/daemon/**` | `.agents/docs/cli/daemon.md` |
| `cli/internal/vault/**`, `cli/internal/crypto/**` | `.agents/docs/cli/vault.md`, `.agents/docs/architecture/security-model.md` |
| `cli/internal/ipc/**` | `.agents/docs/cli/ipc.md` |
| `cli/internal/agent/**` | `.agents/docs/cli/ssh-agent.md` |
| `cli/internal/tui/**` | `.agents/docs/cli/tui.md` |
| `cli/internal/sensitiveauth/**` | `.agents/docs/cli/sensitive-auth.md` |
| `server/**` (routes, handlers) | `.agents/docs/server/api.md` |
| `server/internal/db/**`, migrations | `.agents/docs/server/db-schema.md` |
| `server/**/sync*`, `cli/internal/sync/**` | `.agents/docs/server/sync-protocol.md`, `.agents/docs/cli/ipc.md` |
| `web/src/app/(login\|auth\|api/auth)/**` | `.agents/docs/web/auth-flow.md` |
| `web/src/app/dashboard/**` | `.agents/docs/web/dashboard.md` |
| `.goreleaser.yml`, `.github/workflows/**` | `.agents/docs/ops/release.md` |
| Big-picture / new subsystem | `.agents/docs/architecture/overview.md` |

If no row matches: do not preload. Read the specific file you are modifying.

## Keeping Docs Accurate

- If a matched shard changes, update only the minimal prose still needed and bump `last_verified`.
- Remove stale or obvious text while touching a shard.
- A plan cannot be marked `done` until the relevant docs match shipped behavior.
- If a matched shard is missing or too stale to trust and you cannot fix it in scope, flag it before you commit.

## Notes

- Keep root `CLAUDE.md` pointing to this file.
- `web/` keeps its own `web/AGENTS.md`.
