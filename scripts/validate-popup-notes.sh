#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PYTHON=${PYTHON:-python3}

cd "$ROOT"
exec "$PYTHON" scripts/validate_popup_notes.py "$@"
