# 测自己的 EPUB

跑通 [01-first-epub.md](01-first-epub.md) 之后，你可以把自己的 epub 跑一遍这套工具链。

## 0. 准备

```sh
mkdir -p work
cp /path/to/your-book.epub work/source.epub
```

不要原地覆盖原始 epub。

## 1. 用 epubcheck 跑一次（推荐）

[epubcheck](https://www.w3.org/publishing/epubcheck/) 是 W3C 官方校验工具，是「合法 EPUB」的最低底线。

```sh
brew install epubcheck
epubcheck work/source.epub
```

fatal 必须修；error 一般要修；warning 看情况。

## 2. 用本仓 validator 跑一次

```sh
bash scripts/validate-epub-style-demo.sh --epub work/source.epub
```

这个脚本是为 `templates/epub-style-demo/` 设计的，对真实 epub 会报很多 fixture 相关失败。真正有用的是它附带的通用校验：mimetype、container、OPF 完整性、CSS url() 引用。

## 3. 用 epub-diff 工具确认基线

把 `tools/epub-diff/index.html` 拖入浏览器，两边都选 `work/source.epub`：

- 期望：五层都 identical。
- 如果出现差异：说明 diff 工具或文件读取有问题，重新拉取并重试。

## 4. 调用 layout-auditor 看 findings

```text
请使用 epub-layout-auditor 审稿 work/source.epub
```

或者直接跑：

```sh
python3 scripts/epub_ai_harness.py --mode cleanup work/source.epub
```

## 5. 决定是否清洗

把 findings 对照 [epub-cleanup-flow.md](../guides/epub-cleanup-flow.md)：

- 红线很多（文本错误、缺核心 metadata）-> 不要清洗，先回到源头校对。
- 黄线为主（样式 / 字体 / 结构混乱）-> 可以进入清洗流水线。
- 绿线为主（仅格式化噪声）-> 不一定值得清洗。

## 6. 用阅读器实测

清洗前后都用目标阅读器打开看：

- Apple Books（默认字号 + 大字号）。
- Kindle Previewer（默认 profile + Paperwhite profile + 字号 1/4/7）。
- 多看 / Readest（如目标读者用这些）。

把实测结果记下来；贡献回本仓时按 [CONTRIBUTING.md](../../CONTRIBUTING.md) 的 reader-matrix 规范回写。

## 7. 卡住了？

去看 [07-faq.md](07-faq.md) 或 [glossary.md](glossary.md)。
