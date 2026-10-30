#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 -B "$ROOT/build-scenes.py" "$@"
