#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
HOOK_PATH=$(git rev-parse --git-path hooks/pre-commit)

mkdir -p "$(dirname -- "$HOOK_PATH")"
ln -sf "$ROOT/hooks/pre-commit.epub-handbook" "$HOOK_PATH"
chmod +x "$ROOT/hooks/pre-commit.epub-handbook"

printf '%s\n' "installed $HOOK_PATH -> $ROOT/hooks/pre-commit.epub-handbook"
