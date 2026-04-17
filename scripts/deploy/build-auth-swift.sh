#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
out="$root/build/forged-auth-release"
cd "$root/cli"
rm -rf "$out"

export CLANG_MODULE_CACHE_PATH="${CLANG_MODULE_CACHE_PATH:-/tmp/forged-swift-clang-cache}"
export TMPDIR="${TMPDIR:-/tmp/forged-swift-tmp}"
mkdir -p "$CLANG_MODULE_CACHE_PATH" "$TMPDIR"

build() {
  local arch="$1" target="$2"
  mkdir -p "$out/darwin_${arch}"
  swiftc -target "$target" -o "$out/darwin_${arch}/forged-auth" \
    -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist \
    -Xlinker cmd/forged-auth/Info.plist \
    cmd/forged-auth/main.swift
}

build amd64 x86_64-apple-macos13.0
build arm64 arm64-apple-macos13.0
