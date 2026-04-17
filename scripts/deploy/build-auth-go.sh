#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
out="$root/build/forged-auth-release"
cd "$root/cli"
mkdir -p "$out"

build() {
  local goos="$1" goarch="$2" ext=""
  [[ "$goos" == "windows" ]] && ext=".exe"
  mkdir -p "$out/${goos}_${goarch}"
  GOOS="$goos" GOARCH="$goarch" go build -o "$out/${goos}_${goarch}/forged-auth$ext" ./cmd/forged-auth
}

pids=()
for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  IFS=/ read -r goos goarch <<<"$target"
  build "$goos" "$goarch" & pids+=("$!")
done

status=0
for pid in "${pids[@]}"; do wait "$pid" || status=$?; done
exit "$status"
