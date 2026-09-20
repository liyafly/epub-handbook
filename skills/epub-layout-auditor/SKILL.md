---
name: epub-layout-auditor
description: 只读审核 EPUB 排版改动、比较基线并按证据分级分派专项 skill。用于总审稿或优化前后检查；不代替包结构预检，不自动修书，不把静态扫描当阅读器验收。
---

# EPUB 排版审稿

## 何时用

排版 review、候选比较或问题尚未分类时使用；新输入先做 package/nav 预检。按用户请求区分“只审查”与“审查并修复”，不重复确认已明确的范围。公共返回与验收见 [索引](../README.md)。

## 调什么

```sh
epub run epub.layout.audit --input "book.epub" --json
```

只读，无额外参数。还须阅读实际改动的 XHTML/CSS/OPF 与相关资源；扫描不代替语义和视觉 review。按问题读取 [SPEC](../../docs/final/SPEC-实现约束.md) 对应章节，兼容结论核对 [reader matrix](../../docs/final/reader-matrix.yaml)。

## 返回怎么读

前缀 `epub.layout.audit.`：`summary`、`auditStatus`、`findingsByLevel`、`recommendedSkills`、`actionableFindings`。后者给出位置、证据、置信度与 `autoFixable`；可自动修复标记不等于用户已授权。当前 `toolAvailability` 仅探测 EPUBCheck。

## 依据返回怎么判断

- 优先级：P0 损坏/不可读；P1 裁切、注释失联等功能/兼容问题；P2 间距、字体、fallback；P3 清理与一致性。每项给位置、证据、影响和最小修复，避免笼统“优化一下”。
- 用 `recommendedSkills` 与 [索引](../README.md) 选择最窄分支；已明确的局部问题不扩大成全书重构。
- “好看”需结合书型、页面角色和已有设计：先挑代表性正文、章首、复杂页做候选样例，检查层次、节奏、留白、字体一致性及大字号/窄屏，再决定能否推广。
- 不改正文、章节顺序或图片来规避版式问题。红线失败保留候选，定位后修复；不自动回滚用户改动。未看真实阅读器时只报告静态风险及待验项。
