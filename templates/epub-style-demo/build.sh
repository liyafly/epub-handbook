#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
OUT=${EPUB_STYLE_DEMO_OUT:-"$ROOT/dist/epub-style-demo.epub"}
OUT_DIR=$(dirname -- "$OUT")
LOCK_DIR="$OUT_DIR/.epub-style-demo.lock"

if [ "$#" -ne 0 ]; then
	echo "Usage: sh templates/epub-style-demo/build.sh" >&2
	exit 2
fi

if ! command -v zip >/dev/null 2>&1; then
  echo "zip is required to build the EPUB." >&2
  exit 1
fi

if [ -d "$OUT" ]; then
	echo "Output path is a directory: $OUT" >&2
	exit 1
fi
mkdir -p "$OUT_DIR"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
	echo "Another demo build is active (or a stale lock exists): $LOCK_DIR" >&2
	exit 1
fi
BUILD_TMP=$(mktemp -d "$OUT_DIR/.epub-style-demo.XXXXXX")
TMP="$BUILD_TMP/epub-style-demo.epub"
cleanup() {
	rm -rf "$BUILD_TMP"
	rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

(
  cd "$ROOT"
  zip -X -0 "$TMP" mimetype >/dev/null
  zip -X -r -9 "$TMP" META-INF OEBPS >/dev/null
)

mv -f "$TMP" "$OUT"
printf 'Built %s\n' "$OUT"
