#!/bin/sh
set -eu

repo_root=$(
  CDPATH= cd -- "$(dirname "$0")/../.." && pwd
)

cd "$repo_root/cli"

helpers_dir="$repo_root/build/forged-auth-release"
rm -rf "$helpers_dir"
mkdir -p "$helpers_dir"

build_swift_helper() {
  arch="$1"
  target="$2"
  output_dir="$helpers_dir/darwin_${arch}"

  mkdir -p "$output_dir"

  swift_clang_cache="${CLANG_MODULE_CACHE_PATH:-/tmp/forged-swift-clang-cache}"
  swift_tmpdir="${TMPDIR:-/tmp/forged-swift-tmp}"
  mkdir -p "$swift_clang_cache" "$swift_tmpdir"

  CLANG_MODULE_CACHE_PATH="$swift_clang_cache" \
  TMPDIR="$swift_tmpdir" \
  swiftc -target "$target" \
    -o "$output_dir/forged-auth" \
    -Xlinker -sectcreate \
    -Xlinker __TEXT \
    -Xlinker __info_plist \
    -Xlinker cmd/forged-auth/Info.plist \
    cmd/forged-auth/main.swift
}

build_swift_helper amd64 x86_64-apple-macos13.0
build_swift_helper arm64 arm64-apple-macos13.0
