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
last_verified: 2026-04-23
stable: yes
---

# Mono-repo Layout

The repo holds separate CLI, server, web, npm-wrapper, proto, and agent-doc trees.

## Must know

- `cli/` and `server/` are separate Go modules. There is no `go.work`.
- `web/` is the only pnpm app. `npm/` is a release wrapper, not dev tooling.
- `just` is the top-level task runner. It mainly `cd`s into the right subtree and runs native tools.
- `proto/*.md` is the shared wire-format source of truth. There is no generated shared client package.
- Release workflow is manual; there is no broad push-trigger CI.

## Decisions

- One repo keeps CLI, server, web, and specs on one release cadence.
- No workspace tooling beyond `just`; extra orchestration is not worth it here.
