#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PYTHON=${PYTHON:-python3}

if [ "${1:-}" = "--epub" ]; then
  if [ -z "${2:-}" ]; then
    echo "usage: $0 [--epub path]" >&2
    exit 2
  fi
  cd "$ROOT"
  exec "$PYTHON" scripts/validate_epub_style_demo.py --epub "$2"
fi

cd "$ROOT"
exec "$PYTHON" scripts/validate_epub_style_demo.py
