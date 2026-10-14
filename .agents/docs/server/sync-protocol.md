---
title: Sync Protocol
applies_to:
  - server/internal/api/sync_handlers.go
  - cli/internal/sync/**
depends_on:
  - server/db-schema.md
  - cli/vault.md
last_verified: 2026-10-13
stable: yes
---

# Sync Protocol

Sync is encrypted-blob push/pull with optimistic locking. The server stores blobs; the client owns merge logic.

## Must know

- The server never merges plaintext vault data.
- Push sends `expected_version`; mismatch returns 409.
- Push, pull and status upsert the caller's `devices` row from the `X-Device-*` and `X-Forged-Version` headers, keyed by `X-Device-ID` (client UUID). `X-Device-Name` is query-escaped (macOS names carry curly apostrophes). Requests without a name (the browser) are not tracked, and a row owned by another user is never updated.
- The client rejects malformed, zero, or negative successful Push/Pull versions before sync state or vault work consumes them. Status requires `has_vault`; its version is positive only when a vault exists and omitted/zero otherwise.
- Normal conflict handling is push -> pull -> three-way merge -> one retry push. The remote just merged becomes the base even if that retry push fails, so the next merge does not count its edits as this device's. Retry delays carry up to 20% jitter so devices that failed together do not retry in lockstep. Keys with the same fingerprint (one key added on two devices) merge into the record with the smallest ID. Manual sync has a two-minute IPC budget for that bounded path; each remote call remains limited to 30 seconds.
- First-link bootstrap merge is separate from normal three-way merge.
- Key deletes and SSH-route deletes use tombstones, which never expire: any time cutoff lets a device that was offline longer, or has a skewed clock, bring deleted keys back.
- A three-way merge treats a key that is in the base but missing on one side as deleted there, even with no tombstone. A clean pull merges against the base instead of taking a union. A remote whose version went backwards was recreated, so its base no longer applies and the merge falls back to a union.
- Only rekey changes the server's password wrap (KDF params plus protected key); a push only fills a missing wrap, so a device with an old wrap cannot revert a password change. Status returns the wrap. A device stages a different wrap as `<vault>.pending-wrap` and adopts it only when a typed password opens it and decrypts the vault. A device whose own change has not reached the server keeps `<vault>.rekey-pending` and retries the rekey on every status, pull, and push.
- After a 401, the client has the token owner refresh once and retries: the local clock can still trust a token the server refuses (clock drift, rotated server key).
- If the local vault key cannot decrypt the remote vault, link fails with `ErrRemoteKeyMismatch`. The daemon reports it as the sync error and stops retrying until logout, relink, or the next sync start.
- Key mutations mark sync dirty before the vault save, so a crash between the save and push scheduling cannot lose the change; a mutation that then fails costs one redundant push.
- Key records merge per field against the base: a name, comment or git-signing change made on only one side survives, and the newer `UpdatedAt` decides only when both sides changed the same field, so a skewed clock cannot revert another device's edit.
- The web dashboard's 409 merge follows the same rules: tombstones never expire, a key in the base missing on one side is deleted, key fields merge against the base, and the server's SSH section and unknown fields are kept (the dashboard never edits routes). A server version that went backwards drops the base.
- Merges keep unknown vault fields: a record carries the winning copy's fields (a newer client must bump the record's timestamp, as for any edit), and the top level, metadata and SSH section take remote fields first, then local.
- Local sync state keeps the last synced base blob and last known server version.
- Sync state is replaced atomically and its parent directory is synced where supported. Windows uses `MoveFileEx` with `REPLACE_EXISTING|WRITE_THROUGH` for clean-state replacement and a no-replace `WRITE_THROUGH` move for account-transition staging or restore, so an existing staged state fails before credentials change. Windows has no portable directory flush; native power-loss validation is still required. The dirty marker remains until a clean state save succeeds, so a failed write cannot lose pending work on restart.
- A clean linked restore saves the fetched vault and clean sync state before removing any prior dirty marker, so daemon startup does not repush the fetched remote blob.
- A sync state needs non-null `metadata` and `key_generation`; `keys`, `version_vector` and `tombstones` only need to be present, because a new vault writes them as null.
- Sync state history is integrity-sensitive. A malformed, incomplete, or hash-mismatched state is quarantined behind a recovery marker; clients must not reset, relink, or bootstrap over it automatically.
- The daemon checks `/sync/status` before background or foreground refresh pulls. Only an up-to-date status marks local data fresh; a needed pull marks it when the pull succeeds, and a background refresh gives that pull its own 30-second budget (its short budget only covers the status call). A pull 404 means no remote vault only when the body is the server's "no vault" answer; a proxy or wrong URL 404 is an ordinary error. It pulls the encrypted blob only when the server version changed. After linked history exists, `has_vault:false` sets durable missing-remote state, never freshness or an automatic remote recreation; dirty retries recheck status before any push until a positive status or successful pull/push clears it.
- The CLI reads complete sync responses through a bound before decoding: 64 MiB for a pull, 64 KiB for control replies, and 8 KiB for error diagnostics. A larger response is invalid; these client-side safety ceilings do not define or replace server request or storage limits.
- Push and status paths recheck caller cancellation after a successful remote response before marking a sync snapshot clean or recording remote freshness.
- Pull and link merges apply to the latest local vault state inside one transaction; network calls stay outside that lock. A daemon-owned apply fence rejects a stale link or stopped bus before it can write a fetched remote merge; revocation waits only for an already admitted bounded local commit.
- First link treats keys, key tombstones, SSH routes, SSH route tombstones, and version-vector entries as sync content. If either side has that history it bootstrap-merges; a local push to an existing empty remote starts from that remote version instead of forcing a conflict.
- Sync engines work on state snapshots. If a local mutation arrives during network work, the completed server metadata is kept while the newer mutation stays dirty and queues another push.
- A pull that finds missing remote KDF or protected-key metadata must not settle clean if its repair push fails: it retains the fetched base but marks state dirty so the normal retry path repairs the remote metadata.
- A non-canceled failed clean pull records its error and retry time without marking local data dirty. Retry chooses a push only when local state is dirty; otherwise it refreshes remote state, so a read failure cannot overwrite remote data.
- Stopping a sync bus first cancels and detaches its admitted work. It drops completion-state writes after that stop, drains separately, and a new bus is not installed until the old one has drained; this prevents stale work from publishing state into a new session or account.

## Decisions

- Client-side merge is the zero-knowledge boundary. Do not move merge into the server.
