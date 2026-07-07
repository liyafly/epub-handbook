---
name: epub-typography-optimizer
description: 优化中文/CJK EPUB 排版，包括正文节奏、font-family 链、嵌入字体、生僻字 fallback、标题字体、中西文混排和阅读器安全段落间距。用于文字拥挤、裁切、跨阅读器字体不一致或字体规则需要清理时。
---

# EPUB 字体与正文节奏优化

这个 skill 处理 CJK 阅读舒适度和字体兼容性。不要把它和弹注转换、图片环绕、A-lite 海报混在一起。
英文为主的 EPUB 使用 `epub-english-typography-optimizer`，不要把英文小说套用中文 2em 缩进和 CJK 字体链。

## 固定目标

默认生产排版应当阅读器安全：

- 正文字体使用短的跨平台系统字体链。
- 默认 `font-family` 链最多 4 段：Apple、Windows、Android/开源 CJK、generic。
- 稳定的全书结构角色可直接绑定字体；混合角色和局部例外使用角色类。
- 生僻字子集使用 `.rare` 等专用类。
- 设计字体使用 `.title-tszt`、`.signature-tszt` 等专用类。
- `ibooks:specified-fonts` 仅当正文锁定（直接 `body` 规则）时写入 OPF；自由模式（默认）不加。局部角色字体不需要此 meta。既有书的 `body-font-locked` 可兼容保留，不作为新模板入口。

## 字体链模式

正文分两种模式，由 body 是否带 `font-family` 区分：

**自由模式（默认，base.css 已采用）：** body 与普通正文 p 都不设 `font-family`，读者可随意切换字体；标题、注释等局部角色仍可单独绑定。

**锁定模式：** `fonts.css` 直接给 `body` 提供字体链，普通正文 p 通过继承获得该字体，OPF 加 `<meta property="ibooks:specified-fonts">true</meta>`。

```css
/* 自由模式——body 与普通正文 p 都不设 font-family */

/* 锁定模式——fonts.css 全书直接绑定 */
body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

裸 `p { font-family:... }` 不作为全书锁定入口：它遗漏列表、引用、表格等正文容器，也会误伤注释段落。只锁定某类正文段落时，使用 `.bodytext`、`.prose` 等明确角色类。

嵌入字体按角色绑定。全书 `h1/h2` 角色一致时可直接写元素选择器；题签、混合标题和生僻字仍使用显式类：

```css
h1,
h2 {
  font-family: "tszt-title", serif;
}

.rare {
  font-family: "tszt-rare", serif;
}
```

嵌入字体按 SPEC §8 的三种模式选择：

- 模式 A：设计字体角色，稳定结构可直接绑定，局部位置用类；链为嵌入字体 + generic。
- 模式 B：生僻字子集 `.rare`，链为嵌入字体 + generic。
- 模式 C：嵌入 + 系统字体复合链，链最多 5 段，嵌入字体只出现一次。

如果确实存在生僻字且 fontspec 使用 `forceAll` 打包覆盖正文全部实际用字的字体，可以走 C1-body。链仍要短，并在文档或构建元数据中说明策略：

```css
body {
  font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

子集字库不能走 C1-body；子集只允许通过 `.rare` 等显式类包住需要补字的字符。

## 工作流

1. 读取 `fonts.css`、`base.css`、OPF 字体 manifest 和目标 XHTML。
2. 分类字体用途：
   - 默认阅读字体。
   - 标题字体。
   - 题签/设计字体。
   - 生僻字 fallback。
   - monospace/code 字体。
3. 删除同一链里重复的同平台别名。
4. 把 `@font-face`、只含字体声明的稳定角色选择器和字体工具类移入 `fonts.css`。
5. 把正文节奏留在 `base.css`：
   - 段首缩进与段间距。
   - 行高。
   - 标题 margin。
   - blockquote 和列表节奏。
   - 长英文 token 换行保护。
6. 规则位置也有问题时，配合 `epub-css-layering-optimizer`。
7. OPF 只声明实际使用的字体文件。
8. 保持既有书的字体模式：body 与普通正文 p 都无 `font-family` 时视为自由模式，不要替它加直接 body 规则或 `ibooks:specified-fonts`；已锁定的书保持锁定，并检查 body 规则与 OPF meta 成对出现。历史 `body-font-locked` 只兼容保留。

## 正文节奏清单

- body line-height 在阅读器放大字号时仍要舒适。
- 段首缩进只作用于普通正文，不作用于标题、表格、代码、注释或图注。
- 长 URL、标识符、英文文件名需要换行保护。
- 标题需要分页控制，但不能在短章制造空白页。
- 代码块和表格在大字号下仍可读。
- Ruby 行高需要给注音留出空间。

## 禁止事项

- 不把版权字体放进模板或示例。
- 不把多个嵌入字体塞进默认链来解决生僻字。
- 新建字体 alias、文件名与 class 遵循 `docs/final/字体别名命名规范.md`；不另造书名型、品牌型或重复角色名。
- 不把子集字库挂到 `body` / `h*`。
- 有稳定英文 family/PostScript 名时，不依赖中文字体显示名。
- 不删除 generic fallback。
- 没有明确阅读器 bug 时，不在阅读字体上滥用 `!important`。
- 不把正文做成整页图片。

## 验证 fixture

- `Text/01-body.xhtml`：普通正文和强调。
- `Text/07-font-family-order.xhtml`：font-family 顺序。
- `Text/08-long-mixed-flow.xhtml`：裁切、长 token、中西文混排。
- `Text/10-text-effects.xhtml`：Ruby 和文字效果。

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

代理协议示例（注释说明代理动作，不是独立 shell 命令）：

```sh
# 代理调用当前 skill，并将 dry-run JSON 写入 work/dry-run.json

# 人工审查
cat work/dry-run.json | jq

# 用户确认后，由映射 provider 写出新的 EPUB 产物
```

dry-run 输出格式见 [docs/pipeline/cleanup-flow.md](../../docs/pipeline/cleanup-flow.md)。
