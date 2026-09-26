# epub CLI @VERSION@

下载与操作系统、架构匹配的 `epub_v@VERSION@_<os>_<arch>.tar.gz`，并在解压后运行。

```sh
tar -xzf epub_v@VERSION@_<os>_<arch>.tar.gz
./epub version --json
./epub capabilities --json
```

Windows 使用 `epub.exe` 并在 PowerShell 中运行 `./epub.exe`。校验和用于检查下载完整性。Linux 可运行：

```sh
sha256sum -c SHA256SUMS
```

macOS 使用 `shasum -a 256 -c SHA256SUMS`。Windows PowerShell 可运行 `Get-FileHash .\epub_v@VERSION@_windows_amd64.tar.gz -Algorithm SHA256`，并将输出与 `SHA256SUMS` 中相应行比较。

二进制内嵌 capability contracts、schemas 和样式预设，因此基本命令不依赖仓库 checkout。字体覆盖是独立可选 provider，不包含在 CLI 归档里；具体安装与缺失行为见仓库 `tools-font/README.md`。

如需字体覆盖 provider，请先获取此版本的仓库源码，再按 [`tools-font/README.md`](https://github.com/liyafly/epub-handbook/blob/v@VERSION@/tools-font/README.md) 安装。普通 EPUB 检查、结构处理和排版能力不需要安装 provider；调用字体覆盖能力时若缺少 `uv`，CLI 会明确报告失败。

Release 中的四个平台均由对应原生 GitHub Actions runner 执行版本、能力发现和内嵌预设 dry-run smoke。该 smoke 不代表目标阅读器兼容性验收。macOS 二进制目前未签名或公证；首次运行前请先核对 SHA256。
