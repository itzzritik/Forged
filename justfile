default:
    @just --list

# Build
build-cli:
    cd cli && go build -o ../bin/forged ./cmd/forged
    cd cli && go build -o ../bin/forged-sign ./cmd/forged-sign
    ./scripts/build-forged-auth.sh

build-server:
    cd server && go build -o ../bin/forged-server ./cmd/forged-server

build-web:
    cd web && pnpm build

build: build-cli build-server

# Lint
lint-cli:
    cd cli && golangci-lint run ./...

lint-server:
    cd server && golangci-lint run ./...

lint-web:
    cd web && pnpm lint

lint: lint-cli lint-server

# Run
dev:
    just build-cli
    cd cli && go run ./cmd/forged-dev-service --binary ../bin/forged install

dev-stop:
    cd cli && go run ./cmd/forged-dev-service stop

dev-server:
    cd server && doppler run -- go run ./cmd/forged-server

dev-web:
    cd web && pnpm dev

# Database
migrate:
    cd server && doppler run -- go run ./cmd/migrate

migrate-reset:
    cd server && doppler run -- go run ./cmd/migrate reset

# Clean
clean:
    rm -rf bin
