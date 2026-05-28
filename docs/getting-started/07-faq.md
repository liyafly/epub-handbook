# 常见问题

## 环境 / 安装

**Q：跑 `build.sh` 报「zip is required」？**
A：你的系统没有 `zip` 命令。macOS / Linux 通常自带；Windows 用 WSL 或 Git Bash。

**Q：Python 报「No module named 'lxml'」？**
A：装一下：`python3 -m pip install lxml`。当前核心脚本尽量用标准库，但部分解析任务推荐 lxml。

**Q：Windows 路径里有空格，命令报错？**
A：路径用双引号包起来。

**Q：需要 Node.js 吗？**
A：运行时不需要。只有 `tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor assets 时需要一次 `npm pack`。

## Validator / 校验

**Q：`validate-epub-style-demo.sh` 对我的 epub 报 fixture token？**
A：那个脚本是给本仓 demo 用的；真实 epub 以 epubcheck 和 `validate_text_invariance.py` 为准。

**Q：epubcheck 报 fatal？**
A：先确认 zip 完整、`mimetype` 是第一个 entry、`META-INF/container.xml` 存在且能解析。

**Q：`validate_text_invariance.py` 报文本变化？**
A：这是红线触发。除非用户明确授权，否则回滚这次清洗。

## 阅读器

**Q：Kindle Previewer 转换失败？**
A：先看 [reader-matrix.yaml](../final/reader-matrix.yaml) 的已知 issue；若 Apple Books 也打不开，通常是 EPUB 自身坏了。

**Q：Apple Books 不刷新？**
A：Apple Books 会缓存。先在 Books 里删除旧版本，再重新拖入。

**Q：字体没生效？**
A：检查字体文件、OPF manifest、CSS `@font-face` 引用，以及阅读器是否允许 Publisher Font。

## EPUB Diff 工具

**Q：双击 `tools/epub-diff/index.html` 一片空白？**
A：先跑 `bash tools/epub-diff/scripts/fetch-vendor.sh` 抓 vendor。

**Q：`file://` 下模块加载失败？**
A：在 `tools/epub-diff/` 内跑 `python3 -m http.server 8000`，再打开 `http://localhost:8000/`。

**Q：拖了大 epub 进去，浏览器卡死？**
A：v1 目标支持约 1.5GB；超过或单 entry 过大时浏览器仍可能受限。

## AI 协作

**Q：我没有 Claude Code / Codex，怎么用 skills？**
A：`skills/` 本身就是 Markdown 文档，可以人工跟着走。

**Q：skill 把我的正文文字改了？**
A：这是事故。回滚并运行 `scripts/validate_text_invariance.py` 定位变化。

## 还没解决？

- 看 [glossary.md](glossary.md) 确认术语。
- 看 [reader-matrix.yaml](../final/reader-matrix.yaml) 是否已有记录。
- 提 issue 时附上环境、命令、完整输出和期望行为。
