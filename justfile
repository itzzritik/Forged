default:
    @just --list

# Build
build-cli:
    ./scripts/build-cli.sh

build-server:
    cd server && go build -o ../bin/forged-server ./cmd/forged-server

build-web:
    cd web && bun run build

build: build-cli build-server

# Lint
lint-cli:
    cd cli && golangci-lint run ./...

lint-server:
    cd server && golangci-lint run ./...

lint-web:
    cd web && bun run check

lint: lint-cli lint-server lint-web

# Windows end-to-end suite (drives real forged.exe; see cli/e2e)
e2e:
    cd cli && go test -tags e2e ./e2e -v -count=1 -timeout 20m

# Run
dev:
    just build-cli
    cd cli && go run ./cmd/forged-dev-service --binary ../bin/forged install

dev-stop:
    cd cli && go run ./cmd/forged-dev-service stop

dev-server:
    cd server && doppler run -- go run ./cmd/forged-server

dev-web:
    cd web && bun run dev

auth:
    #!/usr/bin/env node
    // Fresh session via the CLI login flow; reusing the CLI's refresh token would trip family revocation.
    const crypto = require("node:crypto");
    const { execFile } = require("node:child_process");
    const api = process.env.FORGED_API_URL || "https://forged-api.ritik.me";
    const app = process.env.FORGED_APP_URL || "https://forged.ritik.me";
    const web = process.env.FORGED_WEB_URL || "http://localhost:3035";
    const open = (url) => execFile("open", [url]);
    const fail = (message) => { console.error(message); process.exit(1); };
    (async () => {
        const code = crypto.randomBytes(16).toString("hex");
        const verification = crypto.randomBytes(2).toString("hex");
        const verifier = crypto.randomBytes(32).toString("base64url");
        const challenge = crypto.createHash("sha256").update(verifier).digest("base64url");
        const created = await fetch(`${api}/api/v1/auth/sessions`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code, verification, code_challenge: challenge, challenge_method: "S256" }) });
        if (!created.ok) fail(`Could not start login (${created.status}).`);
        console.log(`Approve FORGE-${verification.toUpperCase()} in the browser…`);
        open(`${app}/login?code=${code}`);
        for (const deadline = Date.now() + 5 * 60_000; Date.now() < deadline; await new Promise((r) => setTimeout(r, 2000))) {
            const { status } = await (await fetch(`${api}/api/v1/auth/sessions/${code}`)).json();
            if (status === "error") fail("Login failed.");
            if (status !== "approved") continue;
            const exchanged = await fetch(`${api}/api/v1/auth/sessions/${code}/exchange`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code_verifier: verifier }) });
            if (!exchanged.ok) fail(`Could not finish login (${exchanged.status}).`);
            const s = await exchanged.json();
            const query = new URLSearchParams({ access_token: s.access_token, access_expires_at: s.access_expires_at, refresh_token: s.refresh_token, refresh_expires_at: s.refresh_expires_at, user_id: s.user_id, email: s.email, name: s.name || "" });
            open(`${web}/api/auth/callback?${query}`);
            return console.log(`Signed in. Opening ${web}/dashboard`);
        }
        fail("Timed out waiting for approval.");
    })();

# Database
migrate:
    cd server && doppler run -- go run ./cmd/migrate

migrate-reset:
    cd server && doppler run -- go run ./cmd/migrate reset

# Clean
clean:
    rm -rf bin
