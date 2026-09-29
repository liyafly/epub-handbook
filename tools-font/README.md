# 字体工具

本目录保存 EPUB Handbook 的私有字体 provider，**不打包进 Go 发行包**，也不属于 `internal/` 的层级图。coverage、subset 与 check 共用 `epub-font` Python 项目、依赖锁和安装入口；Go capability 由 `internal/extern` 通过 PATH 启动 CLI。

## 安装与能力

```sh
uv tool install --editable tools-font/epub-font
epub-font --help
```

- `epub-font coverage BOOK.epub` 分析嵌入字体覆盖、字体链和阅读器风险，并可输出 JSON 与自包含 HTML 报告。`epub.font.coverage.analyze` 通过 PATH 调用该子命令。
- `epub-font subset BOOK.epub --out NEW.epub` 按全书字符集裁切 manifest 字体并检查结果。`epub.font.subset` 通过 PATH 调用该子命令；候选经 pipeline 红线检查后才写出。
- `epub-font check BOOK.epub` 使用独立采字逻辑检查 manifest 字体全量覆盖，可单独用于人工复核。它不复用 subset 的采集器。

缺少 provider 或 provider 无法启动时，Go capability 返回带稳定 finding ID 的结构化失败，不会静默跳过；被 Ctrl-C 或 deadline 打断时透传取消语义。

覆盖分析会跳过 U+0300 以下字符（包含 ASCII）、U+2000–U+2E7F 通用标点，且不收 CSS 生成字符，因此适合判断字体链与阅读器风险，不能证明字体“全量”覆盖。全量校验使用 `epub-font check`；子集命令另用独立收集器比较源字体与输出字体，阻断子集引入的覆盖损失。

详细选项见 [`epub-font/README.md`](epub-font/README.md)。

## `font-preview.html`

**单文件、离线、双击即用、零第三方依赖**的字体预览工具。

- 拖入 `.ttf` / `.otf` / `.woff` / `.ttc` / `.woff2` 字体文件
- 显示字体名与版本（手写 SFNT name 表解析器）
- 输入任意文字，在多个字体间实时对比渲染效果
- 字号可调（12–96px）
- 不上传、不联网、纯本地运行

用任何浏览器打开 `font-preview.html` 即可使用。

## 生僻字 → 大字库 → 造字工作流

`epub-font coverage` 与 `font-preview.html` 协同使用：

1. 生成报告：`epub-font coverage book.epub -o report.json`（同时写出自包含 `report.html`，双击即自动渲染）。
2. 打开 `report.html`：看「生僻字 / 出 GBK」统计与「🔗 字体链体检」；用顶部按钮 **复制生僻·未覆盖字 / 复制全部生僻字**。
3. 把复制出的字集粘进 `font-preview.html` 的输入框，拖入候选**大字库**，肉眼确认是否都有字形。
4. 或用 `--validate-with 大字库.ttf` 让工具计算残余；报告「📚 候选大字库验证」面板里可复制残余字。
5. 残余字 = 造字 / 合成字库的输入清单。

> 注意区分两类问题：① 字根本没嵌（`生僻·未覆盖` 非空）→ 需大字库/造字；② 字嵌了但链写错（链体检 fail，如聊斋）→ 改字体链（全字库放链首 + generic 兜底），不需要新字库。
