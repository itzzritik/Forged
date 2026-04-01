default:
    @just --list

# Build
build-cli:
    cd cli && go build -o ../bin/forged ./cmd/forged
    cd cli && go build -o ../bin/forged-sign ./cmd/forged-sign

build-server:
    cd server && npm run build

build: build-cli

# Lint
lint-cli:
    cd cli && golangci-lint run ./...

lint-server:
    cd server && npm run lint

lint: lint-cli

# Run
dev:
    cd cli && go run ./cmd/forged daemon

# Clean
clean:
    rm -rf bin
