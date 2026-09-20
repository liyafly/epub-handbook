---
name: epub-css-layering-optimizer
description: 保守修补 EPUB CSS 分号、装饰行和旧字体链，人工判断样式归层与级联。用于样式清理；自动去重、语义分层和局部作用域归并均已停用，不改正文。
---

# EPUB CSS 分层与清理

## 何时用

区分两件事：CLI 修补缺失分号、装饰分隔行与已知旧字体链；新增样式、语义拆层和冲突级联需人工判断。先读 [SPEC §7](../../docs/final/SPEC-实现约束.md) 的完整分层契约；便签边框按需读 [专门指南](../../docs/how-to/note-box-border-styles.md)。

## 调什么

```sh
epub run epub.css.layering.optimize --input "before.epub" --output "candidate.epub" --dry-run --json
# 审查后
epub run epub.css.layering.optimize --input "before.epub" --output "candidate.epub" --json
epub redline --check all "before.epub" "candidate.epub"
```

不要请求 `merge_scoped_local_css=true`：该语义归并已因 lossless 安全停用，只产生 warning，不会改 link/body class。

## 返回怎么读

前缀 `epub.css.layering.optimize.`：`cssFilesBefore/After`、`fontDeclarationsRewritten`、`warnings`；`semanticFactoringDisabled=true`、`scopedMergeDisabled=true`、`duplicateDeduplication=disabled` 是当前策略。旧的拆分/去重计数键保留但为 0，不代表功能可用。公共状态见 [索引](../README.md)。

## 依据返回怎么判断

- 核对具体声明改动，字体链变化不能误伤既有嵌入角色；不以“CSS 数量越少”作为成功标准。即使 CSS 逐字节相同，独立 manifest id、refines/fallback 或 @import 也可能赋予不同语义，不自动删除副本。
- 人工归层：字体/角色绑定进 fonts；通用正文进 base；notes/effects/literary/media/vertical/poster 按组件职责。XHTML 先依赖后覆盖，manifest 只保留实际资源。
- 局部 CSS 的引用集合、相对 URL 或级联关系不清时保留，不猜测可合并；不得整文档重序列化。EPUB/WebKit 前缀与基础 fallback 不因浏览器正常而删掉。
- 红线失败保留候选，定位原因；通过后还需 diff 和相关页面的视觉检查，正文不变并不证明 CSS 外观不变。
