#!/usr/bin/env bash
set -euo pipefail

NEW_TAG="${1:?usage: $0 <tag>}"
NOTES_FILE="dist/release-notes.md"

PREVIOUS_TAG="$(git describe --tags --abbrev=0 "${NEW_TAG}^" 2>/dev/null || true)"

{
  echo "## What's Changed"
  echo
  if [[ -n "$PREVIOUS_TAG" ]]; then
    git log --pretty='- %s (%h)' "${PREVIOUS_TAG}..${NEW_TAG}"
    echo
    echo "**Full Changelog**: https://github.com/${GITHUB_REPOSITORY}/compare/${PREVIOUS_TAG}...${NEW_TAG}"
  else
    git log --pretty='- %s (%h)' "$NEW_TAG"
  fi
} > "$NOTES_FILE"

gh release create "$NEW_TAG" \
  --notes-file "$NOTES_FILE" \
  dist/*.tar.gz dist/*.zip dist/checksums.txt
