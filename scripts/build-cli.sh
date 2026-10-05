#!/bin/sh
set -eu

repo_root=$(
  CDPATH= cd -- "$(dirname "$0")/.." && pwd
)

build_id="${FORGED_BUILD_ID:-dev-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
ldflags="-X github.com/itzzritik/forged/cli/internal/buildinfo.ID=$build_id"
# Windows binaries need .exe: the daemon and launchers look up forged-auth.exe and forged-sign.exe.
exe=$(cd "$repo_root/cli" && go env GOEXE)

mkdir -p "$repo_root/bin"

cd "$repo_root/cli"
go build -ldflags "$ldflags" -o "$repo_root/bin/forged$exe" ./cmd/forged
go build -o "$repo_root/bin/forged-sign$exe" ./cmd/forged-sign

cd "$repo_root"
./scripts/build-forged-auth.sh

"./bin/forged$exe" __daemon-freshen --quiet || true
