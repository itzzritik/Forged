---
title: Release Pipeline
applies_to:
  - .goreleaser.yml
  - .github/workflows/**
  - scripts/install.sh
  - scripts/deploy/**
  - scripts/msi/**
  - justfile
  - npm/**
  - server/Dockerfile
depends_on:
  - ops/platform-packaging.md
last_verified: 2026-10-06
stable: partial
---

# Release Pipeline

One manual GitHub Actions workflow publishes CLI releases.

## Must know

- Live channels today are GitHub Releases and npm.
- Homebrew, Scoop, nfpm, and MSI config exists but is currently skipped.
- macOS binaries are not notarized. Windows binaries are not Authenticode-signed. Linux archives are unsigned.
- The macOS Swift helper must be built on macOS and passed into the release flow as an artifact.
- The npm wrapper is a launcher for native platform packages, not a JS implementation.
- The npm packages have no install scripts: npm 12+ blocks them and warns on every install. After an upgrade the daemon restarts itself onto the new build (see daemon docs); the next `forged` launch's build-id check is the fallback.
- CLI builds embed a daemon build id. Local `just build-cli` refreshes an installed daemon after rebuilding; releases use the commit id for the daemon freshness check.
- GoReleaser artifact metadata can be either a top-level array or an object with `artifacts`; npm packaging must accept both.
- Publish runs queue instead of canceling: a canceled run can leave a partial npm/GitHub release. The release job waits on a `Checks` job (Go vet/test for cli and server, web lint/typecheck/build), the repo's only automated gate since there is no push CI.
- GoReleaser time depends on the Go build cache, keyed on `go.sum`: the first release after a dependency change, or after 7 idle days (GitHub cache eviction), compiles cold (~2 min longer).

## Decisions

- Keep one release workflow and one version for all platforms.
- Keep GoReleaser as the packaging center.
