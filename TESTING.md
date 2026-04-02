# Local Testing Guide

Test the full Forged stack locally: CLI, daemon, sync server, auth.

## Prerequisites

- Go 1.25+
- PostgreSQL (via Docker or Homebrew)
- `just` command runner (`brew install just`)

## 1. Start PostgreSQL

Using Docker:
```bash
docker run -d --name forged-db \
  -e POSTGRES_USER=forged \
  -e POSTGRES_PASSWORD=forged \
  -e POSTGRES_DB=forged \
  -p 5432:5432 \
  postgres:17
```

Or if you have Postgres locally:
```bash
createdb forged
```

## 2. Run migrations

```bash
psql "postgres://forged:forged@localhost:5432/forged" -f server/migrations/001_init.sql
```

## 3. Build everything

```bash
just build
```

## 4. Start the sync server

```bash
DATABASE_URL="postgres://forged:forged@localhost:5432/forged" \
JWT_SECRET="dev-secret-change-in-production" \
DEV_MODE="true" \
./bin/forged-server
```

You should see:
```
level=INFO msg="server starting" port=8080
```

Test health check:
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## 5. Test auth (dev mode)

Dev mode exposes `POST /api/v1/auth/dev` for testing without OAuth.

```bash
curl -X POST http://localhost:8080/api/v1/auth/dev \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com"}'
```

Response:
```json
{"token":"eyJhbG...","user_id":"uuid","email":"test@example.com"}
```

Save the token:
```bash
export TOKEN="eyJhbG..."
```

## 6. Test device registration

```bash
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"MacBook Pro","platform":"darwin/arm64","hostname":"test-machine","device_public_key":"test-key"}'
```

First device is auto-approved. Save the device ID:
```bash
export DEVICE_ID="uuid-from-response"
```

List devices:
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/devices
```

## 7. Test sync push/pull

Push a vault blob:
```bash
echo "encrypted-vault-data-here" | curl -X POST http://localhost:8080/api/v1/sync/push \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Vault-Version: 0" \
  -H "X-Device-ID: $DEVICE_ID" \
  --data-binary @-
```

Response:
```json
{"version":1}
```

Pull it back:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  -H "X-Device-ID: $DEVICE_ID" \
  http://localhost:8080/api/v1/sync/pull
```

Check sync status:
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/sync/status
```

Test version conflict (push with wrong version):
```bash
echo "new-data" | curl -X POST http://localhost:8080/api/v1/sync/push \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Vault-Version: 999" \
  -H "X-Device-ID: $DEVICE_ID" \
  --data-binary @-
# Should return 409 Conflict
```

## 8. Test the CLI daemon

Open a new terminal:

```bash
# Create vault and start daemon
echo "my-master-password" | ./bin/forged daemon
```

In another terminal:

```bash
# Generate keys
./bin/forged generate my-github-key -c "test@forged"
./bin/forged generate my-deploy-key -c "deploy@forged"

# List keys
./bin/forged list
./bin/forged list --json

# Export public key
./bin/forged export my-github-key

# Host mapping
./bin/forged host my-github-key "github.com" "*.github.com"
./bin/forged host my-deploy-key "*.prod.company.com"
./bin/forged hosts
./bin/forged unhost my-deploy-key "*.prod.company.com"

# Rename
./bin/forged rename my-deploy-key renamed-key

# Status
./bin/forged status
./bin/forged status --json

# Test SSH agent protocol
SSH_AUTH_SOCK=~/.forged/agent.sock ssh-add -l
SSH_AUTH_SOCK=~/.forged/agent.sock ssh-add -L

# Remove
./bin/forged remove renamed-key

# Stop daemon
./bin/forged stop
```

## 9. Test CLI sync with server

Save credentials manually (normally `forged login` does this via browser OAuth):

```bash
mkdir -p ~/.forged
cat > ~/.forged/credentials.json << EOF
{
  "server_url": "http://localhost:8080",
  "token": "$TOKEN",
  "user_id": "your-user-id",
  "email": "test@example.com"
}
EOF
```

Then:
```bash
./bin/forged sync status
```

## 10. Full end-to-end flow

This script runs everything in sequence:

```bash
#!/bin/bash
set -e
FORGED=./bin/forged

# Clean state
rm -rf ~/.forged/

# 1. Get auth token from server (must be running with DEV_MODE=true)
AUTH=$(curl -s -X POST http://localhost:8080/api/v1/auth/dev \
  -H "Content-Type: application/json" \
  -d '{"email":"e2e@test.com"}')
TOKEN=$(echo $AUTH | jq -r .token)
USER_ID=$(echo $AUTH | jq -r .user_id)

# 2. Start daemon
echo "testpassword" | $FORGED daemon &
DAEMON_PID=$!
sleep 2

# 3. Generate keys
$FORGED generate github-key -c "test@forged"
$FORGED generate deploy-key -c "deploy@forged"

# 4. Add host mappings
$FORGED host github-key "github.com" "*.github.com"
$FORGED host deploy-key "*.prod.example.com"

# 5. Verify via SSH agent
SSH_AUTH_SOCK=~/.forged/agent.sock ssh-add -l

# 6. Check status
$FORGED status
$FORGED hosts

# 7. Save sync credentials
mkdir -p ~/.forged
cat > ~/.forged/credentials.json << EOF
{
  "server_url": "http://localhost:8080",
  "token": "$TOKEN",
  "user_id": "$USER_ID",
  "email": "e2e@test.com"
}
EOF

# 8. Check sync status
$FORGED sync status

# 9. Stop and clean
$FORGED stop
wait $DAEMON_PID 2>/dev/null
rm -rf ~/.forged/

echo "All tests passed"
```

## Cleanup

```bash
# Stop server (Ctrl+C in its terminal)
# Remove test database
docker rm -f forged-db
# Remove local state
rm -rf ~/.forged/
```
