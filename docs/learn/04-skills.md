# AI Skills 怎么用

AI 先读 [AGENTS.md](../../AGENTS.md)，再从 [技能索引](../../skills/README.md) 选择最窄的 skill；这里是面向使用者的速查，不重复维护能力状态、参数或返回格式。

## 基本流程

1. 已有 EPUB 先做只读预检：

   ```sh
   epub run epub.package.nav.audit --input "book.epub" --json
   ```

2. 不确定排版问题时用 layout 审稿，已知问题直接进入专项 skill；非 EPUB 源材料先盘点。
3. 只要建议时不改书；需要“更好看的示例”时，先选择代表性正文/章首/复杂页，在独立候选做样例，不直接全书套模板。
4. 授权修改后，保留原件，审查计划并写新候选，运行红线和专项验证，再做 [人工 diff review](../pipeline/epub-diff-review.md)。
5. 静态检查通过与阅读器实际效果分开验收；浏览器预览或转换成功不等于 Kindle/Apple Books 已通过。

## 当前 skill 一览

| Skill | 用途与边界 |
| --- | --- |
| `epub-package-nav-auditor` | 只读预检 OPF、manifest/spine、nav/NCX、资源 |
| `epub-layout-auditor` | 只读总审稿、候选比较与风险分派 |
| `epub-source-intake` | 只读盘点源材料；抽取/OCR 另用外部工具 |
| `epub-content-analyzer` | 只读分析文本角色与歧义 |
| `epub-structure-normalizer` | Go CLI 双阶段目录归类与文件名反混淆 |
| `epub3-migrator` | 审查后迁移旧 EPUB 的 package/nav 与已识别注释 |
| `epub-css-layering-optimizer` | 保守 CSS 清理；语义归层需人工判断 |
| `epub-typography-optimizer` | 中文字体策略与正文节奏，审查后应用 preset |
| `epub-font-coverage-analyzer` | 只读检查缺字和回退，需外部字体 provider |
| `epub-image-layout-optimizer` | 只读图片/图注/环绕候选，授权后人工修复 |
| `epub-alite-converter` | 既有封面式页面转可重排 A-lite，不重新设计全书 |
| `epub-popup-footnote-converter` | 标准 grouped notes；校验器本身不转换 |
| `epub-package-operator` | 明确授权的合并、拆分、元数据或封面操作 |
| `epub-style-demo-maintainer` | 制作样例、静态验证与实测证据闭环 |
| `epub-english-typography-optimizer` | 人工英文排版，当前无自动 runner |
| `epub-literary-structure-formatter` | 人工精排章首、诗信和文白结构 |
| `epub-vertical-ruby-optimizer` | 人工竖排正文与 Ruby 优化 |
| `epub-kindle-compatibility-checker` | 人工静态审核、转换日志与阅读器证据 |
| `epub-legacy-footnote-fallback` | 有明确旧多看需求时人工叠加 fallback |

“AI 会按 skill 操作”不等于“该 capability 已自动实现”。实时状态看 `epub capabilities --json`；输入输出与错误解释见 [公共命令与返回](../../skills/README.md#公共命令与返回)。

示例请求：

> 使用 $epub-layout-auditor 只读审核这本书，给出最小修改建议。
>
> 使用 $epub-literary-structure-formatter 为一个章首和一段连续正文制作候选样例，保留正文与既有字体，验证后再讨论全书推广。
