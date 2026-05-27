#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ASSETS="$ROOT/assets/vendor"
ZIPJS_VERSION="2.8.26"

mkdir -p "$ASSETS"
TMP=$(mktemp -d)
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

cd "$TMP"

npm pack "@zip.js/zip.js@$ZIPJS_VERSION" >/dev/null
tar -xzf zip.js-zip.js-*.tgz
cp package/dist/zip.min.js "$ASSETS/zip.js"
cp package/LICENSE "$ASSETS/zip.js.LICENSE"

echo "Fetched vendor assets to $ASSETS"
ls -lh "$ASSETS"
