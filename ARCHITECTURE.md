# Forged — SSH Key Management, Everywhere

> Forge your keys. Take them anywhere.

A cross-platform SSH key manager that replaces 1Password's SSH agent with a standalone, open-source alternative. Zero-knowledge encrypted sync, intelligent host matching, Git commit signing, and a CLI-first interface — all in a single binary.

---

## Table of Contents

1. [Product Vision](#product-vision)
2. [Core Principles](#core-principles)
3. [Architectural Decisions](#architectural-decisions)
4. [System Architecture](#system-architecture)
5. [Component Deep Dive](#component-deep-dive)
6. [IPC Protocol](#ipc-protocol)
7. [Data Model](#data-model)
8. [Security Architecture](#security-architecture)
9. [Vault Integrity](#vault-integrity)
10. [Sync Protocol](#sync-protocol)
11. [CLI Design](#cli-design)
12. [Cloud Server](#cloud-server)
13. [Host Matching Engine](#host-matching-engine)
14. [Git Signing](#git-signing)
15. [Platform Integration](#platform-integration)
16. [Distribution & Packaging](#distribution--packaging)
17. [Mono Repo Structure](#mono-repo-structure)
18. [Project Phases](#project-phases)
20. [Tech Stack Summary](#tech-stack-summary)
21. [Known Limitations & Future Considerations](#known-limitations--future-considerations)

---

## Product Vision

### Problem

Developers manage SSH keys poorly:
- Keys sit unencrypted on disk (`~/.ssh/id_ed25519`)
- No sync between machines — manual copying of key files
- Wrong key offered to wrong host — trial-and-error authentication
- Git commit signing is a separate, painful setup
- 1Password SSH agent works but costs $36/yr and bundles an entire password manager
- Excessive biometric prompts create friction

### Solution

**Forged** is a standalone SSH key manager that:
- Stores keys in a zero-knowledge encrypted vault
- Syncs keys across all your devices via an encrypted cloud sync
- Automatically offers the correct key for each host
- Signs Git commits with zero additional setup
- Runs as a background daemon — login once, works forever
- CLI-first: full management from the terminal
- Costs nothing (open-source) or minimal for cloud sync

### Target Users

1. **Developers** who SSH into servers and push to Git daily
2. **DevOps/SRE** managing keys across many machines
3. **Teams** who want shared SSH key infrastructure
4. **Security-conscious users** migrating from 1Password's SSH agent
5. **Open-source contributors** who want free, auditable key management

### Competitive Positioning

| Feature | Forged | 1Password | Secretive | macOS Keychain |
|---------|--------|-----------|-----------|----------------|
| Ed25519 | Yes | Yes | No | Yes |
| Cross-platform | Mac/Linux/Win | Mac/Linux/Win | Mac only | Mac only |
| Key sync | Yes | Yes (bundled) | No | No |
| Host matching | Smart | Basic | No | No |
| Git signing | Built-in | Yes | Yes | Manual |
| Auth model | Login once | Touch ID per use | Touch ID per use | Passphrase once |
| Cost | Free / cheap sync | $36/yr | Free | Free |
| Open source | Yes | No | Yes | No |
| Standalone | Yes | No (password mgr) | Yes | N/A |

---

## Core Principles

1. **Zero-knowledge**: The server never sees plaintext keys. All encryption/decryption happens client-side.
2. **Login once, run forever**: Authenticate at daemon start. No repeated prompts.
3. **Smart defaults**: Works out of the box. Reads `~/.ssh/config` and does the right thing.
4. **Single binary**: No runtime dependencies. No Node, no Python, no Docker.
5. **Minimal attack surface**: The daemon is small, auditable, and does one thing well.
6. **Offline-first**: Works without network. Sync is opportunistic, not required.
7. **CLI-first**: The terminal is the primary interface. No web dashboard, no GUI.

---

## Architectural Decisions

Key decisions made during planning, with rationale:

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **UI** | CLI-only, no web dashboard | Target users live in the terminal. Eliminates React/Vite/embed complexity. REST API and dashboard can be added later if needed. |
| **Local HTTP server** | None | No dashboard means no consumer. CLI talks to daemon via Unix socket IPC. Eliminates unnecessary attack surface. |
| **Sync API** | Go on Fly.io (`forged-api.ritik.me`) | Same language as client. Flat pricing suits daemon polling. API only, no HTML. |
| **Web app** | Next.js on Vercel (`forged.ritik.me`) | Landing page, login (OAuth), dashboard, docs, pricing. |
| **Cloud auth** | Google/GitHub OAuth | Passwordless. Login page on Next.js, token exchange on Go API. |
| **Repo structure** | Mono repo, no Turborepo | Go and Next.js have independent build systems with no shared dependency graph. Turborepo adds complexity for zero benefit. A `justfile` + path-filtered CI is sufficient. |
| **CLI ↔ Daemon IPC** | Custom protocol over Unix socket | CLI commands never touch the vault file directly. Single-writer architecture prevents corruption. |
| **Vault writes** | Atomic (write-tmp + fsync + rename) | Prevents corruption on crash. Standard approach used by SQLite, etcd, etc. |
| **Vault ownership** | Daemon is sole owner | File locking (`flock`) ensures only one process reads/writes the vault. CLI commands send requests to the daemon over the socket. |
| **Memory safety** | `mlock()` + best-effort zeroing | Go's GC may copy heap objects. We use `mlock()` to prevent swap and zero on shutdown. Document the limitation honestly; `memguard`-style allocation is a v2 consideration. |
| **API versioning** | `/api/v1/` prefix from day one | Trivial to add now, painful to retrofit. Client v1 and v2 will talk to the same server. |
| **File paths** | XDG-compliant on Linux | Config: `$XDG_CONFIG_HOME/forged/`, data: `$XDG_DATA_HOME/forged/`, runtime: `$XDG_RUNTIME_DIR/forged/`. macOS uses `~/.forged/`. |
| **CLI output** | `--json` flag on all commands | Machine-readable output from day one. Retrofitting is painful. |
| **Password change** | `key_generation` counter | Devices that can't decrypt after a password change prompt user to re-enter. Simple for v1. Per-device encryption keys are the v2 path. |

---

## System Architecture

### High-Level Overview

```
                        ┌──────────────────────────────┐
                        │      Forged Cloud Server      │
                        │                               │
                        │  Next.js App (Landing + API)  │
                        │  PostgreSQL (users, blobs)    │
                        │  Vercel + Neon                │
                        └──────────────┬───────────────┘
                                       │
                          HTTPS (encrypted vault sync)
                                       │
       ┌───────────────────────────────┼───────────────────────────────┐
       │                               │                               │
┌──────▼──────┐                 ┌──────▼──────┐                 ┌──────▼──────┐
│   macOS     │                 │   Linux     │                 │   Windows   │
│             │                 │             │                 │             │
│ Single Go   │                 │ Single Go   │                 │ Single Go   │
│ Binary      │                 │ Binary      │                 │ Binary      │
│             │                 │             │                 │             │
│ ├─ Agent    │                 │ ├─ Agent    │                 │ ├─ Agent    │
│ │  (socket) │                 │ │  (socket) │                 │ │  (pipe)   │
│ ├─ CLI      │                 │ ├─ CLI      │                 │ ├─ CLI      │
│ ├─ IPC      │                 │ ├─ IPC      │                 │ ├─ IPC      │
│ ├─ Signer   │                 │ ├─ Signer   │                 │ ├─ Signer   │
│ └─ Sync     │                 │ └─ Sync     │                 │ └─ Sync     │
└─────────────┘                 └─────────────┘                 └─────────────┘
      │                               │                               │
  launchd                         systemd                      Task Scheduler
  (auto-start)                    (auto-start)                 (auto-start)
```

### Process Architecture

```
forged daemon (long-running background process)
│
├─ SSH Agent Server (goroutine)
│   ├─ Listens on Unix socket / Windows named pipe
│   ├─ Handles SSH_AGENTC_REQUEST_IDENTITIES
│   ├─ Handles SSH_AGENTC_SIGN_REQUEST
│   ├─ Routes signing requests through Host Matching Engine
│   ├─ All keys held decrypted in locked memory (mlock)
│   └─ Concurrent access: sync.RWMutex (multiple readers, exclusive writer)
│
├─ IPC Server (goroutine)
│   ├─ Listens on separate Unix socket for CLI commands
│   ├─ Handles key CRUD, config, status queries
│   ├─ Single writer to vault file
│   └─ Returns structured responses (JSON)
│
├─ Sync Engine (goroutine)
│   ├─ Periodic pull from cloud (configurable interval)
│   ├─ Push on local changes
│   ├─ Conflict resolution (last-writer-wins with version vectors)
│   ├─ Offline queue with exponential backoff retry
│   └─ Never blocks daemon — runs fully async
│
├─ Vault Manager
│   ├─ Encrypts/decrypts key material
│   ├─ Atomic writes (write-tmp + fsync + rename)
│   ├─ File locking (flock / LockFileEx)
│   └─ Derives encryption key from master password (Argon2id)
│
└─ Config Watcher (goroutine)
    ├─ Watches ~/.ssh/config for changes
    ├─ Re-parses host matching rules
    └─ Watches forged config for changes
```

### Startup Sequence

```
forged daemon starts
    │
    ├─ 1. Check for stale socket file
    │     ├─ If exists: try connect → if ECONNREFUSED, remove (stale)
    │     ├─ If exists: try connect → if success, exit ("daemon already running")
    │     └─ If not exists: proceed
    │
    ├─ 2. Check daemon.pid
    │     ├─ If PID alive: exit ("daemon already running")
    │     └─ If PID dead or missing: clean up, proceed
    │
    ├─ 3. Acquire flock on vault file
    │
    ├─ 4. Prompt for master password (or read from env/keyring)
    │
    ├─ 5. Derive vault key (Argon2id), decrypt vault
    │
    ├─ 6. Load keys into locked memory (mlock)
    │
    ├─ 7. Start SSH agent socket listener
    │
    ├─ 8. Start IPC socket listener
    │
    ├─ 9. Start config watcher
    │
    ├─ 10. Start sync engine (if enabled)
    │
    ├─ 11. Write daemon.pid
    │
    └─ 12. Ready — accepting connections
```

### Shutdown Sequence

```
SIGTERM / SIGINT received
    │
    ├─ 1. Stop accepting new connections
    ├─ 2. Finish in-flight signing requests (5s timeout)
    ├─ 3. Zero all key material in memory
    ├─ 4. Remove socket files
    ├─ 5. Remove PID file
    ├─ 6. Release flock
    └─ 7. Exit 0
```

---

## Component Deep Dive

### 1. SSH Agent

The core component. Implements the SSH agent protocol (RFC draft-miller-ssh-agent).

```
┌──────────────────────────────────────────┐
│              SSH Agent Server             │
│                                          │
│  Unix Socket: ~/.forged/agent.sock       │
│  Win Pipe:    \\.\pipe\forged-agent      │
│                                          │
│  Protocol Messages:                      │
│  ├─ REQUEST_IDENTITIES → list all keys   │
│  ├─ SIGN_REQUEST → sign with matched key │
│  ├─ ADD_IDENTITY → import a key          │
│  ├─ REMOVE_IDENTITY → remove a key       │
│  ├─ LOCK → lock agent (clear memory)     │
│  └─ UNLOCK → unlock with passphrase      │
│                                          │
│  Key Selection (REQUEST_IDENTITIES):     │
│  ├─ Return ALL keys (protocol has no     │
│  │   host context)                       │
│  ├─ Order by: host match hints > recent  │
│  │   usage > alphabetical                │
│  └─ SSH tries keys in order (3-6         │
│      attempts), so ordering solves most  │
│      cases without filtering             │
│                                          │
│  Concurrency:                            │
│  ├─ sync.RWMutex on key store            │
│  ├─ Multiple sign requests in parallel   │
│  └─ Write lock only for add/remove       │
└──────────────────────────────────────────┘
```

**Implementation**: Use `golang.org/x/crypto/ssh/agent` which provides the `Agent` interface. Implement the interface, wrap it with host-matching logic.

**Key storage in memory**: Keys are held decrypted in memory while the daemon runs. Memory pages are locked with `mlock()` / `VirtualLock()` to prevent swapping to disk. On shutdown, key memory is explicitly zeroed.

**Known limitation**: Go's GC may copy heap objects before they're zeroed. We mitigate with `mlock()` and best-effort zeroing. For v2, consider `memguard` or `syscall.Mmap`-based allocation outside the Go heap. This is documented honestly in the security model.

### 2. Vault

The vault is the local encrypted store of all keys and metadata.

**macOS paths**:
```
~/.forged/
├── vault.forged          # Encrypted vault file
├── config.toml           # User configuration
├── agent.sock            # SSH agent Unix socket (runtime)
├── ctl.sock              # IPC control socket (runtime)
├── daemon.pid            # PID file (runtime)
└── logs/
    └── forged.log        # Daemon log (rotated, max 10MB, 3 backups)
```

**Linux paths (XDG-compliant)**:
```
$XDG_CONFIG_HOME/forged/      # (~/.config/forged/)
├── config.toml

$XDG_DATA_HOME/forged/        # (~/.local/share/forged/)
├── vault.forged

$XDG_RUNTIME_DIR/forged/      # (/run/user/1000/forged/)
├── agent.sock
├── ctl.sock
├── daemon.pid

$XDG_STATE_HOME/forged/       # (~/.local/state/forged/)
└── logs/
    └── forged.log
```

**Vault file format**:

```
┌─────────────────────────────────────┐
│  Magic bytes: "FORGED\x00\x01"     │  8 bytes
├─────────────────────────────────────┤
│  Version: uint16                    │  2 bytes
├─────────────────────────────────────┤
│  Argon2id parameters:              │
│    Salt (32 bytes)                  │
│    Time cost (uint32)              │
│    Memory cost (uint32)            │
│    Parallelism (uint8)             │
├─────────────────────────────────────┤
│  Nonce (24 bytes, XChaCha20)       │
├─────────────────────────────────────┤
│  Encrypted payload                  │
│  (XChaCha20-Poly1305)              │
│  ┌─────────────────────────────┐   │
│  │  JSON:                      │   │
│  │  {                          │   │
│  │    "keys": [...],           │   │
│  │    "hosts": [...],          │   │
│  │    "metadata": {...},       │   │
│  │    "version_vector": {...}, │   │
│  │    "tombstones": [...],     │   │
│  │    "key_generation": 1      │   │
│  │  }                          │   │
│  └─────────────────────────────┘   │
├─────────────────────────────────────┤
│  Auth tag (16 bytes, Poly1305)     │
└─────────────────────────────────────┘
```

**Encryption chain**:
1. User provides master password
2. Argon2id derives 256-bit key from password + salt (time=3, memory=64MB, parallelism=4)
3. XChaCha20-Poly1305 encrypts the vault payload
4. Auth tag provides integrity verification

**Why XChaCha20-Poly1305**: 24-byte nonce eliminates nonce-reuse risk (critical when syncing across devices). Faster than AES-GCM on systems without AES-NI (common on ARM/budget machines). Used by age, WireGuard, and Bitwarden.

**Argon2id parameters**: Stored in the vault header, not hardcoded. Defaults tuned for modern hardware (64MB, 3 iterations). `forged benchmark` command available to test and recommend parameters for the current machine.

**Vault format versioning**:
- Header contains a version field (uint16)
- Daemon reads any version <= current
- Daemon always writes latest version
- On open, if version < current, auto-migrate in-memory and rewrite
- Never drop support for reading old versions

---

## IPC Protocol

CLI commands talk to the daemon over a separate Unix socket (`ctl.sock`), not the SSH agent socket.

### Why separate sockets

The SSH agent socket speaks the SSH agent protocol (binary, fixed message types). Management commands (add key, list keys, get status) need a richer protocol. Mixing them would require protocol extensions that break SSH client compatibility.

### Protocol

Simple request-response over the Unix socket:

```
┌─────────────────────────┐
│  Length: uint32 (4 bytes)│  ← Total message length
├─────────────────────────┤
│  JSON payload            │
│  {                       │
│    "command": "list",    │
│    "args": {...}         │
│  }                       │
└─────────────────────────┘
```

Response:

```
┌─────────────────────────┐
│  Length: uint32 (4 bytes)│
├─────────────────────────┤
│  JSON payload            │
│  {                       │
│    "status": "ok",       │
│    "data": {...}         │
│  }                       │
└─────────────────────────┘
```

### IPC Commands

```
list                    → List all keys (name, type, fingerprint)
add                     → Add/import a key
generate                → Generate a new key pair
remove                  → Remove a key by name
rename                  → Rename a key
export                  → Get public key
host                    → Map key to host patterns
unhost                  → Remove host mapping
hosts                   → List host-key mappings
status                  → Daemon status (uptime, key count, sync)
lock                    → Lock vault (zero keys from memory)
unlock                  → Unlock vault
sync-trigger            → Force sync now
sync-status             → Sync status
config-get              → Get config value
config-set              → Set config value
activity                → Recent auth events
```

### CLI → Daemon flow

```
$ forged list
    │
    ├─ CLI connects to ctl.sock
    ├─ CLI sends: {"command": "list"}
    ├─ Daemon queries in-memory key store
    ├─ Daemon responds: {"status": "ok", "data": {"keys": [...]}}
    ├─ CLI formats output (table or JSON based on --json flag)
    └─ CLI exits

$ forged add mykey --file ~/.ssh/id_ed25519
    │
    ├─ CLI reads private key from file
    ├─ CLI connects to ctl.sock
    ├─ CLI sends: {"command": "add", "args": {"name": "mykey", "private_key": "..."}}
    ├─ Daemon acquires write lock (RWMutex)
    ├─ Daemon adds key to in-memory store
    ├─ Daemon writes vault (atomic: tmp + fsync + rename)
    ├─ Daemon releases write lock
    ├─ Daemon responds: {"status": "ok"}
    ├─ Daemon triggers sync push (async, non-blocking)
    └─ CLI prints success
```

### Error handling

When the daemon is not running, every CLI command that requires it shows:

```
$ forged list
Error: daemon is not running. Start it with: forged start
```

Not a cryptic "connection refused" or a hang.

---

## Data Model

### Key Object (in vault JSON)

```json
{
  "id": "uuid-v4",
  "name": "personal-github",
  "type": "ssh-ed25519",
  "public_key": "ssh-ed25519 AAAAC3Nza...",
  "private_key": "base64...",
  "comment": "ritik@macbook",
  "fingerprint": "SHA256:abc123...",
  "created_at": "2026-03-15T10:00:00Z",
  "updated_at": "2026-03-15T10:00:00Z",
  "last_used_at": "2026-04-01T12:04:00Z",
  "tags": ["github", "personal"],
  "host_rules": [
    {
      "match": "github.com",
      "type": "exact"
    },
    {
      "match": "*.github.com",
      "type": "wildcard"
    }
  ],
  "git_signing": true,
  "version": 3,
  "device_origin": "device-uuid-macbook"
}
```

Note: `private_key` is stored in plaintext within the vault JSON because the entire vault payload is encrypted. There is no double-encryption of individual keys.

### Tombstone Object (for sync conflict resolution)

```json
{
  "key_id": "uuid-v4",
  "deleted_at": "2026-04-01T12:00:00Z",
  "deleted_by_device": "device-uuid-macbook"
}
```

Tombstones are pruned after 90 days. Devices that haven't synced in 90 days do a full vault replacement instead of a merge.

### Device Object

```json
{
  "id": "uuid-v4",
  "name": "MacBook Pro",
  "platform": "darwin/arm64",
  "hostname": "ritiks-macbook",
  "registered_at": "2026-01-01T00:00:00Z",
  "last_sync_at": "2026-04-01T12:00:00Z",
  "forged_version": "1.2.0",
  "public_key": "device-specific-key-for-e2e-sync"
}
```

### Activity Event

```json
{
  "timestamp": "2026-04-01T12:04:00Z",
  "type": "sign_request",
  "key_name": "personal-github",
  "key_fingerprint": "SHA256:abc123...",
  "remote_host": "github.com",
  "result": "success",
  "client_pid": 12345,
  "client_name": "git"
}
```

---

## Security Architecture

### Threat Model

| Threat | Mitigation |
|--------|-----------|
| **Disk theft / lost laptop** | Vault encrypted with Argon2id + XChaCha20-Poly1305. Without master password, vault is opaque bytes. |
| **Server compromise** | Zero-knowledge: server stores only encrypted blobs. No plaintext keys ever leave the client. |
| **Memory dump / swap** | Key memory pages locked with `mlock()`. Daemon zeros key material on lock/shutdown. |
| **Local process snooping on agent socket** | Socket file permissions set to `0600`. Only the owning user can connect. |
| **Man-in-the-middle on sync** | TLS for transport. Vault payload independently encrypted with client-side key. Double encryption. |
| **Master password brute force** | Argon2id with high parameters (64MB memory, 3 iterations). Rate limiting on cloud login. |
| **Rogue device added to account** | New device registration requires approval from an existing device OR entering a device pairing code. |
| **Agent forwarding abuse** | Forged logs all signing requests with client PID and name. Optional: restrict which keys are available for forwarded sessions. |
| **Vault corruption (crash mid-write)** | Atomic writes: write to tmp, fsync, rename. Never partial writes to the vault file. |
| **Concurrent vault access** | Daemon holds exclusive flock. CLI never touches vault directly. |
| **Go GC copies key material** | Mitigated with mlock (prevents swap). Best-effort zeroing. Documented limitation. |

### Key Hierarchy

```
Master Password (user-provided)
    │
    ├─ Argon2id(salt_A, time=3, mem=64MB, p=4)
    │   └─ Vault Encryption Key (256-bit) — NEVER sent to server
    │       │
    │       ├─ Encrypts local vault file (XChaCha20-Poly1305)
    │       │
    │       └─ HKDF-SHA256(context="forged-sync")
    │           └─ Sync Key — encrypts vault blob for cloud upload
    │
    └─ Argon2id(salt_B, time=3, mem=64MB, p=4)
        └─ Auth Key → bcrypt → server stores this hash
           (server can authenticate user but CANNOT decrypt vault)
```

This is the same dual-derivation approach used by 1Password and Bitwarden.

### What the server stores

```
┌─────────────────────────────────────┐
│  Server Database (per user)          │
│                                      │
│  user_id: "uuid"                     │
│  email: "ritik@example.com"          │
│  auth_hash: bcrypt(auth_key)         │  ← Account auth only
│  encrypted_vault: "opaque blob"      │  ← Server can't read this
│  vault_version: 42                   │  ← Optimistic locking
│  key_generation: 1                   │  ← Bumped on password change
│  devices: [...]                      │
│  created_at: "..."                   │
│  updated_at: "..."                   │
└─────────────────────────────────────┘
```

### Master Password Change

1. User changes password on Device A
2. New `key_generation` counter bumped (1 → 2)
3. Vault re-encrypted with new vault key, pushed to server
4. Device B pulls new vault, can't decrypt (wrong key generation)
5. Device B prompts: "Master password was changed. Enter new password."
6. User enters new password on Device B, vault decrypts, continues

**v2 path**: Per-device encryption keys. Each device has its own key pair. Vault blob encrypted to all device public keys. Password change re-encrypts locally without affecting other devices.

---

## Vault Integrity

This section covers how we prevent data loss — the most critical non-feature requirement.

### Atomic Writes

Every vault write follows this sequence:

```
1. Write to vault.forged.tmp (same directory, same filesystem)
2. fsync(vault.forged.tmp)  — force to disk
3. rename(vault.forged.tmp → vault.forged) — atomic on all major filesystems
```

If the daemon crashes at step 1 or 2, the original vault is untouched. If it crashes during step 3 (extremely unlikely — rename is atomic), the filesystem guarantees either the old or new file, never a partial.

### File Locking

The daemon acquires an exclusive `flock()` on the vault file at startup. This prevents:
- Two daemon instances running simultaneously
- CLI commands directly modifying the vault
- External tools corrupting the file

On Windows: `LockFileEx()` with exclusive lock.

### Single-Writer Architecture

```
CLI command ──(IPC over ctl.sock)──► Daemon ──(flock)──► vault.forged
                                       │
                                       ├─ Read: RWMutex.RLock (concurrent)
                                       └─ Write: RWMutex.Lock (exclusive)
```

The daemon is the **sole process** that reads or writes the vault file. All CLI commands go through IPC. This eliminates an entire class of race conditions.

---

## Sync Protocol

### Design Goals

1. Offline-first: changes queue locally, sync when network available
2. Multi-device: any device can add/modify/delete keys
3. Conflict resolution: deterministic, no user intervention needed
4. Bandwidth efficient: whole-vault sync (vault is small, <50KB typical)

### Sync Flow

```
Device A (makes change)              Cloud Server              Device B
    │                                     │                        │
    ├─ User adds key "new-server"         │                        │
    ├─ Vault version: 41 → 42            │                        │
    ├─ Encrypt vault blob                 │                        │
    │                                     │                        │
    ├─ POST /api/v1/sync/push ──────────►│                        │
    │   { vault_blob, version: 42,        │                        │
    │     device_id, changes: [...] }     │                        │
    │                                     │                        │
    │  ◄── 200 OK ────────────────────────│                        │
    │                                     │                        │
    │                                     │   (periodic poll)      │
    │                                     │                        │
    │                                     │◄── GET /api/v1/sync/pull
    │                                     │    { last_version: 41 }│
    │                                     │                        │
    │                                     ├── 200 { vault_blob,  ─►│
    │                                     │    version: 42 }       │
    │                                     │                        │
    │                                     │              Decrypt ──┤
    │                                     │              Merge   ──┤
    │                                     │              Done    ──┤
```

### Conflict Resolution

**Strategy: Whole-vault replacement with version vector**

Each device maintains a version counter. The vault includes a version vector:

```json
{
  "version_vector": {
    "device-macbook": 15,
    "device-ubuntu": 8,
    "device-windows": 3
  }
}
```

On conflict (two devices push simultaneously):
1. Server rejects the second push (optimistic locking: `UPDATE vaults SET ... WHERE version = $expected`)
2. Rejected device pulls latest vault, merges locally:
   - Key additions: union (keep both)
   - Key deletions: tombstone with timestamp (latest delete wins)
   - Key modifications: last-writer-wins based on `updated_at`
3. Merged vault pushed with new version

**Tombstone TTL**: Tombstones pruned after 90 days. Devices offline >90 days do full vault replacement.

**Why whole-vault sync (not per-key)**:
- Vault is small (typical: 5-20 keys = under 50KB encrypted)
- Eliminates partial-sync corruption risk
- Simpler server (just stores a blob)
- Matches 1Password and Bitwarden's approach

### Sync Failure Handling

- Sync failures are logged, not shown to user unless they run `forged sync status`
- Failed pushes queue locally, retry with exponential backoff (1s, 2s, 4s, ... max 5m)
- The daemon never blocks on sync — all sync runs in a background goroutine
- `forged status` shows: `Sync: last success 2 hours ago (server unreachable)`

---

## CLI Design

### Command Structure

```
forged                              # Show status (daemon running? keys loaded?)
forged setup                        # First-time setup wizard
forged daemon                       # Start daemon in foreground (for debugging)
forged start                        # Start daemon (via system service)
forged stop                         # Stop daemon
forged status                       # Show daemon status, key count, sync status

forged add <name>                   # Import a key from file or clipboard
forged generate <name>              # Generate a new Ed25519 key pair
forged list                         # List all keys (name, type, fingerprint)
forged remove <name>                # Remove a key
forged export <name>                # Export public key to stdout
forged rename <old> <new>           # Rename a key

forged host <name> <patterns...>    # Map a key to host patterns
forged hosts                        # List all host-key mappings
forged unhost <name> <pattern>      # Remove a host mapping

forged sync                         # Force sync now
forged sync status                  # Show sync status

forged sign                         # Git signing helper (called by git)

forged login                        # Authenticate with cloud server
forged logout                       # Clear cloud credentials
forged register                     # Create cloud account

forged lock                         # Lock vault (clear keys from memory)
forged unlock                       # Unlock vault

forged config                       # Open config in $EDITOR
forged config get <key>             # Get config value
forged config set <key> <value>     # Set config value

forged logs                         # Tail daemon logs
forged benchmark                    # Test Argon2id speed, recommend parameters

forged doctor                       # Diagnose common issues (Phase 5)
forged migrate                      # Import from 1Password / ssh-agent (Phase 5)
```

### Global Flags

```
--json                              # Machine-readable JSON output (all commands)
--verbose                           # Verbose logging
--config <path>                     # Override config file path
```

### Example Workflows

**First time setup**:
```bash
$ brew install itzzritik/tap/forged
$ forged setup

Welcome to Forged! Let's set up your SSH key manager.

Create a master password: ••••••••••••
Confirm: ••••••••••••

Would you like to:
  1. Import existing SSH keys from ~/.ssh/
  2. Import from 1Password
  3. Generate new keys
  4. Start fresh

> 1

Found 3 keys in ~/.ssh/:
  id_ed25519 (ssh-ed25519, SHA256:abc...)
  github_work (ssh-ed25519, SHA256:def...)
  deploy_key (ssh-rsa, SHA256:ghi...)

Imported 3 keys. Give them names:
  id_ed25519  → personal-github
  github_work → work-github
  deploy_key  → prod-deploy

Setting up daemon...
  Created ~/.forged/config.toml
  Installed launchd service
  Started daemon
  Updated ~/.ssh/config (added IdentityAgent)

Would you like to set up cloud sync? (y/n) y
  → Visit https://forged.ritik.me/register or run: forged register

Setup complete! Your SSH agent is running.
  Socket: ~/.forged/agent.sock
```

**Daily use**:
```bash
# Just works — no interaction needed
$ git push origin main    # Agent provides the right key automatically
$ ssh oracle              # Agent provides homelab key

# Add a new key
$ forged generate staging-server
Generated ssh-ed25519 key: staging-server
Public key:
  ssh-ed25519 AAAAC3Nza... staging-server

$ forged host staging-server "staging.company.com" "*.staging.company.com"
Mapped staging-server → staging.company.com, *.staging.company.com

# Check what's happening
$ forged status
Daemon: running (PID 12345, uptime 3d 2h)
Keys: 4 loaded
Sync: enabled (last sync 2 min ago, 3 devices)
Socket: ~/.forged/agent.sock

# Machine-readable output for scripts
$ forged list --json
[
  {"name": "personal-github", "type": "ssh-ed25519", "fingerprint": "SHA256:abc..."},
  {"name": "work-github", "type": "ssh-ed25519", "fingerprint": "SHA256:def..."}
]
```

---

## Cloud Infrastructure

Two separate deployments:

### Sync API (`forged-api.ritik.me`) - Go on Fly.io

API only, no HTML. Handles sync, device management, OAuth token exchange.

| Component | Technology |
|-----------|-----------|
| Language | Go (stdlib `net/http`) |
| Database | PostgreSQL (`pgx`) |
| Auth | Google/GitHub OAuth + JWT |
| Hosting | Fly.io |

**API Routes** (all under `/api/v1/`):

```
GET    /api/v1/auth/google            Redirect to Google OAuth
GET    /api/v1/auth/google/callback   Exchange code, redirect to CLI with token
GET    /api/v1/auth/github            Redirect to GitHub OAuth
GET    /api/v1/auth/github/callback   Exchange code, redirect to CLI with token
POST   /api/v1/sync/push              Upload encrypted vault blob
GET    /api/v1/sync/pull              Download encrypted vault blob
GET    /api/v1/sync/status            Sync metadata
GET    /api/v1/devices                List registered devices
POST   /api/v1/devices                Register a new device
DELETE /api/v1/devices/:id            Deauthorize a device
POST   /api/v1/devices/:id/approve    Approve a pending device
GET    /api/v1/account                Account info
POST   /api/v1/account/delete         Delete account + all data
GET    /health                        Health check
```

### Web App (`forged.ritik.me`) - Next.js on Vercel

Everything users see in a browser.

| Component | Technology |
|-----------|-----------|
| Framework | Next.js (App Router) |
| Styling | Tailwind CSS |
| Hosting | Vercel |

**Pages**:

- `/` - Landing page (hero, features, comparison, CTA)
- `/login` - OAuth login page (Google/GitHub buttons), redirects CLI callback with token
- `/pricing` - Free (local) / Pro (cloud sync) / Team (per user)
- `/docs` - Installation, setup, configuration guides
- `/security` - Security model explanation
- `/dashboard` - Manage devices, billing, plan (future)

### Auth Flow

```
$ forged login
  1. CLI starts localhost:RANDOM/callback listener
  2. Opens browser to forged.ritik.me/login?callback=http://localhost:RANDOM/callback
  3. User sees login page with Google/GitHub buttons
  4. User clicks GitHub
  5. Next.js redirects to forged-api.ritik.me/api/v1/auth/github?callback=...
  6. Go server redirects to GitHub OAuth
  7. User authorizes on GitHub
  8. GitHub redirects back to Go server with code
  9. Go server exchanges code for user info, creates/upserts user, generates JWT
  10. Go server redirects to localhost:RANDOM/callback?token=...&email=...
  11. CLI receives token, saves to credentials.json
  12. Done
```

### Database Schema

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    name TEXT,
    provider TEXT NOT NULL,
    provider_id TEXT,
    key_generation INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE vaults (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    encrypted_blob BYTEA NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ DEFAULT now(),
    updated_by_device UUID,
    UNIQUE(user_id)
);

CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    platform TEXT NOT NULL,
    hostname TEXT,
    device_public_key TEXT NOT NULL,
    registered_at TIMESTAMPTZ DEFAULT now(),
    last_seen_at TIMESTAMPTZ DEFAULT now(),
    approved BOOLEAN DEFAULT false
);

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    device_id UUID REFERENCES devices(id),
    action TEXT NOT NULL,
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

---

## Host Matching Engine

The differentiator. Parses `~/.ssh/config` and Forged's own config to determine which key to offer for which host.

### The fundamental problem

The SSH agent protocol has no host context. When an SSH client asks the agent "list your identities" (`REQUEST_IDENTITIES`), it doesn't say which host it's connecting to. This is a known protocol limitation.

### How Forged handles it

**Strategy: Intelligent key ordering, not filtering.**

Return ALL keys on `REQUEST_IDENTITIES`, but order them so the most likely key is first. SSH servers typically allow 3-6 auth attempts before disconnecting. With intelligent ordering, the right key is tried first.

```
SSH client connects to git@github.com
    │
    ▼
SSH client asks agent: "list your identities"
    │
    ▼
Forged returns all keys, ordered by:
    │
    ├─ 1. Forged config rules (explicit mapping)
    │     [[hosts]]
    │     match = ["github.com"]
    │     key = "personal-github"
    │
    ├─ 2. ~/.ssh/config IdentityFile hints
    │     Host github.com
    │       IdentityFile ~/.ssh/github_key
    │     → Forged maps this filename to a vault key
    │
    ├─ 3. Key comment matching
    │     Key comment "github-work" matches host pattern
    │
    ├─ 4. Recent usage
    │     Keys used more recently are ranked higher
    │
    └─ 5. Remaining keys in alphabetical order
```

### Forged config for host matching (`config.toml`)

```toml
[agent]
socket = "~/.forged/agent.sock"     # Unix socket path
log_level = "info"

[sync]
server = "https://forged-api.ritik.me"   # Cloud sync endpoint
interval = "5m"                      # Sync interval
enabled = true

# Host-to-key mapping rules
[[hosts]]
name = "GitHub Personal"
match = ["github.com", "*.github.com"]
key = "personal-github"             # Key name in vault
git_signing = true                   # Use this key for git signing too

[[hosts]]
name = "GitHub Work"
match = ["github-work"]             # Matches Host alias in ~/.ssh/config
key = "work-github"

[[hosts]]
name = "Production Servers"
match = ["*.prod.company.com", "10.0.*"]
key = "prod-deploy"

[[hosts]]
name = "Home Lab"
match = ["oracle", "proxmox", "192.168.*"]
key = "homelab"

# Default: if no rule matches, offer all keys
[hosts.default]
strategy = "all"                    # "all" | "none" | "first"
```

### Supported match patterns

- Exact hostname: `github.com`
- Wildcard: `*.prod.company.com`
- IP ranges: `10.0.*`, `192.168.68.*`
- SSH config Host aliases: matches the `Host` keyword, not just the real hostname
- Port-based: `host:2222`
- Regex (opt-in): `~^bastion-\d+\.aws\.`

---

## Git Signing

Forged provides a `forged-sign` binary that implements the SSH signing protocol, replacing 1Password's `op-ssh-sign`.

**Git configuration** (set up by `forged setup`):

```gitconfig
[user]
    signingkey = ssh-ed25519 AAAAC3Nza... # From forged vault

[gpg]
    format = ssh

[gpg "ssh"]
    program = /usr/local/bin/forged-sign  # Symlink or subcommand wrapper

[commit]
    gpgsign = true
```

**How `forged-sign` works**:
1. Git calls `forged-sign` with the data to sign
2. `forged-sign` connects to the running daemon via the agent socket
3. Daemon signs the data with the key designated for git signing (from host config)
4. Signed data returned to Git

**Implementation**: The signing protocol is identical to what `ssh-keygen -Y sign` does. The forged daemon handles the actual signing via its in-memory keys.

**Allowed signers management**:
- `forged setup` creates `~/.ssh/allowed_signers` with your own public key
- `forged` can optionally manage this file for team setups

---

## Platform Integration

### macOS

```
Installation:
  brew install itzzritik/tap/forged

Daemon management:
  ~/Library/LaunchAgents/dev.forged.agent.plist

  <?xml version="1.0" encoding="UTF-8"?>
  <plist version="1.0">
  <dict>
      <key>Label</key>
      <string>dev.forged.agent</string>
      <key>ProgramArguments</key>
      <array>
          <string>/opt/homebrew/bin/forged</string>
          <string>daemon</string>
      </array>
      <key>RunAtLoad</key>
      <true/>
      <key>KeepAlive</key>
      <true/>
      <key>StandardOutPath</key>
      <string>/Users/USER/.forged/logs/forged.log</string>
      <key>StandardErrorPath</key>
      <string>/Users/USER/.forged/logs/forged.log</string>
  </dict>
  </plist>

File paths:
  Config:   ~/.forged/config.toml
  Vault:    ~/.forged/vault.forged
  Socket:   ~/.forged/agent.sock
  IPC:      ~/.forged/ctl.sock
  PID:      ~/.forged/daemon.pid
  Logs:     ~/.forged/logs/forged.log

SSH config injection:
  Host *
      IdentityAgent "~/.forged/agent.sock"

Notes:
  - Gatekeeper notarization required for distribution (Apple Developer, $99/yr)
  - Plan for codesign + notarytool in CI (Phase 6)
  - Do NOT touch macOS Keychain — avoid system prompts
  - launchd throttles restarts if daemon crashes repeatedly
```

### Linux

```
Installation:
  curl -fsSL https://get.forged.ritik.me | sh
  # or: apt install forged (from our repo)
  # or: brew install itzzritik/tap/forged

Daemon management:
  ~/.config/systemd/user/forged.service

  [Unit]
  Description=Forged SSH Agent
  After=default.target

  [Service]
  Type=simple
  ExecStart=/usr/local/bin/forged daemon
  Restart=always
  RestartSec=5

  [Install]
  WantedBy=default.target

  $ systemctl --user enable forged
  $ systemctl --user start forged

File paths (XDG-compliant):
  Config:   $XDG_CONFIG_HOME/forged/config.toml    (~/.config/forged/)
  Vault:    $XDG_DATA_HOME/forged/vault.forged      (~/.local/share/forged/)
  Socket:   $XDG_RUNTIME_DIR/forged/agent.sock      (/run/user/1000/forged/)
  IPC:      $XDG_RUNTIME_DIR/forged/ctl.sock
  PID:      $XDG_RUNTIME_DIR/forged/daemon.pid
  Logs:     $XDG_STATE_HOME/forged/logs/forged.log  (~/.local/state/forged/)

Notes:
  - Not everyone uses systemd. Document "run forged daemon however your init system works"
  - SELinux/AppArmor may restrict socket creation and mlock — document and provide policy files post-launch
```

### Windows

```
Installation:
  scoop bucket add itzzritik https://github.com/itzzritik/scoop-bucket
  scoop install forged
  # or: winget install itzzritik.forged
  # or: download MSI from GitHub releases

Daemon management:
  Windows Task Scheduler (created by `forged setup`)
  - Trigger: At log on
  - Action: Start forged.exe daemon
  - Settings: Do not stop if computer switches to battery

SSH agent:
  Named pipe: \\.\pipe\forged-agent
  Support: OpenSSH for Windows named pipe protocol ONLY
  (PuTTY Pageant protocol is legacy — not supported)

  PowerShell:
  $env:SSH_AUTH_SOCK = "\\.\pipe\forged-agent"
  # Persist in $PROFILE

File paths:
  Config:   %APPDATA%\forged\config.toml
  Vault:    %APPDATA%\forged\vault.forged
  PID:      %APPDATA%\forged\daemon.pid
  Logs:     %APPDATA%\forged\logs\forged.log

Notes:
  - Named pipe ACLs must be set to owner-only (explicit security descriptor)
  - Use LockFileEx for vault file locking
  - Use VirtualLock for memory locking
```

---

## Distribution & Packaging

### Build Pipeline (GoReleaser)

```yaml
# .goreleaser.yml
project_name: forged

builds:
  - main: ./cli/cmd/forged
    binary: forged
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}

  - main: ./cli/cmd/forged-sign
    binary: forged-sign
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]
    ldflags: [-s -w]

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

brews:
  - repository:
      owner: itzzritik
      name: homebrew-tap
    homepage: https://forged.ritik.me
    description: "SSH key management — forge your keys, take them anywhere"
    install: |
      bin.install "forged"
      bin.install "forged-sign"

nfpms:
  - package_name: forged
    vendor: Forged
    homepage: https://forged.ritik.me
    maintainer: Ritik Srivastava
    description: "SSH key management"
    formats:
      - deb
      - rpm

scoops:
  - repository:
      owner: itzzritik
      name: scoop-bucket
    homepage: https://forged.ritik.me
    description: "SSH key management"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

### Release Process

```
Tag v1.0.0 on main branch
    │
    ▼
GitHub Actions triggered
    │
    ├─ GoReleaser builds all platforms
    ├─ Creates GitHub Release with:
    │   ├─ forged-darwin-arm64.tar.gz
    │   ├─ forged-darwin-amd64.tar.gz
    │   ├─ forged-linux-amd64.tar.gz
    │   ├─ forged-linux-arm64.tar.gz
    │   ├─ forged-windows-amd64.zip
    │   ├─ forged_1.0.0_amd64.deb
    │   ├─ forged-1.0.0.x86_64.rpm
    │   └─ checksums.txt (SHA256)
    ├─ Updates Homebrew tap formula
    ├─ Updates Scoop bucket manifest
    └─ Publishes to APT repo (GitHub Pages or S3)
```

### Install script (`get.forged.ritik.me`)

```bash
#!/bin/sh
set -e

REPO="itzzritik/forged"
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d '"' -f 4)

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

URL="https://github.com/$REPO/releases/download/$LATEST/forged-${OS}-${ARCH}.tar.gz"

echo "Downloading forged $LATEST for $OS/$ARCH..."
curl -fsSL "$URL" | tar xz -C /usr/local/bin/ forged forged-sign

echo "Forged installed! Run 'forged setup' to get started."
```

---

## Mono Repo Structure

### Why mono repo

- Go client and Next.js server share a sync protocol contract — changes must be atomic
- Single issue tracker, single CI, one place to look
- Every successful client+server open-source project uses mono repo

### Why NOT Turborepo / Nx / Lerna

Go and Next.js have completely independent build systems. Go doesn't know about npm. npm doesn't know about Go. There's no shared build graph. A monorepo orchestrator would sit on top adding complexity for zero benefit.

### Tooling

| Concern | Solution |
|---------|----------|
| Build orchestration | `justfile` (like Make, but modern) |
| Go builds | `go build` — Go modules handle everything |
| Next.js builds | `npm run build` inside `server/` |
| CI/CD | GitHub Actions with path filters |
| Linting | Each language uses its own tools |

### Justfile

```just
# Development
dev-cli:
    cd cli && go run ./cmd/forged daemon

dev-server:
    cd server && npm run dev

# Build
build-cli:
    cd cli && go build -o bin/forged ./cmd/forged
    cd cli && go build -o bin/forged-sign ./cmd/forged-sign

build-server:
    cd server && go build -o ../bin/forged-server ./cmd/forged-server

build: build-cli build-server

# Lint
lint-cli:
    cd cli && golangci-lint run

lint-server:
    cd server && golangci-lint run

lint: lint-cli lint-server
```

### CI Path Filters

```yaml
# .github/workflows/ci-cli.yml
on:
  push:
    paths: ['cli/**', 'proto/**']
  pull_request:
    paths: ['cli/**', 'proto/**']

# .github/workflows/ci-server.yml
on:
  push:
    paths: ['server/**', 'proto/**']
  pull_request:
    paths: ['server/**', 'proto/**']
```

Both workflows trigger when `proto/` changes (shared contract).

### Directory Layout

```
forged/
├── cli/                              # Go module (client + daemon)
│   ├── cmd/
│   │   ├── forged/                   # Main CLI + daemon entry point
│   │   │   └── main.go
│   │   └── forged-sign/              # Git signing helper binary
│   │       └── main.go
│   ├── internal/
│   │   ├── agent/                    # SSH agent protocol implementation
│   │   │   ├── agent.go              # Agent interface
│   │   │   ├── server.go             # Socket listener
│   │   ├── vault/                    # Encrypted vault management
│   │   │   ├── vault.go              # Vault CRUD operations
│   │   │   ├── crypto.go             # Encryption/decryption
│   │   │   ├── format.go             # Vault file format (binary header + JSON payload)
│   │   ├── hostmatch/                # Host matching engine
│   │   │   ├── matcher.go            # Pattern matching logic
│   │   │   ├── sshconfig.go          # ~/.ssh/config parser
│   │   ├── ipc/                      # CLI ↔ daemon communication
│   │   │   ├── server.go             # Daemon-side IPC handler
│   │   │   ├── client.go             # CLI-side IPC client
│   │   │   ├── protocol.go           # Message types and serialization
│   │   ├── sync/                     # Cloud sync client
│   │   │   ├── client.go             # HTTP sync client
│   │   │   ├── merge.go              # Conflict resolution
│   │   ├── config/                   # Configuration management
│   │   │   ├── config.go             # Config struct + parsing
│   │   │   ├── paths.go              # Platform-specific paths (XDG, etc.)
│   │   │   └── defaults.go           # Default values
│   │   ├── daemon/                   # Daemon lifecycle
│   │   │   ├── daemon.go             # Start/stop/status, signal handling
│   │   │   ├── launchd.go            # macOS launchd integration
│   │   │   ├── systemd.go            # Linux systemd integration
│   │   │   └── windows.go            # Windows Task Scheduler integration
│   │   ├── signer/                   # Git signing protocol
│   │   │   └── signer.go
│   │   └── platform/                 # Platform-specific code
│   │       ├── mlock_unix.go         # Memory locking (Unix)
│   │       ├── mlock_windows.go      # Memory locking (Windows)
│   │       ├── socket_unix.go        # Unix domain socket
│   │       └── pipe_windows.go       # Windows named pipe
│   ├── go.mod
│   └── go.sum
├── server/                           # Go sync API (forged-api.ritik.me)
│   ├── cmd/forged-server/main.go     # Entry point
│   ├── internal/
│   │   ├── api/                      # HTTP handlers
│   │   ├── auth/                     # OAuth + JWT
│   │   ├── db/                       # PostgreSQL queries
│   │   └── middleware/               # Auth, logging, CORS
│   ├── migrations/                   # SQL migration files
│   ├── go.mod
│   └── Dockerfile
├── web/                              # Next.js web app (forged.ritik.me)
│   ├── src/app/
│   │   ├── page.tsx                  # Landing page
│   │   ├── login/                    # OAuth login page
│   │   ├── pricing/                  # Pricing page
│   │   └── docs/                     # Documentation
│   └── package.json
├── proto/                            # Shared contract specifications
│   ├── vault-format.md               # Binary vault format spec (versioned)
│   ├── sync-api.md                   # Cloud sync HTTP API contract
│   └── ipc.md                        # CLI ↔ daemon socket protocol
├── scripts/
│   ├── install.sh                    # Universal install script
│   └── completions/                  # Shell completions (bash, zsh, fish)
├── justfile                          # Build/lint orchestration
├── .goreleaser.yml                   # Release configuration
├── .github/
│   └── workflows/
│       ├── ci-cli.yml                # Lint Go (path filtered)
│       ├── ci-server.yml             # Lint Next.js (path filtered)
│       └── release.yml               # GoReleaser on tag
├── LICENSE
└── README.md
```

---

## Project Phases

### Phase 1: Core Agent (Week 1-2)

**Goal**: A working SSH agent that can replace ssh-agent.

**Explicitly skip in Phase 1**:
- memguard / mmap-based key storage (document limitation)
- Shell completions
- Windows anything
- Sync anything
- `forged doctor`, `forged migrate`, `forged benchmark`

**Deliverable**: Can replace `ssh-agent` for basic daily SSH/Git use on macOS/Linux.

#### Batch 1: Scaffold

- [ ] Go module init (`cli/go.mod`)
- [ ] Directory structure (`cmd/forged/main.go`, all `internal/*` packages as empty files)
- [ ] `justfile` with build/lint targets
- [ ] `proto/` directory with vault format and IPC specs (markdown)
- [ ] `.goreleaser.yml` (basic config that builds correctly)
- [ ] Cobra CLI skeleton with subcommands (stubs that print "not implemented")
- [ ] `internal/config/paths.go` — XDG paths on Linux, `~/.forged/` on macOS
- [ ] `--json` and `--verbose` global flags wired up

#### Batch 2: Vault Crypto

- [ ] `internal/vault/crypto.go` — Argon2id key derivation, XChaCha20-Poly1305 encrypt/decrypt
- [ ] `internal/vault/format.go` — binary header (magic, version, salt, nonce) + JSON payload
- [ ] `internal/vault/vault.go` — create, open, save vault
- [ ] Atomic writes (write to tmp + fsync + rename)
- [ ] File locking (flock)

#### Batch 3: Key Management

- [ ] `internal/vault/keys.go` — add, remove, list, generate, export key operations
- [ ] Ed25519 key generation (`crypto/ed25519`)
- [ ] Import from PEM/OpenSSH file format
- [ ] `sync.RWMutex` on the key store

#### Batch 4: Daemon Lifecycle

- [ ] `internal/daemon/daemon.go` — start, stop, PID file, signal handling
- [ ] Stale socket detection on startup (try connect → ECONNREFUSED → remove)
- [ ] Graceful shutdown (stop accepting, zero keys, remove socket, remove PID)
- [ ] `internal/platform/mlock_unix.go` — lock key memory pages
- [ ] Log rotation with lumberjack (max 10MB, 3 backups)
- [ ] `forged daemon` command (foreground, for debugging)
- [ ] `forged start` / `forged stop` (launchd on macOS, systemd on Linux)

#### Batch 5: IPC (CLI ↔ Daemon)

- [ ] `internal/ipc/protocol.go` — length-prefixed JSON messages
- [ ] `internal/ipc/server.go` — daemon listens on `ctl.sock`, dispatches commands
- [ ] `internal/ipc/client.go` — CLI connects, sends command, reads response
- [ ] Wire up CLI commands: `forged list`, `forged add`, `forged generate`, `forged remove`, `forged export`, `forged status`
- [ ] Clear error: "daemon is not running" when socket missing

#### Batch 6: SSH Agent

- [ ] `internal/agent/agent.go` — implement `ssh/agent.Agent` interface
- [ ] `internal/agent/server.go` — listen on `agent.sock`, accept connections
- [ ] `REQUEST_IDENTITIES` → return all keys from vault
- [ ] `SIGN_REQUEST` → sign with requested key
- [ ] Basic key ordering (alphabetical for now, smart ordering in Phase 2)

#### Batch 7: Setup + Integration

- [ ] `forged setup` — interactive wizard (create vault, import keys from `~/.ssh/`, install daemon service, update `~/.ssh/config` with `IdentityAgent`)
- [ ] Basic `~/.ssh/config` parser for `IdentityFile` hints
- [ ] `forged status` — show daemon status, key count, socket path

#### Batch dependency chain

```
Batch 1 (Scaffold)
    │
    ▼
Batch 2 (Vault Crypto) ──► Batch 3 (Key Management)
                                │
                                ▼
                           Batch 4 (Daemon Lifecycle)
                                │
                                ▼
                           Batch 5 (IPC)
                                │
                                ▼
                           Batch 6 (SSH Agent)
                                │
                                ▼
                           Batch 7 (Setup + Integration)
```

### Phase 2: Host Matching + Git Signing (Week 3-4)

**Goal**: Smart key selection and Git commit signing.

- [ ] Host matching engine (exact, wildcard, IP range, SSH config aliases)
- [ ] Forged config file (`config.toml`) with host rules
- [ ] `forged host` command for mapping keys to hosts
- [ ] `forged-sign` binary for Git signing protocol
- [ ] `forged setup` configures git signing automatically
- [ ] `~/.ssh/allowed_signers` management
- [ ] Activity logging (which key used for which host/operation)
- [ ] CLI: `forged hosts`, `forged host`, `forged unhost`
- [ ] Shell completions (bash, zsh, fish) via cobra

**Deliverable**: Correct key automatically selected per host. Git commits signed.

### Phase 3: Cloud Sync (Week 5-7)

**Goal**: Encrypted key sync across devices.

- [ ] Go server scaffold in `server/` (stdlib net/http, pgx, JWT)
- [ ] PostgreSQL schema + migrations (users, vaults, devices, audit)
- [ ] Auth routes: register (bcrypt), login (JWT)
- [ ] Sync routes: push/pull encrypted vault blobs (optimistic locking)
- [ ] Device routes: register, list, approve, deauthorize
- [ ] Dockerfile + Fly.io deployment config
- [ ] Client-side sync engine in Go (push/pull/merge)
- [ ] HKDF-SHA256 sync key derivation from vault key
- [ ] Conflict resolution (version vectors, tombstones with 90-day TTL)
- [ ] Offline queue with exponential backoff retry
- [ ] Master password change flow (`key_generation` counter)
- [ ] CLI: `forged login`, `forged register`, `forged sync`, `forged sync status`
- [ ] `proto/sync-api.md` specification
- [ ] CI path filters (cli/ and server/ build independently)

**Deliverable**: Keys sync across multiple machines with zero-knowledge encryption.

### Phase 4: Windows + Cross-Platform (Week 8-9)

**Goal**: Windows support and polish.

- [ ] Windows named pipe SSH agent (OpenSSH protocol only, no Pageant)
- [ ] Windows named pipe ACLs (owner-only)
- [ ] `VirtualLock` for memory locking on Windows
- [ ] `LockFileEx` for vault file locking on Windows
- [ ] Windows Task Scheduler integration
- [ ] Windows installer (MSI via WiX or go-msi)
- [ ] Scoop package
- [ ] Cross-platform CI matrix (macOS arm64, macOS amd64, Linux amd64, Linux arm64, Windows amd64)
- [ ] `forged doctor` — diagnose common issues per platform
- [ ] `forged migrate` — import from 1Password, Bitwarden, plain ssh-agent
- [ ] `forged benchmark` — test Argon2id speed, recommend parameters

**Deliverable**: Works on all three major platforms.

### Phase 5: Launch Prep (Week 10-11)

**Goal**: Open-source release on GitHub.

- [ ] README with demo GIF/video
- [ ] Documentation site (within Next.js app, MDX)
- [ ] Security model writeup (`/security` page)
- [ ] GoReleaser CI/CD pipeline
- [ ] Homebrew tap, Scoop bucket, APT repo
- [ ] Install script (`get.forged.ritik.me`)
- [ ] GitHub Actions for automated releases
- [ ] macOS Gatekeeper notarization (codesign + notarytool)
- [ ] License selection (MIT or Apache 2.0)
- [ ] Submit to Hacker News, Reddit r/programming, r/golang, r/commandline
- [ ] Product Hunt launch

**Deliverable**: Public, installable, documented open-source release.

---

## Tech Stack Summary

### Client (Go binary)

| Dependency | Purpose |
|-----------|---------|
| `golang.org/x/crypto/ssh` | SSH agent protocol, key parsing |
| `golang.org/x/crypto/ssh/agent` | Agent interface implementation |
| `golang.org/x/crypto/chacha20poly1305` | XChaCha20-Poly1305 vault encryption |
| `golang.org/x/crypto/argon2` | Argon2id key derivation |
| `golang.org/x/crypto/hkdf` | Sync key derivation |
| `github.com/BurntSushi/toml` | Config file parsing |
| `github.com/spf13/cobra` | CLI framework + shell completions |
| `github.com/fsnotify/fsnotify` | File watching (config changes) |
| `github.com/natefinch/lumberjack` | Log rotation |
| `net` (stdlib) | Unix socket / named pipe |
| `encoding/json` (stdlib) | IPC protocol serialization |

### Cloud Server (Go)

| Dependency | Purpose |
|-----------|---------|
| `net/http` (stdlib) | HTTP server |
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `golang.org/x/crypto/bcrypt` | Password hashing |
| `github.com/golang-jwt/jwt/v5` | JWT authentication |
| Fly.io | Hosting |
| PostgreSQL (Fly Postgres) | Database |

### Build & Release

| Tool | Purpose |
|------|---------|
| `just` | Local build/lint orchestration |
| GoReleaser | Cross-compile, package, release |
| GitHub Actions | CI/CD with path-filtered workflows |
| Homebrew tap | macOS distribution |
| Scoop bucket | Windows distribution |
| APT repo | Debian/Ubuntu distribution |

---

## Known Limitations & Future Considerations

### Documented limitations (ship with these)

1. **Go GC and key material**: Go's garbage collector may copy key material in memory before it's zeroed. We use `mlock()` to prevent swap and best-effort zeroing. This is documented in the security model. Production-grade mitigation (memguard, mmap outside GC) is a v2 item.
2. **SSH agent protocol has no host context**: We mitigate with intelligent key ordering, not filtering. Works for typical setups (3-6 keys). Users with 20+ keys may need explicit host rules.

### Future considerations (not planned, not promised)

1. **Per-device encryption keys**: Each device gets its own key pair. Vault encrypted to all device public keys. Eliminates password-change propagation issues.
2. **Recovery key**: Random 256-bit key displayed once during setup, stored encrypted in vault. Allows vault recovery if master password is forgotten.
3. **Team features**: Shared key vaults for organizations, key rotation policies.
4. **SSH certificates**: Support for SSH CA-signed certificates (short-lived keys).
5. **FIDO2 integration**: Optional YubiKey support for unlocking the vault.
6. **Biometric unlock**: Optional Touch ID / Windows Hello for vault unlock (not per-operation).
7. **Key rotation reminders**: Notify when keys are older than N months.
8. **SSH config generation**: Auto-generate `~/.ssh/config` entries from Forged host rules.
9. **Self-hosted server**: Docker image for self-hosting the sync server.
10. **TUI dashboard**: Terminal UI (bubbletea/lipgloss) for visual key management without leaving the terminal.
