---
title: Mono-repo Layout
applies_to:
  - justfile
  - .github/workflows/**
  - cli/go.mod
  - server/go.mod
  - npm/**
depends_on:
  - ops/release.md
last_verified: 2026-04-21
stable: yes
---

# Mono-repo Layout

Single repo for CLI (`cli/`), server (`server/`), web (`web/`), shared
proto (`proto/`), npm wrapper (`npm/`), and agent KB (`.agents/`). The
CLI is the product; server + web support it.

## Must know

- **Two independent Go modules, no `go.work`.** `cli/go.mod` and
  `server/go.mod` are separate module roots. Commands run from inside
  each subtree (`cd cli && go build ...`). Cross-module imports are not
  intended — shared wire format lives in `proto/` as hand-written specs,
  not shared Go code.
- **`just` is the only orchestrator.** No Turborepo, Nx, or
  pnpm-workspaces. The justfile is thin — a dozen recipes that `cd` into
  the right subtree and invoke native tooling (`go build`, `pnpm`,
  `doppler`). If a recipe grows beyond that, it belongs in `scripts/`,
  not the justfile.
- **`just dev` installs a dev service, it does not run the binary in the
  foreground.** See `cli/daemon.md`.
- **No path-filtered CI today.** `.github/workflows/publish.yml` is the
  only workflow and runs on manual `workflow_dispatch` only. The `push`
  trigger block is commented out. There is no lint/test CI.
- **`npm/` is a release artifact, not dev tooling.** The wrapper ships
  the CLI as `@getforged/cli` with per-platform optional deps built at
  release time. See `ops/release.md` and `ops/platform-packaging.md`.
- **`proto/*.md` are the source of truth for wire formats**, not
  generated Go/TS code. Each side hand-implements from the spec.

## Decisions

- One repo, not split — CLI, server, and web share a single release
  cadence and the proto specs. Splitting would fork versioning for one
  developer.
- No Go workspace — keeps each module's `go.sum` independent and makes
  `cd cli && go build` behave the same locally and in CI. Adding
  `go.work` later is non-breaking but unnecessary today.
- No JS monorepo tooling — `web/` is the only pnpm project; `npm/` is
  published, not built. A workspace manager would only add config.
