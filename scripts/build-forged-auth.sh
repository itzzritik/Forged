#!/bin/sh
set -eu

repo_root=$(
  CDPATH= cd -- "$(dirname "$0")/.." && pwd
)

mkdir -p "$repo_root/bin"
cd "$repo_root/cli"

output="$repo_root/bin/forged-auth"

case "$(uname -s)" in
  Darwin)
    swift_clang_cache="${CLANG_MODULE_CACHE_PATH:-/tmp/forged-swift-clang-cache}"
    swift_tmpdir="${TMPDIR:-/tmp/forged-swift-tmp}"

    mkdir -p "$swift_clang_cache" "$swift_tmpdir"

    CLANG_MODULE_CACHE_PATH="$swift_clang_cache" \
    TMPDIR="$swift_tmpdir" \
    swiftc -o "$output" \
      -Xlinker -sectcreate \
      -Xlinker __TEXT \
      -Xlinker __info_plist \
      -Xlinker cmd/forged-auth/Info.plist \
      cmd/forged-auth/main.swift
    ;;
  *)
    go build -o "$output" ./cmd/forged-auth
    ;;
esac
