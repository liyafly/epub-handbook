#!/usr/bin/env bash
set -euo pipefail

: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_ENV:?GITHUB_ENV is required}"

archive="$RUNNER_TEMP/epubcheck.zip"
destination="$RUNNER_TEMP/epubcheck"
checksum=74a59af8602bf59b1d04266a450d9cdcb5986e36d825adc403cde0d95e88c9e8

curl -fL -o "$archive" https://github.com/w3c/epubcheck/releases/download/v5.1.0/epubcheck-5.1.0.zip
if command -v sha256sum >/dev/null 2>&1; then
  printf '%s  %s\n' "$checksum" "$archive" | sha256sum -c -
else
  printf '%s  %s\n' "$checksum" "$archive" | shasum -a 256 -c -
fi
mkdir -p "$destination"
unzip -q "$archive" -d "$destination"
test -f "$destination/epubcheck-5.1.0/epubcheck.jar"
printf 'EPUBCHECK_JAR=%s\n' "$destination/epubcheck-5.1.0/epubcheck.jar" >> "$GITHUB_ENV"
