#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
STAMP=$(date +%Y%m%d-%H%M%S)
OUT_DIR="$ROOT/dist"
OUT="${1:-$OUT_DIR/kindle-minimal-repro-$STAMP.epub}"
TMP="$OUT.tmp"

if ! command -v zip >/dev/null 2>&1; then
  echo "zip is required to build the EPUB." >&2
  exit 1
fi

if [ -e "$OUT" ] || [ -e "$TMP" ]; then
  echo "Output path already exists: $OUT" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT")"

(
  cd "$ROOT"
  zip -X -0 "$TMP" mimetype >/dev/null
  zip -X -r -9 "$TMP" META-INF OEBPS >/dev/null
)

mv "$TMP" "$OUT"
echo "$OUT"
