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
DIST_SHA_FILE="$PIPELINE_DIR/dist-sha256"
DIST_SHA_TMP="$PIPELINE_DIR/.dist-sha256.$$"
EPUB_BIN=${EPUB_BIN:-epub}

file_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		echo "sha256sum or shasum is required to protect an existing dist artifact." >&2
		return 1
	fi
}

run_check() {
	report_path=$1
	shift
	label=$1
	if [ "$label" = run ]; then
		label=$2
	fi
	if "$EPUB_BIN" "$@" >"$report_path" 2>&1; then
		warnings=$(grep -c '"level": "warn"' "$report_path" || :)
		printf 'PASS %s (warnings: %s; report: %s)\n' "$label" "${warnings:-0}" "${report_path#"$BOOK_DIR"/}"
	else
		status=$?
		printf 'FAIL %s (report: %s)\n' "$label" "${report_path#"$BOOK_DIR"/}" >&2
		cat "$report_path" >&2
		return "$status"
	fi
}

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
rm -f "$PIPELINE_DIR/font-subset.json" "$PIPELINE_DIR/nav-audit.json" "$PIPELINE_DIR/redline.txt" \
	"$PIPELINE_DIR/previous-dist-redline.txt"
BUILD_TMP=$(mktemp -d "$PIPELINE_DIR/build.XXXXXX")
cleanup() {
	rm -rf "$BUILD_TMP"
	rm -f "$DIST_SHA_TMP"
	rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

DIST_EXISTS_AT_START=false
DIST_SHA_AT_START=""
if [ -e "$OUTPUT" ] || [ -L "$OUTPUT" ]; then
	DIST_EXISTS_AT_START=true
	if [ -L "$OUTPUT" ] || [ ! -f "$OUTPUT" ]; then
		echo "Refusing to replace a non-regular dist artifact: $OUTPUT" >&2
		exit 1
	fi
	if [ ! -f "$DIST_SHA_FILE" ]; then
		printf 'Refusing to overwrite existing dist without its accepted SHA-256 receipt: %s\n' "$OUTPUT" >&2
		printf 'Review/import any dist edits into %s, then record the reviewed artifact SHA in %s.\n' "$EPUB_DIR" "$DIST_SHA_FILE" >&2
		exit 1
	fi
	DIST_SHA_AT_START=$(file_sha256 "$OUTPUT")
	RECORDED_DIST_SHA=$(cat "$DIST_SHA_FILE")
	if [ "$DIST_SHA_AT_START" != "$RECORDED_DIST_SHA" ]; then
		printf 'Refusing to overwrite dist: its SHA-256 differs from the last accepted build.\n' >&2
		printf 'Artifact: %s\nRecorded: %s\nCurrent:  %s\n' "$OUTPUT" "$RECORDED_DIST_SHA" "$DIST_SHA_AT_START" >&2
		printf 'Review the EPUB changes and merge intended edits into %s before rebuilding.\n' "$EPUB_DIR" >&2
		exit 1
	fi
fi

FULL_EPUB="$BUILD_TMP/full-font.epub"
FINAL_EPUB="$BUILD_TMP/final.epub"
SRC_DIR="$BUILD_TMP/src"
FONT_CONFIG="$BOOK_DIR/fonts.json"

mkdir -p "$SRC_DIR"
cp -R "$EPUB_DIR/mimetype" "$EPUB_DIR/META-INF" "$EPUB_DIR/OEBPS" "$SRC_DIR"/
find "$SRC_DIR" -type f -exec chmod 0644 {} +
find "$SRC_DIR" -exec touch -h -t 198001010000 {} +
(
	cd "$SRC_DIR"
	zip -X -0 "$FULL_EPUB" mimetype >/dev/null
	find META-INF OEBPS -type f \
		! -name '.*' ! -name Thumbs.db ! -name '*~' ! -path '*/.*/*' |
		LC_ALL=C sort | zip -X -9 "$FULL_EPUB" -@ >/dev/null
)

HAS_FONTS=false
if [ -f "$FONT_CONFIG" ] || (
	cd "$SRC_DIR"
	find META-INF OEBPS -type d -name '.*' -prune -o -type f \( \
		-iname '*.ttf' -o -iname '*.otf' -o -iname '*.woff' -o -iname '*.woff2' -o \
		-iname '*.ttc' -o -iname '*.otc' \) ! -name '.*' -print -quit | grep -q .
); then
	HAS_FONTS=true
fi

if [ "$HAS_FONTS" = true ]; then
	if [ -f "$FONT_CONFIG" ]; then
		run_check "$PIPELINE_DIR/font-subset.json" run epub.font.subset --input "$FULL_EPUB" --output "$FINAL_EPUB" --json "font_config=$FONT_CONFIG"
	else
		run_check "$PIPELINE_DIR/font-subset.json" run epub.font.subset --input "$FULL_EPUB" --output "$FINAL_EPUB" --json
	fi
else
	cp "$FULL_EPUB" "$FINAL_EPUB"
fi

run_check "$PIPELINE_DIR/nav-audit.json" run epub.package.nav.audit --input "$FINAL_EPUB" --json
run_check "$PIPELINE_DIR/redline.txt" redline --check all "$FULL_EPUB" "$FINAL_EPUB"

if [ -f "$OUTPUT" ]; then
	printf 'INFO comparing candidate with previous dist (review only; does not block): %s\n' "$OUTPUT"
	"$EPUB_BIN" redline --check all "$OUTPUT" "$FINAL_EPUB" \
		>"$PIPELINE_DIR/previous-dist-redline.txt" 2>&1 || true
	cat "$PIPELINE_DIR/previous-dist-redline.txt"
fi

# Do not overwrite an artifact changed or created while this build was running.
if [ "$DIST_EXISTS_AT_START" = true ]; then
	if [ ! -f "$OUTPUT" ] || [ -L "$OUTPUT" ]; then
		echo "Refusing to replace dist because the previous artifact disappeared during this build." >&2
		exit 1
	fi
	DIST_SHA_NOW=$(file_sha256 "$OUTPUT")
	if [ "$DIST_SHA_NOW" != "$DIST_SHA_AT_START" ]; then
		echo "Refusing to replace dist because it changed while this build was running." >&2
		exit 1
	fi
elif [ -e "$OUTPUT" ] || [ -L "$OUTPUT" ]; then
	echo "Refusing to replace dist because an artifact appeared while this build was running." >&2
	exit 1
fi

# BUILD_TMP is under PIPELINE_DIR, on the same volume as DIST_DIR. Rename only
# after every check succeeds so a failed build preserves the last good EPUB.
FINAL_SHA=$(file_sha256 "$FINAL_EPUB")
mv -f "$FINAL_EPUB" "$OUTPUT"
printf '%s\n' "$FINAL_SHA" >"$DIST_SHA_TMP"
mv -f "$DIST_SHA_TMP" "$DIST_SHA_FILE"
printf 'Built %s\n' "$OUTPUT"
