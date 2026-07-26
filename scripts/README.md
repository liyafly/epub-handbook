# Python 脚本索引

普通用户通常只需要样式 demo 的 `build.sh` 和 `epub_cleanup_pipeline.py`。
其余脚本是专业流水线、AI provider、校验器或内部模块；先读根目录 `AGENTS.md`
再调用。

所有公开 Python CLI 都可以用 `python3 scripts/<name>.py --help` 查看参数。已有
EPUB 必须从复制件开始，正文红线和人工 diff review 不能省略。

## 普通人入口

| 脚本 | 用途 |
| --- | --- |
| `templates/epub-style-demo/build.sh` | 构建本仓样式演示 EPUB |
| `epub_cleanup_pipeline.py` | 一键建立 before、运行保守清洗基线并输出审计报告 |

新书构建入口不在本目录：复制 `templates/book-starter/` 后运行其 `build.sh`。

## 已有 EPUB：检查、迁移与清洗

| 脚本 | 用途 |
| --- | --- |
| `epub_lint.py` | 按 SPEC 检查任意 EPUB 的通用结构与样式错误 |
| `epub_preflight_harness.py` | 修改前检查 DRM、加密、包结构和可处理性 |
| `epub_structure_tool.py` | dry-run / 应用资源目录格式化与 manifest-id 文件名反混淆 |
| `epub3_migration_harness.py` | 规划或写出保守 EPUB 3 迁移 |
| `epub3_migration_apply_harness.py` | 向新产物应用迁移并输出 SHA 与 JSON 审计 |
| `epub3_oneclick_converter.py` | 兼容旧调用方的 EPUB 3 迁移 CLI façade |
| `epub_refinement_harness.py` | 只读生成弹注、字体、图片和排版建议 |
| `epub_cleanup_loop.py` | 在白名单 action 与逐轮正文 gate 下自动收敛清洗 |
| `epub_ai_harness.py` | 兼容旧调用方的 EPUB / source 分析与 skill 路由 façade |
| `validate_text_invariance.py` | 比较 before / after 的正文、metadata、spine 等红线 |

## 内容、字体、图片与样式

| 脚本 | 用途 |
| --- | --- |
| `epub_content_analyzer.py` | 面向 CLI 的文本结构角色与排版策略分析 |
| `epub_content_analysis.py` | 上述分析器使用的只读核心模块 |
| `epub_font_coverage_adapter.py` | 调用独立字体项目，输出 cmap、缺字和 reader profile 风险 |
| `epub_image_layout_advisor.py` | 只读生成逐图布局候选和决策记录模板 |
| `epub_css_cleanup.py` | 合并重复 CSS、收敛字体链和局部样式 |
| `epub_anthology_refinement.py` | 精排合订书卷封及相邻版权页 |
| `epub_style_preset_tool.py` | 列出、预览或应用中文书型 CSS 预设 |
| `epub_decision_log.py` | 记录、查询可复用的脱敏排版决策 |

## 封面、元数据、合并与拆分

| 脚本 | 用途 |
| --- | --- |
| `epub_cover_replace_harness.py` | 在新产物中替换封面并输出 JSON 报告 |
| `epub_metadata_edit_harness.py` | 在新产物中修改允许的 metadata 字段 |
| `epub_package_merge_harness.py` | 合并多本 EPUB 并输出审计报告 |
| `epub_package_split_harness.py` | 按明确 TOC 索引拆分 EPUB |
| `epub_package_tool.py` | 兼容旧调用方的 package 操作 CLI façade |

## Provider、适配与内部支撑

| 脚本 / 目录 | 用途 |
| --- | --- |
| `epub_handbook_cli.py` | Python CLI / AI agent provider 的稳定 JSON 总入口 |
| `python_provider_adapter.py` | 从文件 JSON 请求调用 allow-list Python provider |
| `render_adapter_catalog.py` | 从中立 capability manifest 生成 agent / product catalog |
| `epub_lib.py` | 多个脚本共用的标准库 EPUB package helper |
| `epub_text_gate.py` | cleanup loop 使用的可编程正文红线封装 |
| `epub_xhtml_transforms.py` | 最小 diff、确定性的 XHTML 字符串转换 |
| `build_demo_epubs.py` | 构建本仓自造清洗 / diff 演示 EPUB |
| `epub3_conversion/` | EPUB 3 转换的分层核心实现 |
| `epub_ai/` | EPUB / source 建模、detector、报告与路由实现 |
| `epub_package/` | 封面、metadata、合并、拆分与引用重写实现 |

## 仓库校验

| 脚本 | 用途 |
| --- | --- |
| `validate_ai_entrypoints.py` | 确认 `AGENTS.md` 是唯一 AI 规则源并检查入口引用 |
| `validate_skills_basic.py` | 检查 skill frontmatter、metadata 与基础契约 |
| `validate_docs_consistency.py` | 检查活跃文档链接、字体规则、矩阵和跨层一致性 |
| `validate_contracts.py` | 检查版本化 capability manifest |
| `validate_python_entrypoint_inventory.py` | 检查公开 Python / AI 入口清单 |
| `validate_epub_style_demo.py` | 用标准库校验样式 demo fixture |
| `validate_popup_notes.py` | 校验弹注结构与 EPUB 产物中的弹注约束 |
| `validate-epub-style-demo.sh` | 样式 demo 校验的 shell 入口 |
| `validate-popup-notes.sh` | 弹注校验的 shell 入口 |
| `install-hooks.sh` | 可选安装仓库自带 git hook 模板 |

## 测试

`scripts/test_*.py` 是对应入口和内部模块的回归测试，`scripts/test_support/` 提供
共享 fixture。改 Python 时先跑相关单文件，结束时跑全套：

```sh
python3 scripts/test_epub_cleanup_pipeline.py
for test_file in scripts/test_*.py; do
  python3 "$test_file"
done
```

Swift / Python capability parity 由 `test_swift_python_parity.py` 覆盖；入口目录清单由
`test_validate_python_entrypoint_inventory.py` 覆盖。
