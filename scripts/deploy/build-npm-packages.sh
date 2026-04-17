#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST_DIR="$ROOT/dist"
OUT_DIR="$DIST_DIR/npm"
WRAPPER_DIR="$ROOT/npm/cli"
TEMPLATE="$ROOT/npm/platform/package.template.json"
RAW_VERSION="${1:-${VERSION:-0.0.0-dev}}"
VERSION="${RAW_VERSION#v}"

find_archive() {
  local os="$1"
  local arch="$2"
  local format="$3"

  if [[ -f "$DIST_DIR/artifacts.json" ]]; then
    node - <<'NODE' "$DIST_DIR/artifacts.json" "$os" "$arch" "$format"
const fs = require("node:fs");
const [file, os, arch, format] = process.argv.slice(2);
const data = JSON.parse(fs.readFileSync(file, "utf8"));
const artifacts = Array.isArray(data) ? data : data.artifacts || [];
const match = artifacts.find((artifact) => {
  if (artifact.type !== "Archive") return false;
  if (artifact.goos !== os || artifact.goarch !== arch) return false;
  const path = artifact.path || artifact.name || "";
  return format === "zip" ? path.endsWith(".zip") : path.endsWith(".tar.gz");
});

if (!match) {
  process.exit(1);
}

process.stdout.write(match.path || match.name);
NODE
    return
  fi

  local extension=".tar.gz"
  if [[ "$format" == "zip" ]]; then
    extension=".zip"
  fi

  find "$DIST_DIR" -maxdepth 1 -type f \
    \( -name "forged*${os}*${arch}*${extension}" -o -name "*${os}*${arch}*${extension}" \) \
    | head -n1
}

normalize_path() {
  local input="$1"
  if [[ "$input" == /* ]]; then
    printf '%s\n' "$input"
  else
    printf '%s/%s\n' "$ROOT" "$input"
  fi
}

extract_archive() {
  local archive="$1"
  local target="$2"

  mkdir -p "$target"
  if [[ "$archive" == *.zip ]]; then
    unzip -qq "$archive" -d "$target"
  else
    tar -xzf "$archive" -C "$target"
  fi
}

copy_binary() {
  local extracted_dir="$1"
  local source_name="$2"
  local dest_path="$3"

  local source_path
  source_path="$(find "$extracted_dir" -type f -name "$source_name" | head -n1)"
  if [[ -z "$source_path" ]]; then
    echo "missing $source_name in extracted archive" >&2
    exit 1
  fi
  cp "$source_path" "$dest_path"
}

copy_platform() {
  local os="$1"
  local arch="$2"
  local npm_os="$3"
  local npm_cpu="$4"
  local pkg="$5"
  local format="tar.gz"
  local ext=""

  if [[ "$os" == "windows" ]]; then
    format="zip"
    ext=".exe"
  fi

  local archive_rel
  archive_rel="$(find_archive "$os" "$arch" "$format")"
  if [[ -z "$archive_rel" ]]; then
    echo "missing archive for $os/$arch" >&2
    exit 1
  fi
  local archive
  archive="$(normalize_path "$archive_rel")"

  local pkg_dir="$OUT_DIR/$pkg"
  local extract_dir="$OUT_DIR/.extract-${os}-${arch}"
  mkdir -p "$pkg_dir/bin"
  rm -rf "$extract_dir"
  extract_archive "$archive" "$extract_dir"

  copy_binary "$extract_dir" "forged${ext}" "$pkg_dir/bin/forged${ext}"
  copy_binary "$extract_dir" "forged-sign${ext}" "$pkg_dir/bin/forged-sign${ext}"
  copy_binary "$extract_dir" "forged-auth${ext}" "$pkg_dir/bin/forged-auth${ext}"

  sed \
    -e "s|__NAME__|$pkg|g" \
    -e "s|__VERSION__|$VERSION|g" \
    -e "s|__OS__|$os|g" \
    -e "s|__ARCH__|$arch|g" \
    -e "s|__NPM_OS__|$npm_os|g" \
    -e "s|__NPM_CPU__|$npm_cpu|g" \
    "$TEMPLATE" > "$pkg_dir/package.json"
}

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

copy_platform darwin amd64 darwin x64 @getforged/cli-darwin-x64
copy_platform darwin arm64 darwin arm64 @getforged/cli-darwin-arm64
copy_platform linux amd64 linux x64 @getforged/cli-linux-x64
copy_platform linux arm64 linux arm64 @getforged/cli-linux-arm64
copy_platform windows amd64 win32 x64 @getforged/cli-win32-x64
copy_platform windows arm64 win32 arm64 @getforged/cli-win32-arm64

mkdir -p "$OUT_DIR/cli"
cp -R "$WRAPPER_DIR/." "$OUT_DIR/cli/"
node - <<'NODE' "$OUT_DIR/cli/package.json" "$VERSION"
const fs = require("node:fs");
const [path, version] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
pkg.version = version;
for (const dependency of Object.keys(pkg.optionalDependencies)) {
  pkg.optionalDependencies[dependency] = version;
}
fs.writeFileSync(path, JSON.stringify(pkg, null, 2) + "\n");
NODE

rm -rf "$OUT_DIR"/.extract-*
