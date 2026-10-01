#!/usr/bin/env sh
set -eu

TEMPLATE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/book-starter-dist-guard.XXXXXX")
cleanup() {
	rm -rf "$TMP_ROOT"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

BOOK_DIR="$TMP_ROOT/book"
EPUB_DIR="$BOOK_DIR/03 制作工作区/epub"
FAKE_EPUB="$TMP_ROOT/fake-epub"
BUILD="$EPUB_DIR/build.sh"
OUTPUT="$BOOK_DIR/03 制作工作区/dist/book.epub"
RECEIPT="$BOOK_DIR/03 制作工作区/.pipeline/dist-sha256"

file_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

mkdir -p "$EPUB_DIR"
cp -R "$TEMPLATE_DIR/mimetype" "$TEMPLATE_DIR/META-INF" "$TEMPLATE_DIR/OEBPS" \
	"$TEMPLATE_DIR/build.sh" "$EPUB_DIR/"
cat >"$FAKE_EPUB" <<'EOF'
#!/usr/bin/env sh
printf '{}\n'
EOF
chmod +x "$FAKE_EPUB"

run_build() {
	EPUB_BIN="$FAKE_EPUB" sh "$BUILD"
}

if ! run_build >"$TMP_ROOT/first-build.log" 2>&1; then
	cat "$TMP_ROOT/first-build.log" >&2
	echo "first build failed" >&2
	exit 1
fi
[ -f "$OUTPUT" ] && [ -f "$RECEIPT" ] || {
	echo "first build did not create dist and its SHA receipt" >&2
	exit 1
}

printf '\n<!-- source update -->\n' >>"$EPUB_DIR/OEBPS/package.opf"
if ! run_build >"$TMP_ROOT/source-update.log" 2>&1; then
	cat "$TMP_ROOT/source-update.log" >&2
	echo "rebuild after a source change failed" >&2
	exit 1
fi

printf 'manual repair' >>"$OUTPUT"
EDITED_SHA=$(file_sha256 "$OUTPUT")
if run_build >"$TMP_ROOT/edited-dist.log" 2>&1; then
	echo "build unexpectedly overwrote a manually edited dist" >&2
	exit 1
fi
[ "$(file_sha256 "$OUTPUT")" = "$EDITED_SHA" ] || {
	echo "manual dist edit changed after blocked build" >&2
	exit 1
}
grep -q 'differs from the last accepted build' "$TMP_ROOT/edited-dist.log" || {
	cat "$TMP_ROOT/edited-dist.log" >&2
	echo "blocked build did not explain the SHA mismatch" >&2
	exit 1
}

rm -f "$RECEIPT"
if run_build >"$TMP_ROOT/missing-receipt.log" 2>&1; then
	echo "build unexpectedly overwrote dist without a receipt" >&2
	exit 1
fi
[ "$(file_sha256 "$OUTPUT")" = "$EDITED_SHA" ] || {
	echo "dist changed after missing-receipt block" >&2
	exit 1
}
grep -q 'without its accepted SHA-256 receipt' "$TMP_ROOT/missing-receipt.log" || {
	cat "$TMP_ROOT/missing-receipt.log" >&2
	echo "blocked build did not explain the missing receipt" >&2
	exit 1
}

echo "PASS book-starter dist edit guard"
