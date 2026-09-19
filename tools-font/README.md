# 字体工具

本目录是 EPUB Handbook 的字体相关独立工具，**不打包进发行包**，也不属于 `internal/` 的层级图 ——
它是被 `internal/extern` 以外部进程调起的独立 provider（架构定位见 `AGENTS.md` 的「架构分工」表）。

## coverage-detector：安装与缺失时的行为

`coverage-detector/` 是独立的 Python + FontTools 项目，用 `uv` 管理；
**安装与命令行用法见 `coverage-detector/README.md`**（首次需在该目录执行一次 `uv sync`）。

`epub run epub.font.coverage.analyze` 由 `internal/extern` 调起同一个 `uv run python -m src.cli`。
**缺少 `uv` 时不会静默跳过**：能力以 `status: failed` 显式失败，findings 里给出
`uv is required for tools-font/coverage-detector`；`uv` 存在但进程起不来（权限、工作目录缺失等）时
报 `coverage detector could not be started: <原因>`，而不是伪装成"跑完且干净退出"。
被 Ctrl-C 或 deadline 打断时透传取消语义（`status: cancelled`），不会被误判成工具故障。

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
