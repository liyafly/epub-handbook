# Agent 与产品适配层

这里不保存 capability 的身份或 EPUB 规则；两者只存在于 `../contracts/` 以及既有 `docs/final/` / demo evidence。

现有 OpenAI skill、Claude Code skill token 和 Python CLI 继续保留原位置。新的 adapter catalog 由中立 manifest 生成：

```sh
python3 scripts/render_adapter_catalog.py --adapter openai
python3 scripts/render_adapter_catalog.py --adapter claude
python3 scripts/render_adapter_catalog.py --adapter mcp
python3 scripts/render_adapter_catalog.py --adapter cli
python3 scripts/render_adapter_catalog.py --adapter gui
```

生成结果是给 adapter 接入方消费的 JSON 投影，不反向取代 `SKILL.md`、`agents/openai.yaml` 或 `scripts/*.py` 的现有兼容入口。每个投影只使用 capability ID、schema、redline、依赖和 legacy skill slug，绝不携带 Python 脚本路径或 UI 文案。

`python/public-entrypoints.v1.json` 是所有 Python CLI / Agent 公开入口的 inventory；`python/provider-catalog.v1.json` 与 `scripts/python_provider_adapter.py` 是唯一允许执行 Python entrypoint 的 adapter 实现。它们只给 CLI / AI Agent 用，通过 request/result JSON 运行 allow-listed provider。Swift package、macOS GUI 与未来 iOS target 不得调用它。

## JSON CLI 表面

Python 的统一入口保持在 Python adapter 层；它不会调用 Swift，也不会被 GUI 调用：

```sh
python3 scripts/epub_handbook_cli.py catalog --format json
python3 scripts/epub_handbook_cli.py run epub.package.nav.audit \
  --input /absolute/path/book.epub \
  --result /absolute/path/result.json --format json

python3 scripts/epub_handbook_cli.py run epub.text.content.analyze \
  --input /absolute/path/book.epub \
  --result /absolute/path/content-analysis.json --format json

python3 scripts/epub_handbook_cli.py run epub.font.coverage.analyze \
  --input /absolute/path/book.epub \
  --result /absolute/path/font-coverage.json --format json
```

`catalog` 投影全部 manifest capability；`run` 只执行
`python/provider-catalog.v1.json` 中已 allow-list 的 Python provider。未接管的
capability 会返回 JSON 失败对象和非零退出码，不能根据 catalog 猜测脚本路径。

合并、拆分、元数据、封面和 EPUB3 迁移是写操作，需要多个输入或额外参数，不能安全映射到通用 provider 的单 artifact request。它们登记在 `python/public-entrypoints.v1.json`，由以下具体 harness 执行，不加入 `provider-catalog.v1.json`：

- `scripts/epub_package_merge_harness.py`
- `scripts/epub_package_split_harness.py`
- `scripts/epub_metadata_edit_harness.py`
- `scripts/epub_cover_replace_harness.py`
- `scripts/epub3_migration_apply_harness.py`

这五个入口都要求显式新输出位置；是否登记 capability 与是否可由通用 provider 执行是两件独立的事。

Swift 的独立原生 CLI 是 `swift run epub-handbook-swift`，仅暴露已完成双跑的
`inspect`、`validate-redlines` 与 `epub.notes.popup.normalize`。它不调用上面的
Python adapter；macOS/iOS GUI 也不调用任一 CLI，而是直接使用 Swift package。
