---
title: SSH Agent
applies_to:
  - cli/internal/agent/**
  - cli/internal/hostmatch/**
  - cli/internal/sshrouting/**
  - cli/internal/platform/pipe_windows.go
depends_on:
  - cli/daemon.md
last_verified: 2026-04-22
stable: yes
---

# SSH Agent

Implements the OpenSSH agent protocol against the vault keystore. Answers
`REQUEST_IDENTITIES` and `SIGN_REQUEST`; all mutating ops (`ADD_IDENTITY`,
`REMOVE_IDENTITY`, `REMOVE_ALL_IDENTITIES`) are stubbed to error-with-hint
pointing the user at the TUI Key tab. Socket is `agent.sock` (Unix) or
`\\.\pipe\forged-agent` (Windows).

## Must know

- **Per-connection `sessionAgent` scopes keys by client PID.** The accept
  loop resolves the peer PID and wraps the base agent so `List`/`Sign`
  return only fingerprints the SSH routing service pre-approved for that
  attempt. IF PID lookup fails THEN connection falls back to the
  unscoped agent (all keys visible).
- **Cold daemon sessions can now hydrate on first agent use.**
  `REQUEST_IDENTITIES`, `SIGN_REQUEST`, and `SIGNERS` will call into
  sensitiveauth when no active vault session exists. Success hydrates the
  daemon, then the request continues.
- **Agent access now reuses the same 4-hour shared session as the TUI.**
  If that session expires, the next list / sign / signers request has to
  re-auth through sensitiveauth before the daemon hydrates again.
- **`Lock`/`Unlock` agent opcodes are not the real control plane.**
  Forged ignores the agent passphrase and relies on sensitiveauth plus
  daemon session clear instead.
- **Key ordering preserves keystore insertion order.** OpenSSH tries
  identities in the order the agent returns them, so routing puts the
  exact-match fingerprint first and falls back to host+user hints.
- **Routing is pull, not push.** The SSH `Match exec` hook calls
  `forged __ssh-route-prepare` for every outbound SSH; candidates live in
  memory keyed by attempt token + client PID. No token → no narrowing.
  If the daemon is cold and routing has no keyStore yet, prepare falls
  back to an empty candidate set so the agent can hydrate first and then
  serve the unscoped request.
- **Host-match patterns**: `~prefix` = Go `regexp`; `*` = wildcard
  (case-insensitive); otherwise exact case-insensitive. No CIDR/IP
  syntax — IPs match as literal strings. `!` negation is NOT supported.
- **RSA SHA-2 upgrade is flag-driven.** Modern OpenSSH sets
  `SignatureFlagRsaSha256`/`512`; without it, RSA keys sign with SHA-1.
  Signer must implement `ssh.AlgorithmSigner` or the flag is silently
  dropped.
- **`RefreshMissingKey` fires on sign-miss.** A signer not found triggers
  a 750ms sync pull before giving up — masks cross-device lag but adds
  latency to real not-found cases.
- **Windows named-pipe server is wired** (`ListenPipe` with SDDL), but
  the daemon does not actually bind it yet — see `cli/ipc.md` and
  `cli/daemon.md`. Treat Windows agent as unavailable.
- Activity logging hook is `syncBus.AgentAccess(reason)` — fire-and-
  forget; no back-pressure. Reasons are `ssh_agent_list`, `ssh_agent_sign`,
  `ssh_agent_signers`, `sign_missing_key`. Agent startup can happen before
  sync exists, so missing sync wiring must stay a no-op and must never
  crash the daemon.
- `Extension` returns `ErrExtensionUnsupported` for everything. No
  confirm-required, no FIDO, no constraints.

## Decisions

- Mutating ops deliberately rejected. Agent is a read-only signer; key
  lifecycle is TUI-only so every add/remove goes through the vault
  write path and sync bus. Do NOT re-enable agent-side writes.
- Per-PID scoping over global identity lists — prevents a second SSH
  client on the same machine from seeing keys chosen for another
  attempt. The unscoped fallback is the compatibility escape hatch.
