---
title: Vault
applies_to:
  - cli/internal/vault/**
  - cli/internal/actions/vault.go
  - cli/internal/actions/keys.go
depends_on:
  - architecture/security-model.md
last_verified: 2026-04-22
stable: partial
---

# Vault

Local encrypted SSH-key file at `<data dir>/vault.forged` (mode `0600`,
dir `0700`). Sibling `.lock` holds a platform advisory lock for
concurrent RW opens. Crypto hierarchy in `architecture/security-model.md`;
byte layout in `proto/vault-format.md`.

Save = serialize → AES-256-GCM with fresh 12-byte nonce → temp file →
fsync → chmod 0600 → rename. KeyStore mutates under RWMutex: `Generate`
(Ed25519), `Add`/`AddFromFile` (Ed25519/RSA/ECDSA → OpenSSH PEM), `List`,
`Get`, `View` (full goes through sensitiveauth), `Rename`, `Remove`,
`Export`, `SetGitSigning`.

## Must know

- **Removes are tombstones, not hard deletes** — sync would resurrect a
  hard-deleted key. Tombstones carry `{key_id, deleted_at,
  deleted_by_device}` and are keyed by **UUID**, not name: rename-then-delete
  produces a tombstone for the original UUID.
- **`ChangePassword` rewraps the Protected Symmetric Key only.** Per-item
  ciphertexts and the Symmetric Key itself are preserved; no per-item
  re-encryption.
- **`key_generation` exists but `ChangePassword` does NOT bump it.** Under
  the hardening plan, rotating the inner Symmetric Key is open — any such
  change MUST bump this counter AND re-encrypt every per-item cipher-key
  wrap.
- **`proto/vault-format.md` is stale** (XChaCha20 + 24-byte nonce + no PSK
  field). Shipped: AES-256-GCM + 12-byte nonce + 60-byte Protected
  Symmetric Key in header.
- **`OpenReadOnly` skips the advisory lock.** Fine for verification, but
  two writers can collide if a caller forgets `Open`.
- **File locking is advisory only.** A process that bypasses `vault.Open`
  and reads the raw file is not blocked.
- **Sync uses the Symmetric Key directly.** The old HKDF-derived sync key
  was removed with the PSK rollout.
- **`ChangePassword` at the actions layer restarts the service cold.** IF
  service reinstall / restart fails THEN local vault is still updated;
  user sees "run Doctor". Server `/rekey` is best-effort.
- **`RecoverSymmetricKey` now exists for auth flows.** It unwraps the
  Protected Symmetric Key without decrypting the full vault payload, and
  sensitiveauth uses it to tie local-unlock enrollment refresh to real
  master-password verification.
- **`OpenWithSymmetricKey` now exists for daemon hydrate.** It decrypts
  the vault payload from an already recovered Symmetric Key, so the daemon
  can move between cold and active session states without reusing the
  master-password path.
- **`OpenReadOnlyWithSymmetricKey` now exists for cold metadata reads.**
  It lets callers recover key summaries from local enrollment without
  opening the master-password path or taking the vault write lock.
- **`ChangePassword` now invalidates and rebuilds local unlock trust
  best-effort.** The old local-unlock blob is removed first so stale
  trust does not survive a password change. If refresh fails, the vault
  password change still succeeds and the caller gets a warning.
- Private-key bytes load into `Key.PrivateKey` only after
  `DecryptAllPrivateKeys`. Metadata ops don't need the password longer
  than necessary.
- IF `Save` fails THEN keystore reverts in-memory mutations before returning.
- IF `Close` runs THEN Symmetric Key buffer is zeroed before releasing lock.
- Writer always writes `CurrentVersion` (2); readers refuse anything else.
  No on-read migration yet.

## Decisions

- Protected Symmetric Key so password change rewraps 60 bytes. Do NOT
  revert to encrypting payload directly under a password-derived key.
- Tombstone deletes because sync is multi-device; hard deletes would
  resurrect on pull.
- Sync blobs use the Symmetric Key directly — one key, one wrap format.
- Atomic temp+rename for every write. Crash mid-save leaves previous good
  file; no WAL (file is small, full rewrite is cheap).
