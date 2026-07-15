---
title: Sync Protocol
applies_to:
  - server/internal/api/sync_handlers.go
  - cli/internal/sync/**
depends_on:
  - server/db-schema.md
  - cli/vault.md
last_verified: 2026-07-15
stable: yes
---

# Sync Protocol

Sync is encrypted-blob push/pull with optimistic locking. The server stores blobs; the client owns merge logic.

## Must know

- The server never merges plaintext vault data.
- Push sends `expected_version`; mismatch returns 409.
- Normal conflict handling is pull -> three-way merge -> one retry push.
- First-link bootstrap merge is separate from normal three-way merge.
- Key deletes and SSH-route deletes use tombstones.
- Local sync state keeps the last synced base blob and last known server version.
- Sync state is replaced atomically and its parent directory is synced where supported. Windows uses `MoveFileEx` with `REPLACE_EXISTING|WRITE_THROUGH` for clean-state replacement because it has no portable directory flush; native power-loss validation is still required. The dirty marker remains until a clean state save succeeds, so a failed write cannot lose pending work on restart.
- Sync state history is integrity-sensitive. A malformed, incomplete, or hash-mismatched state is quarantined behind a recovery marker; clients must not reset, relink, or bootstrap over it automatically.
- The daemon checks `/sync/status` before background or foreground refresh pulls. It pulls the encrypted blob only when the server version changed.
- Pull and link merges apply to the latest local vault state inside one transaction; network calls stay outside that lock.
- First link treats keys, key tombstones, SSH routes, SSH route tombstones, and version-vector entries as sync content. If either side has that history it bootstrap-merges; a local push to an existing empty remote starts from that remote version instead of forcing a conflict.
- Sync engines work on state snapshots. If a local mutation arrives during network work, the completed server metadata is kept while the newer mutation stays dirty and queues another push.
- A non-canceled failed clean pull records its error and retry time without marking local data dirty. Retry chooses a push only when local state is dirty; otherwise it refreshes remote state, so a read failure cannot overwrite remote data.
- Stopping a sync bus first cancels and detaches its admitted work. It drops completion-state writes after that stop, drains separately, and a new bus is not installed until the old one has drained; this prevents stale work from publishing state into a new session or account.

## Decisions

- Client-side merge is the zero-knowledge boundary. Do not move merge into the server.
