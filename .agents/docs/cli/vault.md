---
title: Vault
applies_to:
  - cli/internal/vault/**
  - cli/internal/crypto/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-10-13
stable: partial
---

# Vault

The local vault is the encrypted source of truth for keys, metadata, and synced security state.

## Must know

- The vault symmetric key is the real data-encryption root. The master password only unwraps it.
- The vault owns the in-process transaction lock shared by key operations and sync. Sync updates a cloned snapshot and publishes it only after the encrypted file is written successfully.
- Writable opens acquire the persistent OS lock file before reading vault state. Only a lock held by another process reads as "locked by another process"; a file system that cannot lock (NFS without lockd, some WSL mounts) gets its own error. Creation and linked restore use that same lock and refuse an existing target, so neither can replace a concurrently published vault. The lock path is never unlinked during release, so every process keeps locking the same file identity.
- Saves use `platform.WriteFileDurable`: temp file, fsync, replace, then a directory fsync (Windows: write-through `MoveFileEx`, retried briefly while another process holds the file open), so a power loss keeps either the old or the new vault. A symlinked vault (dotfiles or cloud folder) is written at its real path, so the link survives. A writable open sweeps temp files left by a crashed save.
- Closing a vault makes stale references unusable before its symmetric key is zeroed.
- Password verification can recover the vault symmetric key without opening the whole vault for normal use.
- Unsupported vault headers fail closed. A newer header tells the user to upgrade Forged; an older header tells them to use a compatible release. Neither path suggests recreating or overwriting the vault.
- Password change rewraps the vault symmetric key. It does not rotate that key today.
- `<vault>.pending-wrap` is a server wrap from another device's password change; password opens fall back to it, and a writable open persists it. A local password change drops it. `<vault>.rekey-pending` is set before a local password change and cleared once the server has the new wrap.
- Password change revokes existing local unlock trust before it writes the new password wrap; if durable revocation fails, the password remains unchanged.
- A failed password-change save restores the vault's previous in-memory KDF and protected-key header, so any later save cannot silently activate a password reported as failed.
- Local unlock trust is device-local even though the vault itself is shared.
- Private keys are now decrypted on demand instead of being kept plaintext in session memory.
- Public-key signer lookup distinguishes a true absent vault-signable raw key from vault, decrypt, requested-key/private-key parse, certificate, or hardware-key failures so only an actual remote-key miss may trigger a bounded sync refresh.
- Legacy DSA keys remain viewable, exportable, removable, and syncable for recovery, but are not operational signing keys.
- Key removal validates any reviewed fingerprint while holding the vault transaction lock, before writing its tombstone.
- `ResolveName` acts only on an exact name; anything fuzzier returns not-found with suggestions, so a stale name never renames, reveals, or selects a different key. A unique byte-exact name always resolves, even an empty one or one that whitespace-normalizes to another key's name.
- New key names reject terminal control characters. Import names strip them; legacy or synced values remain render-sanitized until renamed.
- SSH route entries include proof metadata, operation class, success timestamps, bounded attempt history, and route tombstones; the vault remains the synced source of truth for learned routes.
- Clearing learned SSH routes must use `KeyStore.ClearSSHRoute` so tombstones are written; deleting route JSON would let sync resurrect old memory.
- Export and change-password are intentionally stricter than normal unlock flows.
- JSON fields this build does not know (top level, metadata, keys, tombstones, SSH section, routes, route tombstones) are kept in `Extra` and written back, so data from a newer Forged survives this build's saves and syncs. Code that rebuilds one of these structs must carry its `Extra`; a sync merge that picks one side's key or route also takes the other side's unknown fields, or an older device that edited first would drop them.

## Decisions

- Keep master password and account login separate.
- Keep device trust local; do not sync local-unlock state.
