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
last_verified: 2026-04-21
stable: partial
---

# Release Pipeline

One GitHub Actions workflow (`Publish`) drives every CLI release.
`workflow_dispatch` with `patch | minor | major` is the only trigger; the
workflow computes + pushes the tag. Stages: compute version → build Swift
`forged-auth` on macOS (cached, artifact) → on Ubuntu download artifact,
GoReleaser cross-compiles `forged` + `forged-sign` + non-macOS
`forged-auth`, publishes npm + GitHub Release, signed SSH commit+tag on
`main`.

Live channels: **GitHub Releases** and **npm** (`@getforged/cli` wrapper +
per-platform optional deps, `--provenance`). Server container and web are
deployed separately.

## Must know

- **Homebrew tap, Scoop bucket, nfpm deb/rpm, MSI are configured but
  SKIPPED** (`--skip=announce,publish,homebrew,nfpm,scoop`). Only GitHub
  Releases + npm ship today.
- **macOS binaries are NOT signed or notarized.** Headline gap. Users
  `xattr -d com.apple.quarantine` or accept Gatekeeper.
- **Windows has no Authenticode cert.** SmartScreen warns on first run.
  MSI config exists (`scripts/msi/forged.wxs`) but isn't built.
- **Linux archives and (skipped) deb/rpm are unsigned.** No GPG.
- **macOS `forged-auth` only ships because the `swift-helpers` artifact
  is downloaded in the release job.** A local `goreleaser release`
  without pre-running `build-auth-swift.sh` on a Mac produces archives
  with no macOS helper.
- **Every job guards `github.actor != 'github-actions[bot]'`** so the bot's
  push back to `main` cannot re-trigger the workflow. The `push` trigger
  is commented out; a pushed tag alone does nothing.
- **`npm/cli/package.json` is committed source that the workflow rewrites
  in place.** Do not hand-edit `optionalDependencies` — always overwritten.
  Wrapper and optional-dep versions must match exactly.
- **Release notes are `git log` between tags, not a curated changelog.**
  Commit messages are the user-facing changelog.
- **Install paths**: `curl | sh` via `scripts/install.sh` and `npm install
  -g @getforged/cli`. Other managers' configs exist but are skipped.
- IF GoReleaser fails THEN no npm + no Release; the tag stays local on
  the runner and is GC'd by `prune-orphan-tags.sh` on the next run.
- IF a platform npm package is already published at a target version
  THEN `publish-npm-packages.sh` skips it — idempotent.
- Required secrets: `JARVIS_SSH_KEY` (signed commit+tag, push), `JARVIS_GH_PAT`
  (release authored as bot), `GITHUB_TOKEN` (tag pruning), npm OIDC
  (`--provenance`). Homebrew/Scoop/APT keys would be needed to re-enable
  skipped channels.

## Decisions

- GoReleaser over custom scripts — one config for cross-compile, archives,
  checksums, channel manifests.
- One tag, all platforms. Splitting per-OS fragments the bump and signed tag.
- Install script stays minimal POSIX `sh`. Users wanting more go via npm
  or native package managers.
- npm package is a binary wrapper, not a JS port. Resolves the right
  `@getforged/cli-<os>-<cpu>` optional dep and execs. Never add runtime
  logic to the wrapper.
