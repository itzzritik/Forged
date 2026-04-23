---
title: Web Dashboard
applies_to:
  - web/src/app/dashboard/**
  - web/src/lib/vault-crypto*
  - web/src/lib/vault-store.ts
  - web/src/hooks/use-vault.ts
  - web/src/components/dashboard/**
depends_on:
  - architecture/security-model.md
  - server/sync-protocol.md
last_verified: 2026-04-23
stable: yes
---

# Web Dashboard

The dashboard keeps vault crypto in a shared worker. The UI thread never owns raw password or raw symmetric-key bytes.

## Must know

- The worker owns password-derived material and unwrap logic.
- The main thread only gets a non-extractable `CryptoKey` handle.
- IndexedDB can cache that live key handle for a limited window; `localStorage` only keeps a UI hint.
- Push/pull uses the same encrypted sync model as the CLI, including client-side merge.
- `useVault` is the single source of dashboard vault state.
- Session expiry during dashboard API calls shows up through the web auth layer, not a separate vault-auth path.

## Decisions

- Keep crypto in the worker.
- Keep merge on the client, not the server.
