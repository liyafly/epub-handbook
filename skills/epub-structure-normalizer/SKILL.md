---
name: epub-structure-normalizer
description: 用纯 Python 标准库整理已有 EPUB 的资源目录，并按固定顺序先格式化、再按 OPF manifest id 还原被混淆的资源文件名，同步重写 OPF、XHTML、CSS、NCX、SVG、SMIL 和标准字体混淆 URI。用于 EPUB 内部路径散乱、文件名不可读、需要先做结构规范化再精排，或用户明确要求文件名反混淆时；不用于 DRM 解密。
---

# EPUB 结构格式化与文件名反混淆

使用 `scripts/epub_structure_tool.py` 处理已有 EPUB。脚本只依赖 Python 标准库，始终写出新文件，不原地覆盖输入。

## 固定流程

默认使用 `normalize`，固定执行：

```text
原始 EPUB -> format 目录格式化 -> deobfuscate-filenames 文件名反混淆 -> normalized EPUB
```

`normalize` 内部使用临时中间 EPUB，并只保留最终输出。不要跳过格式化直接执行文件名反混淆，除非正在定位单步故障。

## 边界

- `format`：把 manifest 资源整理到 OPF 同级的 `Text/`、`Styles/`、`Images/`、`Fonts/`、`Audio/`、`Video/`、`Misc/`，保留原文件名。
- `deobfuscate-filenames`：整理目录，并优先按 OPF manifest `id` 生成可读文件名。
- `normalize`：先执行 `format`，再执行 `deobfuscate-filenames`。这是正常使用入口。
- 同步更新 OPF、XHTML、CSS、NCX、SVG、SMIL 中的本地链接。
- 允许 EPUB 标准字体混淆；移动字体时同步更新 `META-INF/encryption.xml` 的 URI，不修改字体字节。
- 遇到正文、样式、图片或未知算法加密时立即停止。本 skill 不提供 DRM 解密。
- CSS 中引用不存在的字体文件时，不猜测它对应哪个嵌入字体，也不删除声明。保留 `local()` fallback，并把 `missing-css-font-fallback` 警告交给人工复核。
- CSS 中非字体资源断链仍是阻断错误，必须修复后才能进入后续清洗。
- 不改写作者正文，不做 EPUB3 迁移，不替代 `epub-package-nav-auditor`。

## 工作流

1. 保留原始输入：

```sh
mkdir -p work/before work/after
cp input.epub work/before/source.epub
```

2. 检查加密边界和 package：

```sh
python3 scripts/epub_structure_tool.py inspect \
  work/before/source.epub \
  --report-format json
```

3. 使用组合入口 dry-run：

```sh
python3 scripts/epub_structure_tool.py normalize \
  work/before/source.epub \
  --output work/after/step-0-normalized.epub \
  --dry-run \
  --report-format json > work/step-0-normalize.dry-run.json
```

4. Review 两个阶段的 `mappings` 和 `warnings`，移除 `--dry-run` 写出新 EPUB，并保存实际报告：

```sh
python3 scripts/epub_structure_tool.py normalize \
  work/before/source.epub \
  --output work/after/step-0-normalized.epub \
  --report-format json > work/step-0-normalize.json
```

5. 立刻把实际报告作为路径映射传给红线 gate：

```sh
python3 scripts/validate_text_invariance.py \
  work/before/source.epub \
  work/after/step-0-normalized.epub \
  --check all \
  --path-map work/step-0-normalize.json
python3 scripts/epub_preflight_harness.py \
  work/after/step-0-normalized.epub \
  --format json
```

如果 `inspect` 已确认 `META-INF/encryption.xml` 只含 EPUB 标准字体混淆，在红线命令额外添加 `--allow-font-obfuscation`。

6. 用 Calibre Editor 或 VS Code 做人工 diff review，再进入 EPUB3 迁移、OPF/nav 审核和排版专项 skill。

## 单步排障

只在定位故障时分别运行：

```sh
python3 scripts/epub_structure_tool.py format input.epub --output formatted.epub --dry-run
python3 scripts/epub_structure_tool.py deobfuscate-filenames formatted.epub --output deobfuscated.epub --dry-run
```

## 禁止事项

- 不把 `deobfuscate-filenames` 描述成内容解密或 DRM 解密。
- 不在未 review `mappings` 的情况下批量覆盖输出。
- 不跳过 `validate_text_invariance.py` 和 diff review。
- 不把结构规范化与 EPUB3 package 迁移混成一步；规范化完成后再用 `scripts/epub3_migration_harness.py`。

## 验证

修改脚本或本 skill 后至少运行：

```sh
python3 scripts/test_epub_structure_tool.py
python3 scripts/test_validate_text_invariance.py
python3 scripts/validate_skills_basic.py
git diff --check
```

## 参考

行为目标参考 [cnwxi/epub_tool](https://github.com/cnwxi/epub_tool) 的 `reformat` 与文件名 `decrypt` 工作流。本仓脚本是重新实现的 stdlib-only 保守版本；第三方说明见 `THIRD_PARTY.md`。
