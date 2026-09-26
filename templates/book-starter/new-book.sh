#!/usr/bin/env sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "Usage: sh templates/book-starter/new-book.sh <book-directory>" >&2
	exit 2
fi

TEMPLATE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DESTINATION=$1
case "$DESTINATION" in
	/*) ;;
	*) DESTINATION="$PWD/$DESTINATION" ;;
esac

if [ -e "$DESTINATION" ]; then
	echo "Book directory already exists: $DESTINATION" >&2
	exit 1
fi

PARENT=$(dirname -- "$DESTINATION")
NAME=$(basename -- "$DESTINATION")
case "$NAME" in
	""|"."|"..")
		echo "Choose a new book directory, not its parent or the filesystem root." >&2
		exit 2
		;;
esac
mkdir -p "$PARENT"
PARENT=$(CDPATH= cd -- "$PARENT" && pwd)
DESTINATION="$PARENT/$NAME"
if [ -e "$DESTINATION" ]; then
	echo "Book directory already exists: $DESTINATION" >&2
	exit 1
fi

TMP=$(mktemp -d "$PARENT/.book-starter.XXXXXX")
cleanup() {
	rm -rf "$TMP"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

mkdir -p "$TMP/01 源文件" "$TMP/02 校对材料" "$TMP/03 制作工作区/epub"
EPUB_DIR="$TMP/03 制作工作区/epub"
cp -R "$TEMPLATE_DIR/mimetype" "$TEMPLATE_DIR/META-INF" "$TEMPLATE_DIR/OEBPS" \
	"$TEMPLATE_DIR/README.md" "$TEMPLATE_DIR/build.sh" "$EPUB_DIR/"

cat >"$TMP/01 源文件/README.md" <<'EOF'
# 冻结源文件

已有 EPUB 底本和必要原始素材放在这里，记录 SHA-256，保持文件不变。新作可在有原始材料时再添加。
EOF

cat >"$TMP/02 校对材料/README.md" <<'EOF'
# 校对材料

仅在实际发生校对、参考版比较或获得正文修订授权时添加材料与逐项决策。
EOF

cat >"$TMP/.gitignore" <<'EOF'
# Temporary build reports and the single replaceable delivery artifact.
/03 制作工作区/.pipeline/
/03 制作工作区/dist/

# Local editor noise.
.DS_Store
Thumbs.db
EOF

cat >"$TMP/THIRD_PARTY.md" <<'EOF'
# 第三方材料

在书级 Git 中加入来源于第三方的正文、图片或字体前，记录来源、作者、许可和保留理由。
许可未核实时，不配置公开远程，也不发布原始材料。

| 路径 | 来源 | 作者 | 许可 | 保留理由 |
| --- | --- | --- | --- | --- |
EOF

cat >"$TMP/制作说明.md" <<'EOF'
# 制作说明

## 当前源文件

- 解包源目录：`03 制作工作区/epub/`
- 字体母版：与源树一起维护；每次构建都从完整字体生成本次字形子集。
- 最终产物：`03 制作工作区/dist/book.epub`（构建通过后覆盖）

## 构建与验证

- 命令：`sh '03 制作工作区/epub/build.sh'`
- 最近构建：待首次构建后填写
- 产物 SHA-256：待首次构建后填写
- 阅读器实测：待验证；记录阅读器名称、版本和 artifact SHA 后再标为通过

## 重要制作决策

- 字体许可、字体链或阅读器差异：待填写
- 正文校订或其他授权差异：无
EOF

git -C "$TMP" init -b main >/dev/null
mv "$TMP" "$DESTINATION"
TMP=""
trap - EXIT HUP INT TERM

printf 'Created book workspace: %s\n' "$DESTINATION"
printf 'Edit: %s\n' "$DESTINATION/03 制作工作区/epub/OEBPS/"
printf 'Build: sh "%s/03 制作工作区/epub/build.sh"\n' "$DESTINATION"
printf 'Git:   git -C "%s" status --short\n' "$DESTINATION"
