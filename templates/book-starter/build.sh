#!/usr/bin/env sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ "$#" -gt 0 ] && [ -d "$1" ]; then
	BOOK_DIR=$(CDPATH= cd -- "$1" && pwd)
	shift
	WORK_DIR="$BOOK_DIR/03 制作工作区"
	if [ -d "$BOOK_DIR/epub" ]; then WORK_DIR=$BOOK_DIR; fi
	EPUB_DIR="$WORK_DIR/epub"
elif [ "$(basename -- "$(dirname -- "$PROJECT_DIR")")" = "03 制作工作区" ]; then
	WORK_DIR=$(CDPATH= cd -- "$PROJECT_DIR/.." && pwd)
	BOOK_DIR=$(CDPATH= cd -- "$WORK_DIR/.." && pwd)
	EPUB_DIR=$PROJECT_DIR
else
	WORK_DIR=$PROJECT_DIR
	BOOK_DIR=$PROJECT_DIR
	EPUB_DIR=$PROJECT_DIR
fi
COMMAND=${1:-build}
if [ "$#" -gt 0 ]; then shift; fi
if [ "$#" -ne 0 ]; then echo "Usage: build.sh [BOOK_DIRECTORY] [fonts|clean|accept-dist]" >&2; exit 2; fi
case "$COMMAND" in build|fonts|clean|accept-dist) ;; *) echo "Unknown command: $COMMAND" >&2; exit 2;; esac

DIST_DIR="$WORK_DIR/dist"
PIPELINE_DIR="$WORK_DIR/.pipeline"
LOCK_DIR="$PIPELINE_DIR/build.lock"
OUTPUT="$DIST_DIR/${EPUB_OUTPUT_NAME:-book.epub}"
case "${EPUB_OUTPUT_NAME:-book.epub}" in */*|..|.) echo "EPUB_OUTPUT_NAME must be a filename" >&2; exit 2;; esac
DIST_SHA_FILE="$PIPELINE_DIR/dist-sha256"
DIST_SHA_TMP="$PIPELINE_DIR/.dist-sha256.$$"
EPUB_BIN=${EPUB_BIN:-epub}
FONT_BIN=${EPUB_FONT_BIN:-epub-font}
LOCAL_BIN_DIR=${EPUB_LOCAL_BIN-"$HOME/.local/bin"}
if [ -n "$LOCAL_BIN_DIR" ] && [ -d "$LOCAL_BIN_DIR" ]; then PATH="$PATH:$LOCAL_BIN_DIR"; export PATH; fi
if [ ! -f "$EPUB_DIR/mimetype" ] && [ -d "$WORK_DIR" ]; then
	# Multi-volume workspaces keep one forwarding entry per volume.
	VOLUMES_FOUND=false
	for VOLUME_BUILD in "$WORK_DIR"/*/build.sh; do
		if [ -f "$VOLUME_BUILD" ]; then
			VOLUMES_FOUND=true
			sh "$VOLUME_BUILD" "$COMMAND"
		fi
	done
	if [ "$VOLUMES_FOUND" = true ]; then exit 0; fi
fi
FONT_CONFIG="$WORK_DIR/fonts.json"
if [ ! -f "$FONT_CONFIG" ]; then FONT_CONFIG="$BOOK_DIR/fonts.json"; fi

if [ "$COMMAND" = fonts ]; then
	if [ ! -f "$FONT_CONFIG" ]; then echo "No fonts.json; legacy source fonts stay in place."; exit 0; fi
	exec "$FONT_BIN" inject --config "$FONT_CONFIG" --tree "$EPUB_DIR"
fi
if [ "$COMMAND" = clean ]; then
	python3 - "$PIPELINE_DIR" <<'PY'
from pathlib import Path
import shutil, sys
root = Path(sys.argv[1])
if (root / 'build.lock').exists():
    raise SystemExit('Build lock exists; stop the active build before cleaning')
builds = sorted((p for p in root.glob('build.*') if p.is_dir() and not p.is_symlink()), key=lambda p: p.stat().st_mtime, reverse=True)
for path in builds[1:]:
    print('Removing', path)
    shutil.rmtree(path)
if builds:
    print('Retained latest diagnostic build:', builds[0])
PY
	exit 0
fi

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
if [ ! -f "$EPUB_DIR/mimetype" ] || [ ! -d "$EPUB_DIR/META-INF" ] || { [ ! -d "$EPUB_DIR/OEBPS" ] && [ "${EPUB_LEGACY_LAYOUT:-0}" != 1 ]; }; then
	echo "Expected mimetype, META-INF/, and OEBPS/ under $EPUB_DIR" >&2
	exit 1
fi
if [ -L "$EPUB_DIR/mimetype" ] || find "$EPUB_DIR" -type l -print -quit | grep -q .; then
	echo "EPUB source tree contains a symlink; resolve it before building." >&2
	exit 1
fi

mkdir -p "$DIST_DIR" "$PIPELINE_DIR"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
	echo "Another build is active (or a stale lock exists): $LOCK_DIR" >&2
	exit 1
fi
BUILD_TMP=""
BUILD_SUCCEEDED=false
cleanup() {
	build_status=$?
	trap - EXIT
	if [ -n "$BUILD_TMP" ]; then
		if [ "$BUILD_SUCCEEDED" = true ]; then
			rm -rf "$BUILD_TMP"
		else
			# Keep the failed input/candidate and the reports from this attempt.
			# A later build may replace the fixed-name reports in PIPELINE_DIR.
			for report_name in font-subset.json nav-audit.json redline.txt previous-dist-redline.txt; do
				if [ -f "$PIPELINE_DIR/$report_name" ]; then
					if ! cp "$PIPELINE_DIR/$report_name" "$BUILD_TMP/$report_name"; then
						printf 'Could not snapshot report; inspect original: %s\n' "$PIPELINE_DIR/$report_name" >&2
					fi
				fi
			done
			printf 'Build failed; retained diagnostic files: %s\n' "$BUILD_TMP" >&2
		fi
	fi
	rm -f "$DIST_SHA_TMP"
	rmdir "$LOCK_DIR" 2>/dev/null || true
	exit "$build_status"
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
	if [ "$COMMAND" != accept-dist ] && [ ! -f "$DIST_SHA_FILE" ]; then
		printf 'Refusing to overwrite existing dist without its accepted SHA-256 receipt: %s\n' "$OUTPUT" >&2
		printf 'Review/import any dist edits into %s, then record the reviewed artifact SHA in %s.\n' "$EPUB_DIR" "$DIST_SHA_FILE" >&2
		exit 1
	fi
	DIST_SHA_AT_START=$(file_sha256 "$OUTPUT")
	RECORDED_DIST_SHA=$(cat "$DIST_SHA_FILE" 2>/dev/null || true)
	if [ "$COMMAND" != accept-dist ] && [ "$DIST_SHA_AT_START" != "$RECORDED_DIST_SHA" ]; then
		printf 'Refusing to overwrite dist: its SHA-256 differs from the last accepted build.\n' >&2
		printf 'Artifact: %s\nRecorded: %s\nCurrent:  %s\n' "$OUTPUT" "$RECORDED_DIST_SHA" "$DIST_SHA_AT_START" >&2
		printf 'Review the EPUB changes and merge intended edits into %s before rebuilding.\n' "$EPUB_DIR" >&2
		exit 1
	fi
fi
if [ "$COMMAND" = accept-dist ] && [ "$DIST_EXISTS_AT_START" != true ]; then
	echo "accept-dist requires an existing dist artifact." >&2; exit 1
fi

# Only retire the latest reports after the existing dist passes its receipt
# preflight. Failed build directories are independent and are never pruned here.
rm -f "$PIPELINE_DIR/font-subset.json" "$PIPELINE_DIR/nav-audit.json" "$PIPELINE_DIR/redline.txt" \
	"$PIPELINE_DIR/previous-dist-redline.txt"
BUILD_TMP=$(mktemp -d "$PIPELINE_DIR/build.XXXXXX")
FULL_EPUB="$BUILD_TMP/full-font.epub"
FINAL_EPUB="$BUILD_TMP/final.epub"
SRC_DIR="$BUILD_TMP/src"

mkdir -p "$SRC_DIR"
if [ "${EPUB_LEGACY_LAYOUT:-0}" = 1 ]; then
	# Existing books may retain an audited non-OEBPS layout. This explicit
	# compatibility path packages the existing source without renaming it.
	find "$EPUB_DIR" -mindepth 1 -maxdepth 1 ! -name '.*' ! -name build.sh -exec cp -R {} "$SRC_DIR/" \;
else
	cp -R "$EPUB_DIR/mimetype" "$EPUB_DIR/META-INF" "$EPUB_DIR/OEBPS" "$SRC_DIR"/
fi
if [ -f "$FONT_CONFIG" ]; then
	# Legacy configs need no provider until actual fonts are subsetted.
	if python3 - "$FONT_CONFIG" <<'PY'
import json, sys
config = json.load(open(sys.argv[1], encoding='utf-8'))
sys.exit(0 if any('from' in job for job in config.get('fonts', [])) else 1)
PY
	then
		"$FONT_BIN" inject --config "$FONT_CONFIG" --tree "$SRC_DIR" >"$BUILD_TMP/font-inject.json"
	fi
fi
find "$SRC_DIR" -type f -exec chmod 0644 {} +
find "$SRC_DIR" -exec touch -h -t 198001010000 {} +
(
	cd "$SRC_DIR"
	zip -X -0 "$FULL_EPUB" mimetype >/dev/null
	find . -type f ! -path './mimetype' \
		! -name '.*' ! -name Thumbs.db ! -name '*~' ! -path '*/.*/*' |
		sed 's|^./||' | LC_ALL=C sort | zip -X -9 "$FULL_EPUB" -@ >/dev/null
)

HAS_FONTS=false
if [ -f "$FONT_CONFIG" ] || (
	cd "$SRC_DIR"
	find . -mindepth 1 -type d -name '.*' -prune -o -type f \( \
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
if [ -f "$PIPELINE_DIR/font-subset.json" ]; then
	python3 - "$PIPELINE_DIR/font-subset.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding='utf-8'))
report = doc.get('facts', {}).get('epub.font.subset.providerReport', {})
for font in report.get('fonts', []):
    for warning in font.get('warnings', []):
        print('WARNING', warning)
PY
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
if [ "$COMMAND" = accept-dist ]; then
	# All member bytes, including CSS and images, must match the source-derived
	# candidate. Redlines alone do not detect forgotten stylesheet edits.
	python3 - "$OUTPUT" "$FINAL_EPUB" <<'PY'
import sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as accepted, zipfile.ZipFile(sys.argv[2]) as candidate:
    a, b = set(accepted.namelist()), set(candidate.namelist())
    changed = sorted(a ^ b | {name for name in a & b if accepted.read(name) != candidate.read(name)})
    if changed:
        raise SystemExit('Dist still differs from rebuilt source; merge/review first: ' + ', '.join(changed))
PY
	FINAL_SHA=$(file_sha256 "$OUTPUT")
else
	FINAL_SHA=$(file_sha256 "$FINAL_EPUB")
fi
# Prepare the delivery log before replacing the accepted artifact.
python3 - "$BOOK_DIR" "$PROJECT_DIR" "$FONT_CONFIG" "$FINAL_SHA" "$WORK_DIR/交付记录.tsv" "$BUILD_TMP/delivery.tsv" <<'PY'
from datetime import datetime, timezone
from pathlib import Path
import json, subprocess, sys
book, handbook, config, sha, log, temporary = sys.argv[1:]
def revision(path):
    p = subprocess.run(['git', '-C', path, 'rev-parse', '--verify', 'HEAD'], capture_output=True, text=True)
    if p.returncode:
        return 'uncommitted-dirty'
    status = subprocess.run(['git', '-C', path, 'status', '--porcelain', '--untracked-files=normal'], capture_output=True, text=True, check=True).stdout
    dirty = any('交付记录.tsv' not in line for line in status.splitlines())
    return p.stdout.strip() + ('-dirty' if dirty else '')
fonts = []
if Path(config).is_file():
    fonts = [f"{job['target']}={job['sha256']}" for job in json.loads(Path(config).read_text())['fonts'] if 'from' in job]
header = '日期\t书源commit\t手册commit\tdist sha256\t字体库版本\n'
existing = Path(log).read_text() if Path(log).exists() else header
row = '\t'.join([datetime.now(timezone.utc).isoformat(), revision(book), revision(handbook), sha, ';'.join(fonts) or 'source-tree']) + '\n'
Path(temporary).write_text(existing + row)
PY
if [ "$COMMAND" != accept-dist ]; then mv -f "$FINAL_EPUB" "$OUTPUT"; fi
printf '%s\n' "$FINAL_SHA" >"$DIST_SHA_TMP"
mv -f "$DIST_SHA_TMP" "$DIST_SHA_FILE"
mv -f "$BUILD_TMP/delivery.tsv" "$WORK_DIR/交付记录.tsv"
BUILD_SUCCEEDED=true
printf 'Built %s\n' "$OUTPUT"
