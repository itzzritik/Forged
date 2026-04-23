---
title: Sync Protocol
applies_to:
  - server/internal/api/sync_handlers.go
  - cli/internal/sync/**
depends_on:
  - server/db-schema.md
  - cli/vault.md
last_verified: 2026-04-23
stable: yes
---

# Sync Protocol

Sync is encrypted-blob push/pull with optimistic locking. The server stores blobs; the client owns merge logic.

## Must know

- The server never merges plaintext vault data.
- Push sends `expected_version`; mismatch returns 409.
- Normal conflict handling is pull -> three-way merge -> one retry push.
- First-link bootstrap merge is separate from normal three-way merge.
- Key deletes use tombstones. SSH-route deletes are still weaker and can resurrect in races.
- Local sync state keeps the last synced base blob and last known server version.

## Decisions

- Client-side merge is the zero-knowledge boundary. Do not move merge into the server.
