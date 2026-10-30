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

mkdir -p "$TMP/01 源文件" "$TMP/03 制作工作区/epub"
for section in 参考 正文校订 版式 历史; do
	mkdir -p "$TMP/02 校对材料/$section"
	touch "$TMP/02 校对材料/$section/.gitkeep"
done
EPUB_DIR="$TMP/03 制作工作区/epub"
cp -R "$TEMPLATE_DIR/mimetype" "$TEMPLATE_DIR/META-INF" "$TEMPLATE_DIR/OEBPS" "$EPUB_DIR/"
cat >"$TMP/build.sh" <<'EOF'
#!/bin/sh
exec sh "${EPUB_HANDBOOK_ROOT:-$HOME/Developer/epub-handbook}/templates/book-starter/build.sh" "$(dirname "$0")" "$@"
EOF

cp "$TEMPLATE_DIR/README.md" "$TMP/README.md"

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
# Preview-only injected masters; source subset fonts are ignored individually
# when a book mixes locked library fonts and retained embedded subsets.
/03 制作工作区/epub/OEBPS/Fonts/

# Local editor noise.
.DS_Store
Thumbs.db
EOF

cat >"$TMP/THIRD_PARTY.md" <<'EOF'
# 第三方材料

在书级 Git 中加入来源于第三方的正文或图片前，记录来源、作者、许可和保留理由。
共享字体的来源与许可统一见字体库 `字体清单.tsv`（默认 `../fonts/`，可由 EPUB_FONT_LIBRARY 指定）。
许可未核实时，不配置公开远程，也不发布原始材料。

| 路径 | 来源 | 作者 | 许可 | 保留理由 |
| --- | --- | --- | --- | --- |
EOF

cat >"$TMP/制作说明.md" <<'EOF'
# 制作说明

## 当前状态
- 最新交付：见 `03 制作工作区/交付记录.tsv` 最后一行（尚未构建）
- 构建：`sh build.sh`；字体母版来自共享库，字体配置在 `03 制作工作区/fonts.json`
- 目标阅读器：待填写；阅读器实测待验证
- 使用的配方：无

## 待办
- [ ] 完成书目信息、内容与目标阅读器验证

## 已知阅读器差异
- 尚未实测

## 决策
- 正文校订：无；细节与授权材料放 `02 校对材料/`
EOF

git -C "$TMP" init -b main >/dev/null
mv "$TMP" "$DESTINATION"
TMP=""
trap - EXIT HUP INT TERM

printf 'Created book workspace: %s\n' "$DESTINATION"
printf 'Edit: %s\n' "$DESTINATION/03 制作工作区/epub/OEBPS/"
printf 'Build: sh "%s/build.sh"\n' "$DESTINATION"
printf 'Git:   git -C "%s" status --short\n' "$DESTINATION"
