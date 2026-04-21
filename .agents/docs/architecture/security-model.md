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
last_verified: 2026-04-21
stable: partial
---

# Security Model

Zero-knowledge. Server stores opaque encrypted vault blob, KDF params,
Protected Symmetric Key, generation counter. On-disk byte layout in
`proto/vault-format.md`.

Primitives: Argon2id (t=3, m=64 MiB, p=4) → master key; HKDF-SHA256
(domain `forged-stretch`) → stretched key; AES-256-GCM with 12-byte
random nonce for every symmetric encryption; Ed25519 default for generated
SSH keys (RSA/ECDSA importable).

Key hierarchy (Bitwarden-style Protected Symmetric Key): password+salt →
master key → stretched key (master zeroed). Random 32-byte Symmetric Key
generated at vault creation, wrapped by stretched key = Protected
Symmetric Key in header. On unlock: stretched unwraps PSK → Symmetric Key;
stretched zeroed. Symmetric Key encrypts payload and wraps per-item
cipher keys that wrap private-key bytes.

## Must know

- **`FORGED_MASTER_PASSWORD` is stored plaintext** in the launchd plist /
  systemd unit. Top open security gap. `.agents/plan/security-hardening/`
  is rewriting this bootstrap — do not build new features depending on
  this env var.
- **Daemon keeps decrypted private keys in memory for its whole lifetime**
  once unlocked. "Lock" via `sensitiveauth` gates the IPC surface only;
  it does NOT re-encrypt in-memory keys.
- **`proto/vault-format.md` is stale.** It says XChaCha20-Poly1305 + 24-byte
  nonce; shipped code is AES-256-GCM + 12-byte nonce. Trust the vault
  package header constants until proto is updated.
- **Password change rewraps the Symmetric Key only.** Per-item ciphertexts
  and the Symmetric Key itself are unchanged; no per-item re-encryption.
  True Symmetric Key rotation is a separate unimplemented operation.
- **Migration 005 dropped** `master_password_hash`, `vault_unlock_attempts`,
  `vault_locked_until`. Do not re-add them — reverses zero-knowledge.
- Browser crypto runs in a Web Worker; main thread only ever holds a
  non-extractable `CryptoKey` handle.
- IF the master key, stretched key, or password is persisted anywhere
  (disk, log, audit) THEN critical bug.
- IF a `Save` reuses a nonce under the same key THEN GCM security is
  destroyed. Fresh random 12-byte nonce per write is the invariant.
- IF a vault is opened THEN Argon2id params come from the header, not
  hardcoded defaults.

## Decisions

- Protected Symmetric Key (Bitwarden-modeled) so password change rewraps a
  60-byte blob instead of re-encrypting every item. Do NOT revert to
  encrypting the payload directly under a password-derived key.
- Zero-knowledge is load-bearing. No server-side password verification,
  lockout, or "forgot password" reset. Losing the password loses the vault.
- AES-GCM over XChaCha20-Poly1305 for stdlib + hardware acceleration.
  Random-nonce discipline is the safety invariant; do not switch to a
  counter-based nonce without re-audit.
- Biometric unlock grants a session lease, not crypto. The Symmetric Key
  still derives from the master password.
