#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
out="$root/dist/npm"
version="${1:?usage: $0 <version> <checksums.txt>}"
version="${version#v}"
checksums="${2:?usage: $0 <version> <checksums.txt>}"
repo="${GITHUB_REPOSITORY:-itzzritik/Forged}"

rm -rf "$out"
mkdir -p "$out"
cp -R "$root/npm/." "$out/"
cp "$root/README.md" "$root/LICENSE" "$out/"

node - "$out/package.json" "$version" "$checksums" "$repo" <<'NODE'
const fs = require("node:fs");
const [path, version, checksums, repo] = process.argv.slice(2);
const platforms = { darwin: "darwin", linux: "linux", windows: "win32" };
const arches = { amd64: "x64", arm64: "arm64" };
const base = `https://github.com/${repo}/releases/download/v${version}/`;

const archives = {};
for (const line of fs.readFileSync(checksums, "utf8").split("\n")) {
  const [sha256, file] = line.trim().split(/\s+/);
  const match = /^forged_(.+)_([a-z]+)_([a-z0-9]+)\.(tar\.gz|zip)$/.exec(file || "");
  if (!match || match[1] !== version || !platforms[match[2]] || !arches[match[3]]) continue;
  archives[`${platforms[match[2]]}-${arches[match[3]]}`] = { url: base + file, sha256 };
}

const missing = Object.values(platforms).flatMap(p => Object.values(arches).map(a => `${p}-${a}`)).filter(k => !archives[k]);
if (missing.length) {
  console.error(`missing release archives for: ${missing.join(", ")}`);
  process.exit(1);
}

const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
pkg.version = version;
pkg.forged = { archives };
fs.writeFileSync(path, JSON.stringify(pkg, null, 2) + "\n");
NODE
