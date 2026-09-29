# 自造 EPUB 演示样本

本目录放完全由本仓自造的 EPUB demo。它们用于演示清洗流水线、红线 gate 和 外部 diff 工具（Calibre / VS Code，见 [EPUB diff review](../../docs/pipeline/epub-diff-review.md)），不依赖公版书来源。

`dist/` 是本地生成目录，默认不入 Git。用户下载仓库后可以运行构建脚本生成这些 EPUB，用来查看、验证和做 diff 参考。

## 样本

| slug | 用途 | before / after |
| --- | --- | --- |
| `city-field-notes` | 样式分层、脚注、表格、代码、资源改动 | 文本不变，红线应通过 |
| `paper-garden` | 诗段、Ruby、blockquote、竖排增强 | 文本不变，红线应通过 |
| `loop-auto-fix` | 多轮 loop 正向演示：章节根元素故意漏语言属性 | 审计应检出 `missing-html-lang`（auto_fixable），正文不变 |
| `redline-trap` | 故意改写正文的反例 | 红线应失败 |
| `legacy-epub2` | EPUB2/NCX、XHTML 1.1 DTD 与命名实体、跨文件尾注链接与同页普通文本尾注、EPUB2 封面元数据、GB18030 和 Latin-1 CSS 资源 | normalize 是空操作（`mappings=[]`、`rewrittenFiles=0`），并原样保留两份旧编码 CSS；migrate 转换同页普通尾注标记并通过全项 redline，保留跨文件尾注链接；迁移后的 XHTML 应为良构 |

## 生成

```sh
bash templates/cleanup-demo-books/build.sh
```

输出在 `templates/cleanup-demo-books/dist/`。如果脚本逻辑变化，需要重新生成并本地验证这些 demo EPUB。

`dist/` 里的 `.epub` 和 `manifest.json` 都是可再生文件；不要提交生成产物。提交改动应落在对应的样本源目录（如 `city-field-notes/`、`paper-garden/`）、`build_demo_epubs.py`、`build.sh`、测试或说明文档中。

`build.sh` 调用 `build_demo_epubs.py` 生成 fixture，因此构建这些样本需要 Python 3。该生成器是测试辅助工具，不属于用户 EPUB 执行面。

## 验证

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/city-field-notes-before.epub \
  templates/cleanup-demo-books/dist/city-field-notes-after-clean.epub

epub redline --check all \
  templates/cleanup-demo-books/dist/paper-garden-before.epub \
  templates/cleanup-demo-books/dist/paper-garden-after-clean.epub
```

反例必须失败：

```sh
epub redline --check all \
  templates/cleanup-demo-books/dist/redline-trap-before.epub \
  templates/cleanup-demo-books/dist/redline-trap-after-text-changed.epub
```

## Diff 演示

按 [EPUB diff review](../../docs/pipeline/epub-diff-review.md) 用 Calibre Editor 或 VS Code 选 before / after 对：

- `city-field-notes`：应看到样式、资源和结构层变化；文本层保持一致。
- `paper-garden`：应看到 CSS 与资源变化；文本层保持一致。
- `redline-trap`：应看到文本层变化；这对文件只用于反例演示，不是合法清洗结果。

## 多轮 loop 正向演示

```sh
bash templates/cleanup-demo-books/build.sh
epub run epub.package.nav.audit \
  --input templates/cleanup-demo-books/dist/loop-auto-fix-before.epub \
  --json
```

Go CLI 没有多轮自动 loop 命令：报告中应出现 `missing-html-lang` finding（`auto_fixable: true`）。修复按 [清洗流水线](../../docs/pipeline/cleanup-flow.md) 的固定顺序逐能力执行，最后用 `epub redline --check all` 验证正文不变。

## EPUB2 与旧编码回归样本

`legacy-epub2-before.epub` 是一份自造的 EPUB2 输入：目录依赖 NCX 且没有 nav，章节使用 XHTML 1.1 DOCTYPE 和 `&nbsp;`、`&mdash;`。第 1 章的普通 `[1]` 尾注链接到单独的 `notes.xhtml`，用于验证跨文件链接保持有效；第 2 章包含转换器支持的同页 `[2]` 普通文本尾注，用于验证 G23 仅把迁移后的 noteref/backlink 标记视为控件文字。封面通过 `<meta name="cover">` 指向 `Images/cover.jpg`。正文使用 `book.css`，另保留未引用的 `legacy.css`（GB18030）与 `latin1.css`（Latin-1）资源，用于验证 normalize 遍历 ZIP 内样式资源时保持原字节。

修复 G14 后，`epub.structure.normalize` 对没有待改 URL 的两份 CSS 应保持逐字节不变。修复 G13、G15、G23、G24 后，EPUB3 迁移应仅因补入 nav 产生已限定的红线差异，保留封面对应关系与尾注正文，并输出可由 XML 解析器读取的 XHTML；全项 redline 应通过。旧编码 CSS 不在 OPF manifest 中，因此 EPUBCheck 不会把它们当作活动样式表解析；CI 会同时检查原始样本和修复后的 EPUB3 候选。
