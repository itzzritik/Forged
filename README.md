# Forged

SSH key manager that replaces 1Password's SSH agent. Zero-knowledge encrypted vault, intelligent host matching, Git commit signing — single binary, works offline, open source.

Keys sit encrypted at rest with Argon2id + XChaCha20-Poly1305. The daemon holds them decrypted in locked memory, serves them over the standard SSH agent protocol, and zeroes everything on shutdown. No browser, no Electron, no subscriptions.

## Install

```
brew install forgedkeys/tap/forged
```

Or download from [releases](https://github.com/forgedkeys/forged/releases), or:

```
curl -fsSL https://get.forged.dev | sh
```

## Setup

```
forged setup
```

Creates an encrypted vault, imports your existing `~/.ssh/` keys, installs the daemon, and configures `~/.ssh/config` to use the Forged agent. One command.

## Usage

Once the daemon is running, SSH and Git just work — no interaction needed:

```
ssh oracle                       # right key, automatically
git push origin main             # signed commit, automatically
```

Manage keys when you need to:

```
forged generate staging-server
forged add prod-key --file ~/.ssh/deploy_ed25519
forged list
forged export staging-server     # public key to stdout
forged remove old-key
```

Map keys to hosts:

```
forged host github-personal "github.com" "*.github.com"
forged host prod-deploy "*.prod.company.com" "10.0.*"
forged hosts
```

Everything supports `--json` for scripting:

```
forged list --json
forged status --json
```

## How it works

```
forged daemon (background process)
│
├─ SSH Agent        Unix socket, standard protocol
│                   ssh-add -l works, any SSH client works
│
├─ IPC Server       CLI talks to daemon over a control socket
│                   single writer to vault, no corruption
│
├─ Vault            Argon2id key derivation → XChaCha20-Poly1305
│                   atomic writes (tmp + fsync + rename)
│                   flock, 0600 permissions
│
└─ Key Store        in-memory, mlock'd, zeroed on shutdown
```

The vault is a single encrypted file. The daemon is the only process that reads or writes it. CLI commands go through IPC. There is no local HTTP server, no React dashboard, no WebSocket — just a Unix socket speaking the SSH agent protocol and a control socket for management.

## Security model

- **Vault encryption**: Argon2id (64MB, 3 iterations) derives a 256-bit key. XChaCha20-Poly1305 encrypts the payload. 24-byte random nonce per write eliminates reuse risk.
- **Memory**: Private keys held in `mlock`'d pages. Zeroed on shutdown and lock.
- **Disk**: Atomic writes prevent corruption. File lock prevents concurrent access. Vault file is `0600`.
- **Agent socket**: `0600` permissions. Only the owning user can connect.
- **No network required**: Works entirely offline. Sync is opt-in and zero-knowledge (server stores opaque blobs).

What the server never sees: your master password, your vault encryption key, your private keys. The same dual-derivation approach used by 1Password and Bitwarden.

## Comparison

| | Forged | 1Password | Secretive | ssh-agent |
|---|---|---|---|---|
| Cross-platform | Mac/Linux/Win | Mac/Linux/Win | Mac only | Mac/Linux |
| Key sync | Yes | Yes (bundled) | No | No |
| Host matching | Smart | Basic | No | No |
| Git signing | Built-in | Yes | Yes | Manual |
| Auth model | Login once | Touch ID per use | Touch ID per use | Per session |
| Cost | Free | $36/yr | Free | Free |
| Open source | Yes | No | Yes | Yes |
| Standalone | Yes | No | Yes | Yes |

## Commands

```
forged setup                     first-time wizard
forged daemon                    start in foreground
forged start / stop              manage system service
forged status                    daemon info + key count

forged generate <name>           new Ed25519 key pair
forged add <name> --file <path>  import existing key
forged list                      show all keys
forged remove <name>             delete a key
forged export <name>             print public key
forged rename <old> <new>        rename a key

forged host <key> <patterns>     map key to hosts
forged hosts                     list mappings
forged unhost <key> <pattern>    remove mapping

forged lock / unlock             clear or restore keys in memory
forged logs                      tail daemon logs
forged config                    manage configuration
```

## Project structure

```
forged/
├── cli/                Go binary (daemon + CLI)
│   ├── cmd/forged/     entry point + cobra commands
│   └── internal/
│       ├── agent/      SSH agent protocol (ExtendedAgent)
│       ├── vault/      encrypted vault + key store
│       ├── ipc/        CLI ↔ daemon communication
│       ├── daemon/     lifecycle, signals, PID management
│       ├── hostmatch/  SSH config parser + key discovery
│       ├── config/     paths (XDG on Linux, ~/.forged/ on macOS)
│       └── platform/   mlock, socket helpers
├── server/             Next.js cloud sync (Vercel + Neon)
├── proto/              vault format + IPC protocol specs
└── justfile            build / lint orchestration
```

## License

MIT
