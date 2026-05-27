#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ASSETS="$ROOT/assets/vendor"
PIERRE_VERSION="1.2.3"
ZIPJS_VERSION="2.8.26"

mkdir -p "$ASSETS"
TMP=$(mktemp -d)
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

cd "$TMP"

npm pack "@pierre/diffs@$PIERRE_VERSION" >/dev/null
tar -xzf pierre-diffs-*.tgz
rm -rf "$ASSETS/pierre-diffs"
cp -R package/dist "$ASSETS/pierre-diffs"
cp package/LICENSE.md "$ASSETS/pierre-diffs.LICENSE"
printf "%s\n" "import './pierre-diffs/style.js'; import './pierre-diffs/components/web-components.js';" > "$ASSETS/pierre-diffs.js"
rm -rf package pierre-diffs-*.tgz

npm pack "@zip.js/zip.js@$ZIPJS_VERSION" >/dev/null
tar -xzf zip.js-zip.js-*.tgz
cp package/dist/zip.min.js "$ASSETS/zip.js"
cp package/LICENSE "$ASSETS/zip.js.LICENSE"

echo "Fetched vendor assets to $ASSETS"
ls -lh "$ASSETS"
