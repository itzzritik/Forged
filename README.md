# Forged

> Forge your keys. Take them anywhere.

Your SSH keys deserve better than sitting unencrypted in `~/.ssh/`. Forged is a standalone SSH key manager that encrypts your keys, syncs them across machines, and keeps SSH routing low-touch while learning which key works for each host.

Open-source replacement for 1Password and Bitwarden's SSH agent.

<!-- TODO: Add demo GIF here -->
<!-- ![demo](https://forged.ritik.me/demo.gif) -->

## Why

- Your keys sit unencrypted on disk. Anyone with access to your laptop has them.
- You copy key files between machines manually. Or you don't, and each machine has different keys.
- SSH tries every key until one works. You've hit "too many authentication failures" before.
- Git commit signing is a separate, painful setup nobody finishes.
- 1Password and Bitwarden work, but they bundle an entire password manager for one feature.

Forged fixes all of this in a single binary.

## Install

```bash
brew install forged
```

```bash
npm i -g @getforged/cli
```

## Quick start

```bash
# One-time setup: creates encrypted vault, imports your ~/.ssh/ keys
forged setup

# Start the daemon
forged daemon

# That's it. SSH and Git just work now.
ssh myserver                     # right key, automatically
git push origin main             # same-host provider conflicts handled automatically
```

## Key management

```bash
forged generate my-key                          # new Ed25519 key
forged add work-key --file ~/.ssh/id_ed25519    # import existing
forged list                                     # show all keys
forged view my-key                              # inspect a key
forged remove old-key                           # delete a key
```

## How it works

Forged runs as a background daemon. It speaks the standard SSH agent protocol, so every SSH client already supports it. Your keys are encrypted at rest and only decrypted in locked memory while the daemon runs.

```
forged daemon
├── SSH Agent          standard protocol, ssh-add works
├── Encrypted Vault    Argon2id + XChaCha20-Poly1305
├── Local Routing      learns host affinity + advanced provider routing locally
└── Key Store          in-memory, mlock'd, zeroed on shutdown
```

No browser. No Electron. No local web server. Just a Unix socket and a CLI.

## SSH integration

Forged keeps SSH integration low-touch. By default it manages its own files under `~/.ssh/forged/` and adds at most one `Include` line to your main `~/.ssh/config`.

The base include only points SSH at the Forged agent. Forged does not rewrite your existing host blocks. If it detects an advanced same-host provider conflict, like multiple GitHub identities on `github.com`, it generates local routing rules only inside `~/.ssh/forged/config`.

Forged never needs repo-local Git config for this flow. Advanced routing stays local to the machine and disappears cleanly when you disable Forged.

Use `forged doctor` to see which SSH agent currently owns `IdentityAgent`. If you want to switch to another tool or uninstall Forged, run `forged disable` first. That removes only Forged-managed SSH config and leaves the rest of your `~/.ssh` setup alone.

## Security

Keys are encrypted with Argon2id (64MB memory-hard KDF) and XChaCha20-Poly1305. The vault file is written atomically to prevent corruption and locked to prevent concurrent access. Private keys live in mlock'd memory pages and are explicitly zeroed on shutdown.

The daemon is the only process that touches the vault. CLI commands talk to it over a control socket. The agent socket is 0600, owner-only.

Cloud sync (coming soon) is zero-knowledge. The server stores opaque encrypted blobs. It never sees your master password, encryption key, or private keys.

## Comparison

| | Forged | 1Password | Bitwarden | Secretive | ssh-agent |
|---|---|---|---|---|---|
| Standalone | Yes | No | No | Yes | Yes |
| Cross-platform | Mac/Linux/Win | Mac/Linux/Win | Mac/Linux/Win | Mac only | Mac/Linux |
| Key sync | Yes | Bundled | Bundled | No | No |
| SSH routing | Adaptive local | Basic | No | No | No |
| Git signing | Built-in | Yes | No | Yes | Manual |
| Auth model | Login once | Per use | Per use | Per use | Per session |
| Open source | Yes | No | Yes | Yes | Yes |

## All commands

```
forged setup                     first-time wizard
forged daemon                    start in foreground
forged start / stop              manage via system service
forged status                    daemon info + key count

forged generate <name>           new Ed25519 key pair
forged add <name> --file <path>  import existing key
forged list                      all keys in vault
forged remove <name>             delete a key
forged view <name> [--full]      inspect a key
forged export [--out <path>]     export the full vault
forged rename <old> <new>        rename a key

forged lock / unlock             clear or restore keys in memory
forged logs                      tail daemon logs
forged config                    manage configuration
```

All commands support `--json` for scripting.
