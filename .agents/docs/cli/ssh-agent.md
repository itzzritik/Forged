---
title: SSH Agent
applies_to:
  - cli/internal/agent/**
  - cli/internal/hostmatch/**
  - cli/internal/sshrouting/**
  - cli/internal/platform/pipe_windows.go
depends_on:
  - cli/daemon.md
last_verified: 2026-07-15
stable: yes
---

# SSH Agent

Forged implements the OpenSSH agent protocol from the vault keystore. Listing and signing are supported; agent-side key mutation is not.

## Must know

- OpenSSH routing is primarily config-driven: managed `Match exec` prepares short-lived `%C` public-key slot files and enables them through per-slot `Match exec test -f ...` blocks with `IdentitiesOnly yes`.
- `%C` is connection-scope, so concurrent same-host routes share the slot directory. The service tracks attempts by client PID and updates attempt state plus the active-candidate slot union under one route lock; completion or expiry for one PID cannot remove a live sibling's slots. Agent signing still filters by PID.
- The routing service keeps an in-memory public route/key cache after vault lock. This lets route prepare emit candidate public-key hints after system lock so OpenSSH reaches the agent and external System Auth can run at signing time. The service rechecks the broker immediately before private probes, direct-probe signatures, or learned-proof writes, so stale sessions only use that public cache.
- Routed OpenSSH clients are scoped by peer PID as a fallback. Route preparation reserves a zero-candidate PID entry before cancellable work, so a failed or timed-out prepare exposes zero keys instead of falling back to the full vault even when stale `%C` slots exist.
- GitHub/GitLab repo routes are considered proven only after a provider repo probe. Exact proven repo routes emit only the proven key; same-owner and same-host history only rank candidates.
- Explicit `ssh` client commands resolve as plain SSH targets even when launched from inside a Git working tree.
- Cold daemon sessions can hydrate on first agent use if policy allows it.
- A signing lookup refreshes sync only for a typed absent vault-signable raw public key. Vault, decrypt, requested-key/private-key parse, certificate, and hardware-key failures return directly instead of adding a futile network wait.
- The listener retries temporary accept failures with bounded backoff, exits quietly when closed, and logs terminal failures. Shutdown cancels active agent authorization and missing-key refresh before closing every tracked client, so an idle SSH process cannot pin the daemon. The stock single-request agent loop cannot observe an ordinary peer disconnect while an agent call is in flight; daemon shutdown is the server connection-lifecycle cancellation boundary.
- External agent use goes through `ActionExternal`, not the TUI-style view path.
- The daemon's in-memory activity ring records SSH signing outcomes only: successful signatures, resolved signing failures, and resolved route denials include the selected public fingerprint and peer PID when available; authorization, vault, and unresolved-key outcomes use fixed `denied` or `failed` results without key metadata. A denied key-list authorization is recorded without key metadata. Canceled contexts and canceled system-auth prompts create no event. Repeated non-success events with the same fields are suppressed for 30 seconds. It never records signed data, signatures, raw error text, credentials, key names, or routine key-list polling.
- `forged-sign` now does an auth preflight so Git commit signing can show cleaner auth errors.
- Raw SSH agent protocol is still limited in how much error detail it can surface back to callers.
- SSH route preparation has one 45-second server work budget covering cold-session auth, retry, and either probe type. Provider and direct-SSH probes keep their 20-second total and 4-second per-key child caps; the hook waits 50 seconds and the server connection 55 seconds so work can return a final response before either transport closes.
- On Windows, agent and control pipes use distinct opaque current-user token-SID hashes. Automatic SSH routing is intentionally unavailable: the daemon starts no route service, removes a uniquely marked product-owned routing section from the private managed config, and preserves its agent prefix. Duplicate route markers or a missing generated `PermitLocalCommand` boundary block migration instead of being overwritten. Normal SSH-agent and commit-signing behavior remains available.

## Decisions

- Agent mutation operations stay unsupported; the TUI is the write surface. Protocol `Lock` and `Unlock` also fail explicitly because Forged does not persist or verify an agent passphrase; real locking is handled by the sensitive-session broker.
- Agent and control traffic stay on separate sockets.
- Private keys are never written for routing. Stable hint files under the managed SSH config and runtime route slots contain public keys only.
