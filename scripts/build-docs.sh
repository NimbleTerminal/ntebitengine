#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
# Isolate the guide's pinned theme module from the Go application's workspace.
GOWORK=off hugo --source docs/hugo --destination ../../web/docs --minify
