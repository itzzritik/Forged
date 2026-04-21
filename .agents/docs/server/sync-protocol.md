---
title: Sync Protocol
applies_to:
  - server/internal/api/sync_handlers.go
  - cli/internal/sync/**
depends_on:
  - server/db-schema.md
  - cli/vault.md
last_verified: 2026-04-21
stable: yes
---

# Sync Protocol

Encrypted-blob push/pull with optimistic locking and client-side
three-way merge. Server is dumb: stores `{blob, version, kdf_params,
protected_symmetric_key, updated_by_device}` per user, bumps version on
write. Conflict resolution, tombstoning, and routing merge all live in
the client. See `server/api.md` for route shapes; `cli/vault.md` for
on-disk crypto.

## Must know

- **`proto/sync-api.md` is stale.** It still documents `Content-Type:
  application/octet-stream`, `X-Vault-Version` header, and `/auth/register`
  + `/auth/login` (removed with migration 005). Shipped is a JSON envelope
  (`blob` base64, `kdf_params`, `protected_symmetric_key`,
  `expected_version`, `device_id`). Cross-referenced in `server/api.md`.
- **Optimistic locking via `expected_version`.** 409 → client pulls,
  three-way merges against `LastSyncedBaseBlob`, pushes the merged blob
  with the new expected version. IF the retry also 409s THEN surface
  error; no nested retry loop.
- **`expected_version = 0` is the first-push sentinel.** Server
  upserts only when the existing row has `version = 0` (i.e. empty
  shell). See `server/db-schema.md` — there is a hole if a vault is
  hard-deleted server-side between pull and push.
- **Merge rule: union keys, LWW per field, tombstone deletes.** Per-key
  fields (name, comment, gitSigning, private-key wrap) compare by
  `updated_at`; ties broken by deviceID. A tombstone keyed by UUID
  suppresses any "resurrected" key from the other side regardless of
  timestamps. Tombstones TTL 90 days.
- **SSH routes merge by canonical route key.** Per-entry LWW; no
  tombstone for routes — removal races can resurrect a stale route.
- **`BootstrapMerge` is a SEPARATE path** used only on first link, when
  no common base exists. It fingerprint-matches keys between local and
  remote to avoid duplicating an imported key, whereas `MergeThreeWay`
  matches by UUID.
- **`vault_version` in the DB is independent of `key_generation` in the
  blob.** The server cannot see `key_generation` — if a true Symmetric
  Key rotation ever ships, the server cannot enforce "all devices are on
  generation N before accepting".
- **Agent sign-miss triggers a 750ms pull** via `SyncCoordinator.
  RefreshMissingKey`. Masks cross-device lag but adds latency to real
  not-found. Exclusive to the SSH agent path.
- **`SyncState` persists to `.forged/state.json`.** Contains
  `LastSyncedBaseBlob` (base64 of the last successfully pushed ciphertext),
  `LastKnownServerVersion`, `Dirty`, `LinkedUserID`. IF any of these
  drift from actual server state (manual DB edit, restore from backup)
  THEN first push will 409 and the client will attempt merge-and-retry.
- **Retry backoff is fixed**: 1s, 2s, 5s, 10s, 30s, 1m, 5m, 15m. No jitter.
- **Bus debounce is 500ms** between mutation and push. A rapid-fire
  mutation coalesces; under debounce, `Dirty` is set and the last event
  wins.
- IF `credentials.json` is missing THEN the bus no-ops (stays Dirty
  locally). Sync resumes on next successful login.

## Decisions

- Client-side merge because the server is zero-knowledge and cannot read
  the blob. Server-side merge would break zero-knowledge.
- Union adds + LWW fields + tombstone deletes over CRDTs — simple, exact,
  and sufficient for human-rate mutations on small key counts.
- Single-writer optimistic locking via an INT column over a
  per-entry version vector at the DB level. The vector lives inside
  the encrypted blob.
