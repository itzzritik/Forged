default:
    @just --list

# Build
build-cli:
    cd cli && go build -o ../bin/forged ./cmd/forged
    cd cli && go build -o ../bin/forged-sign ./cmd/forged-sign

build-server:
    cd server && go build -o ../bin/forged-server ./cmd/forged-server

build: build-cli build-server

# Lint
lint-cli:
    cd cli && golangci-lint run ./...

lint-server:
    cd server && golangci-lint run ./...

lint: lint-cli lint-server

# Run
dev:
    cd cli && go run ./cmd/forged daemon

dev-server:
    cd server && go run ./cmd/forged-server

# Clean
clean:
    rm -rf bin
