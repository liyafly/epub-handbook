# EPUB 3 补充：列表 / 字体 / HTML 标签速查

> 主手册（`EPUB3_制作完全参考手册.md`）的速查式补充（一）。
> 适用范围与主手册一致：可重排 EPUB 3.3，固定版式 (FXL) 不在内。
> 目标阅读器：Apple Books 4.x、Thorium 3.x、Calibre 7.x、Kindle Previewer 3 / KFX、Kobo、KOReader 2024+。
>
> 系列其他文档：
> - 弹注与 Ruby → `EPUB3_补充_弹注与Ruby.md`
> - 图片与海报页 → `EPUB3_补充_图片与海报页.md`
> - 其他 CSS（夜间模式、首字下沉、竖排等）→ `EPUB3_补充_其他CSS.md`

---

## 目录

1. [CSS 列表清单](#一css-列表清单)
2. [CSS 字体清单](#二css-字体清单)
3. [HTML 标签清单](#三html-标签清单)
4. [中文字体跨系统名称大全](#四中文字体跨系统名称大全)
5. [中英文混排](#五中英文混排标签级独立字体样式)
6. [对主手册的补丁建议（本主题相关）](#六对主手册的补丁建议本主题相关)
7. [阅读器兼容矩阵（节选）](#七阅读器兼容矩阵节选)

---

## 一、CSS 列表清单

### 1.1 `list-style-type` 取值速查

| 值 | 显示 | Apple Books | Thorium | Calibre | KFX | 备注 |
|---|---|---|---|---|---|---|
| `disc` | ● | ✅ | ✅ | ✅ | ✅ | `ul` 默认 |
| `circle` | ○ | ✅ | ✅ | ✅ | ✅ | |
| `square` | ■ | ✅ | ✅ | ✅ | ✅ | |
| `decimal` | 1, 2, 3 | ✅ | ✅ | ✅ | ✅ | `ol` 默认 |
| `decimal-leading-zero` | 01, 02 | ✅ | ✅ | ✅ | ✅ | |
| `lower-roman` | i, ii, iii | ✅ | ✅ | ✅ | ⚠️ 老固件超 10 截断 | |
| `upper-roman` | I, II, III | ✅ | ✅ | ✅ | ⚠️ 同上 | |
| `lower-alpha` | a, b, c | ✅ | ✅ | ✅ | ✅ | |
| `upper-alpha` | A, B, C | ✅ | ✅ | ✅ | ✅ | |
| `cjk-ideographic` | 一, 二, 三 | ✅ | ✅ | ✅ | ⚠️ 回退 `decimal` | KFX 不稳，用计数器替代 |
| `hiragana` / `katakana` | あ / ア | ✅ | ✅ | ✅ | ❌ | 日文 |
| `none` | （无） | ✅ | ✅ | ✅ | ✅ | |

### 1.2 自定义标记 / 计数器（速查）

| 用途 | 推荐写法 |
|---|---|
| 图标项目符号 | `::before { content: "▸" }`（替代 `list-style-image`） |
| 中文一二三 | `counter(c, cjk-ideographic) "、"` 在 `::before` 中 |
| 多级编号 1.1.1 | `counter(l1) "." counter(l2)` |
| 任务清单 | `::before { content: "☐" }`，完成态 `.done::before { content: "☑" }` |
| 嵌套缩进 | 用 `padding-left` 不用 `margin-left`（阅读器对 margin 有覆盖） |

### 1.3 列表 CSS 模块（可直接放进 `main.css`）

```css
/* 基础间距 */
ul, ol { margin: 1em 0; padding-left: 2em; }
li     { margin: 0.3em 0; text-indent: 0; }      /* 列表项强制清缩进 */
li p   { text-indent: 0; }
dl { margin: 1em 0; }
dt { font-weight: bold; margin-top: 0.8em; }
dd { margin: 0.2em 0 0 2em; }

/* 自定义图标列表 */
ul.icon { list-style: none; padding-left: 1.4em; }
ul.icon > li { position: relative; }
ul.icon > li::before {
  content: "▸";
  position: absolute;
  left: -1.1em;
  font-size: 0.85em;
  color: inherit;            /* 跟随夜间模式 */
}

/* 中文一二三编号 */
ol.cn-num { list-style: none; counter-reset: cn; padding-left: 2.4em; }
ol.cn-num > li { position: relative; counter-increment: cn; }
ol.cn-num > li::before {
  content: counter(cn, cjk-ideographic) "、";
  position: absolute;
  left: -2.4em;
  width: 2em;
  text-align: right;
}

/* 任务清单 */
ul.task { list-style: none; padding-left: 1.6em; }
ul.task li { position: relative; }
ul.task li::before { content: "☐"; position: absolute; left: -1.4em; }
ul.task li.done::before { content: "☑"; }
ul.task li.done { color: #888; text-decoration: line-through; }
```

---

## 二、CSS 字体清单

### 2.1 `@font-face` 模板

```css
@font-face {
  font-family: "BookSerif";
  font-style: normal;
  font-weight: 400;
  src: url("../fonts/BookSerif-Regular.ttf") format("truetype");
}
```

每个字重 / 字形单独写一条 `@font-face`（同名 `font-family` + 不同 `font-style` / `font-weight` + 对应字体文件）。

### 2.2 字体格式

跨设备只用 `.otf` / `.ttf`。Kindle KFX 不识别 `.woff` / `.woff2`，压体积靠子集化。

### 2.3 Apple Books 字体启用开关（不写则嵌入字体不生效）

`content.opf` 内：

```xml
<package xmlns:ibooks="http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/">
  <metadata>
    <meta property="ibooks:specified-fonts">true</meta>
  </metadata>
</package>
```

### 2.4 CSS 字体特性可用度

| 属性 | Apple Books | Thorium | Calibre | KFX |
|---|---|---|---|---|
| `font-feature-settings` | ✅ | ✅ | ✅ | ⚠️ 仅基础 |
| `font-variation-settings`（变量字体） | ✅ | ✅ | ✅ | ⚠️ 静态字重回退 |
| `text-emphasis`（着重号） | ✅ | ✅ | ✅ | ✅ |

### 2.5 用户设置如何覆盖嵌入字体

| 阅读器 | 用户选项 | 是否生效 |
|---|---|---|
| Apple Books | "原版字体" | ✅ |
| Apple Books | 其他具体字体 | ❌ |
| Kindle | "出版社字体" | ✅ |
| Kindle | Bookerly 等 | ❌ |
| Thorium | 默认 | ✅，除非勾"Override publisher font" |
| Calibre Viewer | 默认 | 跟随偏好 |

实务：在书前的"排版说明"页提示读者切到"原版/出版社字体"。

---

## 三、HTML 标签清单

### 3.1 列表标签

| 标签 | 用途 | 备注 |
|---|---|---|
| `<ol>` | 有序列表 | 配 `start`、`reversed` |
| `<ul>` | 无序列表 | |
| `<li>` | 列表项 | 可嵌 `<ol>`/`<ul>` |
| `<dl>` | 定义列表 | 术语表、API 参数 |
| `<dt>` | 术语 | |
| `<dd>` | 解释 | 可多个 `<dd>` 对一个 `<dt>` |

### 3.2 注释 / 弹注相关标签

| 标签 | 用途 | 备注 |
|---|---|---|
| `<a epub:type="noteref" role="doc-noteref">` | 注释引用（弹注触发） | 含 `id` 与 `href` |
| `<aside epub:type="footnote" role="doc-footnote">` | 单条注释 | 包整段内容；不要写 `display:none` |
| `<section epub:type="footnotes">` | 注释容器 | 放章末，**与 noteref 同一 xhtml 文件** |
| `<a epub:type="backlink" role="doc-backlink">` | 返回链 | aside 内可选 |
| `<sup>` / `<sub>` | 上下标 | 包 noteref 时需显式 `font-size` |
| `<cite>` | 作品标题 | 比 `<i>` 更精确 |

### 3.3 Ruby 注音标签

| 标签 | 用途 | 备注 |
|---|---|---|
| `<ruby>` | 注音容器 | 包基字 + rt + 可选 rp |
| `<rb>` | 基字（被注的汉字 / 词） | EPUB 3 推荐使用；多字注一词时必需 |
| `<rt>` | 注音文本 | 显示在基字上方（横排）或右方（竖排） |
| `<rp>` | 注音括号（后备） | 不支持 ruby 的阅读器才显示，包住 `(` `)` |

### 3.4 图片 / 媒体标签

| 标签 | 用途 | 备注 |
|---|---|---|
| `<img alt="" src="">` | 图片 | **`alt` 必填**；自闭合 |
| `<figure>` | 图片容器 | 含图 + 说明 |
| `<figcaption>` | 图片说明 | `<figure>` 内首/末元素 |
| `<svg>` | 矢量图 | 可直接嵌入或外链 |
| `<picture>` + `<source>` | 多分辨率响应 | Apple Books / Thorium 支持；KFX 不支持，会取第一个 `<img>` |

### 3.5 列表外观推荐写法

| 想做 | 推荐写法 |
|---|---|
| 图片项目符号 | `::before { content: "▸" }` 或 SVG 字符 |
| 中文一二三 | `counter(c, cjk-ideographic)` 在 `::before` 中 |
| 隐藏标记 | `list-style: none` |

---

## 四、中文字体跨系统名称大全

每款字体在不同系统的注册名不一样。**CSS 必须中英并写**，按"嵌入字体 → macOS → Windows → Android/Linux → Kindle → 中文名兜底 → 通用关键字"顺序排列。

### 4.1 宋体类（衬线，正文）

| 系统 | 主名 | 别名 |
|---|---|---|
| macOS / iOS | `Songti SC` | `STSong`、`STSongti-SC-Regular`、`STSongti-SC-Light`、`宋体-简` |
| macOS / iOS（繁） | `Songti TC` | `STSongti-TC-Regular`、`宋體-繁` |
| Windows | `SimSun` | `宋体`、`NSimSun`、`新宋体`、`SimSun-ExtB` |
| Windows（繁） | `PMingLiU` | `MingLiU`、`MingLiU_HKSCS`、`細明體` |
| Android / HarmonyOS | `Noto Serif CJK SC` | `Source Han Serif SC` |
| Linux | `Noto Serif CJK SC` | `AR PL UMing CN`、`WenQuanYi Bitmap Song` |
| Kindle 内置 | `STSong` | `宋体-简`、繁体作 `Song T` |
| 开源跨平台 | `Source Han Serif SC` | `Noto Serif CJK SC`、`I.MingCP`（古籍繁体） |

### 4.2 黑体类（无衬线，标题）

| 系统 | 主名 | 别名 |
|---|---|---|
| macOS / iOS | `PingFang SC` | `苹方-简`、`PingFangSC-Regular`、`PingFangSC-Medium` |
| macOS / iOS（旧） | `Heiti SC` | `STHeiti`、`STHeitiSC-Light`、`STHeitiSC-Medium`、`黑体-简` |
| Windows | `Microsoft YaHei` | `微软雅黑`、`Microsoft YaHei UI` |
| Windows（旧） | `SimHei` | `黑体` |
| Windows（清晰屏） | `DengXian` | `等线` |
| Windows（繁） | `Microsoft JhengHei` | `微軟正黑體` |
| Android / HarmonyOS | `HarmonyOS Sans SC` | `Noto Sans CJK SC`、`Source Han Sans SC`、`Droid Sans Fallback`、`MIUI Sans` |
| Linux | `Noto Sans CJK SC` | `WenQuanYi Micro Hei`、`AR PL UKai CN` |
| Kindle 内置 | `STHeiti` | `黑体-简` |
| 开源跨平台 | `Source Han Sans SC` | `Noto Sans CJK SC`、`HarmonyOS Sans SC`、`Alibaba PuHuiTi`、`OPPO Sans`、`Misans`、`得意黑`（DingTalk JinBuTi） |

### 4.3 楷体类（引文、题词）

| 系统 | 主名 | 别名 |
|---|---|---|
| macOS / iOS | `Kaiti SC` | `STKaiti`、`STKaiti-SC-Regular`、`STKaiti-SC-Bold`、`楷体-简` |
| macOS / iOS（繁） | `Kaiti TC` | `STKaiti-TC-Regular`、`楷體-繁` |
| Windows | `KaiTi` | `楷体`、`STKaiti`、`楷体_GB2312` |
| Windows（繁） | `DFKai-SB` | `標楷體` |
| Android | （多数无独立楷体） | 回退 `Noto Serif CJK SC` |
| Kindle 内置 | `STKai` | `楷体-简` |
| 开源跨平台 | `LXGW WenKai` | `霞鹜文楷`、`cwTeXKai`、`TW-Kai` |

### 4.4 仿宋类（公文、古籍）

| 系统 | 主名 | 别名 |
|---|---|---|
| macOS / iOS | `STFangsong` | `仿宋-简` |
| Windows | `FangSong` | `仿宋`、`STFangsong`、`仿宋_GB2312` |
| Android | （多数无） | 回退 `Source Han Serif SC` |
| Kindle 内置 | （无） | |
| 开源跨平台 | `FZFangSong-Z02S` | 方正字库公益版 |

### 4.5 圆体类（科普、儿童读物）

| 系统 | 主名 | 别名 |
|---|---|---|
| macOS / iOS | `Yuanti SC` | `STYuan`、`圆体-简` |
| Windows | `YouYuan` | `幼圆` |
| Android | `HarmonyOS Sans SC Rounded` | |
| Kindle 内置 | `STYuan` | |
| 开源跨平台 | `Source Han Sans Rounded SC` | |

### 4.6 西文字体（衬线 / 无衬线 / 等宽）

| 风格 | macOS / iOS | Windows | 开源跨平台 |
|---|---|---|---|
| 衬线 | `Times New Roman`、`Georgia`、`Palatino`、`Hoefler Text` | `Times New Roman`、`Georgia`、`Cambria` | `EB Garamond`、`Source Serif Pro`、`Crimson Pro` |
| 无衬线 | `Helvetica`、`Helvetica Neue`、`San Francisco`、`Avenir` | `Arial`、`Calibri`、`Segoe UI` | `Source Sans Pro`、`Inter`、`Roboto`、`Open Sans` |
| 等宽 | `SF Mono`、`Menlo`、`Monaco` | `Consolas`、`Courier New` | `JetBrains Mono`、`Fira Code`、`Source Code Pro`、`DejaVu Sans Mono` |

### 4.7 跨系统回退链（直接抄）

```css
/* 中文宋体（正文） */
.song, body {
  font-family:
    "BookSongCJK",                                      /* 嵌入字体若有 */
    "Songti SC", "STSong",                              /* macOS/iOS */
    "SimSun", "NSimSun",                                /* Windows */
    "Source Han Serif SC", "Noto Serif CJK SC",         /* Android/Linux/开源 */
    "STSong",                                           /* Kindle */
    "宋体",                                              /* 中文名兜底 */
    serif;
}

/* 中文黑体（标题） */
.hei, h1, h2, h3, h4, h5, h6 {
  font-family:
    "BookSansCJK",
    "PingFang SC", "Heiti SC", "STHeiti",
    "Microsoft YaHei", "SimHei",
    "Source Han Sans SC", "Noto Sans CJK SC", "HarmonyOS Sans SC",
    "STHeiti",
    "黑体",
    sans-serif;
}

/* 楷体 */
.kai {
  font-family:
    "LXGW WenKai",
    "Kaiti SC", "STKaiti", "KaiTi",
    "STKai",
    "楷体",
    serif;
}

/* 仿宋 */
.fang {
  font-family:
    "STFangsong", "FangSong", "FZFangSong-Z02S",
    "仿宋",
    serif;
}

/* 等宽 */
code, pre {
  font-family:
    "SF Mono", "Consolas",
    "JetBrains Mono", "Source Code Pro", "Fira Code",
    "DejaVu Sans Mono", "Menlo", "Monaco",
    monospace;
}
```

**规则**：

- 中文字体名必须用引号；`serif` / `sans-serif` / `monospace` 关键字**不**加引号
- 嵌入字体名（`BookSongCJK` 等）放最前；通用关键字放最后
- Kindle 上把 `STSong` 写在 `"宋体"` 之前，避免误命中 Bookerly

---

## 五、中英文混排：标签级独立字体/样式

### 5.1 两种实现方法

| 方法 | 改 HTML 否 | 适用场景 |
|---|---|---|
| **A. `font-family` 长链**（推荐） | 否 | 默认正文中英混排，按字符 fallback |
| **B. `:lang()` 选择器** | 是（加 `lang="en"`） | 整段语言切换、按语言分样式 |

### 5.2 方法 A：`font-family` 长链（默认推荐）

阅读器对每个字符独立 fallback：第一个字体里若该字符不存在，则用下一个。把**英文字体放前面**，中文字体放后面：

```css
body {
  font-family:
    "EB Garamond",                /* 英文优先 */
    "Songti SC", "STSong",        /* 中文 */
    "SimSun",
    "Source Han Serif SC",
    serif;
}
```

效果：英文字符用 EB Garamond，中文自动 fallback 到宋体。**零改 HTML**，最实用。

### 5.3 方法 B：`:lang()` 选择器（精确控制）

需要在 HTML 上标 `lang`：

```xml
<html lang="zh-CN">
<body>
  <p>这是中文段落，引用 <span lang="en">Pride and Prejudice</span>。</p>
</body>
</html>
```

CSS：

```css
:lang(zh) {
  font-family: "Songti SC", "SimSun", "Source Han Serif SC", serif;
  font-style: normal;                        /* 中文不仿斜体 */
}
:lang(zh) em, :lang(zh) i, :lang(zh) cite {
  font-style: normal;
  text-emphasis: dot;                        /* 改用着重号 */
  -webkit-text-emphasis: dot;
}

:lang(en) {
  font-family: "EB Garamond", "Times New Roman", serif;
  font-style: italic;                        /* 英文可用真斜体 */
}
:lang(en) em, :lang(en) i, :lang(en) cite {
  font-style: italic;
}
```

特点：每段语言独立控制字体、字号、行距、强调样式。**最适合中英混排正式排版**。

### 5.4 同一标签按语言分样式的常见用法

| 元素 | 中文样式 | 英文样式 |
|---|---|---|
| `em` / `i` / `cite` | 着重号（`text-emphasis: dot`） | 真斜体（`font-style: italic`） |
| `strong` / `b` | 思源黑体 Bold | EB Garamond Bold |
| `h1`–`h6` | 思源黑体 + 字距 0.05em | Source Sans Pro + 字距 0 |
| `code` | 等线 / 思源黑体（中文等宽难） | JetBrains Mono |
| 行距 | `line-height: 1.75` | `line-height: 1.55` |

## 六、对主手册的补丁建议（本主题相关）

| 主手册位置 | 改动 |
|---|---|
| §五 CSS 速查表 / 列表与表格 | 在"有序/无序列表"后补一行 `list-style-type` 取值 + 一行 `counter()` |
| §六 基础样式表 / 列表 | 补 `li { text-indent: 0; }` |
| §八 中文排版 / 字体选择 | 用 §四.7 的回退链替换原有简短回退链 |
| §八 中文排版 | 补一节"中英文混排"，引用 §五.1–§五.3 |
| §九 避坑清单 / 图片与字体 | "WOFF2 → OTF/TTF"补理由"Kindle KFX 不识别 woff2"；加一行"WebP / AVIF / HEIC 全拒" |

---

## 七、阅读器兼容矩阵（节选）

| 特性 | Apple Books 4.x | Thorium 3.x | Calibre 7.x | Kindle KFX | KOReader 2024+ |
|---|---|---|---|---|---|
| `list-style-type: cjk-ideographic` | ✅ | ✅ | ✅ | ⚠️ 回退 decimal | ✅ |
| `counter()` 中文编号 | ✅ | ✅ | ✅ | ⚠️ 回退 decimal | ✅ |
| `::before` 自定义标记 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `@font-face` (`.otf` / `.ttf`) | ✅（需 `ibooks:specified-fonts`） | ✅ | ✅ | ✅ | ✅ |
| `@font-face` (`.woff2`) | ⚠️ 新版部分 | ✅ | ✅ | ❌ | ✅ |
| 变量字体 | ✅ | ✅ | ✅ | ⚠️ 静态回退 | ✅ |
| `:lang()` 选择器 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `text-emphasis` 着重号 | ✅ | ✅ | ✅ | ✅ | ✅ |

> ✅ 正常 ⚠️ 部分版本 / 部分场景 ❌ 不支持

---

**文档版本**：2026-05-15
**对应标准**：EPUB 3.3
**关联文档**：`EPUB3_制作完全参考手册.md`（主手册）、《EPub指南——从入门到放弃》（赤霓，2023-04-18）