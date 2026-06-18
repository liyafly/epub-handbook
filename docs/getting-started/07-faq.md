# 常见问题

## 环境 / 安装

**Q：跑 `build.sh` 报「zip is required」？**
A：你的系统没有 `zip` 命令。macOS / Linux 通常自带；Windows 用 WSL 或 Git Bash。

**Q：Python 报「No module named 'lxml'」？**
A：装一下：`python3 -m pip install lxml`。当前核心脚本尽量用标准库，但部分解析任务推荐 lxml。

**Q：Windows 路径里有空格，命令报错？**
A：路径用双引号包起来。

**Q：需要 Node.js 吗？**
A：本仓运行时不需要。Calibre / VS Code 也不依赖 Node.js。

## Validator / 校验

**Q：`validate-epub-style-demo.sh` 对我的 epub 报 fixture token？**
A：那个脚本是给本仓 demo 用的；真实 epub 以 `scripts/epub_preflight_harness.py`、`scripts/epub_lint.py` 和 `validate_text_invariance.py` 为准，EPUBCheck 交给 GitHub Actions 跑。

**Q：CI 里的 EPUBCheck 报 fatal？**
A：先确认 zip 完整、`mimetype` 是第一个 entry、`META-INF/container.xml` 存在且能解析。本地先跑 `scripts/epub_lint.py` 定位包结构问题。

**Q：`validate_text_invariance.py` 报文本变化？**
A：这是红线触发。除非用户明确授权，否则回滚这次清洗。

## EPUB2 / EPUB3

**Q：正文文件是 `.xhtml`，是不是说明这本书已经是 EPUB3？**
A：不是。EPUB2 也使用 XHTML。先打开 OPF，看 `<package version="2.0">` 还是 `<package version="3.0">`。详细区别见 [08-epub2-epub3-compatibility.md](08-epub2-epub3-compatibility.md)。

**Q：EPUB2 能不能使用 EPUB3 的弹出注释？**
A：部分阅读器可以识别兼容混写。需要在 XHTML 根 `<html>` 上增加 `xmlns:epub="http://www.idpf.org/2007/ops"`，再写 `epub:type="noteref"` 和 `epub:type="footnote"`。但这不会把 OPF2 升级成 EPUB3，`<aside>` 也不是严格 EPUB2 XHTML 主路径。请按 [EPUB2 外壳中的 popup note 兼容写法](../guides/epub2-popup-note-compatibility.md) 保留双向链接并逐阅读器实测。

**Q：加了 `xmlns:epub`，为什么还是没有弹窗？**
A：namespace 只声明 `epub:` 前缀。阅读器还要支持对应语义，正文链接要指向正确 `id`，注释目标要保留真实文本。平台不识别弹窗时，必须能退化为普通跳转。

## 阅读器

**Q：Kindle Previewer 转换失败？**
A：先看 [reader-matrix.yaml](../final/reader-matrix.yaml) 的已知 issue；若 Apple Books 也打不开，通常是 EPUB 自身坏了。

**Q：Apple Books 不刷新？**
A：Apple Books 会缓存。先在 Books 里删除旧版本，再重新拖入。

**Q：字体没生效？**
A：检查字体文件、OPF manifest、CSS `@font-face` 引用，以及阅读器是否允许 Publisher Font。

## EPUB Diff

**Q：怎么对比两个 EPUB？**
A：见 [EPUB diff review](../pipeline/epub-diff-review.md)。主路径用 Calibre Editor 的「Compare to another book」；精细路径用 `unzip` + `git diff --no-index` 或 VS Code。

**Q：Calibre 的 Compare 菜单是灰的？**
A：Tweak Book 必须先进入编辑状态；随便存一次（Cmd+S）再 Compare。

**Q：可以批处理多本 EPUB 的 diff 吗？**
A：可以；走精细路径，用 `unzip` + `shasum` + `diff` 的 shell 组合。在 PR 里贴 `git diff --no-index --stat` 输出即可。

## AI 协作

**Q：AI 模型应该先读哪个文件？**
A：先读根目录 [AGENTS.md](../../AGENTS.md)。`CLAUDE.md` 只是 Claude Code 的兼容入口，规则只维护一份。

**Q：我没有 Claude Code / Codex，怎么用 skills？**
A：`skills/` 本身就是 Markdown 文档，可以人工跟着走。

**Q：skill 把我的正文文字改了？**
A：这是事故。回滚并运行 `scripts/validate_text_invariance.py` 定位变化。

## 还没解决？

- 看 [glossary.md](glossary.md) 确认术语。
- 看 [reader-matrix.yaml](../final/reader-matrix.yaml) 是否已有记录。
- 提 issue 时附上环境、命令、完整输出和期望行为。
