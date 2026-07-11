# 排版决策记录

`records/` 保存经过人工确认、值得跨书复用的排版判断。它不是正文摘录库，也不代替 reader-matrix 或 SPEC。

## 两层存储

- 书级：`work/<book>/reports/decisions.json`，随本地清洗工作目录保存，不入 Git。该文件仍使用本工具的逐行 JSON 格式。
- 仓库级：`records/typeset-decisions.jsonl`，只收录 `scope: "global"` 且确认可复用的判断，进入 Git。

把书级判断提升为全局规则前，应重新核对适用场景、阅读器证据和隐私边界。

授权正文校订生成的 `text-review-decisions.json`、静态审阅 HTML 和逐篇 diff 属于本地审校证据，不是本目录的排版决策日志。它们可能包含正文片段，只能放在 `work/<book>/reports/`，不得改名为这里的 `decisions.json`、不得用 `epub_decision_log.py` 导入，也不得提升到仓库级 `records/`。

## Schema

当前 schema 版本为 **1**。每行是一个完整 JSON object；未来新增、删除或改变字段语义时，必须在本文件记录版本与迁移方式。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | string | 文件内唯一，格式 `dec-0001` |
| `date` | string | `YYYY-MM-DD` |
| `source` | string | `manual-review`、`handshake` 或 `refinement` |
| `book` | string | 本地别名，可留空，不要求真实书名 |
| `scene` | string | `image-layout`、`popup-note`、`font-chain`、`chapter-head`、`poster`、`vertical`、`css-layering` 或 `other` |
| `finding` | string | 稳定的问题类型，例如 `lone-image-no-figure` |
| `context` | object | 只允许选择器、class、结构形状、阅读器、版本和 artifact |
| `candidates` | string[] | 人工可选方案 |
| `chosen` | string | 必须来自 `candidates` |
| `rationale` | string | 选择理由，可写完整但不得粘贴正文 |
| `scope` | string | `book` 或 `global` |
| `reusable` | boolean | `scope: global` 时为 `true`，否则为 `false` |

## 隐私红线

**禁止保存正文文本、正文摘录、受版权保护的段落或可识别的私密元数据。** `context` 只接受 `selector`、`classes`、`structure`、`reader`、`readers`、`reader_version`、`artifact`。工具会拒绝 `text=`、`content=` 等未授权字段。

## 使用

追加全局决策：

```sh
uv run python scripts/epub_decision_log.py add \
  --file records/typeset-decisions.jsonl \
  --scene image-layout \
  --finding lone-image-no-figure \
  --context 'selector=div.pic > img' \
  --candidates figure.img-left,figure.img-right,figure-fullwidth \
  --chosen figure.img-right \
  --rationale "图注偏长，右浮动后行长关系更稳" \
  --scope global \
  --source manual-review
```

查询：

```sh
uv run python scripts/epub_decision_log.py list \
  --file records/typeset-decisions.jsonl \
  --scene image-layout --format md

uv run python scripts/epub_decision_log.py match \
  --file records/typeset-decisions.jsonl \
  --scene image-layout \
  --finding lone-image-no-figure \
  --format json
```

写入前工具会校验整个文件；任一损坏行、重复 id、未知枚举或隐私字段都会阻止追加。成功写入使用同目录临时文件和原子替换。
