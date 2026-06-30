# EPUB 文本内容分析与字体排版建议设计

> 状态：已实施并通过仓库回归验证。
> 范围：只读分析、字体覆盖能力整合、skills/contracts/adapters 接入、文档与测试治理。

## 目标

新增可重复、可测试的文本内容分析能力，同时回答：

1. 这段文字在书中是什么结构角色；
2. 该角色适合使用什么字体角色与可重排排版策略。

分析器不修改正文。无法可靠判断的文本必须输出 `unknown` 或 `review_required=true`，不得用低置信度规则自动写回 EPUB。

## 架构

采用“确定性脚本 + AI skill”分层：

- 脚本负责解析输入、提取文本块、计算结构和文本特征、生成候选角色与排版建议。
- skill 读取机器报告和原始上下文，处理中文语义歧义，并把结果分派给 typography、literary structure、notes 等现有能力。
- 写入变换继续由已有专项能力负责；分析器本身始终只读。

## 输入与输出

第一版支持 EPUB、XHTML/HTML、Markdown 和纯文本。PDF、扫描件与 OCR 结果继续先走 `epub-source-intake`。

每个文本块输出：

- 来源文件、稳定 locator、标签、class、语言和相邻块关系；
- 长度、CJK/Latin/数字/标点比例及引号、破折号、章节词等特征；
- 一个或多个候选角色、置信度和证据；
- 字体角色、缩进、对齐、段距、行高与分页建议；
- 是否需要人工复核。

默认报告不保存完整正文。可选本地片段只允许写入 `work/<book>/reports/`，不得进入仓库级决策记录。

## 角色集合

首版覆盖：正文、标题、副标题、对话、引文、题记、诗歌、书信、列表、图注、注释、代码、文言、白话或译文、场景分隔和未知角色。

判断优先级：显式语义和 XHTML 结构 > 稳定 class/epub:type > 相邻块关系 > 内容启发式。内容启发式不得单独产生可自动写回的高置信结论。

## 字体与排版建议

建议使用角色别名而不是具体商业字体名：`inherit`、`st`、`kt`、`fs`、`ht`、`en`、`mono`、`tszt-*`。普通正文默认继承阅读器字体；只有全书明确选择锁定模式时才建议 `body-font-locked`。

嵌入字体、缺字和 Kindle 回退风险由独立的字体覆盖检测器判断。文本角色分析器只说明“需要哪类字体”，不声称字体文件实际覆盖字符。

## 能力接入

- 新增 `epub.text.content.analyze` 只读 capability、CLI、skill 和 provider entrypoint。
- 新增 `epub.font.coverage.analyze` 只读 capability 与薄适配入口；实现仍留在 `tools-font/coverage-detector` 独立 uv 项目。
- refinement harness 只推荐或调用公开入口，不跨目录导入独立项目内部模块。

## 错误处理

- DRM、损坏 EPUB、缺失 OPF/container：立即失败，不猜测。
- XHTML 解析失败：报告文件级错误；其他文件可继续分析，但总状态为 warn。
- 低置信或规则冲突：输出候选列表和复核标志，不静默择一。
- 文本编码不可判定：失败或显式要求用户指定编码，不用替换字符吞错。

## 测试

测试覆盖所有角色、中文歧义、简繁和中英混排、无标点古文、低置信边界、locator 稳定性、隐私输出、CLI/schema/adapter/skill 接入、字体覆盖回归、文档链接和规范一致性。

现有 Python、独立 coverage detector、demo validators、EPUB lint、contracts、skills、AI entrypoints 和 Swift tests 都属于最终回归矩阵。

## 同步修复

实施同时修复已发现的硬规则分叉、失效计划引用、旧字体 alias、reader-matrix 治理问题和本地设计稿状态；新增一致性校验，避免 Markdown、HTML、template 和 skill 再次漂移。
