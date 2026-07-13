---
title: Sync Protocol
applies_to:
  - server/internal/api/sync_handlers.go
  - cli/internal/sync/**
depends_on:
  - server/db-schema.md
  - cli/vault.md
last_verified: 2026-07-14
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
- The daemon checks `/sync/status` before background or foreground refresh pulls. It pulls the encrypted blob only when the server version changed.
- Pull and link merges apply to the latest local vault state inside one transaction; network calls stay outside that lock.
- Stopping a sync bus drains work admitted before the stop before the vault can close.

## Decisions

- Client-side merge is the zero-knowledge boundary. Do not move merge into the server.
