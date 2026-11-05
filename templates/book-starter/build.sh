#!/usr/bin/env sh
# 旧布局书的兼容入口：书根 build.sh 转发到这里，这里再转发给 `epub book`。
# 全部构建逻辑在 Go CLI（internal/pipeline/book_*.go）；新书直接用 `epub book build`。
#
#   sh build.sh [书目录] [build|fonts|clean|accept-dist|preview] [其他 epub book 参数]
set -eu

BOOK_DIR=.
if [ "$#" -gt 0 ] && [ -d "$1" ]; then
	BOOK_DIR=$1
	shift
fi
COMMAND=${1:-build}
if [ "$#" -gt 0 ]; then shift; fi

LOCAL_BIN_DIR=${EPUB_LOCAL_BIN-"$HOME/.local/bin"}
if [ -n "$LOCAL_BIN_DIR" ] && [ -d "$LOCAL_BIN_DIR" ]; then PATH="$PATH:$LOCAL_BIN_DIR"; export PATH; fi

exec "${EPUB_BIN:-epub}" book "$COMMAND" "$BOOK_DIR" "$@"
