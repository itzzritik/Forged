---
title: Platform Packaging
applies_to:
  - .goreleaser.yml
  - scripts/msi/**
  - npm/**
depends_on:
  - ops/release.md
last_verified: 2026-10-31
stable: partial
---

# Platform Packaging

Forged ships today as release archives and one npm launcher package (`forgedssh`) that downloads them.

## Must know

- Only GitHub Releases and npm are live channels right now. npm is the only install path; there is no curl installer.
- The launcher, not forged, runs `forged update` and the TUI's update request (exit code 85): Windows can't replace a folder whose exe is running, and npm must replace the launcher's. It runs the npm beside the running Node (the resolved `bin/npm` link; `node_modules\npm` on Windows), refuses when `npm root -g` doesn't hold the package (pnpm, bun or another npm would install a second copy), and offers `sudo` only after a permission error.
- When its package folder is not writable, the launcher uses a per-user cache: macOS `~/Library/Caches/forged`, Linux `${XDG_CACHE_HOME:-~/.cache}/forged`, Windows `%LOCALAPPDATA%\forged\cache`.
- macOS, Windows, and Linux packages still lack the final signing/notarization story. On Windows 11 this is a functional gap: Smart App Control blocks unsigned, unknown binaries outright, and Defender's behaviour model flags unsigned binaries that register a logon task (`Behavior:Win32/Persistence.A!ml`); it then deletes the task and quarantines every copy of that build, the npm package's included, so `forged` stops working until the detection is allowed. Authenticode signing is required for a reliable Windows release.
- `forged-auth` is a Swift helper on macOS and a Go helper on Linux (pkexec) and Windows (Windows Hello); the daemon expects it next to its own executable.
- Windows binaries must keep the `.exe` suffix (`go build -o` does not add it); dev builds go through `scripts/build-cli.sh`.

## Decisions

- Keep the npm package as a thin binary launcher with no install scripts and no npm dependencies.
- Keep dormant package-manager config declarative until those channels are actually turned on.
