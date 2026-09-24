# AI Skills 怎么用

AI 先读 [AGENTS.md](../../AGENTS.md)，再从 [技能索引](../../skills/README.md) 选择最窄的 skill；这里是面向使用者的速查，不重复维护能力状态、参数或返回格式。

## 基本流程

1. 已有 EPUB 先做只读预检：

   ```sh
   epub run epub.package.nav.audit --input "book.epub" --json
   ```

2. 不确定问题时用 `epub-audit` 只读审查，再按发现转清洗、包操作或专项排版；非 EPUB 源材料先盘点。
3. 只要建议时不改书；需要“更好看的示例”时，先选择代表性正文/章首/复杂页，在独立候选做样例，不直接全书套模板。
4. 授权修改后，保留原件，审查计划并写新候选，运行红线和专项验证，再做 [人工 diff review](../pipeline/epub-diff-review.md)。
5. 静态检查通过与阅读器实际效果分开验收；浏览器预览或转换成功不等于 Kindle/Apple Books 已通过。

## 当前 skill 一览

| Skill | 用途与边界 |
| --- | --- |
| `epub-audit` | 只读检查包结构、导航、排版、文本角色、图片和字体覆盖 |
| `epub-cleanup` | 按 runbook 规范目录、迁移 package、调整 CSS/CJK 样式并检查标准弹注 |
| `epub-package-ops` | 明确授权后合并、拆分、改元数据、换封面或转 A-lite |
| `epub-source-intake` | 只读盘点源材料；抽取/OCR 另用外部工具 |
| `epub-special-layout` | 英文、文学结构、竖排/Ruby 与多看旧版弹注 fallback |
| `epub-reader-verify` | Kindle 风险、版式 demo、转换器与目标阅读器证据 |

“AI 会按 skill 操作”不等于“该 capability 已自动实现”。实时状态看 `epub capabilities --json`；输入输出与错误解释见 [公共命令与返回](../../skills/README.md#公共命令与返回)。

示例请求：

> 使用 $epub-audit 只读审核这本书，给出最小修改建议。
>
> 使用 $epub-special-layout 为一个章首和一段连续正文制作候选样例，保留正文与既有字体，验证后再讨论全书推广。
