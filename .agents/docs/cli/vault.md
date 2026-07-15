---
title: Vault
applies_to:
  - cli/internal/vault/**
  - cli/internal/crypto/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-07-15
stable: partial
---

# Vault

The local vault is the encrypted source of truth for keys, metadata, and synced security state.

## Must know

- The vault symmetric key is the real data-encryption root. The master password only unwraps it.
- The vault owns the in-process transaction lock shared by key operations and sync. Sync updates a cloned snapshot and publishes it only after the encrypted file is written successfully.
- Writable opens acquire the persistent OS lock file before reading vault state. Creation and linked restore use that same lock and refuse an existing target, so neither can replace a concurrently published vault. The lock path is never unlinked during release, so every process keeps locking the same file identity.
- Closing a vault makes stale references unusable before its symmetric key is zeroed.
- Password verification can recover the vault symmetric key without opening the whole vault for normal use.
- Unsupported vault headers fail closed. A newer header tells the user to upgrade Forged; an older header tells them to use a compatible release. Neither path suggests recreating or overwriting the vault.
- Password change rewraps the vault symmetric key. It does not rotate that key today.
- Password change revokes existing local unlock trust before it writes the new password wrap; if durable revocation fails, the password remains unchanged.
- A failed password-change save restores the vault's previous in-memory KDF and protected-key header, so any later save cannot silently activate a password reported as failed.
- Local unlock trust is device-local even though the vault itself is shared.
- Private keys are now decrypted on demand instead of being kept plaintext in session memory.
- Public-key signer lookup distinguishes a true absent vault-signable raw key from vault, decrypt, requested-key/private-key parse, certificate, or hardware-key failures so only an actual remote-key miss may trigger a bounded sync refresh.
- Legacy DSA keys remain viewable, exportable, removable, and syncable for recovery, but are not operational signing keys.
- Key removal validates any reviewed fingerprint while holding the vault transaction lock, before writing its tombstone.
- New key names reject terminal control characters. Import names strip them; legacy or synced values remain render-sanitized until renamed.
- SSH route entries include proof metadata, operation class, success timestamps, bounded attempt history, and route tombstones; the vault remains the synced source of truth for learned routes.
- Clearing learned SSH routes must use `KeyStore.ClearSSHRoute` so tombstones are written; deleting route JSON would let sync resurrect old memory.
- Export and change-password are intentionally stricter than normal unlock flows.

## Decisions

- Keep master password and account login separate.
- Keep device trust local; do not sync local-unlock state.
