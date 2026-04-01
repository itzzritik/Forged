# IPC Protocol Specification

CLI commands communicate with the daemon over a Unix domain socket (`ctl.sock`).

## Transport

- Unix domain socket (stream)
- macOS: `~/.forged/ctl.sock`
- Linux: `$XDG_RUNTIME_DIR/forged/ctl.sock`

## Message Framing

Each message is length-prefixed:

```
┌────────────────────────────┐
│ Length (4 bytes, uint32 BE) │
├────────────────────────────┤
│ JSON payload (Length bytes) │
└────────────────────────────┘
```

## Request Format

```json
{
  "command": "string",
  "args": {}
}
```

## Response Format

```json
{
  "status": "ok | error",
  "data": {},
  "error": "string (only when status=error)"
}
```

## Commands

| Command | Args | Response Data |
|---------|------|---------------|
| `list` | none | `{"keys": [...]}` |
| `add` | `{"name": "string", "private_key": "string", "comment": "string"}` | `{"key": {...}}` |
| `generate` | `{"name": "string", "comment": "string"}` | `{"key": {...}, "public_key": "string"}` |
| `remove` | `{"name": "string"}` | none |
| `rename` | `{"old_name": "string", "new_name": "string"}` | none |
| `export` | `{"name": "string"}` | `{"public_key": "string"}` |
| `status` | none | `{"daemon_pid": int, "uptime_seconds": int, "key_count": int, "sync": {...}}` |
| `host` | `{"key_name": "string", "patterns": ["string"]}` | none |
| `unhost` | `{"key_name": "string", "pattern": "string"}` | none |
| `hosts` | none | `{"mappings": [...]}` |
| `lock` | none | none |
| `unlock` | `{"password": "string"}` | none |
| `sync-trigger` | none | none |
| `sync-status` | none | `{"enabled": bool, "last_sync": "timestamp", "devices": int}` |
| `config-get` | `{"key": "string"}` | `{"value": "any"}` |
| `config-set` | `{"key": "string", "value": "any"}` | none |
| `activity` | `{"limit": int}` | `{"events": [...]}` |

## Error Handling

- If the daemon is not running, the CLI fails to connect and prints: `Error: daemon is not running. Start it with: forged start`
- If a command fails, the response has `"status": "error"` with a human-readable `"error"` field
