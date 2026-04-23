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
last_verified: 2026-04-23
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

## Decisions

- Keep one release workflow and one version for all platforms.
- Keep GoReleaser as the packaging center.
