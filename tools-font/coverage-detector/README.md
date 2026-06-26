# EPUB Font Coverage Detector

独立 Python CLI 工具，给定一本 EPUB：

1. 抽取全书会渲染的文字
2. 解析每段文字真实命中的 `font-family` 链
3. 读嵌入字体 cmap，判定每个字符的覆盖位置
4. 按 reader profile（ideal-browser / kindle-pessimistic）预测结果
5. 产出 JSON 报告 + 人类可读摘要

## 安装

```sh
uv sync
```

## 使用

```sh
uv run python -m src.cli book.epub                       # 打印摘要
uv run python -m src.cli book.epub --output report.json  # 写 JSON + 自包含 HTML 报告
uv run python -m src.cli book.epub --validate-with bigfont.ttf   # 候选大字库验证（残余=造字输入）
uv run python -m src.cli book.epub --standard-table custom.txt   # 可选：叠加自备字表
```

> `pyproject.toml` 当前 `package = false`，console 脚本未安装，固定用 `python -m src.cli` 运行。
> 标准字区（生僻字判定）默认用 stdlib `gb2312`/`gbk` 编解码器生成，**无需任何字表文件**。

## 依赖

- `fontTools` — cmap / IVS / 子集探测
- `tinycss2` — CSS 解析与务实层叠
- `lxml` — XHTML 解析与节点定位

## 设计

详见 `docs/superpowers/specs/2026-06-25-epub-字体覆盖检测器-design.md`
