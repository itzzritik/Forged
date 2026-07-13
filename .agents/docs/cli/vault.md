---
title: Vault
applies_to:
  - cli/internal/vault/**
  - cli/internal/crypto/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-07-14
stable: partial
---

# Vault

The local vault is the encrypted source of truth for keys, metadata, and synced security state.

## Must know

- The vault symmetric key is the real data-encryption root. The master password only unwraps it.
- The vault owns the in-process transaction lock shared by key operations and sync. Sync updates a cloned snapshot and publishes it only after the encrypted file is written successfully.
- Closing a vault makes stale references unusable before its symmetric key is zeroed.
- Password verification can recover the vault symmetric key without opening the whole vault for normal use.
- Password change rewraps the vault symmetric key. It does not rotate that key today.
- Local unlock trust is device-local even though the vault itself is shared.
- Private keys are now decrypted on demand instead of being kept plaintext in session memory.
- SSH route entries include proof metadata, operation class, success timestamps, bounded attempt history, and route tombstones; the vault remains the synced source of truth for learned routes.
- Clearing learned SSH routes must use `KeyStore.ClearSSHRoute` so tombstones are written; deleting route JSON would let sync resurrect old memory.
- Export and change-password are intentionally stricter than normal unlock flows.

## Decisions

- Keep master password and account login separate.
- Keep device trust local; do not sync local-unlock state.
