#!/usr/bin/env bash
set -euo pipefail

git fetch --tags --prune
released="$(gh release list --limit 1000 --json tagName --jq '.[].tagName')"

for tag in $(git tag -l 'v*'); do
  if ! grep -qx "$tag" <<< "$released"; then
    echo "pruning orphan tag: $tag"
    git push --delete origin "$tag" || true
  fi
done
