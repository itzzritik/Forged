---
title: Platform Packaging
applies_to:
  - .goreleaser.yml
  - scripts/install.sh
  - scripts/msi/**
  - npm/**
depends_on:
  - ops/release.md
last_verified: 2026-04-23
stable: partial
---

# Platform Packaging

Forged ships today as release archives and an npm wrapper around platform-native binaries.

## Must know

- Only GitHub Releases and npm are live channels right now.
- npm publishes one wrapper package plus per-platform optional dependency packages.
- `scripts/install.sh` is intentionally simple and unsigned.
- macOS, Windows, and Linux packages still lack the final signing/notarization story.
- `forged-auth` is a real native helper on macOS and a stub elsewhere.

## Decisions

- Keep the npm package as a thin binary launcher.
- Keep dormant package-manager config declarative until those channels are actually turned on.
