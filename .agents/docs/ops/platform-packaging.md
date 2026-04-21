---
title: Platform Packaging
applies_to:
  - .goreleaser.yml
  - scripts/install.sh
  - scripts/msi/**
  - npm/**
depends_on:
  - ops/release.md
last_verified: 2026-04-21
stable: partial
---

# Platform Packaging

Per-channel packaging for the CLI. The release pipeline that invokes
these is in `ops/release.md`; this shard covers what actually ships on
each channel and the signing/trust state.

## Must know

- **Only GitHub Releases + npm are live.** `.goreleaser.yml` declares
  Homebrew cask, Scoop bucket, and nfpm deb/rpm, but the release step
  passes `--skip=homebrew,nfpm,scoop`. Those configs are kept current
  but unpublished — re-enable by dropping skips and wiring the tap/bucket
  credentials.
- **macOS: unsigned, not notarized.** Gatekeeper quarantines the binary
  on first run. Users must `xattr -d com.apple.quarantine` or approve in
  System Settings. No Developer ID cert is configured.
- **Windows: unsigned.** SmartScreen "unrecognized app" warning on
  first run. `scripts/msi/forged.wxs` is a hand-authored WiX source but
  no MSI is built or signed in the pipeline.
- **Linux: unsigned archives, no GPG on (skipped) deb/rpm.** No
  repository (APT/YUM) is hosted. Only the raw tarball ships.
- **npm ships 1 wrapper + 6 native packages.** `@getforged/cli` has
  `optionalDependencies` pointing at `@getforged/cli-<os>-<cpu>` for
  `darwin-x64`, `darwin-arm64`, `linux-x64`, `linux-arm64`, `win32-x64`,
  `win32-arm64`. Each platform package carries a single pre-built binary
  under `bin/` and declares `os` + `cpu` so npm installs only the match.
- **Platform npm packages are generated from `npm/platform/package.template.json`**
  by `build-npm-packages.sh`; the template uses `__NAME__` / `__OS__`
  style placeholders. Do not hand-create platform package dirs.
- **`scripts/install.sh` is intentionally dumb** — POSIX `sh`, fetches
  the latest GitHub Release tarball for the detected `uname`,
  copies three binaries (`forged`, `forged-sign`, `forged-auth`) into
  `/usr/local/bin` or `~/.local/bin`. No checksum verification, no
  signature check, no uninstall. Users wanting managed installs use npm.
- **`forged-auth` is macOS-only in practice.** Linux/Windows archives
  also contain a `forged-auth` binary (a Go stub from
  `build-auth-go.sh`); the real Swift Touch ID helper only exists on
  macOS builds when the swift-helpers artifact was produced.

## Decisions

- Channels stay declarative even when skipped so re-enabling is a
  single-line flag change, not a config rewrite.
- npm wrapper never contains runtime logic — it resolves the platform
  optional dep and execs the binary. Keeps the JS surface zero.
- No self-hosted APT/YUM repo — operational cost for one user today.
  Users on Linux use `install.sh` or npm.
