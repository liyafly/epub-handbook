#!/usr/bin/env sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ "$(basename -- "$(dirname -- "$PROJECT_DIR")")" = "03 制作工作区" ]; then
	WORK_DIR=$(CDPATH= cd -- "$PROJECT_DIR/.." && pwd)
	BOOK_DIR=$(CDPATH= cd -- "$WORK_DIR/.." && pwd)
	EPUB_DIR=$PROJECT_DIR
else
	WORK_DIR=$PROJECT_DIR
	BOOK_DIR=$PROJECT_DIR
	EPUB_DIR=$PROJECT_DIR
fi

DIST_DIR="$WORK_DIR/dist"
PIPELINE_DIR="$WORK_DIR/.pipeline"
LOCK_DIR="$PIPELINE_DIR/build.lock"
OUTPUT="$DIST_DIR/book.epub"
EPUB_BIN=${EPUB_BIN:-epub}

if ! command -v zip >/dev/null 2>&1; then
	echo "zip is required." >&2
	exit 1
fi
if ! command -v "$EPUB_BIN" >/dev/null 2>&1; then
	echo "EPUB CLI not found: $EPUB_BIN (set EPUB_BIN to its executable path)." >&2
	exit 1
fi
if [ ! -f "$EPUB_DIR/mimetype" ] || [ ! -d "$EPUB_DIR/META-INF" ] || [ ! -d "$EPUB_DIR/OEBPS" ]; then
	echo "Expected mimetype, META-INF/, and OEBPS/ under $EPUB_DIR" >&2
	exit 1
fi
if [ -L "$EPUB_DIR/mimetype" ] || find "$EPUB_DIR/META-INF" "$EPUB_DIR/OEBPS" -type l -print -quit | grep -q .; then
	echo "EPUB source tree contains a symlink; resolve it before building." >&2
	exit 1
fi

mkdir -p "$DIST_DIR" "$PIPELINE_DIR"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
	echo "Another build is active (or a stale lock exists): $LOCK_DIR" >&2
	exit 1
fi
BUILD_TMP=$(mktemp -d "$PIPELINE_DIR/build.XXXXXX")
cleanup() {
	rm -rf "$BUILD_TMP"
	rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

FULL_EPUB="$BUILD_TMP/full-font.epub"
FINAL_EPUB="$BUILD_TMP/final.epub"
FONT_CONFIG="$BOOK_DIR/fonts.json"

(
	cd "$EPUB_DIR"
	zip -X -0 "$FULL_EPUB" mimetype >/dev/null
	zip -X -r -9 "$FULL_EPUB" META-INF OEBPS -x '*/.DS_Store' >/dev/null
)

HAS_FONTS=false
if [ -f "$FONT_CONFIG" ] || find "$EPUB_DIR/OEBPS" -type f \( \
	-iname '*.ttf' -o -iname '*.otf' -o -iname '*.woff' -o -iname '*.woff2' -o \
	-iname '*.ttc' -o -iname '*.otc' \) -print -quit | grep -q .; then
	HAS_FONTS=true
fi

if [ "$HAS_FONTS" = true ]; then
	if [ -f "$FONT_CONFIG" ]; then
		"$EPUB_BIN" run epub.font.subset --input "$FULL_EPUB" --output "$FINAL_EPUB" --json "font_config=$FONT_CONFIG"
	else
		"$EPUB_BIN" run epub.font.subset --input "$FULL_EPUB" --output "$FINAL_EPUB" --json
	fi
else
	"$EPUB_BIN" run epub.package.nav.audit --input "$FULL_EPUB" --json
	cp "$FULL_EPUB" "$FINAL_EPUB"
fi

"$EPUB_BIN" run epub.package.nav.audit --input "$FINAL_EPUB" --json
"$EPUB_BIN" redline --check all "$FULL_EPUB" "$FINAL_EPUB"

# BUILD_TMP is under PIPELINE_DIR, on the same volume as DIST_DIR. Rename only
# after every check succeeds so a failed build preserves the last good EPUB.
mv -f "$FINAL_EPUB" "$OUTPUT"
printf 'Built %s\n' "$OUTPUT"
