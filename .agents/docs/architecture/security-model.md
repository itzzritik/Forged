---
title: Security Model
applies_to:
  - cli/internal/vault/**
  - cli/internal/sensitiveauth/**
  - web/src/lib/vault-crypto*
  - web/src/lib/vault-crypto-worker.ts
  - web/src/workers/**
  - server/internal/api/vault_handlers.go
  - server/migrations/**
last_verified: 2026-07-15
stable: partial
---

# Security Model

Forged is zero-knowledge. The server stores the encrypted vault blob, KDF params, and the Protected Symmetric Key, but never the master password or plaintext keys.

## Must know

- Key hierarchy is: password + salt -> master key -> stretched key -> unwrap Protected Symmetric Key -> vault symmetric key.
- Password change rewraps the vault symmetric key. It does not re-encrypt every item and it does not rotate the inner symmetric key.
- Local unlock trust is per-device:
  - secure-store or headless device-key A/B slots, selected by `local-unlock.json` version 2 under one cross-process lock
  - Windows: CurrentUser-DPAPI `auth/local-unlock-a.dpapi` / `local-unlock-b.dpapi` blobs bound to the installation ID (the original single blob remains migration-only)
  - `~/.config/forged/auth/headless-unlock-a.key` / `headless-unlock-b.key` when no OS secure store exists
  - `~/.config/forged/auth/local-unlock.json`, which is durably switched only after its inactive key slot is ready
  - `~/.config/forged/auth/device.id`
- The daemon now starts cold. It does not need a stored plaintext master password to boot.
- Active auth creates a shared session. The session can be cleared by expiry, system lock/sleep, or TUI idle lock.
- Account login metadata lives at `~/.config/forged/auth/account.json`. Account access and refresh tokens use the OS credential store when available: macOS Keychain, Linux Secret Service, or Windows DPAPI. Headless/no-store fallback uses `~/.config/forged/auth/account-secret.enc` plus a `0600` local key file.
- Stored account secrets carry their server and user identity. Saves write an inactive credential slot before atomically switching metadata; mismatches and post-migration downgrades fail closed.
- Private keys are now decrypted on demand. They are not kept plaintext for the whole session anymore.
- Operational SSH signing accepts RSA, ECDSA, and Ed25519 only. Legacy DSA data remains recoverable in the encrypted vault but cannot sign or route.
- Local vault files and remote restore metadata are rejected before Argon2id derivation when their KDF costs are invalid or resource-exhausting.
- SSH auto-routing writes public hint files and short-lived route snippets only. Learned route proofs and route tombstones stay inside the encrypted vault. Provider probes use strict host-key checking and never treat `ssh -T` account auth as repo proof.
- Export and change-password stay master-password-only.
- `proto/vault-format.md` still lags the shipped AEAD details. Trust the vault code, not the proto doc, for current crypto constants.

## Decisions

- Protected Symmetric Key is the core model. Do not switch back to encrypting the whole vault directly under a password-derived key.
- Server-side password verification stays forbidden.
- AES-GCM stays the shipped symmetric primitive unless the whole vault format is re-audited.
