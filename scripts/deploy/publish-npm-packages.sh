#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
dist="$root/dist/npm"
: "${1:?usage: $0 <version>}"
version="${1#v}"

publish_if_needed() {
  local dir="$1" name pkg_version
  [[ -f "$dir/package.json" ]] || { echo "missing package.json in $dir" >&2; exit 1; }

  read -r name pkg_version < <(node -p "const p=require('$dir/package.json');p.name+' '+p.version")

  [[ "$pkg_version" == "$version" ]] || { echo "version mismatch for $name: expected $version, found $pkg_version" >&2; exit 1; }

  if npm view "${name}@${version}" version >/dev/null 2>&1; then
    echo "skipping ${name}@${version}; already published"
    return 0
  fi

  npm publish --access public --provenance "$dir"
}

shopt -s nullglob
platforms=("$dist"/@getforged/cli-*)
[[ ${#platforms[@]} -gt 0 ]] || { echo "no platform packages in $dist/@getforged" >&2; exit 1; }

pids=()
for pkg in "${platforms[@]}"; do
  publish_if_needed "$pkg" & pids+=("$!")
done

status=0
for pid in "${pids[@]}"; do wait "$pid" || status=$?; done
[[ $status -eq 0 ]] || { echo "one or more platform publishes failed" >&2; exit "$status"; }

publish_if_needed "$dist/cli"
