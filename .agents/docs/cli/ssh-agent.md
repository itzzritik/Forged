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

- OpenSSH routing is config-driven: Forged's managed `Host *` block always applies `ControlMaster no`, `ControlPath none`, `IdentitiesOnly yes`, and `IdentityFile none` before any hook executes. The first `Match exec` clears a `%C.ready` marker and prepares public-key slots. Every later slot block shell-`exec`s a hidden verifier; the daemon binds that helper to its direct SSH parent and confirms the exact live process owns both the route and that slot fingerprint. An unavailable helper, IPC failure, stale marker, or another process sharing `%C` therefore activates no identity slot. Explicit user SSH overrides remain outside this guarantee.
- Dynamic OpenSSH tokens passed to route hooks are shell-quoted; the success hook carries only `%C`, because it does not need host, port, or user values.
- `%C` is connection-scope, so concurrent same-host routes share the slot directory. The service tracks attempts by a Linux/macOS process instance (PID plus immutable start identity) and admits a shared slot union only when every active candidate fits its six public slots; a new unrepresentable route stays a zero-key guard. Agent signing filters by that exact process instance.
- Route attempt tokens reject the `.` and `..` path components before they enter route state or runtime cleanup, so raw same-user IPC cannot make snippet cleanup escape its token directory.
- The routing service keeps an in-memory public route/key cache after vault lock. This lets route prepare emit candidate public-key hints after system lock so OpenSSH reaches the agent and external System Auth can run at signing time. The service rechecks the broker immediately before private probes, direct-probe signatures, or learned-proof writes, so stale sessions only use that public cache.
- Routed OpenSSH clients require a positive client PID. Control IPC derives the helper PID from kernel credentials, verifies that it is the claimed SSH client's direct child twice around client-instance capture, then agent serving independently derives the kernel peer process instance. Preparation persists a zero-candidate guard before cancellable work and waits for an already-running unscoped operation from that process before publishing slots. Matching connections latch route scope until close; unrelated nonmatching clients retain normal agent access unless runtime state is ambiguous, in which case every client is deny-only.
- Agent signing and routing expose only RSA, ECDSA, and Ed25519 keys. Legacy DSA keys remain available for vault recovery actions but never appear in agent lists, signer sets, or routing hints.
- Route authorization has one immutable five-minute deadline from prepare. Expiry clears candidates, rewrites every affected shared `%C` slot union, and retains a zero-key guard until the exact process instance and all admitted route sockets are gone; list rechecks after authorization and signing admission holds a route read lease through the cryptographic operation. Route state contains only process identity, timestamps, and public fingerprints and is atomically persisted before slots. Startup validates every slot's public fingerprint against that journal; missing/corrupt/mismatched state is deny-only. A runtime write failure stays deny-only until a complete state-and-slot rewrite succeeds. The Routing screen exposes an explicit reset only for ambiguous runtime state and warns the user to close active SSH sessions first.
- GitHub/GitLab repo routes are considered proven only after a provider repo probe. Exact proven repo routes emit only the proven key; same-owner and same-host history only rank candidates.
- Clearing all routes writes tombstones one at a time and schedules sync after any successful write, including when a later clear fails.
- Bracketed scp-style IPv6 Git remotes are split after the closing bracket and normalized into a valid bracketed canonical URL, so they do not corrupt route matching.
- Direct OpenSSH `%h` values can retain IPv6 brackets; routing strips one outer pair before building both its canonical URL and probe dial address.
- Explicit `ssh` client commands resolve as plain SSH targets even when launched from inside a Git working tree.
- Cold daemon sessions can hydrate on first agent use if policy allows it.
- A signing lookup refreshes sync only for a typed absent vault-signable raw public key. Vault, decrypt, requested-key/private-key parse, certificate, and hardware-key failures return directly instead of adding a futile network wait.
- The listener retries temporary accept failures with bounded backoff, exits quietly when closed, and logs terminal failures. Shutdown cancels active agent authorization and missing-key refresh before closing every tracked client, so an idle SSH process cannot pin the daemon. The stock single-request agent loop cannot observe an ordinary peer disconnect while an agent call is in flight; daemon shutdown is the server connection-lifecycle cancellation boundary.
- On Linux and macOS, agent admission requires the kernel-reported peer UID to match the daemon user before the connection is tracked; unavailable peer/process identity is deny-only while routing is enabled.
- External agent use goes through `ActionExternal`, not the TUI-style view path.
- The daemon's in-memory activity ring records SSH signing outcomes only: successful signatures, resolved signing failures, and resolved route denials include the selected public fingerprint and peer PID when available; authorization, vault, and unresolved-key outcomes use fixed `denied` or `failed` results without key metadata. A denied key-list authorization is recorded without key metadata. Canceled contexts and canceled system-auth prompts create no event. Repeated non-success events with the same fields are suppressed for 30 seconds. It never records signed data, signatures, raw error text, credentials, key names, or routine key-list polling.
- `forged-sign` now does an auth preflight so Git commit signing can show cleaner auth errors.
- `forged-sign` owns agent-backed `ssh-keygen -Y sign` and delegates other SSH-signature operations, including Git verification, to the system `ssh-keygen`; enabling commit signing configures the matching `gpg.ssh.allowedSignersFile` before it enables auto-signing.
- Raw SSH agent protocol is still limited in how much error detail it can surface back to callers.
- SSH route preparation has one 45-second server work budget covering cold-session auth, retry, and either probe type. Provider and direct-SSH probes keep their 20-second total and 4-second per-key child caps; the hook waits 50 seconds and the server connection 55 seconds so work can return a final response before either transport closes.
- Provider probes retain at most 64 KiB each of stdout and stderr while draining excess. Any overflow is inconclusive, never a successful or denied provider result.
- A parsed Git `ERR` pkt-line is never provider-proof success; broad packet-prefix heuristics cannot persist a route proof from an error response.
- SSH-agent signing accepts only an exact zero, RSA-SHA256, or RSA-SHA512 flag value. Reserved, combined, and unsupported-algorithm requests fail instead of silently default-signing.
- SSH integration migration removes only exact historical Forged marker blocks and managed include paths. An edited or ambiguous marker fails before the user config changes; inactive legacy files remain in place instead of being deleted recursively.
- On Windows, agent and control pipes use distinct opaque current-user token-SID hashes. Automatic SSH routing is intentionally unavailable: the daemon starts no route service, removes a uniquely marked product-owned routing section from the private managed config, and preserves its agent prefix. It accepts the exact legacy one-line or current five-directive routing prefix; duplicate route markers or a changed prefix block migration instead of being overwritten. Normal SSH-agent and commit-signing behavior remains available.

## Decisions

- Agent mutation operations stay unsupported; the TUI is the write surface. Protocol `Lock` and `Unlock` also fail explicitly because Forged does not persist or verify an agent passphrase; real locking is handled by the sensitive-session broker.
- Agent and control traffic stay on separate sockets.
- Private keys are never written for routing. Stable hint files under the managed SSH config and runtime route slots contain public keys only.
- Automatic route-scoped access intentionally disables normal OpenSSH multiplexing for Forged-managed connections: a mux can move later agent traffic to a different process and defeat the kernel-bound route guard.
