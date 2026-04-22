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
last_verified: 2026-04-22
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

- **Local unlock enrollment foundation now exists.** Successful
  master-password verification can create / refresh:
  - `config/local-unlock.json` with a sealed wrapped Symmetric Key
  - `config/install.id` with the local install binding
  - a secure-storage device key entry
  The daemon can now hydrate from that enrollment on demand, and the
  installed service now starts cold with no stored plaintext master
  password.
- **Private-key access now runs under one shared 4-hour session.**
  Successful TUI auth or external-use auth refreshes that window.
  Expiry clears the broker session and the live daemon session.
- **Daemon still keeps decrypted private keys in memory for the whole
  active session** once hydrated. The remaining hardening work is about
  shortening that window further and tightening lock/sleep invalidation.
- **`proto/vault-format.md` is stale.** It says XChaCha20-Poly1305 + 24-byte
  nonce; shipped code is AES-256-GCM + 12-byte nonce. Trust the vault
  package header constants until proto is updated.
- **Password change rewraps the Symmetric Key only.** Per-item ciphertexts
  and the Symmetric Key itself are unchanged; no per-item re-encryption.
  True Symmetric Key rotation is a separate unimplemented operation.
- **Password change now invalidates and rebuilds local unlock trust
  best-effort.** If refresh fails, the password change still succeeds and
  the caller gets a warning.
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
- Biometric unlock can now hydrate a cold daemon session from local
  enrollment. Shared-session expiry now exists. The remaining gaps are
  platform-complete lock/sleep invalidation, external-use policy
  hardening, and shortening the in-memory live-key window.
