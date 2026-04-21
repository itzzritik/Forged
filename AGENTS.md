# Repo Rules

## Planning

- All plans live in `.agents/plan/`. Do not place planning Markdown elsewhere in the repo unless the user asks.
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

- Update `Status` and `Last updated` on every meaningful edit.
- Move items between `Pending` / `In Progress` / `Done` as state changes.
- Do not delete completed plan files automatically.
- When a plan is completed, tell the user which files were completed and on what date, then ask whether they should be deleted.

## Code Rules

- Keep code minimal, clean, and modular. Avoid bloat and redundancy.
- Prefer stdlib over third-party code when it is adequate.
- Add comments only when they explain why, not what.
- Wrap errors with context, for example: `fmt.Errorf("doing x: %w", err)`.

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

If your change modifies files under a shard's `applies_to` glob AND alters behavior the shard describes:

1. Update the affected prose so it matches current product behavior.
2. Bump `last_verified` in the shard frontmatter to today's date.
3. Write the doc update alongside the code change so they land in the same commit when the user stages.

A plan cannot be marked `done` until its docs reflect the shipped behavior.

If a matched shard is missing or inaccurate and fixing it is out of scope, flag it to the user before committing the code change.
