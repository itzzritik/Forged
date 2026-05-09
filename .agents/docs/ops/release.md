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
last_verified: 2026-05-10
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
- The npm wrapper runs a best-effort `postinstall` daemon freshen after install/update. It must never fail package installation; TUI boot and Doctor remain the repair fallback.
- CLI builds embed a daemon build id. Local `just build-cli` refreshes an installed daemon after rebuilding; releases use the commit id for the daemon freshness check.
- GoReleaser artifact metadata can be either a top-level array or an object with `artifacts`; npm packaging must accept both.

## Decisions

- Keep one release workflow and one version for all platforms.
- Keep GoReleaser as the packaging center.
