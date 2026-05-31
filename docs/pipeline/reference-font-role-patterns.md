# 文学 EPUB 字体角色参考

> 状态：脱敏分析；来源是本地参考 EPUB，仅记录结构模式，不提交原书、字体文件、真实书名、作者或私有 metadata。
>
> 用途：为已有 EPUB 清洗和文学书排版提供“按文本角色管理字体”的最小模型。对外字体硬约束仍以 [SPEC-实现约束.md §3、§8](../final/SPEC-实现约束.md) 为准。

## 1. 样本概况

本地样本是 EPUB2 包：

- OPF `version="2.0"`；
- 正文使用 XHTML；
- 包内有 6 个 `.ttf` 文件；
- 字体声明集中在一个 CSS；
- 文学排版规则集中在另一个 CSS；
- `body` 的宋体声明被注释掉，没有强制整本正文使用嵌入字体。

值得吸收的是最后一点：正文让阅读器保持稳定基线，只对有明确语义的文本角色施加字体差异。

## 2. 样本里的角色分工

| 文本角色 | 样本做法 | 清洗后推荐角色类 | 推荐方向 |
| --- | --- | --- | --- |
| 普通正文 | 不强制挂嵌入字体 | `.type-body` | 系统宋体链 |
| 一级标题 | 专用标题宋 + 标宋 / 黑体 fallback | `.type-title` | 系统黑体链；设计字体只在授权明确时局部嵌入 |
| 小标题 | 标宋 / 黑体 fallback | `.type-subtitle` | 系统黑体链 |
| 引文、序言 | 仿宋 / 楷体链 | `.type-quote` | 系统仿宋链 |
| 括注、脚注 | 楷体链 | `.type-note` | 系统楷体链 |
| 局部强调 | 仿宋 / 楷体链，字号略放大 | `.type-emphasis` | 系统楷体链；颜色和字号只做辅助 |
| 版权页、出版信息 | 仿宋 / 楷体链，字号较小 | `.type-meta` | 系统仿宋链 |
| 卷标 | 黑体链 | `.type-subtitle` | 系统黑体链 |

角色差异来自“这段文字是什么”，不是“这个平台可能装了哪些字体”。先标语义角色，再让每个角色使用短 fallback 链。

## 3. 样本中不要照抄的部分

### 3.1 CSS 资源断链

样本 CSS 声明了 8 个 `url(...)` 字体目标，但包内只有 6 个实际字体文件。其中两个 URL 不存在：

```text
../Fonts/st.ttf
../Fonts/h3.ttf
```

这说明“能在某个阅读器里显示”不等于字体资源正确。系统 `local(...)` fallback 可能掩盖断链。

清洗时必须运行：

```sh
python3 scripts/epub_preflight_harness.py input.epub --format json
```

如果 CSS 引用的字体文件不存在，不要猜测缺失字体内容，也不要把别的字体文件改名顶替。保留报告并人工判断：删除失效 URL、补授权字体，或只保留系统 fallback。

### 3.2 `local(...)` 链过长

样本为了覆盖多平台，在一个 `@font-face` 里堆叠很多本地字体别名。这样做的问题是：

- 实际命中字体难以预测；
- 不同平台字形差异变大；
- 调试时很难知道读者看到的是嵌入字体还是系统字体；
- 私有字体名、旧字体名和平台字体名混在一起。

本仓默认链保持短小：

```css
.type-body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

.type-title {
  font-family: "Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
}

.type-note {
  font-family: "Kaiti SC", "STKaiti", "KaiTi", serif;
}
```

如果使用嵌入字体，按 SPEC 的模式 A / B / C 管理，不把多个嵌入字体反复塞进同一条链。

### 3.3 字体授权不能从文件存在推断

样本包里有 `.ttf`，不代表它们可以复制到新书、demo 或仓库。分析只借鉴角色分工。需要嵌入字体时，单独核对授权和 OPF manifest。

## 4. 一键转换器提供的角色类

`scripts/epub3_oneclick_converter.py` 注入 `Styles/epub3-enhancements.css`，其中包含：

```text
type-body
type-title
type-subtitle
type-quote
type-note
type-emphasis
type-meta
```

转换器只提供角色 palette，不自动猜测语义。例如它不会把任意 `<b>` 改成 `.type-emphasis`，也不会把任意 `.cp` 认定为版权信息。清洗者应在人工 diff review 可见的前提下逐类分派。

建议顺序：

1. 先运行一键流水线，拿到 `reports/refinement.json`。
2. 列出书内已有 class 与 element 用法。
3. 将标题、引文、注释、强调、metadata 映射到角色类。
4. 每次只改一类，跑文本红线 gate。
5. 在 Apple Books、Kindle Previewer 和目标阅读器检查默认字号与大字号。

## 5. 角色映射记录模板

```md
| 原 class / element | 新角色 | 是否改 XHTML class | 字体链 | 理由 | 阅读器复测 |
| --- | --- | --- | --- | --- | --- |
| h1 | type-title | no | 黑体系统链 | 章节标题 | pending |
| .quote | type-quote | yes | 仿宋系统链 | 引文 | pending |
| .footnote | type-note | yes | 楷体系统链 | 注释 | pending |
```

这个表放进单书工作目录的 `notes.md`，不放真实书稿到仓库。
