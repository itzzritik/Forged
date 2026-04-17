#!/usr/bin/env bash
set -euo pipefail

git fetch --tags --prune
released="$(gh release list --limit 1000 --json tagName --jq '.[].tagName')"

orphans=()
for tag in $(git tag -l 'v*'); do
  if ! grep -qx "$tag" <<< "$released"; then
    orphans+=("$tag")
  fi
done

if [[ ${#orphans[@]} -eq 0 ]]; then
  echo "no orphan tags"
  exit 0
fi

echo "pruning ${#orphans[@]} orphan tag(s): ${orphans[*]}"
git push --delete origin "${orphans[@]}"
