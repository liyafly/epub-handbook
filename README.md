# epub-handbook

[English](README.en.md)

把你已有的 EPUB 排得更好看，并且在 Apple Books、Kindle 和 Reeden 上都能正常显示。`epub` 是一个命令行工具：它把书解包成可以编辑的文件夹，你改 CSS，它负责打包和检查。正文一个字都不能变，目录和注释要能用，字号要能调，检查都通过才输出成品。每本书的设计由你决定；工具负责不把书做坏、效果失败时能回退、按阅读器分别出版本。支持 macOS、Windows 和 Linux，只处理没有 DRM 的书。

## 安装

**从源码构建**（需要 [Go](https://go.dev/dl/) 1.27 或更新版本）：

```sh
git clone https://github.com/liyafly/epub-handbook.git
cd epub-handbook
go build -o epub ./cmd/epub        # Windows 用 go build -o epub.exe ./cmd/epub
```

把生成的 `epub` 放进 PATH 里的目录，或者用完整路径运行。

**下载二进制**：[GitHub Releases](https://github.com/liyafly/epub-handbook/releases) 提供 macOS、Windows 和 Linux 的压缩包，可用 `SHA256SUMS` 核对。`epub book` 命令是 v0.5.0 之后加入的，新版本发布之前请从源码构建。macOS 二进制未签名，第一次运行可能被系统拦下。

**字体工具（可选）**：只有书里嵌入字体时才需要 `epub-font`。它是 Python 程序，用 [uv](https://docs.astral.sh/uv/) 从仓库安装：

```sh
uv tool install --editable --reinstall tools-font/epub-font
```

不嵌字体的书不用装。详见 [字体工具说明](tools-font/README.md)。

## 五步上手

1. 导入：`epub book import 原书.epub 我的书`，冻结原件、检查结构、解包到 `我的书/epub/`。
2. 预览：`epub book preview 我的书 --open`，打包一份不做检查的预览版并打开（Mac 上建议 `--open=Reeden`，见 [阅读器](docs/guide/readers.md)）。
3. 改样式：编辑 `我的书/epub/OEBPS/` 下的 CSS，可以从 [配方](docs/guide/recipes.md) 复制，然后再预览。
4. 构建：`epub book build 我的书`，完整检查后把成品写到 `我的书/out/`。
5. 送到阅读器：Apple Books 直接导入，Kindle 用 Send to Kindle，见 [阅读器](docs/guide/readers.md)。

## 使用指南

- [快速开始](docs/guide/quick-start.md)：完整流程、书目录、每项检查的含义、构建拒绝时怎么办
- [阅读器](docs/guide/readers.md)：Apple Books、Kindle、Reeden，以及按阅读器出不同版本
- [配方](docs/guide/recipes.md)：可复制的特殊效果和各阅读器表现
- [常见问题](docs/guide/faq.md)
- [术语表](docs/guide/glossary.md)

## 维护者与 AI 工具

AI 工具（Claude Code、Codex 等）和维护者先读 [AGENTS.md](AGENTS.md)。完整文档索引、CLI 退出码、输入限额和能力清单见 [维护者文档索引](docs/README.md)。贡献方式见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 范围

只做可重排的 EPUB 3。不生成 MOBI、AZW3 等 Kindle 专有格式，不做阅读器，也不模拟阅读器渲染：显示效果以真实阅读器为准。

## 许可

免费使用。代码、模板和 skills 使用 [MIT](LICENSES/MIT.txt) 许可，文档使用 [CC BY 4.0](LICENSES/CC-BY-4.0.txt)，详见 [LICENSE](LICENSE)。第三方材料的来源和许可见 [THIRD_PARTY.md](THIRD_PARTY.md)。
