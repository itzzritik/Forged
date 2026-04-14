#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="$ROOT/dist/npm"
RAW_VERSION="${1:-${VERSION:-}}"
VERSION="${RAW_VERSION#v}"

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>" >&2
  exit 1
fi

package_name() {
  local package_dir="$1"

  node - <<'NODE' "$package_dir/package.json"
const fs = require("node:fs");
const [path] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
process.stdout.write(pkg.name);
NODE
}

package_version() {
  local package_dir="$1"

  node - <<'NODE' "$package_dir/package.json"
const fs = require("node:fs");
const [path] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
process.stdout.write(pkg.version);
NODE
}

publish_if_needed() {
  local package_dir="$1"
  local name
  local version

  if [[ ! -f "$package_dir/package.json" ]]; then
    echo "missing package.json in $package_dir" >&2
    exit 1
  fi

  name="$(package_name "$package_dir")"
  version="$(package_version "$package_dir")"

  if [[ "$version" != "$VERSION" ]]; then
    echo "version mismatch for $name: expected $VERSION, found $version" >&2
    exit 1
  fi

  if npm view "${name}@${VERSION}" version >/dev/null 2>&1; then
    echo "Skipping ${name}@${VERSION}; already published"
    return 0
  fi

  npm publish --access public --provenance "$package_dir"
}

shopt -s nullglob

platform_dirs=("$DIST_DIR"/@getforged/cli-*)
if [[ ${#platform_dirs[@]} -eq 0 ]]; then
  echo "no platform packages found in $DIST_DIR/@getforged" >&2
  exit 1
fi

for package_dir in "${platform_dirs[@]}"; do
  publish_if_needed "$package_dir"
done

publish_if_needed "$DIST_DIR/cli"
