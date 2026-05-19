# Skills

这个目录保存 Codex/Claude Code 可直接读取的 EPUB 转换技能。这里的 skill 是“转换契约”，不是下游 `epub-pro` 的实现代码。

## 当前 Skills

| Skill | 用途 | 对应模板 |
|---|---|---|
| `epub-alite-converter` | 把封面、卷首、章节扉页或海报页转换为 A-lite 可重排整页方案 | `templates/epub-style-demo/OEBPS/Text/03-vertical-alite.xhtml` |
| `epub-popup-footnote-converter` | 把普通注释、尾注或旧式注释转换为项目标准弹出注释结构 | `templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml` |
| `epub-legacy-footnote-fallback` | 在标准弹注结构上叠加多看、掌阅读者端兼容识别钩子 | `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml` |

## 维护规则

- 不改 `SKILL.md` frontmatter 的字段名。
- 保持 skill 面向行为：触发场景、固定目标、工作流、验证清单、禁止事项。
- 不在 skill 里写下游引擎架构或平台分发逻辑。
- 修改结构性规则时，同步检查 `docs/final/EPUB 3 HTML CSS 属性速查表.html` 的预览和 `templates/epub-style-demo/` 的样本。
- 新增 skill 前，先判断是否只是模板样式样本；如果只是样式验证，优先放进 `templates/`。
