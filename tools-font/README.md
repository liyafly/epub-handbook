# 字体工具

本目录是 EPUB Handbook 的字体相关独立工具，**不打包进发行包**，也不属于 `internal/` 的层级图 ——
它是被 `internal/extern` 以外部进程调起的独立 provider（架构定位见 `AGENTS.md` 的「架构硬约束」）。

## coverage-detector：安装与缺失时的行为

`coverage-detector/` 是独立的 Python + FontTools 项目，用 `uv` 管理；
**安装与命令行用法见 `coverage-detector/README.md`**（首次需在该目录执行一次 `uv sync`）。

`epub run epub.font.coverage.analyze` 由 `internal/extern` 调起同一个 `uv run python -m src.cli`。
**缺少 `uv` 时不会静默跳过**：能力以 `status: failed` 显式失败，findings 里给出
`uv is required for tools-font/coverage-detector`；`uv` 存在但进程起不来（权限、工作目录缺失等）时
报 `coverage detector could not be started: <原因>`，而不是伪装成"跑完且干净退出"。
被 Ctrl-C 或 deadline 打断时透传取消语义（`status: cancelled`），不会被误判成工具故障。

覆盖分析的字符清单会跳过 U+0300 以下字符（包含 ASCII）、U+2000–U+2E7F 通用标点区间，且不收 CSS 生成字符；它适合判断字体链与阅读器风险，不能证明嵌入字体“全量”覆盖。全量校验使用独立 CLI `epub-font check`，默认检查全部 manifest 字体。

## font-preview.html

**单文件、离线、双击即用、零第三方依赖**的字体预览工具。

- 拖入 `.ttf` / `.otf` / `.woff` / `.ttc` / `.woff2` 字体文件
- 显示字体名与版本（手写 SFNT name 表解析器）
- 输入任意文字，在多个字体间实时对比渲染效果
- 字号可调（12–96px）
- 不上传、不联网、纯本地运行

用任何浏览器打开 `font-preview.html` 即可使用。

## 生僻字 → 大字库 → 造字 工作流

`coverage-detector` 与 `font-preview.html` 协同走完整条逻辑：

1. 生成报告：`cd coverage-detector && uv run python -m src.cli book.epub -o report.json`（同时写出自包含 `report.html`，双击即自动渲染）。
2. 打开 `report.html`：看「生僻字 / 出 GBK」统计与「🔗 字体链体检」；用顶部按钮 **复制生僻·未覆盖字 / 复制全部生僻字**。
3. 把复制出的字集粘进 `font-preview.html` 的输入框，拖入候选**大字库**，肉眼确认是否都有字形。
4. 或 `--validate-with 大字库.ttf` 让工具直接算残余；报告「📚 候选大字库验证」面板里 **复制残余字**。
5. 残余字 = 造字 / 合成字库的输入清单。

> 注意区分两类问题：① 字根本没嵌（`生僻·未覆盖` 非空）→ 需大字库/造字；② 字嵌了但链写错（链体检 fail，如聊斋）→ 改字体链（全字库放链首 + generic 兜底），不需要新字库。

## epub-font：字体子集化与全量校验（含可变字体）

`epub-font/` 是独立 Python + FontTools provider（与 coverage-detector 同样用 `uv` 管理，不打包进 Go CLI 发行包）。除了直接使用 `epub-font subset` / `epub-font check`，书级构建还可通过 Go capability `epub.font.subset` 调用 provider；该能力把检查通过的字体 entry 写进内存候选，再由 pipeline 统一完成红线和最终写出。安装方式及详细选项见 [`epub-font/README.md`](epub-font/README.md)。
它把 EPUB 中**已存在**的字体条目替换成按全书字符集裁切后的字节，支持静态字体与可变字体
（`instance` 钉住轴出静态字重 / `limit` 收窄轴范围 / `keep` 保留变体），并核验 cmap、IVS、竖排替换字形、轮廓、轴与格式。
不指定 `--config` 时自动处理全部静态 manifest 字体；可变字体必须在配置中显式填写 `variation.mode`。OPF / CSS / XHTML 不变，alias 与包内路径保持稳定。
产物仍需 `epub redline --check all`、`epub run epub.package.nav.audit`、`epub run epub.font.coverage.analyze` 与目标阅读器实测。
