# Sync API Specification

Base URL: `https://forged-api.ritik.me` (production)

All routes under `/api/v1/`. Authenticated routes require `Authorization: Bearer <jwt>`.

## Auth

### POST /api/v1/auth/register

Request:
```json
{"email": "user@example.com", "password": "auth-derived-key"}
```

Response (201):
```json
{"token": "jwt...", "user_id": "uuid"}
```

Note: `password` is NOT the master password. The client derives an auth key from the master password using Argon2id with a separate salt, then sends that. The server bcrypts it. The server never sees the master password.

### POST /api/v1/auth/login

Request:
```json
{"email": "user@example.com", "password": "auth-derived-key"}
```

Response (200):
```json
{"token": "jwt...", "user_id": "uuid"}
```

## Sync

### POST /api/v1/sync/push

Upload encrypted vault blob with optimistic locking.

Headers:
- `Authorization: Bearer <jwt>`
- `Content-Type: application/octet-stream`
- `X-Vault-Version: <expected-version>` (0 for first push)
- `X-Device-ID: <device-uuid>`

Body: raw encrypted vault bytes (max 1MB)

Response (200):
```json
{"version": 42}
```

Response (409 Conflict):
```json
{"error": "version conflict: vault was updated by another device"}
```

On conflict, client must pull, merge locally, then push again.

### GET /api/v1/sync/pull

Download encrypted vault blob.

Headers:
- `Authorization: Bearer <jwt>`
- `X-Device-ID: <device-uuid>`

Response (200):
- Body: raw encrypted vault bytes
- Header: `X-Vault-Version: <version>`

Response (404):
```json
{"error": "no vault found"}
```

### GET /api/v1/sync/status

Headers:
- `Authorization: Bearer <jwt>`

Response (200):
```json
{"has_vault": true, "version": 42, "updated_at": "2026-04-01T12:00:00Z"}
```

## Devices

### POST /api/v1/devices

Register a new device. First device is auto-approved.

Request:
```json
{
  "name": "MacBook Pro",
  "platform": "darwin/arm64",
  "hostname": "ritiks-macbook",
  "device_public_key": "..."
}
```

### GET /api/v1/devices

List all registered devices.

### POST /api/v1/devices/:id/approve

Approve a pending device.

### DELETE /api/v1/devices/:id

Deauthorize and remove a device.

## Account

### GET /api/v1/account

Get account info.

### POST /api/v1/account/delete

Delete account and all associated data.
