# 测自己的 EPUB

跑通 [做一本书](做一本书.md) 之后，你可以把自己的 EPUB 跑一遍这套工具链。

## 0. 准备

```sh
W=$(mktemp -d)
cp /path/to/your-book.epub "$W/source.epub"
```

不要原地覆盖原始 epub。

## 1. 用本仓预检和 lint 跑一次

本机不要求安装 EPUBCheck。日常先用本仓 CLI 检查 ZIP、container、OPF、manifest 引用和清洗风险；正文不变红线由 `epub redline` 比对清洗前后产物；EPUBCheck 只在 GitHub Actions 里作为 CI gate 跑。

```sh
epub run epub.package.nav.audit --input "$W/source.epub" --json
```

error 必须修，warning 看情况记录。

## 2. 用外部 diff 工具确认基线（可选）

如果你想确认 diff 工作流就绪，把 `$W/source.epub` 拷贝一份做自比：

```sh
cp "$W/source.epub" "$W/source-copy.epub"
```

按 [EPUB diff review](../pipeline/epub-diff-review.md) 用 Calibre Editor 比较 `$W/source.epub` 与 `$W/source-copy.epub`。

- 期望：所有文件 unchanged。
- 如果 Calibre 报差异：说明拷贝过程中改动了文件，重新拷贝。

## 3. 调用 epub-audit 看 findings

```text
请使用 epub-audit 审稿 $W/source.epub
```

或者直接跑：

```sh
epub run epub.package.nav.audit --input "$W/source.epub" --json
epub run epub.text.content.analyze --input "$W/source.epub" --json
epub run epub.font.coverage.analyze --input "$W/source.epub" --json
```

## 4. 决定是否清洗

把 nav.audit / 精排分析 / findings 对照 [cleanup-flow.md](../pipeline/cleanup-flow.md)：

- 红线很多（文本错误、缺核心 metadata）-> 不要清洗，先回到源头校对。
- 不是 EPUB3 或缺 nav -> 先走 `epub run epub.package.migrate.epub3` 生成 EPUB3 基线。
- 黄线为主（样式 / 字体 / 结构混乱）-> 可以进入清洗流水线。
- 绿线为主（仅格式化噪声）-> 不一定值得清洗。

决定清洗后，按 cleanup-flow 主线 S0–S6 执行（迁移在 S3）。

## 5. 用阅读器实测

清洗前后都用目标阅读器打开看：

- Apple Books（默认字号 + 大字号）。
- Kindle Previewer（默认 profile + Paperwhite profile + 字号 1/4/7）。
- 多看 / Readest（如目标读者用这些）。

把实测结果记下来；贡献回本仓时按 [CONTRIBUTING.md](../../CONTRIBUTING.md) 的 reader-matrix 规范回写。

## 6. 卡住了？

去看 [07-faq.md](07-faq.md) 或 [glossary.md](glossary.md)。
