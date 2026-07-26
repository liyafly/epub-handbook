# EPUB 3 终极实践手册

> 版本：2026-05-23
> 定位：把现有主手册、补充篇、`wang-chapterpage-demo-v2.epub` 实测结果收敛为一套最终推荐方案。  
> 原则：只写推荐路径，围绕 A-lite、标准弹注、标准 CSS 和授权嵌入字体。

---

## 一、最终方案总览

| 模块 | 最终方案 |
|---|---|
| 正文 | EPUB 3.3 可重排；自由模式由阅读器选字，锁定模式可在 `fonts.css` 直接给稳定的 body / h* 角色绑定系统链或嵌入字体（详见 §四） |
| 整页海报 / 卷首 / 章节扉页 | A-lite：可重排整页、无 FXL、无 `vh/vw`、无绝对定位 |
| 标题 / 题签 / 特殊排版 | 仅"必须特定字体"的题签 / 卷头题字嵌入（模式 A，链 ≤ 2 段）；其他标题默认走系统黑体链 |
| Apple Books 字体 | 正文字体锁定时加 OPF `ibooks:specified-fonts=true` + 测试“原版字体”；局部角色字体不受此 meta 影响 |
| Kindle 字体 | 嵌入 `.ttf` / `.otf`，主字体放 `body`，测试 Publisher Font 开关 |
| 弹出注释 | 图片图标触发，单个 `aside epub:type="footnote"` 内用 `ol/li` 聚合本文件注释，`◎` 返回；需兼容多看旧版时在同一结构上叠加 `duokan-*` fallback |
| 波浪线 | `text-decoration: underline` 兜底 + `text-decoration-style: wavy` 渐进增强；Kindle 退化为普通下划线 |
| 着重号 | 标准 `text-emphasis: filled dot` |
| Ruby 注音 | 标准 `ruby + rt`，段落加行距兜底 |
| 英文小说正文 | `lang="en"` + 短 serif 链，首段无缩进、后续段落缩进，插图居中 figure，避免固定页高 |
| 章节头图 | 普通可重排章首使用 `figure.chapter-head-art` 小章标或 `figure.chapter-head-banner` 满栏横幅 + 真实 `h1`；强视觉首屏才走 A-lite |

---

## 二、推荐目录结构

```text
book.epub
├── mimetype
├── META-INF/
│   └── container.xml
└── OEBPS/
    ├── content.opf
    ├── toc.ncx
    ├── Text/
    │   ├── nav.xhtml
    │   ├── cover.xhtml
    │   ├── chapter-poster.xhtml
    │   └── ch01.xhtml
    ├── Styles/
    │   ├── fonts.css
    │   ├── base.css
    │   └── poster.css
    ├── Images/
    │   ├── cover.jpg
    │   ├── note.png
    │   └── poster-bg.png
    └── Fonts/
        ├── st.ttf
        ├── kt.ttf
        └── tszt-rare.ttf
```

`fonts.css` 管字体，`base.css` 管正文组件，`poster.css` 管 A-lite 海报页。

> 上面 `Fonts/` 目录与三个示例字体文件**仅在嵌入字体场景下需要**：
> - 默认路径（不嵌字体）：删掉 `Fonts/` 目录与 OPF 字体 item；`fonts.css` 内所有 `@font-face` 保持注释。
> - 角色字体嵌入：按需保留对应字体文件；稳定结构角色可直接绑定，局部角色使用类。
> - 模式 C1-body（设计锁定或生僻字 + 正文全角色覆盖）：保留一份覆盖正文角色全部实际用字的 `Fonts/st-all.ttf`。
>
> 字体 alias、文件名与 class 按 [字体别名命名规范](字体别名命名规范.md) 使用角色缩写；字体内部正式名称和授权信息不因 alias 而改变。

---

## 三、OPF 模板

```xml
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf"
         version="3.0"
         unique-identifier="bookid"
         xml:lang="zh-CN"
         prefix="rendition: http://www.idpf.org/vocab/rendition/#
                 ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx</dc:identifier>
    <dc:title>书名</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-05-18T00:00:00Z</meta>
    <meta property="rendition:layout">reflowable</meta>
    <meta property="rendition:orientation">auto</meta>
    <meta property="rendition:spread">auto</meta>
    <!-- 仅锁定模式时添加：<meta property="ibooks:specified-fonts">true</meta> -->
    <meta name="cover" content="cover-img"/>
  </metadata>

  <manifest>
    <item id="nav" href="Text/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>

    <item id="css-fonts" href="Styles/fonts.css" media-type="text/css"/>
    <item id="css-base" href="Styles/base.css" media-type="text/css"/>
    <item id="css-poster" href="Styles/poster.css" media-type="text/css"/>

    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="poster-01" href="Text/chapter-poster.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch01" href="Text/ch01.xhtml" media-type="application/xhtml+xml"/>

    <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="note-icon" href="Images/note.png" media-type="image/png"/>
    <item id="poster-bg" href="Images/poster-bg.png" media-type="image/png"/>

    <item id="font-st" href="Fonts/st.ttf" media-type="font/ttf"/>
    <item id="font-kt" href="Fonts/kt.ttf" media-type="font/ttf"/>
    <item id="font-tszt-rare" href="Fonts/tszt-rare.ttf" media-type="font/ttf"/>
  </manifest>

  <spine toc="ncx" page-progression-direction="ltr">
    <itemref idref="cover-page"/>
    <itemref idref="nav"/>
    <itemref idref="poster-01"/>
    <itemref idref="ch01"/>
  </spine>
</package>
```

规则：

- 全书默认 `reflowable`。
- A-lite 海报页仍是普通 spine item。
- 字体文件、注释图标、背景图都进入 `manifest`。
- `ibooks:specified-fonts=true` 仅当正文字体锁定时添加；自由模式（默认）不加。新建锁定版在 `fonts.css` 直接绑定 `body`，普通正文 `p` 继承该字体；局部角色字体不单独触发此 meta。当前是仓库包结构 policy，阅读器行为仍按 `reader-matrix.yaml` 的 `07-font-family-order` 待复测。详见 SPEC §8。

> OPF manifest 中的 `Fonts/*` item **仅在嵌入字体场景下保留**。
> `ibooks:specified-fonts=true` 仅当正文字体锁定时添加：
> - 自由模式（默认，body 与普通正文 p 都不设 font-family）：**不加**。
> - 锁定模式（直接 `body` 规则）：**添加**；普通正文 p 直接继承。
> - 标题、题签、生僻字等局部角色使用嵌入字体不单独触发此 meta。上述三项是仓库 policy，不等同于已完成 Apple Books 实测；版本化证据待 `reader-matrix.yaml` 的 `07-font-family-order` 复测补齐。

---

## 四、字体方案

### 4.1 `fonts.css`

以下 `@font-face` 声明**仅在嵌入字体场景下取消注释并启用**；默认路径不需要这一节，整段保持注释（与 `templates/epub-style-demo/OEBPS/Styles/fonts.css` §一 一致）。

> 此节的三个示例字体名都是占位符，实际工程换成授权字体名即可。

```css
@charset "utf-8";

@font-face {
  font-family: "st";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/st.ttf") format("truetype");
}

@font-face {
  font-family: "kt";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/kt.ttf") format("truetype");
}

@font-face {
  font-family: "tszt-rare";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/tszt-rare.ttf") format("truetype");
}
```

### 4.2 正文字体（正文自由 / 正文锁定）

这里的“自由”只描述普通正文是否继承全书 `body` 字体，不等于 EPUB 里不能有标题、序言或注释等局部角色字体。正文分自由 / 锁定两种模式（规则见 SPEC §8）：

```css
/* 自由模式（默认）：body 与普通正文 p 都不设 font-family */

/* 锁定模式：全书直接绑定 body，并同步 OPF 加 ibooks:specified-fonts=true */
body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

普通正文 `p` 会继承 `body`，不需要重复声明。不要用裸 `p { font-family:... }` 代替全书入口：列表、引用和表格不会被覆盖，注释段落却可能被误锁。只锁定某类正文段落时，使用 `.bodytext`、`.prose` 等角色类。既有书的 `.body-font-locked` 可兼容保留，但新模板不再采用。字体绑定放在 `fonts.css`，正文盒模型仍放在 `base.css`。

> 反例：不要把链写成同平台别名堆叠（如追加 `STSongti-*` / `NSimSun` / `宋体`），违反 SPEC §8。

锁定模式可使用跨平台系统中文字体链（Apple `Songti SC` + Windows `SimSun` + Android / 跨平台开源 `Noto Serif CJK SC` + `serif`）。这些名称表达预期 fallback 顺序，实际命中仍取决于阅读器、操作系统和版本，交付前按 reader-matrix 记录实测。

设计上需要固定正文外观，或全书含生僻字时，只要嵌入字体覆盖正文角色全部实际用字，就可按模式 C1-body 放在直接 `body` 锁定链链首：`body { font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`。局部补字子集仍只能使用 `.rare`。`fontspec` 同步切到 `forceAll`，OPF manifest 挂对应字体 item。

### 4.3 特殊标题字体

```css
h1,
h2 {
  font-family: "kt", serif;
}
```

当全书 `h1` / `h2` 各自只有一个字体角色时，直接绑定最清楚。若同一标题层级混有题签、卷首页和普通标题，则改用 `.poster-title`、`.inscription`、`.title-kai` 等角色类。嵌入设计字体只写书内字体名 + 通用族兜底，避免系统字体提前替换设计字形。

> 上述写法属于模式 A（链 ≤ 2 段，仅嵌入字体 + generic）。如果项目未嵌入 `kt`，把这条规则改为系统楷体链 `.title-kai { font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif; }`（与 `fonts.css` 的 `.font-kt` 同源），不要保留死链。

### 4.4 生僻字

```css
.rare {
  font-family: "tszt-rare", serif;
}
```

> 旧写法 `"tszt-rare", "st", serif` 是反例——生僻字字体后面挂正文嵌入宋体，缺字时落到系统宋体的豆腐。三种推荐写法（按需求选一）：(模式 B 纯生僻字) `.rare { font-family: "tszt-rare", serif; }`；(模式 C1 设计前置) `.font-st-design { font-family: "st-design", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`；(模式 C2 嵌入兜底) `.font-st-tszt { font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", "tszt-rare", serif; }`。


### 正文全角色覆盖方案（模式 C1-body）

当设计上需要固定正文外观，或正文存在生僻字，且正文角色字体覆盖全部实际用字时，允许把该嵌入字体直接挂在 `body`；标题角色同理可直接挂在统一层级的 `h*`。

```css
body {
  font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

h1, h2, h3, h4, h5, h6 {
  font-family: "ht-all", "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
}
```

要点：

- 嵌入字体必须覆盖该角色经过 CSS 继承与局部角色覆盖后真正承担的全部文字和标点；可以是完整字体，也可以是按该角色全部文本生成的子集，但不能是只含少数字符的补字子集；
- OPF manifest 声明该字体 item，`fontspec` 切到 `forceAll`；
- 不允许把只含少数字符的局部补字子集（如 `tszt-rare`）走本路径——它挂到 `body` 会在未收录字符处落豆腐；按正文角色全部实际字符生成并经 `cmap` 复核的完整角色子集可以走本路径；
- 体积说明：全字符集 CJK 字体单 weight 约 8–15 MB；启用前评估包体增长是否可接受；
- 对已由该正文角色完整覆盖的文字不再叠加 `.rare`；其他独立角色若仍缺字，继续按自己的角色清单处理。

生僻字字体只放子集。

### 4.5 自由版与锁定版双版本

同时交付两版时，先把正文、注释、图片、目录和结构定稿为一个内容基线，再从这个基线派生字体变体，不分别维护两份正文：

- 正文自由版：`body` 与普通正文 `p` 不声明 `font-family`；标题、序言、注释等局部角色仍可按需使用系统链或嵌入字体。
- 正文锁定版：在自由版内容基线上增加正文角色字体、直接 `body` 规则和对应 OPF 字体 item；按默认规则同步添加 `ibooks:specified-fonts=true`。
- 两版允许差异只限 `fonts.css`、字体文件、OPF 中与字体有关的 manifest/meta（含 `ibooks:specified-fonts`），以及该 meta 所需的 `<package prefix>` 中 `ibooks:` 声明。每个 rendition 的唯一 `dcterms:modified` 可按各自实际打包时间不同；比较时只忽略其值，不忽略缺失、多份或格式错误。同一批成对构建优先共享一次取值的 `BUILD_TIMESTAMP` / `SOURCE_DATE_EPOCH`，减少无意义 diff。XHTML、核心 `dc:*` metadata、spine、注释、图片和其他资源应保持一致。
- 为以后锁定字体准备的 CSS 模板放在工作目录，不打包进正文自由版；交付包不得保留指向缺失字体的 `@font-face`、CSS URL 或 OPF item。
- 子集化后保持 `st` / `kt` / `fs` 等角色 alias 与包内路径稳定，只替换字体文件字节，并重新跑字体覆盖、preflight、EPUB lint 和两版正文一致性检查。

既有书若按用户明确要求在正文自由版保留 `ibooks:specified-fonts=true`，这属于书级历史例外：在本地报告记录原因，并用 `epub_lint.py --allow-free-body-ibooks-meta` 显式校验，不把该做法写回新书模板。

### 4.6 校对期完整母版与发行子集

校对会新增或替换字符时，不必为此维护一个内嵌 CJK 全集的校对 EPUB。推荐只维护一个内容版本：

1. 在 EPUB 包外保留授权清晰、可子集化的完整 TTF/OTF 母版及许可证；
2. 构建时汇总全部 XHTML（包括 `nav.xhtml`）、正文、标题、表格、图片 `alt` / `title` / `aria-label` 和 CSS 生成文字；
3. 正文锁定字体与统一标题字体按“当前全书字符集”生成角色子集，避免文字在角色间移动后缺字；
4. 数学字体若依赖 OpenType MATH 表、伸缩构件或组合字形，优先保留完整数学字体；
5. 每轮校对后重建子集并检查 `cmap`；出现 `risk`、`fail` 或未解析运行时停止交付；
6. 校对完成后冻结母版哈希、子集哈希和许可证，仍只交付同一个 EPUB。

可变 TTF 很适合作为母版：从同一份文件实例化正文、常规标题和半粗标题的静态字重，再按书内实际字符子集化。完整母版不要写进 OPF，也不要与交付 EPUB 混放；只将静态子集及许可证放入 ZIP。

---

## 五、正文基础样式

```css
@charset "utf-8";

html {
  font-size: 100%;
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  -webkit-box-sizing: inherit;
  box-sizing: inherit;
}

body {
  margin: 0;
  padding: 0 1em;
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
  font-size: 1em;
  line-height: 1.7;
  text-align: justify;
  /* 自由模式默认不设 font-family；锁定模式的直接 body 规则放在 fonts.css */
}

p {
  margin: 0.6em 0;
  text-indent: 2em;
}

h1, h2, h3, h4, h5, h6 {
  line-height: 1.35;
  page-break-after: avoid;
}

h1 + p,
h2 + p,
h3 + p,
blockquote p,
figure + p,
hr + p {
  text-indent: 0;
}

img {
  max-width: 100%;
  height: auto;
}

code, pre, kbd, samp {
  font-family: "SF Mono", "Consolas", "Source Code Pro", monospace;
}
```

长篇中文书可另设经实测的紧凑阅读档，例如 `body { line-height: 1.6; }` 与 `p { margin: 0 0 1em; text-indent: 2em; }`；这组值与 Readest 的 CJK 覆盖默认值一致，适合用作对照样张，不替代上面的通用起点。行距不是平台常量，仍需用目标字号和屏宽实测；不要用 `!important` 阻止阅读器或读者调整。

### 墨水屏适配要点

电子墨水屏（Kindle / Kobo / 墨案等）只有灰阶，渲染速度慢于 LCD/OLED，CSS 需额外注意：

| 做 | 不要做 |
|---|---|
| 用 `#000` / `#222` / `#555` 等高对比度灰 | 用 `#aaa` / `#bbb` 浅灰文字（墨水屏几乎看不见） |
| 用线条 / 边框 / 加粗 / 间距区分层级 | 用颜色区分层级（墨水屏只有灰阶） |
| 给图片留白边 | 给图片做阴影（墨水屏渲染慢） |
| `font-weight: bold` 区分标题 | `text-shadow` 区分文字（墨水屏不显示） |
| 段间距用 `margin` | 用 `<br/><br/>` 凑空白 |
| 隔行背景用 `#f0f0f0` | 用更浅的灰（如 `#fafafa`，墨水屏上看不出差异） |

---

## 五点二、英文小说正文

英文小说和中文正文不要共用同一套段落节奏。简单英文 prose EPUB 的稳定结构是：章节图单独居中，章节标题居中，首段无缩进并可用 `::first-letter` 做轻量首字，后续段落缩进，插图使用居中 `figure`，不依赖固定页高或固定行数。

```html
<body class="english-fiction" xml:lang="en" lang="en">
  <section epub:type="chapter">
    <figure class="en-illustration">
      <img src="../Images/ch01.jpg" alt="Chapter illustration"/>
    </figure>
    <h1 class="english-chapter-title">I. Chapter Title</h1>
    <p class="en-noindent en-first-letter">The first paragraph starts without indent.</p>
    <p class="en-noindent en-dropcap-host"><span class="en-dropcap">A</span> second opening paragraph demonstrates a lowered initial.</p>
    <p>Following paragraphs use a modest first-line indent.</p>
  </section>
</body>
```

```css
.english-fiction {
  font-family: Georgia, "Times New Roman", "Noto Serif", serif;
  line-height: 1.55;
  hyphens: auto;
  -webkit-hyphens: auto;
}

.english-fiction p {
  margin: 0;
  text-indent: 1.35em;
  text-align: left;
}

.english-fiction .en-noindent {
  text-indent: 0;
}

.en-first-letter::first-letter {
  font-size: 1.75em;
  line-height: .8;
  font-weight: 700;
}

.en-dropcap-host {
  text-indent: 0;
}

.en-dropcap {
  float: left;
  font-family: "Snell Roundhand", "Segoe Script", cursive;
  font-size: 3.3em;
  line-height: .78;
  font-weight: 400;
  padding-right: .1em;
  margin-top: .04em;
}
```

英文正文不强制 `text-align: justify` 作为通用主路径。窄屏、大字号或阅读器断字支持弱时，英文 justify 容易产生大词距；除非目标平台已验证 hyphenation，优先左对齐。首字建议先用 `::first-letter`，避免把单词拆成 `<span>T</span>he` 后影响朗读或复制；旧式 span 首字和 float drop cap 可作为增强，但必须在大字号下复测。若下沉首字需要特殊字体，生产书应嵌入授权字体并声明 OPF font item；demo 可用 `"Snell Roundhand", "Segoe Script", cursive` 这类系统手写体链代替。

---

## 五点三、章节头图设置

部分书籍排版会在每章标题前放装饰图。可以借鉴这个结构，但要保持标题和正文是真实文本：头图只做气氛、系列感或栏目识别，不承载章节标题。普通章首分两类：小型章标使用保守宽度，横幅头图可以铺满正文内容栏。

```html
<header class="chapter-header">
  <figure class="chapter-head-art">
    <img src="../Images/chapter-mark.png" alt=""/>
  </figure>
  <p class="chapter-kicker">第一章</p>
  <h1 class="decorated-chapter-title">章节标题</h1>
  <p class="chapter-subtitle">可选副标题</p>
</header>
```

```css
.chapter-head-art {
  margin: .8em auto .7em;
  text-align: center;
  text-indent: 0;
  page-break-inside: avoid;
}

.chapter-head-art img {
  display: block;
  width: 35%;
  max-width: 7.5em;
  min-width: 4.5em;
  height: auto;
  margin: 0 auto;
}

.chapter-head-art-roomy img {
  width: 40%;
  max-width: 9em;
}

.chapter-head-banner img {
  display: block;
  width: 100%;
  max-width: 100%;
  height: auto;
  margin: 0 auto;
}
```

同一本 EPUB 里优先把小章标的保守宽度作为默认 fallback：`35%` 左右加 `max-width`。空间充足且已复测时，再对少数页面加增强类到 `40%` 左右。横幅头图使用 `width:100%; max-width:100%`，高度由源图比例决定；若需要更矮或更高，应裁好横向源图，而不是在 CSS 里硬写高度。EPUB 的“满屏宽”通常只能稳定做到“满正文内容栏宽”，不要为了贴屏幕边缘去破坏用户页边距。不要用 `vh`、absolute positioning 或大段顶部空白来控制章首；如果需要整页视觉封面，走 A-lite，而不是把普通章节做成固定版式。

---

## 五点五、图片环绕兼容路径

图文环绕使用 `<figure>` 浮动作为主路径。Kindle App 实测 figure 也能环绕；关键是 figure 有明确百分比宽度，并且前后正文足够长，让阅读器有足够行数展示绕排。direct `img` 直挂 float 不作为主路径，避免部分阅读器图片显示过小。

```html
<figure class="img-left">
  <img src="../Images/poster.png" alt="左浮动"/>
  <figcaption>图片说明。</figcaption>
</figure>
<p>
  正文从图片右侧环绕，段落要足够长才能观察绕排。
</p>
```

```css
figure.img-left {
  float: left;
  width: 30%;
  margin: 0 1em 0.6em 0;
  text-align: center;
}

figure.img-right {
  float: right;
  width: 30%;
  margin: 0 0 0.6em 1em;
  text-align: center;
}

figure.img-left img,
figure.img-right img {
  width: 100%;
  height: auto;
}
```

推荐把 figure 宽度先放在 `25%–35%` 之间，本 demo 默认 `30%`。这个范围不是 EPUB 标准常量，而是兼顾 Kindle App 绕排阈值与 Readest 图片可读性的保守起点：宽度越大，图片越清楚，但剩余文本列越窄；宽度越小，环绕更稳，但图片可能显得偏小。`50%` 在某些设备上也能成功，是因为它仍然是百分比宽度，且当时的屏幕与字号还给正文留下了足够列宽；但它更接近阈值，窄屏或大字号下更容易被阅读器改排到图片下方。正式书稿要按目标阅读器、屏幕宽度、用户字号和正文长度实测微调。

不要用 `em` 做 Kindle 主路径。`em` 会随用户字号放大，导致浮动盒和剩余列宽一起变化；百分比宽度绑定页面宽度，所以大字号下更稳定。图片高度不固定：内层 `img` 用 `height:auto` 保持天然宽高比；`aspect-ratio` 不作为 EPUB 主路径，因为旧阅读器支持不稳定，而且 figure 还要容纳 caption 的自然高度。短段落无法证明环绕失败，实际测试要让正文至少有数行能贴住浮动图片。

### 五点五点一、不同宽高比插图并排

原书若把一张窄高图和一张宽图放在同一行，并要求图形垂直协调、图题底部对齐，不需要用 HTML table。保留两张独立 `figure`，在每张图里增加一个等高的图像区：

```xhtml
<div class="figure-pair">
  <figure class="figure figure-narrow">
    <div class="figure-stage"><img src="figure-a.png" alt="……"/></div>
    <figcaption>图 A</figcaption>
  </figure>
  <figure class="figure figure-wide">
    <div class="figure-stage"><img src="figure-b.png" alt="……"/></div>
    <figcaption>图 B</figcaption>
  </figure>
</div>
```

```css
.figure-pair { display:flex; align-items:stretch; gap:1em; }
.figure-pair .figure { display:flex; flex-direction:column; margin:0; }
.figure-stage {
  display:flex;
  flex:1 1 auto;
  align-items:center;
  justify-content:center;
}
.figure-stage img { display:block; width:100%; height:auto; }
```

图像区负责垂直居中，纵向 `figure` 让图题自然落在共同底线；窄屏下把外层改为 `display:block`。`table` 只能作为某个目标阅读器无法正确渲染 Flex、且已经实测的专用回退，并应声明 `role="presentation"`；非表格内容默认仍使用 `figure`。

---

## 五点六、文白对照左右兼容

文白对照和原文/译文对照可以做左右并排，但不要把它做成表格、flex 或固定版式。稳定路径是先写源序上下结构，再只在足够宽的阅读区域用 `float` 增强成左右栏；Kindle 电子墨水、小屏、大字号或其他阅读器不支持 media / float 时，必须保持原文在上、译文在下，不能退化成半宽错位。

```html
<section class="parallel-pair parallel-float-pair">
  <p class="classical-text font-st" xml:lang="lzh">文言原文。</p>
  <p class="modern-text font-kt">白话译文。</p>
  <div class="parallel-clear" aria-hidden="true"></div>
</section>
```

```css
.parallel-pair {
  clear: both;
}

.parallel-float-pair {
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
  break-inside: avoid;
}

.parallel-stack-pair {
  page-break-inside: auto;
  -webkit-page-break-inside: auto;
  break-inside: auto;
}

@media (min-width: 40em) {
  .parallel-float-pair .classical-text {
    float: left;
    width: 38%;
  }

  .parallel-float-pair .modern-text {
    float: right;
    width: 58%;
  }

  .parallel-float-pair.parallel-ratio-balanced .classical-text,
  .parallel-float-pair.parallel-ratio-balanced .modern-text {
    width: 48%;
  }

  .parallel-float-pair.parallel-ratio-source-wide .classical-text {
    width: 58%;
  }

  .parallel-float-pair.parallel-ratio-source-wide .modern-text {
    width: 38%;
  }
}

.parallel-clear {
  clear: both;
  height: 0;
  font-size: 0;
  line-height: 0;
}
```

这个结构刻意接近大部头文白书：每组直接放原文段落和译文段落；短组用 `.parallel-float-pair` 标记为可增强，但基础状态仍上下，只有 `min-width:40em` 以上才左右显示；长段或大字号探针用 `.parallel-stack-pair` 保持上下并允许正常分页。不要只在 `@media (orientation: landscape)` 里启用左右布局，也不要把 Kindle 主路径依赖在 `display:flex` 上；这类写法在 Kindle Previewer / KFX 中容易退化不可控。

`38%/58%` 是文言 / 白话的默认起点，不是标准常量。原文和译文篇幅接近时，给 `.parallel-float-pair` 加 `.parallel-ratio-balanced` 使用 `48%/48%`；原文较长、译文较短时，加 `.parallel-ratio-source-wide` 使用 `58%/38%`。单书还可以在后加载 CSS 中写自己的比例类，但两栏总和建议不超过 `96%`，给自然 gutter 留至少 `4%`。比例只影响宽屏增强，不改变窄屏、大字号和长段默认上下的 fallback。

Kindle 专用 AZW3 里可以见到 `table-layout: fixed` + 左右 `td` 的英汉对照做法，实际能显示左右栏。但它不适合作为 EPUB/KDP 源文件的默认建议：表格承载长正文会增加质量审核、大字号、窄屏和辅助技术风险。除非目标就是只交付 Kindle 成品格式并已经逐设备验收，否则优先用 source-order + float。

---

## 五点七、边框、阴影与便签

便签、提示框、资料卡和摘录框与中文/英文正文共用同一个原则：内容必须是真实文本，视觉边框只是辅助。最稳主路径是 `border` / `border-left` / `background` / `padding`；阴影和不规则边缘都只作为增强。不要在通用 EPUB 中用 `transform: rotate()` 旋转整块文本框，Kindle Previewer 3.104（2026-05-23 实测）会在增强排版转换中触发内部错误。

```html
<div class="note-box note-shadow">
  <p class="note-title">提示</p>
  <p>这里是真实文本。阅读器忽略阴影时，边框和底色仍然保留。</p>
</div>
```

```css
.note-box {
  margin: 1.1em 0;
  padding: .8em .9em;
  text-indent: 0;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}

.note-shadow {
  border: 1px solid #c9bda9;
  background: #fffaf0;
  -webkit-box-shadow: .22em .22em 0 #d8ccb9;
  box-shadow: .22em .22em 0 #d8ccb9;
}
```

可用层级：

- 方正框：`border: 1px/2px solid`，最稳。
- 左侧竖线：`border-left`，适合长引用和非虚构提示。
- 虚线/双线：用于草稿、题签、复古效果。
- 投影/内阴影：`box-shadow` / `inset`，可丢失增强；忽略后仍有边框和底色。
- 斜角感便签：用不对称边框、圆角和投影模拟贴纸偏移；不要在通用 Kindle 版本使用 `transform: rotate()`。
- SVG 花边实验：可验证小型内联 SVG 边线在部分阅读器中可显示并通过转换，但不作为通用推荐边框；生产书稿优先降级为双线框、左侧竖线框或普通边框。
- 长条投影框：上下边线 + 左侧色条 + 投影，适合较长资料卡。
- 不规则边缘：不对称 `border-radius` + `outline`，不要依赖 `clip-path`。

不要用复杂滤镜、CSS mask 或多层伪元素承载关键信息。Kindle/旧 WebKit 可能忽略阴影或外轮廓；只要边框和底色还在，就应视为合格降级。

---

## 六、A-lite 整页海报方案

### 6.1 XHTML

```xml
<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
<head>
  <title>分卷页</title>
  <link href="../Styles/fonts.css" rel="stylesheet" type="text/css"/>
  <link href="../Styles/poster.css" rel="stylesheet" type="text/css"/>
</head>
<body class="fullpage poster-bg">
  <section class="fullframe" epub:type="chapter">
    <h1 class="poster-title">汪曾祺全集</h1>
    <p class="poster-subtitle">①小说卷</p>
  </section>
</body>
</html>
```

### 6.2 CSS

```css
@charset "utf-8";

@page {
  margin: 0;
  padding: 0;
}

html {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  box-sizing: inherit;
}

body.fullpage {
  width: 100%;
  height: 100%;
  min-height: 100%;
  margin: 0;
  padding: 0;
  font-size: 16px;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;
  box-sizing: border-box;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-page-break-inside: avoid;
  overflow: hidden;
}

body.poster-bg {
  background-image: url("../Images/poster-bg.png");
  background-repeat: no-repeat;
  background-position: left bottom;
  background-size: 80% auto;
}

/* body.fullpage 仅负责页面骨架；背景通过 poster-bg 等 modifier 类提供。 */

.fullframe {
  width: 100%;
  height: auto;
  min-height: 90%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
  overflow: visible;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}

.poster-title,
.poster-subtitle {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  float: right;
  text-indent: 0;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}

.poster-title {
  clear: right;
  margin: 2% 4% 0 0;
  padding: 0;
  font-family: "kt", serif;
  font-weight: normal;
  font-size: 260%;
  line-height: 1.12;
  letter-spacing: 0;
}

.poster-subtitle {
  clear: none;
  margin: 15% 4% 0 0;
  padding: 0;
  font-family: "kt", serif;
  font-weight: normal;
  font-size: 160%;
  line-height: 1.25;
  letter-spacing: 0;
}
```

### 6.3 既有单图卷封

如果既有 EPUB 的分卷海报只有一张包含全部设计内容的竖版图片，不要为了“铺满”而使用 `cover` 或 `100% 100%`。前者会裁掉边缘文字，后者会拉伸图片。使用 `contain`，并保留原图作为不支持背景图时的 fallback：

```xml
<body class="fullpage poster-bg-contain">
  <section class="fullframe" epub:type="chapter">
    <img class="poster-fallback" src="../Images/poster.png" alt="分卷海报"/>
  </section>
</body>
```

```css
body.poster-bg-contain {
  background-image: url("../Images/poster.png");
  background-repeat: no-repeat;
  background-position: center center;
  background-size: contain;
}

.poster-fallback {
  display: block;
  width: 100%;
  max-width: 100%;
  height: auto;
  max-height: 100%;
}

@supports (background-size: contain) {
  body.poster-bg-contain .poster-fallback {
    visibility: hidden;
  }
}
```

同一本合集有多张卷封时，为每张图增加 `poster-bg-volume-*` modifier，骨架规则只写一份。源文件若已有独立叠加文字，文字仍保留为真实节点；不要重新栅格化。

---

## 六点五、CSS 文件分层

- `fonts.css`：仅放 `@font-face`、只含字体声明的稳定角色选择器，以及系统字体/局部角色 helper。
- `base.css`：正文基础元素（`@page`、`html/body`、标题、段落、列表、表格、代码、普通 `figure/img`、inline 语义、Ruby 默认、`.has-ruby` 行距兜底）。
- `notes.css`：标准 popup footnote、多看 fallback 和注释图标。
- `effects.css`：着重号、波浪线、首字下沉、便签/资料卡边框阴影。
- `literary.css`：章首、章节头图、题记、对话、诗、信件、场景分隔、前置页、英文 prose 结构、文白对照条目结构。
- `media.css`：正文图文环绕、图片网格、公式块。
- `vertical.css`：非海报整页竖排正文。
- `poster.css`：A-lite 海报页（`body.fullpage`、`body.poster-bg`、`.fullframe`、`.poster-title`、`.poster-subtitle`、`.vcol`）。

加载顺序是 `fonts.css → base.css → notes/effects/literary/media/vertical/poster.css`。海报页建议链接 `fonts.css + poster.css`（可按需再链 `base.css`）；正文页链接 `fonts.css + base.css`，再按场景加入组件层。

普通 `html` / `body`、`body.fullpage`、标题、图注和引用不要写页面级 `color`、`background` 或 `background-color`，避免覆盖阅读器的夜间模式、护眼模式和用户主题。局部组件可以保留必要的边框、阴影和背景装饰；A-lite 背景图只写在 `poster-bg` 等 modifier 上。

## 七、弹出注释方案

### 7.1 XHTML

注释触发采用图片图标，项目默认图标放 `Images/note.png`。如果源 EPUB 已有本地注释图标，保留原 `img src` 和资源声明；只有 `[1]`、`注` 等纯文本或数字上标标记需要转换时，才补入默认图标。返回符号采用 `◎`。

任何使用 `epub:type` 的 XHTML 文件都要先在根元素声明 EPUB namespace：

```xml
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
```

`xmlns:epub` 只负责声明 `epub:` 前缀，不会单独把 OPF2 包升级成 EPUB3。已有 EPUB2 因目标阅读器兼容需求暂时不能迁移时，可以尝试带双向链接 fallback 的混合写法，但必须按阅读器实测；见 [EPUB2 外壳中的 Popup Note 兼容写法](../how-to/epub2-popup-note-compatibility.md)。

同一个 XHTML 文件内只放一个注释容器：`aside epub:type="footnote"`。多条注释放在容器内的 `ol.footnote-list`，每条注释用 `li.footnote-item` 承载，正文 `noteref` 直接指向对应 `li` 的 `id`。这样保留 EPUB 3 标准弹注识别点，也保留 demo 的多注释聚合结构。

```xml
<p>
  正文内容
  <sup>
    <a id="note-1"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  第二处正文内容
  <sup>
    <a id="note-2"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-2">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>

  <ol class="footnote-list">
    <li class="footnote-item" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        第一条注释内容。
      </p>
    </li>

    <li class="footnote-item" id="footnote-2">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-2">◎</a>
        第二条注释内容。
      </p>
    </li>
  </ol>
</aside>
```

### 7.2 CSS

```css
sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon {
  text-decoration: none;
}

.noteref-icon img {
  width: auto;
  height: 1em;
  vertical-align: baseline;
}

.footnote-line {
  width: 60%;
  height: 1px;
  margin: 1.5em 0 1em -0.5em;
  border: none;
  border-top: 1px solid #777;
}

.footnote-list {
  margin: 0;
  padding: 0;
  list-style-type: none;
  text-align: left;
}

.footnote-item {
  margin: 0.4em 0;
  padding: 0;
  list-style-type: none;
}

.footnote {
  margin: 0.4em 0;
  text-indent: 0;
  font-size: 0.9em;
  line-height: 1.35;
  text-align: left;
  /* font-family 继承 body：自由模式下为阅读器默认字体；锁定 / C1-body 模式下继承对应字体链 */
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
```

这个结构同时保留标准弹注识别点和 demo 的视觉逻辑：正文点图片，同文件的 `aside` 统一承载本章注释，注释内用 `◎` 返回。图片触发器不需要呈现成高位数字上标；可以保留 `<sup>` 兼容包裹，但 CSS 应把它压回普通行内图标。不要使用多看私有类名或私有 CSS 作为主路径；如从旧多看结构转换，可以把原有 `ol/li` 视觉分组迁移成这里的中性类名。

> 若项目希望注释正文使用独立楷体或仿宋角色，在 `fonts.css` 中给稳定的注释容器绑定系统链或覆盖完整的嵌入字体；`notes.css` 的 `.footnote` 基础类仍只保留结构与视觉属性。使用 `aside[epub|type~="footnote"]` 时，`fonts.css` 必须声明 `@namespace epub "http://www.idpf.org/2007/ops";`；它紧跟可选的 `@charset` / `@import`，并早于 `@font-face` 和普通样式规则。

### 7.3 叠加多看 fallback

只有目标 EPUB 明确需要多看旧版兼容时，才在标准结构上叠加多看类名；不要创建第二份注释容器。

```html
<p>
  需要注释的正文
  <sup>
    <a id="note-legacy-1"
       class="noteref-icon duokan-footnote"
       epub:type="noteref"
       role="doc-noteref"
       href="#footnote-legacy-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
  继续正文。
</p>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line"/></div>
  <ol class="footnote-list duokan-footnote-content">
    <li class="footnote-item duokan-footnote-item" id="footnote-legacy-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-legacy-1">◎</a>
        注释正文仍只保留在同一个章末列表内。
      </p>
    </li>
  </ol>
</aside>
```

---

## 八、文字效果

### 8.1 着重号

```css
.emp {
  font-style: normal;
  text-emphasis: filled dot;
  -webkit-text-emphasis-style: filled dot;
  -epub-text-emphasis-style: filled dot;
  text-emphasis-position: under;
  -webkit-text-emphasis-position: under;
  -epub-text-emphasis-position: under;
}
```

```html
<span class="emp">着重内容</span>
```

### 8.2 波浪线

```css
.wavy {
  text-decoration: underline;
  text-decoration-style: wavy;
  -webkit-text-decoration-style: wavy;
  text-decoration-color: #c03030;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.12em;
}
```

```html
<span class="wavy">波浪线内容</span>
```

Kindle App 只显示基础 underline，不显示 wavy；这是预期降级。不要写 `text-decoration: underline wavy`，旧引擎可能把整条 declaration 丢弃，导致连下划线也没有。

### 8.3 Ruby 注音

```html
<p class="has-ruby">
  <ruby>字<rt>zì</rt></ruby>
</p>
```

```css
ruby {
  ruby-align: center;
}

rt {
  font-size: 0.5em;
  line-height: 1;
  /* font-family 继承 body：注音跟正文同字体 */
}

p.has-ruby {
  line-height: 1.9;
}
```

> 如确需给 ruby 注音单独换字体（例如汉字正文 + 平假名注音用日文字体），按 SPEC §8 模式 A 写系统字体链：`rt[lang="ja"] { font-family: "Hiragino Sans", "Yu Gothic", "Noto Sans CJK JP", sans-serif; }`。

### 8.4 引用

```html
<p>他说：<q>文字要经得起换设备。</q></p>

<blockquote>
  <p>一段较长的引用。</p>
</blockquote>
```

```css
q,
blockquote {
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
}

blockquote {
  margin: 1em 0.5em;
  padding: 0.6em 1em;
  border-left: 3px solid #999;
}

blockquote p {
  text-indent: 0;
}
```

> 引用走楷体是中文出版常见约定，与 `fonts.css` 的 `.font-kt` 同源。若项目希望引用走正文宋体，删掉这条 `font-family` 让它继承 body。

---

## 九、图片与封面

```html
<figure>
  <img src="../Images/example.jpg" alt="图像内容说明"/>
  <figcaption>图 1：说明文字</figcaption>
</figure>
```

```css
figure {
  margin: 1em 0;
  text-align: center;
}

figure img {
  max-width: 100%;
  height: auto;
}

figcaption {
  margin-top: 0.5em;
  font-size: 0.9em;
  text-indent: 0;
  text-align: center;
}
```

格式采用：

- 封面和照片：JPEG。
- 透明图、注释图标、贴图、截图：PNG。
- WebP：仅作现代阅读器增强实验或源文件，不进入 Kindle 主路径；Kindle conversion log 已确认 WebP 会触发不支持/无效图像通知。
- SVG：可作为现代 EPUB 增强或源文件；面向 Kindle 的生产包应准备 JPEG / PNG 栅格化版本。

生产建议：

- 书内图片以 JPEG / PNG 为主。照片、插画优先 JPEG；线稿、截图、图表优先 PNG。
- 面向 Kindle 的图片应提前转为 sRGB JPEG / PNG，避免透明、CMYK、TIFF、多帧 GIF 和 WebP。
- SVG 若包含复杂路径、文字、外部字体或滤镜，不要直接作为 Kindle 主路径；先栅格化，再把文字说明放回 HTML 正文或 `figcaption`。

---

## 十、竖排

全书横排、部分页面竖排时，只在那几个页面局部写竖排类。OPF 的 `spine` 仍保持横排：

```xml
<spine toc="ncx" page-progression-direction="ltr">
```

局部竖排页面的 XHTML：

```xml
<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
<head>
  <title>竖排题词页</title>
  <link href="../Styles/fonts.css" rel="stylesheet" type="text/css"/>
  <link href="../Styles/base.css" rel="stylesheet" type="text/css"/>
</head>
<body class="page-vrl">
  <section class="vrl-section" epub:type="chapter">
    <h1 class="vrl-title">题词</h1>
    <p>一段竖排文字。</p>
    <p>第二段竖排文字。</p>
  </section>
</body>
</html>
```

```css
body.page-vrl {
  margin: 0;
  padding: 1em;
}

.vrl-section {
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  line-height: 1.8;
  height: 100%;
}

.vrl-section p {
  text-indent: 2em;
  margin: 0 0 0 1em;
}

.vrl-title {
  font-family: "kt", serif;
  font-weight: normal;
  line-height: 1.2;
  margin: 0 0 0 1.5em;
}
```

整本竖排书：

```xml
<spine toc="ncx" page-progression-direction="rtl">
```

局部章节扉页竖排时，保留整本 `page-progression-direction="ltr"`。

---

## 十点四点一、版权页

版权页是前置书目信息页，不是必须照相还原的固定画面。清洗扫描来源时，应把 CIP、书名、著译者、出版发行、印刷、版次、印次、ISBN、定价和版权声明保留为真文本，并保持原书的信息与顺序。不要根据 OCR 猜测补字段，也不要把整页扫描图作为正文替代。

```xhtml
<section class="frontmatter copyright-page"
         epub:type="frontmatter copyright-page"
         aria-label="版权信息">
  <p class="cp cp-kai">图书在版编目（CIP）数据</p>
  <p class="cp">书名 / 作者著. — 出版地：出版社，年份</p>
  <p class="cp">ISBN 000-0-00-000000-0</p>
  <hr class="cp-line-rule"/>
  <p class="cp">责任编辑：某某</p>
  <p class="cp">出版发行：某某出版社</p>
  <p class="cp">版权所有，侵权必究</p>
</section>
```

这是存量书转录的保守主路径：每一行按原页顺序保存为真实段落，用普通 `hr` 表示可见分隔线；CSS 只在 `.copyright-page` 内生效。原书确实存在清楚的标签—值关系，或内容是原生电子书时，可以再使用 `h1` / `h2` 与 `dl` / `dt` / `dd`，并用 Grid 作宽屏增强；`dl + Grid` 不是扫描版权页的强制重构目标。

相对字号、自然分页和真实阅读顺序优先；不使用固定页高、绝对定位或 `table` 伪造版面。版权页可绑定授权的无衬线角色字体，但这本身不构成整书正文锁定字体的理由。

---

## 十点五、MathML

Kindle Enhanced Typesetting 支持 MathML。含 MathML 的 XHTML 必须在 OPF manifest 上声明 `properties="mathml"`。

正文中的 `x + y = 12` 等行内表达也适合使用 MathML，并可只给 `<math>` 绑定 `STIX Two Math`。不要把数学字体设给整个中文段落。需要在校对中追踪公式源时，把表达树与 TeX 源放在同一个 `semantics` 中：

```xhtml
<math xmlns="http://www.w3.org/1998/Math/MathML" display="inline">
  <semantics>
    <mrow>
      <mi>x</mi><mo>+</mo><mi>y</mi><mo>=</mo><mn>12</mn>
    </mrow>
    <annotation encoding="application/x-tex">x+y=12</annotation>
  </semantics>
</math>
```

```css
math,
math[display="inline"] {
  font-family: "STIX Two Math", "ControlBook Serif", serif;
}
```

`annotation` 是机器可审计的公式源，不应作为第二份可见文本。若数学字体包含 OpenType MATH 表，发行前应确认子集工具没有裁掉伸缩括号、根号、积分号等构件；不能确认时，数学字体保留全集通常比盲目子集更安全。

```xml
<item id="math"
      href="Text/16-math.xhtml"
      media-type="application/xhtml+xml"
      properties="mathml"/>
```

demo 覆盖常用组合：`mfrac`、`msqrt`、`mroot`、`msub`、`msup`、`msubsup`、`mover`、`munder`、`munderover`、`menclose`、`mfenced`、`mtable`、`mtr`、`mtd`、`semantics`、`annotation`、`mmultiscripts`、`ms`、`mspace`、`mstyle`、`mpadded`、`mphantom`。

公式内部结构留在 MathML，页面布局留在 HTML/CSS。对于已经在 Kindle Previewer 与 Readest 实测的单一生产包，编号公式可采用保守的 HTML table 外层；它只负责对齐，不表示数据表：

```xhtml
<table class="eq-table" role="presentation">
  <tbody>
    <tr>
      <td class="eq-formula">
        <math xmlns="http://www.w3.org/1998/Math/MathML"
              display="block">…</math>
      </td>
      <td class="eq-num" aria-label="公式 1">（1）</td>
    </tr>
  </tbody>
</table>
```

```css
.eq-table { width: 100%; border: 0; border-collapse: collapse; }
.eq-table td { border: 0; padding: 0; vertical-align: middle; }
.eq-formula { width: 100%; text-align: center; }
.eq-num { padding-left: .5em; text-align: right; white-space: nowrap; }
```

通用样本仍可同时保留“公式在前、编号在后”的线性 `div`，再用 `.eq-grid` 作渐进增强。不要把 `mlabeledtr` 当作跨阅读器公式编号主路径，也不要给 `mtable` 设置固定宽度。含说明文字和多个公式的推导行可用可换行 Flex，但源码顺序仍要独立可读。

HTML table 是经过目标版本实测后的保守布局，不是“Kindle 100% 兼容”声明；完整样式和方程组示例见 `docs/how-to/mathml-equation-layout.md`，证据必须记录 artifact、SHA、阅读器名称和版本。

当前单书证据来自匿名外部生产样本 v2：此前版本的线性版权页、`role="presentation"` 公式布局表和 MathML/STIX Two Math 路径已由用户在 Kindle Previewer 3.106 与 Readest 0.11.20 确认可用；2026-07-26 最终校对基线的 SHA-256 为 `43eab14f3dec4645fb25ee5d830de1e3431d3423d61c97ad925fc6b1feb50ec2`。该精确 artifact 已通过结构校验，12 幅无损索引色 PNG 也已与优化前逐像素核对一致；但因新增 36 处行内 MathML 与 12 幅修复图，仍在 `reader-matrix.yaml` 中标为 `warn`，等待两个阅读器重新完成视觉复测后才升级为 `pass`。

不支持 MathML 的目标阅读器需要文本公式或图片公式 fallback；不要把复杂公式只保存在不可读的截图里。

---

## 十一、制作流程

1. 准备并定稿文本、封面、海报背景、注释图标和授权字体；若用参考版校订已有正文，先按 SPEC §10.1.1 完成逐项决策，不直接覆盖现版。
2. 写正文 XHTML 和海报 XHTML；正文、注释、图片和目录先形成单一内容基线。
3. 写 `content.opf`，声明 reflowable、字体、图片、CSS；仅当正文字体锁定时声明 `ibooks:specified-fonts=true`。
4. 写 `fonts.css`，按正文、标题、序言/注释、生僻字等角色分开；稳定结构角色可直接绑定，混合角色使用类。
5. 写 `base.css` 和按需组件 CSS；字体属性不混入注释、文学结构或媒体布局层。
6. 若交付正文自由/锁定双版本，从同一内容基线派生并执行字体差异白名单检查。
7. EPUB lint / EPUBCheck 校验，并核对字体 `cmap` 覆盖实际角色字符。
8. Apple Books 删除旧书后重新导入测试。
9. Kindle Previewer 转换并测试 Publisher Font 开关。
10. Thorium、Calibre、KOReader 抽测正文、注释、海报、字体、夜间模式。

---

## 十二、最终检查清单

### OPF

- [ ] `nav.xhtml` 有 `properties="nav"`。
- [ ] 需 Kindle/旧工具链兼容时，manifest 含 `toc.ncx` 且 `spine toc="ncx"`。
- [ ] 封面图使用 JPEG/PNG，并同时声明 `properties="cover-image"` 与 `<meta name="cover">`。
- [ ] 所有 XHTML / CSS / 图片 / 字体都进入 `manifest`。
- [ ] `rendition:layout` 是 `reflowable`。
- [ ] 正文字体锁定时写 `ibooks:specified-fonts=true`；自由模式（默认）不加。局部角色字体不需要此 meta。
- [ ] `spine` 顺序正确。

### 字体

- [ ] 正文自由版的 `body` 与普通正文 `p` 不声明字体；正文锁定版使用直接 `body` 规则，并与 OPF meta 成对出现。
- [ ] 只有实际使用且授权允许的字体进入 `@font-face`、ZIP 和 OPF；不存在死声明、缺失 URL 或孤儿字体 item。
- [ ] C1-body 嵌入字体覆盖正文角色的全部实际文字、标点与 CSS 生成字符；局部嵌入字体覆盖其明确承担的字符，剩余字符已验证落到声明的 fallback；子集写出后已重新检查 `cmap`。
- [ ] 标题、题签、序言、注释和生僻字字体均按需启用，不把“必须内嵌”当作默认要求；补字子集只走 `.rare` 等局部类。
- [ ] 双版本共用同一内容基线，除字体相关白名单外无其他成员差异。
- [ ] 面向 Kindle 的锁定版已测试 Publisher Font 开关。

### A-lite

- [ ] `body.fullpage` 有 `min-height:100%`。
- [ ] 有 `page-break-before/after/inside`。
- [ ] `body.fullpage` 有 `overflow:hidden`，`.fullframe` 保持 `overflow:visible`。
- [ ] 内部基准字号为 `16px`。
- [ ] 竖排使用 `writing-mode: vertical-rl`。
- [ ] 多列使用 `float:right`。

### 弹注

- [ ] 正文引用是图片图标。
- [ ] `<a>` 有 `epub:type="noteref"` 和 `role="doc-noteref"`。
- [ ] 每个含注释的 XHTML 有一个 `<aside epub:type="footnote" role="doc-footnote">` 注释容器。
- [ ] 多条注释放在同一个容器内的 `ol.footnote-list > li.footnote-item`。
- [ ] noteref 的 `href` 指向对应 `li.footnote-item` 的 `id`。
- [ ] 注释返回符号是 `◎`。
- [ ] noteref、注释 `li` 和外层 aside 在同一 XHTML 文件。

### 标准效果

- [ ] 波浪线使用 `text-decoration-style: wavy`。
- [ ] 着重号使用 `text-emphasis`。
- [ ] Ruby 使用 `ruby + rt`。
- [ ] 图片使用 `figure + img + figcaption`。

---

## 十三、参考

- Apple Books Asset Guide: [Fonts Overview](https://help.apple.com/itc/booksassetguide/en.lproj/itc74d42b31e.html)
- Apple Books Asset Guide: [Defining Book Layout Metadata](https://help.apple.com/itc/booksassetguide/en.lproj/itc2cf4d26eb.html)
- Amazon: [Kindle Publishing Guidelines](https://kindlegen.s3.amazonaws.com/AmazonKindlePublishingGuidelines.pdf?rw_useCurrentProtocol=1)
- W3C: [EPUB 3.3](https://www.w3.org/TR/epub-33/)
- MDN: [text-decoration-style](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/text-decoration-style)
- 本项目：实测素材来自 `wang-chapterpage-demo-v2.epub`，历史决策痕迹见 `archive/experiments/`


### 自检补充（A-lite / 弹注 / 字体）

- [ ] 根 `html` 含 `width/height/min-height:100%`。
- [ ] `body.fullpage` 不携带 `background-*`；背景通过 `poster-bg` 等 modifier 提供。
- [ ] `body.fullpage` 含 `-webkit-text-size-adjust:100%; text-size-adjust:100%`。
- [ ] `.fullframe` 骨架 `padding:0; overflow:visible`，留白靠内部元素 `margin`。
- [ ] 需多看旧版兼容时，noteref 锚带 `duokan-footnote` 且内含 `<img>`。
- [ ] 注释列表 `<ol>` 同时挂 `footnote-list duokan-footnote-content`。
- [ ] 每条 `li.footnote-item` 只额外挂 `duokan-footnote-item`，不重复挂 `duokan-footnote-content`。
- [ ] 正文自由模式下 `body` / 普通 `p` 不设字体；未要求特定设计字形的显式标题、等宽等角色优先使用短系统链。
- [ ] 稳定的正文/标题/前言/注释角色可直接绑定嵌入字体；混合角色使用类，补字子集只使用 `.rare` 等局部类。
- [ ] 任一字体链的链尾必须是 generic family（serif / sans-serif / monospace）。
- [ ] 默认链 ≤ 4 段；嵌入模式 C 复合链 ≤ 5 段，嵌入字体在链里只出现 1 次（第 1 位或倒数第 2 位）。
- [ ] 启用模式 C1-body 时：嵌入字体覆盖最终解析到正文角色的全部实际用字、`fontspec=forceAll`、链 ≤ 5 段、嵌入仅在第 1 位。
