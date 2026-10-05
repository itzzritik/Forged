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
last_verified: 2026-10-05
stable: yes
---

# Mono-repo Layout

The repo holds separate CLI, server, web, npm-wrapper, proto, and agent-doc trees.

## Must know

- `cli/` and `server/` are separate Go modules. There is no `go.work`.
- `server/Dockerfile`'s `golang` image must be at least `server/go.mod`'s `go` version: official images set `GOTOOLCHAIN=local`, so they cannot fetch a newer toolchain.
- `web/` is the only JS app; bun installs and runs its scripts (`bun.lock`), while Next itself still runs on Node. `npm/` is a release wrapper published to and installed from npm, not dev tooling.
- `just` is the top-level task runner. It mainly `cd`s into the right subtree and runs native tools.
- `proto/*.md` is the shared wire-format source of truth. There is no generated shared client package.
- Release workflow is manual; there is no broad push-trigger CI.

## Decisions

- One repo keeps CLI, server, web, and specs on one release cadence.
- No workspace tooling beyond `just`; extra orchestration is not worth it here.
