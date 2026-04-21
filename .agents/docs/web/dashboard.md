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
last_verified: 2026-04-21
stable: yes
---

# Web Dashboard

Browser-side vault under `/dashboard`. All crypto runs in a shared Web
Worker against `crypto.subtle`; the UI thread only holds a
non-extractable `CryptoKey` handle and never the password or stretched
key. `VaultContext` (from `useVault`) is the single source of vault
state across every dashboard page.

## Must know

- **The Symmetric Key never leaves the worker in raw form.** The worker
  imports it as `extractable: false` and `postMessage`s the CryptoKey
  handle out; the main thread can pass it to `crypto.subtle.encrypt/
  decrypt` but cannot `exportKey` it.
- **Stretched master key lives inside the worker.** `derive` stores it
  in worker scope; `decrypt` consumes it to unwrap the Protected
  Symmetric Key, then zeros the masterKey buffer. Cancelling the worker
  discards the stretched key without ever hitting the main thread.
- **IndexedDB caches the unlocked `CryptoKey` for 4 hours** of inactivity
  under `forged-vault/keys/sync-key`. The non-extractable handle is
  structured-clonable, so IDB persists the live handle — reloads skip
  unlock. IF `idbAvailable` is false THEN an in-memory fallback is used
  (cleared on reload).
- **`localStorage.forged-has-key` is a UI-only hint.** Set/cleared
  alongside IDB; used by `hasCachedKeySync()` for render-before-await.
  NOT a security signal.
- **Push uses the same JSON envelope as the CLI.** Conflict resolution
  is inlined in `useVault.pushVault`: on 409, pull, three-way merge via
  `mergeThreeWayRaw` (TS port of the Go merge), push once more with the
  new expected version. A second 409 throws.
- **`lastSyncedBaseRawRef` is the merge base.** Initialized from the
  pull that accompanied unlock. IF a push succeeds THEN update it to the
  just-pushed raw. Losing this ref (e.g. remount without re-unlock)
  degrades three-way merge to two-way LWW.
- **Vault metadata (`kdf_params`, `protected_symmetric_key`) can be
  missing on `/status`** for vaults pushed before migration 004. The
  hook falls through to a `/pull` to try to recover; if still missing,
  shows the "open Forged on a linked machine and Sync Now" error.
- **Browser device ID is local, random, persisted in localStorage.** Not
  registered via `/devices` — the web client does not appear in the
  device list. Push/pull send it in `X-Device-ID` and `device_id` so
  merges attribute "last writer" correctly.
- **`useVaultContext` throws if called outside `DashboardShell`.** Every
  dashboard page can assume a provider. Do not add a conditional in the
  layout.
- **Command palette (Cmd/Ctrl+K) is `CommandDialog` from shadcn `cmdk`.**
  It reads the keys list off `VaultContext`; opening it on a still-locked
  vault shows an empty list, not a prompt.
- **Worker is a singleton module-level global.** Terminating it on error
  is the reset path; the next derive re-spawns. Do not hold a second
  worker reference.
- IF `/api/vault/*` returns 401 THEN `useVault` pushes the router to
  `/login`. Session expiry surfaces here, not in the proxy.

## Decisions

- Worker + non-extractable CryptoKey over plain-object handoff. Prevents
  any UI-thread bug (inline script, extension, dev tool) from exporting
  the Symmetric Key.
- IDB over sessionStorage for the cached key so it survives tab reload
  without encoding/decoding (which would force extractable).
- Merge logic duplicated in TS (`lib/sync/merge.ts`) rather than shelled
  to the server — zero-knowledge forbids server merge.
