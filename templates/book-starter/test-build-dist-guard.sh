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
PIPELINE_DIR="$BOOK_DIR/03 制作工作区/.pipeline"

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
if [ "${FAIL_NAV:-false}" = true ] && [ "$1" = run ] && [ "$2" = epub.package.nav.audit ]; then
	printf '{"findings":[{"level":"error","id":"test.nav-rejected"}]}\n'
	exit 1
fi
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

# A rejected candidate must remain inspectable without changing the last dist.
ACCEPTED_SHA=$(file_sha256 "$OUTPUT")
ACCEPTED_RECEIPT=$(cat "$RECEIPT")
if (FAIL_NAV=true run_build) >"$TMP_ROOT/rejected-build.log" 2>&1; then
	echo "build unexpectedly accepted a rejected candidate" >&2
	exit 1
fi
[ "$(file_sha256 "$OUTPUT")" = "$ACCEPTED_SHA" ] && \
	[ "$(cat "$RECEIPT")" = "$ACCEPTED_RECEIPT" ] || {
	echo "rejected build changed the accepted dist or its receipt" >&2
	exit 1
}
FAILED_BUILD=$(find "$PIPELINE_DIR" -maxdepth 1 -type d -name 'build.*' -print)
[ -n "$FAILED_BUILD" ] && [ -f "$FAILED_BUILD/final.epub" ] && \
	[ -f "$FAILED_BUILD/full-font.epub" ] && [ -f "$FAILED_BUILD/nav-audit.json" ] || {
	echo "rejected build discarded its candidate or diagnostic report" >&2
	exit 1
}
FAILED_CANDIDATE_SHA=$(file_sha256 "$FAILED_BUILD/final.epub")
FAILED_REPORT_SHA=$(file_sha256 "$FAILED_BUILD/nav-audit.json")
grep -q 'test.nav-rejected' "$FAILED_BUILD/nav-audit.json"
[ ! -d "$PIPELINE_DIR/build.lock" ] || {
	echo "rejected build did not release its lock" >&2
	exit 1
}
run_build >"$TMP_ROOT/retry-build.log" 2>&1
[ "$(file_sha256 "$FAILED_BUILD/final.epub")" = "$FAILED_CANDIDATE_SHA" ] && \
	[ "$(file_sha256 "$FAILED_BUILD/nav-audit.json")" = "$FAILED_REPORT_SHA" ] || {
	echo "a later build changed the retained failure evidence" >&2
	exit 1
}
[ "$(find "$PIPELINE_DIR" -maxdepth 1 -type d -name 'build.*' | wc -l | tr -d ' ')" = 1 ] || {
	echo "successful retry retained an unnecessary temporary build tree" >&2
	exit 1
}

printf 'manual repair' >>"$OUTPUT"
EDITED_SHA=$(file_sha256 "$OUTPUT")
LATEST_NAV_SHA=$(file_sha256 "$PIPELINE_DIR/nav-audit.json")
if run_build >"$TMP_ROOT/edited-dist.log" 2>&1; then
	echo "build unexpectedly overwrote a manually edited dist" >&2
	exit 1
fi
[ "$(file_sha256 "$OUTPUT")" = "$EDITED_SHA" ] || {
	echo "manual dist edit changed after blocked build" >&2
	exit 1
}
[ -f "$PIPELINE_DIR/nav-audit.json" ] && \
	[ "$(file_sha256 "$PIPELINE_DIR/nav-audit.json")" = "$LATEST_NAV_SHA" ] || {
	echo "receipt preflight failure discarded the previous diagnostic report" >&2
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

# The centralized entry accepts an explicit book directory, preserving the
# legacy copied-script behavior exercised above.
printf '%s\n' "$EDITED_SHA" >"$RECEIPT"
cat >"$BOOK_DIR/build.sh" <<'EOF'
#!/bin/sh
exec sh "$EPUB_HANDBOOK_ROOT/templates/book-starter/build.sh" "$(dirname "$0")" "$@"
EOF
HANDBOOK_ROOT=$(CDPATH= cd -- "$TEMPLATE_DIR/../.." && pwd)
EPUB_HANDBOOK_ROOT="$HANDBOOK_ROOT" EPUB_BIN="$FAKE_EPUB" sh "$BOOK_DIR/build.sh" >"$TMP_ROOT/forward.log" 2>&1
[ -s "$BOOK_DIR/03 制作工作区/交付记录.tsv" ]

# Injection must fail before dist replacement, including when one font is
# missing or a lock no longer matches. Unit tests verify the real provider's
# diagnostics; this integration verifies that the build preserves dist.
CONFIG="$BOOK_DIR/03 制作工作区/fonts.json"
printf '%s\n' '{"version":1,"fonts":[{"target":"OEBPS/Fonts/st.ttf","from":"master.ttf","sha256":"bad"}]}' >"$CONFIG"
FAKE_FONT="$TMP_ROOT/fake-font"
cat >"$FAKE_FONT" <<'EOF'
#!/bin/sh
echo "error: $INJECT_FAILURE" >&2
exit 2
EOF
chmod +x "$FAKE_FONT"
ACCEPTED_SHA=$(file_sha256 "$OUTPUT")
for reason in 'font library missing regular file: master.ttf' 'SHA-256 mismatch; expected old; actual new'; do
	if INJECT_FAILURE="$reason" EPUB_FONT_BIN="$FAKE_FONT" EPUB_BIN="$FAKE_EPUB" sh "$TEMPLATE_DIR/build.sh" "$BOOK_DIR" >"$TMP_ROOT/inject-failure.log" 2>&1; then
		echo "injection error did not stop the build" >&2; exit 1
	fi
	[ "$(file_sha256 "$OUTPUT")" = "$ACCEPTED_SHA" ]
	grep -q "$reason" "$TMP_ROOT/inject-failure.log"
done
rm "$CONFIG"

# accept-dist refuses stylesheet/source divergence even though redlines do
# not protect CSS bytes. A matching rebuilt member set may re-register SHA.
printf '\n<!-- unmerged source -->\n' >>"$EPUB_DIR/OEBPS/Text/01-chapter.xhtml"
if EPUB_BIN="$FAKE_EPUB" sh "$TEMPLATE_DIR/build.sh" "$BOOK_DIR" accept-dist >"$TMP_ROOT/accept-rejected.log" 2>&1; then
	echo "accept-dist registered a dist with unmerged source changes" >&2; exit 1
fi
EPUB_BIN="$FAKE_EPUB" sh "$TEMPLATE_DIR/build.sh" "$BOOK_DIR" >"$TMP_ROOT/accept-source.log" 2>&1
rm "$RECEIPT"
EPUB_BIN="$FAKE_EPUB" sh "$TEMPLATE_DIR/build.sh" "$BOOK_DIR" accept-dist >"$TMP_ROOT/accept.log" 2>&1
[ "$(cat "$RECEIPT")" = "$(file_sha256 "$OUTPUT")" ]
sh "$TEMPLATE_DIR/build.sh" "$BOOK_DIR" clean >"$TMP_ROOT/clean.log"
[ "$(find "$PIPELINE_DIR" -maxdepth 1 -type d -name 'build.*' | wc -l | tr -d ' ')" = 1 ]
echo "PASS centralized build, injection failures, accept-dist and clean"
