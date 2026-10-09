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
last_verified: 2026-10-09
stable: partial
---

# Release Pipeline

One manual GitHub Actions workflow publishes CLI releases.

## Must know

- Live channels today are GitHub Releases and npm.
- Homebrew, Scoop, nfpm, and MSI config exists but is currently skipped.
- macOS binaries are not notarized. Windows binaries are not Authenticode-signed. Linux archives are unsigned.
- The macOS Swift helper must be built on macOS. Its build is cached by source hash; the macOS job runs only on a cache miss, otherwise the release restores the cached build. If the Linux cache lookup ever stops matching the macOS-saved cache, the macOS job just runs every time.
- The npm wrapper is a launcher for native platform packages, not a JS implementation.
- The wrapper pins every platform package to its own version, and npm silently skips a missing optional dependency, so a wrapper that reaches users before its platform packages installs without a binary. npm can take minutes to serve a publish, or never serve it. The release run therefore publishes only the platform packages and dispatches a second `publish.yml` run (`promote=<version>`); that run waits until every pinned platform package is installable, waits 3 more minutes for registry caches, then publishes the wrapper from the release tag. A platform package that never appears fails the promote run and the wrapper is never published, so users stay on the last complete release. The promote run must stay in `publish.yml`: npm trusted publishing is bound to that workflow file.
- The npm packages have no install scripts: npm 12+ blocks them and warns on every install. After an upgrade the daemon restarts itself onto the new build (see daemon docs); the next `forged` launch's build-id check is the fallback.
- CLI builds embed a daemon build id. Local `just build-cli` refreshes an installed daemon after rebuilding; releases use the commit id for the daemon freshness check.
- GoReleaser artifact metadata can be either a top-level array or an object with `artifacts`; npm packaging must accept both.
- Publish runs queue instead of canceling: a canceled run can leave a partial npm/GitHub release. The release job runs CLI vet/test in parallel with GoReleaser, reusing its Go cache; a failure stops the job before npm publish or the push (CLI only: this workflow releases the CLI, so it never gates on server or web).
- GoReleaser time depends on the Go build cache, keyed on `go.sum`: the first release after a dependency change, or after 7 idle days (GitHub cache eviction), compiles cold (~2 min longer).

## Decisions

- Keep one release workflow and one version for all platforms.
- Keep GoReleaser as the packaging center.
