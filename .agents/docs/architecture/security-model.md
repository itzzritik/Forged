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
last_verified: 2026-05-10
stable: partial
---

# Security Model

Forged is zero-knowledge. The server stores the encrypted vault blob, KDF params, and the Protected Symmetric Key, but never the master password or plaintext keys.

## Must know

- Key hierarchy is: password + salt -> master key -> stretched key -> unwrap Protected Symmetric Key -> vault symmetric key.
- Password change rewraps the vault symmetric key. It does not re-encrypt every item and it does not rotate the inner symmetric key.
- Local unlock trust is per-device:
  - secure-store device key
  - `~/.config/forged/auth/headless-unlock.key` when no OS secure store exists
  - `~/.config/forged/auth/local-unlock.json`
  - `~/.config/forged/auth/device.id`
- The daemon now starts cold. It does not need a stored plaintext master password to boot.
- Active auth creates a shared session. The session can be cleared by expiry, system lock/sleep, or TUI idle lock.
- Account login metadata lives at `~/.config/forged/auth/account.json`. Account access and refresh tokens use the OS credential store when available: macOS Keychain, Linux Secret Service, or Windows DPAPI. Headless/no-store fallback uses `~/.config/forged/auth/account-secret.enc` plus a `0600` local key file.
- Private keys are now decrypted on demand. They are not kept plaintext for the whole session anymore.
- SSH auto-routing writes public hint files and short-lived route snippets only. Learned route proofs and route tombstones stay inside the encrypted vault. Provider probes use strict host-key checking and never treat `ssh -T` account auth as repo proof.
- Export and change-password stay master-password-only.
- `proto/vault-format.md` still lags the shipped AEAD details. Trust the vault code, not the proto doc, for current crypto constants.

## Decisions

- Protected Symmetric Key is the core model. Do not switch back to encrypting the whole vault directly under a password-derived key.
- Server-side password verification stays forbidden.
- AES-GCM stays the shipped symmetric primitive unless the whole vault format is re-audited.
