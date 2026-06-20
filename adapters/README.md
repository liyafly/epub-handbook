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
