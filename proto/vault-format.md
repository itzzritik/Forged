# Vault File Format Specification

Version: 1

## Binary Layout

| Offset | Size | Field |
|--------|------|-------|
| 0 | 8 | Magic bytes: `FORGED\x00\x01` |
| 8 | 2 | Format version (uint16, little-endian) |
| 10 | 32 | Argon2id salt |
| 42 | 4 | Argon2id time cost (uint32, little-endian) |
| 46 | 4 | Argon2id memory cost in KiB (uint32, little-endian) |
| 50 | 1 | Argon2id parallelism (uint8) |
| 51 | 24 | XChaCha20-Poly1305 nonce |
| 75 | variable | Encrypted payload (ciphertext + 16-byte Poly1305 auth tag) |

## Key Derivation

1. Input: master password (UTF-8 bytes) + salt (32 random bytes)
2. Algorithm: Argon2id
3. Default parameters: time=3, memory=65536 KiB (64 MB), parallelism=4
4. Output: 256-bit (32-byte) vault encryption key

## Encryption

1. Algorithm: XChaCha20-Poly1305 (AEAD)
2. Nonce: 24 bytes, randomly generated per write
3. Plaintext: JSON payload (see below)
4. Additional data: none
5. Output: ciphertext + 16-byte authentication tag (appended)

## JSON Payload Schema

```json
{
  "keys": [
    {
      "id": "uuid-v4",
      "name": "string",
      "type": "ssh-ed25519 | ssh-rsa | ecdsa-sha2-nistp256 | ecdsa-sha2-nistp384",
      "public_key": "string (authorized_keys format)",
      "private_key": "string (PEM-encoded)",
      "comment": "string",
      "fingerprint": "string (SHA256:...)",
      "created_at": "RFC3339 timestamp",
      "updated_at": "RFC3339 timestamp",
      "last_used_at": "RFC3339 timestamp | null",
      "tags": ["string"],
      "host_rules": [
        {
          "match": "string",
          "type": "exact | wildcard | regex"
        }
      ],
      "git_signing": "boolean",
      "version": "integer",
      "device_origin": "uuid-v4"
    }
  ],
  "metadata": {
    "created_at": "RFC3339 timestamp",
    "device_id": "uuid-v4",
    "device_name": "string"
  },
  "version_vector": {
    "device-id": "integer"
  },
  "tombstones": [
    {
      "key_id": "uuid-v4",
      "deleted_at": "RFC3339 timestamp",
      "deleted_by_device": "uuid-v4"
    }
  ],
  "key_generation": "integer"
}
```

## Write Protocol

1. Serialize JSON payload
2. Generate 24-byte random nonce
3. Encrypt with XChaCha20-Poly1305 using vault key + nonce
4. Assemble binary: magic + version + argon2id params + nonce + ciphertext
5. Write to `vault.forged.tmp` in the same directory
6. `fsync` the temp file
7. `rename` temp file to `vault.forged` (atomic)

## Versioning

- Reader must support all versions <= current
- Writer always writes the latest version
- On open, if version < current, auto-migrate in-memory and rewrite
