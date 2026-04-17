#!/bin/sh
set -eu

repo_root=$(
  CDPATH= cd -- "$(dirname "$0")/../.." && pwd
)

cd "$repo_root/cli"

helpers_dir="$repo_root/build/forged-auth-release"
mkdir -p "$helpers_dir"

build_go_helper() {
  goos="$1"
  goarch="$2"
  ext=""
  if [ "$goos" = "windows" ]; then
    ext=".exe"
  fi

  output_dir="$helpers_dir/${goos}_${goarch}"
  mkdir -p "$output_dir"
  GOOS="$goos" GOARCH="$goarch" go build -o "$output_dir/forged-auth$ext" ./cmd/forged-auth
}

pids=""
build_go_helper linux amd64 & pids="$pids $!"
build_go_helper linux arm64 & pids="$pids $!"
build_go_helper windows amd64 & pids="$pids $!"
build_go_helper windows arm64 & pids="$pids $!"

status=0
for pid in $pids; do
  wait "$pid" || status=$?
done
exit "$status"
