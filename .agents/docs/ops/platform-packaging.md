---
title: Platform Packaging
applies_to:
  - .goreleaser.yml
  - scripts/install.sh
  - scripts/msi/**
  - npm/**
depends_on:
  - ops/release.md
last_verified: 2026-10-05
stable: partial
---

# Platform Packaging

Forged ships today as release archives and an npm wrapper around platform-native binaries.

## Must know

- Only GitHub Releases and npm are live channels right now.
- npm publishes one wrapper package plus per-platform optional dependency packages.
- `scripts/install.sh` is intentionally simple and unsigned.
- macOS, Windows, and Linux packages still lack the final signing/notarization story. On Windows 11 this is a functional gap: Smart App Control blocks unsigned, unknown binaries outright, and Defender's behaviour model flags unsigned binaries that register a logon task (`Behavior:Win32/Persistence.A!ml`). Authenticode signing is required for a reliable Windows release.
- `forged-auth` is a Swift helper on macOS and a Go helper on Linux (pkexec) and Windows (Windows Hello); the daemon expects it next to its own executable.
- Windows binaries must keep the `.exe` suffix (`go build -o` does not add it); dev builds go through `scripts/build-cli.sh`.

## Decisions

- Keep the npm package as a thin binary launcher.
- Keep dormant package-manager config declarative until those channels are actually turned on.
