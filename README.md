# Forged

> Forge your keys. Take them anywhere.

Your SSH keys deserve better than sitting unencrypted in `~/.ssh/`. Forged is a standalone SSH key manager that encrypts your keys, syncs them across machines, and plugs into SSH with a low-touch agent setup.

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
# First run: creates or restores your vault, repairs SSH wiring, and starts the background service
forged

# Terminal-only repair flow
forged doctor --fix

# That's it. SSH and Git now use the Forged agent.
ssh myserver
git push origin main
```

## Key management

Keys are managed in the `forged` interactive shell:

- **Key** tab: generate Ed25519 keys, import (SSH directory, key file, 1Password, Bitwarden, Forged export), view, rename, delete, export
- **Agent** tab: SSH integration and Git commit signing
- **Manage** tab: log in / sync, lock, master password settings
- **Doctor** tab: health checks and repair

## How it works

Forged runs as a background daemon. `forged` and `forged doctor --fix` repair and start that daemon for you, while `forged daemon` remains available for foreground debugging. It speaks the standard SSH agent protocol, so every SSH client already supports it. Your keys are encrypted at rest and only decrypted in locked memory while the daemon runs.

```
forged daemon
├── SSH Agent          standard protocol, ssh-add works
├── Encrypted Vault    Argon2id + AES-256-GCM
├── SSH Integration    standard IdentityAgent setup
└── Key Store          in-memory, mlock'd, zeroed on shutdown
```

No browser. No Electron. No local web server. Just a Unix socket (a named pipe on Windows) and a CLI.

## SSH integration

Forged keeps SSH integration low-touch. It manages its own SSH file under `~/.config/forged/ssh/forged.conf` and adds at most one `Include` line to your main `~/.ssh/config`.

On Linux and macOS, when the same host accepts multiple keys, Forged narrows each SSH or Git connection with per-attempt OpenSSH snippets and public-key hint files, so OpenSSH normally sees one proven key or a tiny ordered fallback set. GitHub and GitLab repo routes are learned with strict host-key provider probes; broader same-owner and same-host history is used only as a hint for ordering. Windows keeps the standard `IdentityAgent` integration, but automatic per-connection routing is unavailable. Git for Windows ships its own ssh, which cannot reach a named-pipe agent, so setup and `forged doctor --fix` point `core.sshCommand` at Windows OpenSSH unless you already set it.

Forged does not rewrite your existing host blocks or repo-local Git config.

Use `forged doctor` to see which SSH agent currently owns `IdentityAgent`. If you want to switch to another tool or uninstall Forged, turn off SSH integration in the Agent tab first. That removes only Forged-managed SSH config and leaves the rest of your `~/.ssh` setup alone.

## Security

Keys are encrypted with Argon2id (64MB memory-hard KDF) and AES-256-GCM. The vault file is written atomically to prevent corruption and locked to prevent concurrent access. Private keys live in mlock'd memory pages and are explicitly zeroed on shutdown.

The daemon is the only process that touches the vault. CLI commands talk to it over a control socket. The agent socket is 0600, owner-only; on Windows the named pipes are restricted to your account.

Cloud sync (coming soon) is zero-knowledge. The server stores opaque encrypted blobs. It never sees your master password, encryption key, or private keys.

## Comparison

| | Forged | 1Password | Bitwarden | Secretive | ssh-agent |
|---|---|---|---|---|---|
| Standalone | Yes | No | No | Yes | Yes |
| Cross-platform | Mac/Linux/Win | Mac/Linux/Win | Mac/Linux/Win | Mac only | Mac/Linux |
| Key sync | Yes | Bundled | Bundled | No | No |
| SSH integration | Standard agent | Basic | No | No | No |
| Git signing | Built-in | Yes | No | Yes | Manual |
| Auth model | Login once | Per use | Per use | Per use | Per session |
| Open source | Yes | No | Yes | Yes | Yes |

## All commands

```
forged                           open Forged and auto-repair this machine
forged doctor [--fix]            diagnose (and repair) from the terminal
forged logs [daemon|stderr|tui]  follow logs
forged version                   print version information
forged daemon                    start daemon in foreground (debug/service entrypoint)
```
