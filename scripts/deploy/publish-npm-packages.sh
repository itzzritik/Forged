#!/usr/bin/env bash
set -euo pipefail

# Platform packages go first (nothing installs them by name); the wrapper only once every pinned one is installable.
root="$(cd "$(dirname "$0")/../.." && pwd)"
mode="${1:?usage: $0 platforms|wrapper <version> [wrapper-dir]}"
version="${2:?usage: $0 platforms|wrapper <version> [wrapper-dir]}"
version="${version#v}"
poll="${POLL_SECONDS:-15}"
tries="${POLL_TRIES:-60}"
soak="${SOAK_SECONDS:-180}"

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

installable() { npm view "$1@$version" version --prefer-online >/dev/null 2>&1; }

case "$mode" in
platforms)
  shopt -s nullglob
  platforms=("$root"/dist/npm/@getforged/cli-*)
  [[ ${#platforms[@]} -gt 0 ]] || { echo "no platform packages in dist/npm/@getforged" >&2; exit 1; }
  pids=()
  for pkg in "${platforms[@]}"; do
    publish_if_needed "$pkg" & pids+=("$!")
  done
  status=0
  for pid in "${pids[@]}"; do wait "$pid" || status=$?; done
  [[ $status -eq 0 ]] || { echo "one or more platform publishes failed" >&2; exit "$status"; }
  ;;
wrapper)
  dir="$(cd "${3:?usage: $0 wrapper <version> <wrapper-dir>}" && pwd)"
  for name in $(node -p "Object.keys(require('$dir/package.json').optionalDependencies).join(' ')"); do
    for ((i = 1; ; i++)); do
      installable "$name" && break
      (( i < tries )) || { echo "${name}@${version} never became installable; not publishing the wrapper" >&2; exit 1; }
      sleep "$poll"
    done
    echo "${name}@${version} is installable"
  done
  # Registry caches can still serve an older packument for a few minutes after a version appears.
  sleep "$soak"
  publish_if_needed "$dir"
  ;;
*)
  echo "unknown mode: $mode" >&2
  exit 1
  ;;
esac
