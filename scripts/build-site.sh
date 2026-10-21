#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
site_root=$(pwd)
mkdir -p web/demos
for demo in bounce sprites; do
  GOWORK="$site_root/wasm.work" GOOS=js GOARCH=wasm CGO_ENABLED=0 \
    go build -trimpath -o "web/demos/$demo.wasm" "./examples/$demo"
done
GOWORK=off go tool booba-assets --no-index web
node --check web/site.js
node scripts/check-site.mjs
