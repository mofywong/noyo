#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../backend"
export PATH=/usr/local/go/bin:/usr/bin:/bin
export CGO_ENABLED=1 GOOS=linux GOARCH=amd64

if [[ ! -r dist/index.html ]]; then
    echo 'Error: backend/dist/index.html is missing or unreadable in WSL. Build the frontend first.' >&2
    exit 1
fi

# Publish only after compilation and RPATH patching both succeed.
mkdir -p linux
output=$(mktemp linux/.noyo-linux-amd64-pro.XXXXXX)
trap 'rm -f -- "$output"' EXIT
go build -ldflags '-w -s' -o "$output" .
if ! command -v patchelf >/dev/null 2>&1; then
    sudo apt-get update -qq
    sudo apt-get install -y patchelf
fi
patchelf --set-rpath '$ORIGIN/lib:$ORIGIN' "$output"
mv -f -- "$output" linux/noyo-linux-amd64-pro
