#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
dist="$root/dist"
out="$dist/npm"
template="$root/npm/platform/package.template.json"
wrapper="$root/npm/cli"
: "${1:?usage: $0 <version>}"
version="${1#v}"

find_archive() {
  node - "$dist/artifacts.json" "$1" "$2" "$3" <<'NODE'
const fs = require("node:fs");
const [file, os, arch, format] = process.argv.slice(2);
const { artifacts = [] } = JSON.parse(fs.readFileSync(file, "utf8"));
const ext = format === "zip" ? ".zip" : ".tar.gz";
const match = artifacts.find(a =>
  a.type === "Archive" && a.goos === os && a.goarch === arch && (a.path || a.name || "").endsWith(ext)
);
if (!match) { console.error(`no archive for ${os}/${arch}`); process.exit(1); }
process.stdout.write(match.path || match.name);
NODE
}

copy_platform() {
  local os="$1" arch="$2" npm_os="$3" npm_cpu="$4" pkg="$5"
  local format="tar.gz" ext=""
  if [[ "$os" == "windows" ]]; then format="zip"; ext=".exe"; fi

  local archive pkg_dir extract_dir
  archive="$root/$(find_archive "$os" "$arch" "$format")"
  pkg_dir="$out/$pkg"
  extract_dir="$(mktemp -d)"

  mkdir -p "$pkg_dir/bin"
  if [[ "$format" == "zip" ]]; then
    unzip -qq "$archive" -d "$extract_dir"
  else
    tar -xzf "$archive" -C "$extract_dir"
  fi

  for bin in forged forged-sign forged-auth; do
    local src
    src="$(find "$extract_dir" -type f -name "${bin}${ext}" | head -n1)"
    [[ -n "$src" ]] || { echo "missing ${bin}${ext} in $archive" >&2; exit 1; }
    cp "$src" "$pkg_dir/bin/${bin}${ext}"
  done

  sed \
    -e "s|__NAME__|$pkg|g" \
    -e "s|__VERSION__|$version|g" \
    -e "s|__OS__|$os|g" \
    -e "s|__ARCH__|$arch|g" \
    -e "s|__NPM_OS__|$npm_os|g" \
    -e "s|__NPM_CPU__|$npm_cpu|g" \
    "$template" >"$pkg_dir/package.json"

  rm -rf "$extract_dir"
}

rm -rf "$out"
mkdir -p "$out"

copy_platform darwin  amd64 darwin x64   @getforged/cli-darwin-x64
copy_platform darwin  arm64 darwin arm64 @getforged/cli-darwin-arm64
copy_platform linux   amd64 linux  x64   @getforged/cli-linux-x64
copy_platform linux   arm64 linux  arm64 @getforged/cli-linux-arm64
copy_platform windows amd64 win32  x64   @getforged/cli-win32-x64
copy_platform windows arm64 win32  arm64 @getforged/cli-win32-arm64

cp -R "$wrapper/." "$out/cli/"
node - "$out/cli/package.json" "$version" <<'NODE'
const fs = require("node:fs");
const [path, v] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
pkg.version = v;
for (const d of Object.keys(pkg.optionalDependencies)) pkg.optionalDependencies[d] = v;
fs.writeFileSync(path, JSON.stringify(pkg, null, 2) + "\n");
NODE
