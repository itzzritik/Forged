---
title: Release Pipeline
applies_to:
  - .goreleaser.yml
  - .github/workflows/**
  - scripts/deploy/**
  - scripts/msi/**
  - justfile
  - npm/**
  - server/Dockerfile
depends_on:
  - ops/platform-packaging.md
last_verified: 2026-10-31
stable: partial
---

# Release Pipeline

One manual GitHub Actions workflow publishes CLI releases.

## Must know

- Live channels today are GitHub Releases and npm.
- Homebrew, Scoop, nfpm, and MSI config exists but is currently skipped.
- macOS binaries are not notarized. Windows binaries are not Authenticode-signed. Linux archives are unsigned.
- The macOS Swift helper must be built on macOS. Its build is cached by source hash; the macOS job runs only on a cache miss, otherwise the release restores the cached build. If the Linux cache lookup ever stops matching the macOS-saved cache, the macOS job just runs every time.
- npm ships one package, `forgedssh`: a Node launcher plus each release archive's URL and SHA-256 (`forged.archives`), written by `build-npm-packages.sh` from `checksums.txt` at publish time. On first run the launcher downloads its own version's archive from GitHub Releases, checks the hash and unpacks the binaries into its package `bin/` (a per-user cache when that is not writable, e.g. a root-owned global prefix).
- npm is published after the GitHub Release in the same run, so the package never points at a file that is not downloadable yet. If npm publish fails after the release, re-running the workflow would bump the version: rebuild `dist/npm` locally from the release's `checksums.txt` and `npm publish --otp` it. npm trusted publishing is bound to `publish.yml`.
- The npm package has no install scripts: npm 12+ blocks them and warns on every install. So after `npm i -g forgedssh` the daemon keeps its old build until `forged` runs once (the launcher downloads, the build-id check restarts the daemon).
- `@getforged/cli` and its six `@getforged/cli-<os>-<arch>` packages are deprecated. Its last version is a bridge that depends on `forgedssh`.
- CLI builds embed a daemon build id. Local `just build-cli` refreshes an installed daemon after rebuilding; releases use the commit id for the daemon freshness check.
- Publish runs queue instead of canceling: a canceled run can leave a partial npm/GitHub release. The release job runs CLI vet/test in parallel with GoReleaser, reusing its Go cache; a failure stops the job before the push, the GitHub Release or npm (CLI only: this workflow releases the CLI, so it never gates on server or web).
- GoReleaser time depends on the Go build cache, keyed on `go.sum`: the first release after a dependency change, or after 7 idle days (GitHub cache eviction), compiles cold (~2 min longer).

## Decisions

- Keep one release workflow and one version for all platforms.
- Keep GoReleaser as the packaging center.
