# 5 分钟做一本最小 EPUB

## 你需要

- 一台装了 `zip` 命令的电脑（macOS / Linux 自带；Windows 用 Git Bash 或 WSL）。
- Python 3.10+。
- 一个现代浏览器（Chrome / Safari / Firefox 任一，用来开 diff 工具）。

## 步骤

### 1. 克隆仓库

```sh
git clone https://github.com/<owner>/epub-handbook.git
cd epub-handbook
```

### 2. 构建示例 EPUB

```sh
bash templates/epub-style-demo/build.sh
```

输出会打印一个 `.epub` 路径，如：

```text
templates/epub-style-demo/dist/epub-style-demo-20260526-091234.epub
```

### 3. 跑校验

```sh
EPUB="templates/epub-style-demo/dist/epub-style-demo-20260526-091234.epub"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
```

退出码 0 = 通过。

### 4. 改一段文字，重新构建，再用 diff 工具看看改了什么

```sh
# 编辑 templates/epub-style-demo/OEBPS/Text/01-body.xhtml，改一行文字
bash templates/epub-style-demo/build.sh
ls -t templates/epub-style-demo/dist/*.epub | head -2
```

把 `tools/epub-diff/index.html` 用浏览器打开，选这两个 epub 做对比。

### 5. 用阅读器打开

- **macOS**：双击文件，用 Apple Books 打开。
- **iOS**：通过 iCloud Drive 或 AirDrop 发到手机。
- **Kindle**：拖入 Kindle Previewer 3。
- **任何浏览器**：用 epub.js reader 或 Readest。

## 下一步

- 看 [02-anatomy.md](02-anatomy.md) 了解 epub 内部结构。
- 看 [docs/final/](../final/) 学习硬规则。
- 看 [skills/](../../skills/) 了解 AI 协作能力。
