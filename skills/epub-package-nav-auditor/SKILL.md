---
name: epub-package-nav-auditor
description: 只读预检 EPUB ZIP、OPF、manifest/spine、nav/NCX、封面与资源引用。用于清洗入口、资源变更后或阅读器无法打开时；不自动修复或判断视觉质量。
---

# EPUB Package 与导航审核

## 何时用

已有 EPUB 的第一步预检，以及迁移、打包、资源增删改名后的复核。公共返回见 [索引](../README.md)；包规则见 [SPEC](../../docs/final/SPEC-实现约束.md) §5、§5.8、§10。

## 调什么

```sh
epub run epub.package.nav.audit --input "book.epub" --json
```

只读，不传 `--output`。修复后对新产物重跑。

## 返回怎么读

- 前缀 `epub.package.nav.audit.`：`summary`（OPF、资源/spine 计数、版本/语言）、`auditStatus`、`findingsByLevel`、`recommendedSkills`、`actionableFindings`。
- `actionableFindings[]` 包含 `kind/file/locator/params/lane/autoFixable/confidence/evidence`；`autoFixable` 只表示机器可判定，不扩大修改授权。
- `audit.<序号>` 不是稳定问题分类，用 `detail` / `location` 或结构化 `kind` 定位。`nextCommands` 随发现变化；`toolAvailability` 当前只探测 EPUBCheck。

## 依据返回怎么判断

- DRM/未知加密、不可读 ZIP/container/OPF 按根 AGENTS 停止；可修复的版本、properties、导航诊断可进入对应修复，再验证产物，不要求把所有输入错误先手工清零。
- 路径混淆 → structure normalizer；EPUB2/legacy → migrator；注释 → popup skill；包合并/拆分/换封面 → package operator。不要因需要审计就自动执行写操作。
- 不删除 spine 页面来掩盖错误，不猜 `dc:language`，不批量删除字体 metadata 或未识别资源。CSS 字体断链保留声明与 `local()`，不猜替代字体；非字体断链须修复。
- 检查唯一 nav、spine 顺序、引用 fragment、MathML/SVG properties 和封面声明；Kindle/legacy 交付保留 NCX。目录层次问题按需读 [文集导航](../../docs/how-to/anthology-navigation.md)。
- 修复保持现有 id 与 mixed-content 正文，不能依靠浏览器 HTML 容错。新增/删除资源同步 manifest、spine 和导航的实际依赖；有授权删除时记录精确清单及红线差异。
