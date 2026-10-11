---
title: Web Dashboard
applies_to:
  - web/src/app/dashboard/**
  - web/src/components/shell/**
  - web/src/lib/vault-crypto*
  - web/src/lib/vault-store.ts
  - web/src/hooks/use-vault.ts
depends_on:
  - architecture/security-model.md
  - server/sync-protocol.md
last_verified: 2026-10-11
stable: yes
---

# Web Dashboard

The dashboard keeps vault crypto in a shared worker. The UI thread never owns raw password or raw symmetric-key bytes.

## Must know

- The worker owns password-derived material and unwrap logic; the main thread only gets a non-extractable `CryptoKey`.
- IndexedDB can cache that key handle for a limited window; `localStorage` only keeps a UI hint.
- Push/pull uses the CLI sync model with client-side merge. `useVault` is the single source of vault state, and `pushVault` changes state only after the server accepts.
- Every mutation goes through `useSync().commit`. A failure pins a Retry (dialogs with their own retry pass `pin: false`) and lock drops it, because the server may have moved on.
- `rekey` must update the hook's `kdfParams` and `protectedKey`: every push sends them, so stale values put the old password back on the server.
- Export and private-key reveal re-verify the master password every time (`usePasswordPrompt`). A revealed key lives only in component state and hides after 60 s by wall clock, and on key switch, close or lock.
- Single-key shortcuts are ignored while typing, in menus and during IME composition. While a dialog is open only modal maps (the dialog's own keys) answer; `mod+` combos always run.
- Key-bar hints are labels only: clicking one runs the shortcut the page registered for that combo, so every hint needs one.
- Dashboard tokens and fonts are scoped with `:root:has([data-shell])`, so marketing pages never see them.
- Session expiry during dashboard API calls shows up through the web auth layer, not a separate vault-auth path.

## Decisions

- Keep crypto in the worker.
- Keep merge on the client, not the server.
- Dashboard messages use the key bar's message line, not toasts.
