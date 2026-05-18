# EPUB 3 章节扉页与竖排实战 · 补充 05

> 本篇与 §01–§04 一致：可重排 EPUB 3.3，不覆盖固定版式 (FXL) 整本场景。
>
> 实战素材来自 `demo_chapter_pages/wang-chapterpage-demo.epub`。本篇是「§03 图片与海报页」的补充——把那一章的几套理论方案放在 iBooks / Kindle app / Readest / Kindle Previewer 上跑了一遍之后，沉淀下来的取舍。

## 目录

1. [章节扉页：三套写法 + 决策树](#一章节扉页三套写法--决策树)
2. [A-lite 的三件套与「字号大→分两页」的代价](#二a-lite-的三件套与字号大分两页的代价)
3. [竖排两种写法对比：float vs writing-mode](#三竖排两种写法对比float-vs-writing-mode)
4. [iBooks 与 Kindle 的特殊关注](#四ibooks-与-kindle-的特殊关注)
5. [正文文字效果速查](#五正文文字效果速查)

---

## 一、章节扉页：三套写法 + 决策树

### 1.1 三套写法是什么

| | A-lite | A | B |
|---|---|---|---|
| 名字 | min-height + page-break | padding-bottom 比例锁 | rendition:layout-pre-paginated 单页 FXL |
| 盒子高度来源 | `min-height: 100%`——跟着父链一路追到 viewport | `padding-bottom: 156%`——盒子自己的宽度算高度 | `<meta viewport width=1640 height=2560>` 固定画布 |
| 是否锁比例 | 否 | 是（1 : 1.56 等） | 是（画布像素比） |
| 子元素布局 | 正常流（float、margin） | 必须 `position: absolute` | 必须 `position: absolute` + 绝对 px |
| 用户字号设置 | 生效 | 生效 | **失效** |
| 单页文件大小 | 小 | 小 | 中（图片素材要做大） |
| 实现复杂度 | 低 | 中 | 高 |

§03「整版海报页」详细写过 A 与 B 的实现。本篇重点：**对于「整页章节扉页」这个具体需求，A-lite 是首选；A 不适合整页用；B 是必须像素一致时的最后手段。**

### 1.2 决策树

```
开始：你要做的是什么？
│
├── 整页章节扉页（章前画面 + 大字标题，独占一页）
│    └── A-lite ✅  （本篇推荐，源汪曾祺集就用这套）
│
├── 正文流里嵌一张固定比例的小型海报/插图卡
│    └── A     ✅  （比例锁的真正用武之地，见 §03）
│
└── 设计师交稿，整版像素必须一致，可牺牲用户字号偏好
     └── B     ✅  （封底设计图、纪念版扉画，见 §03）
```

### 1.3 为什么 A 不适合整页扉页

A 的盒子是「宽度 × 1.56」高——在窄屏阅读器里，这个高度**比一页还高**。iBooks 用 CSS 多列做横翻分页，会把这个超高盒子切碎成两列；子元素的 `position: absolute` 是基于 `.poster` 算的，但 `.poster` 现在分布在两列里，结果就是「字号、装饰图位置全错」。

我在第一版 demo 里把 A 套到整页扉页上，iBooks 显示完全乱——这不是 iBooks 的 bug，是 A 写法的适用范围被我错放了。**A 的官方定位是「在正文流里嵌一张比例固定的卡片」**，整页扉页要靠 A-lite。

### 1.4 各阅读器实测（章节扉页场景）

| | A-lite | A（整页误用） | B（FXL 单页） |
|---|---|---|---|
| Apple Books / iBooks | ✅ 正常 | ❌ 多列分页器切碎 | ✅ 需 metadata 全套声明 |
| Readest | ✅ 正常 | ⚠️ 部分对 | ⚠️ 单页混排支持参差 |
| Kindle Previewer | ✅ 正常 | ⚠️ | ✅（KFX 2018+） |
| Kindle app | ✅ 正常 | ⚠️ | ✅ |
| Kindle 墨水屏 | ✅ 正常 | ⚠️ | ✅（KFX 8+） |
| KOReader | ✅ 正常 | ⚠️ | ⚠️ 简化为图片 |
| ADE 老版 | ✅ 正常 | ⚠️ | ❌ 退化为 reflowable |

> 「⚠️ 部分对」= 桌面/平板可能勉强对，e-ink 或小屏出错。

---

## 二、A-lite 的三件套与「字号大→分两页」的代价

### 2.1 三件套

整页扉页不分页、不溢出、占满屏的核心，是这三条 CSS 同时存在：

```css
body.fullpage {
  /* ① 撑高度——跟着父链到 viewport */
  width: 100%;
  height: 100%;
  min-height: 100%;

  /* ② 超出剪掉——不交给阅读器分页器 */
  overflow: hidden;

  /* ③ 强分页——把整个 xhtml 包成一个原子页 */
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-page-break-inside: avoid;

  /* 锚基线——避免阅读器字号设置打乱版式 */
  font-size: 16px;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;

  /* 背景图 */
  background-size: cover;
  -webkit-background-size: cover;
  background-position: center center;
  background-repeat: no-repeat;
}
```

汪曾祺集源 EPUB 的 `f3.xhtml`（制作说明页）就是这套——你之前观察到「这一页好像达到了不分页的目标」，原理就在这里。

### 2.2 为什么不要 vh / vw

`100vh` 表面上「就是要占满屏幕一格」，但它在三类阅读器上出问题：

- **Kindle 老 KFX 固件**：不识别 `vh`，盒子塌缩成内容自然高度（背景图按原始比例显示，文字横排，画面变小但不乱码——属于优雅降级，但「不再占满」）。
- **Readest 部分版本**：把 vh 当作 viewport 但与列分页冲突，盒子被切。
- **Kindle Previewer 某些版本**：含 vh/vw 的页面直接打不开。

`min-height: 100%` 没有这些坑——它的兜底语义是「至少有内容这么高」，阅读器无法识别也能继续渲染，只是不一定占满。配合 `page-break-*` 把它当一页处理就够了。

### 2.3 接受「字号大 → 分两页」

用户在 Kindle app 把字号调最大时，A-lite 扉页内容可能溢到第二页——这是 reflowable 的本质代价：

- A-lite 不锁内容高度，字号变大 → 内容变高 → 第二页接着显示
- 文字本身依然可读、效果保留（着重号、波浪线、拼音都还在）
- 用户「看到第二页」的体验，比「画面错位」要好得多

如果客户/编辑要求「永远不分页」，唯一办法是上 B（rendition:layout-pre-paginated），代价就是用户字号设置完全失效。

**建议**：扉页文字保持极简（一行主标题 + 一行副标题足够），少几个字 = 少一行 = 少触发分页。源汪曾祺集只放 `汪曾祺全集 / ①小说卷` 两列，正是这个考虑。

### 2.4 完整 CSS 模板

```css
@page { margin: 0; padding: 0; }

html { height: 100%; width: 100%; margin: 0; padding: 0; min-height: 100%; }

body.fullpage {
  width: 100%;
  height: 100%;
  min-height: 100%;
  -webkit-text-size-adjust: 100%;
  text-size-adjust: 100%;
  font-size: 16px;
  margin: 0;
  padding: 0;
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
  page-break-inside: avoid;
  page-break-before: always;
  page-break-after: always;
  -webkit-page-break-inside: avoid;
  -webkit-page-break-before: always;
  -webkit-page-break-after: always;
  -webkit-background-size: cover;
  background-size: cover;
  background-position: center center;
  background-repeat: no-repeat;
  overflow: hidden;
}

.fullframe {
  width: 100%;
  height: auto;
  min-height: 90%;
  margin: 0;
  padding: 0;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
  overflow: hidden;
}
```

HTML 端：

```xml
<body class="fullpage"
      style="background: #ECEAE7 url(../Images/bg.png) no-repeat left bottom;
             background-size: 80% auto;">
  <div class="fullframe">
    <!-- 标题层 -->
  </div>
</body>
```

> 背景图建议直接写 inline `<body style=...>` 而不是 CSS 类——便于一文一图、不污染共享样式表。

---

## 三、竖排两种写法对比：float vs writing-mode

汪曾祺集的扉页是竖排（「汪曾祺全集」从上往下读）。CSS 有两种实现方式，本 demo 都做了，可以打开 EPUB 在同一本里逐页对照。

### 3.1 float 假竖排（源 EPUB 用的）

```css
.fullframe .vtitle {
  float: right;
  width: 1.2em;             /* 强行限到 ≈ 1 个 CJK 字符宽 */
  word-break: break-all;    /* 任意位置都能换行 */
  text-align: center;
  font-family: "tj", serif;
  font-size: 280%;
  margin: 2% 8% 0 0;
}
```

```xml
<p class="vtitle">汪<br/>曾<br/>祺<br/>全<br/>集</p>
```

原理：盒子被限到一字宽 → CJK 字符宽度 ≈ 1em → 每字到右边界就换行 → 视觉上一字一行 → 看起来是竖排。HTML 里每字之间多打一个 `<br/>` 是保险——某些阅读器算字符宽度不精确，强制断行不会出错。

**优点**：

- ✅ 兼容**所有**阅读器，包括 2018 前的老 Kindle 固件
- ✅ ADE 1.x、2.x 全部支持
- ✅ KOReader 全版本支持

**缺点**：

- ⚠️ 不是真的竖排——西文字符、阿拉伯数字不会自然旋转，会侧躺
- ⚠️ 选中复制时字符顺序不稳定（不同阅读器行为不同）
- ⚠️ XHTML 体积大——每字 `<br/>` 累积起来不小
- ⚠️ 维护成本高——加一个字要加一个 `<br/>`

### 3.2 writing-mode 真竖排（手册推荐）

```css
.fullframe .vtitle {
  /* 三前缀齐发 */
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;

  /* 仍用 float 做多列横排堆叠（见 §3.3） */
  float: right;
  clear: right;

  font-family: "tj", serif;
  font-size: 280%;
  margin: 2% 8% 0 0;
}
```

```xml
<p class="vtitle">汪曾祺全集</p>
```

**优点**：

- ✅ 真竖排——CSS 盒模型层面就是 vertical-rl
- ✅ `text-orientation: mixed` 让 CJK 竖、西文/数字横（自然方向）
- ✅ HTML 干净——一个 `<p>` 一列，加字只改文字
- ✅ 选中复制字符顺序正确

**缺点**：

- ⚠️ 2018 前的 Kindle 老固件不识别——降级为横排（不是错位，只是不竖）
- ⚠️ KOReader 部分版本支持半截（西文方向可能错）
- ⚠️ 必须带 `-webkit-` 和 `-epub-` 前缀，缺一不可

### 3.3 多列竖排：float 横排盒 + writing-mode 控字流

汪曾祺集 `f223` 卷首内页有 3–4 列竖排（汪曾祺 | 全集①小说卷 | 主编... | 副主编...）。原 EPUB 全靠 float 堆，列内靠 word-break。

writing-mode 版的做法：**沿用 float 做横向堆叠，但每列内部用 writing-mode 控字流**。这样既不依赖 `position: absolute`（避免 iBooks 多列分页器坑），又保留真竖排的优点。

```css
.pubframe .inc1 {
  /* 横向 float 堆叠 */
  float: right;
  clear: none;

  /* 列内真竖排 */
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;

  font-family: "qk", serif;
  font-size: 280%;
}

.pubframe .inc1.col-r { margin: 5% 8% 0 0; }   /* 最右列 */
.pubframe .inc1.col-m { margin: 5% 4% 0 0; }   /* 次右列，离上一列 4% */
```

```xml
<div class="pubframe">
  <p class="inc1 col-r">汪曾祺</p>
  <p class="inc1 col-m">全集<span class="inc3">　　<span class="num">①</span>小说<small>卷</small></span></p>
  <p class="auth main">主编╲季红真</p>
  <p class="auth sub">小说卷主编╲李光荣　李建新</p>
</div>
```

注意：在 writing-mode 里 `<br/>` 是「换列」（块方向）而不是「列内换行」。要做列内的纵向间距，用**全角空格 `　`（U+3000）**——它在两种 writing-mode 下都按字符宽度占位，效果自然。

### 3.4 选哪一种？

| 场景 | 选 |
|---|---|
| 想覆盖最老的 Kindle 固件（2018 前）/ 最旧 ADE | float 假竖排 |
| 用户群主要是 Apple Books / Kindle app / 现代墨水屏 | writing-mode 真竖排 |
| 文字要复制粘贴正常 | writing-mode |
| 文字里夹西文 / 数字需要自动旋转 | writing-mode |
| 要常加减字、降维护 | writing-mode |

> 我个人的判断：现在新做的书，**writing-mode 是默认**。源汪曾祺集选 float 是 2021–2022 年的考虑，当时 Kindle 老固件用户基数还大；2026 年绝大多数 Kindle 用户已经在 KFX 8+，迁移到 writing-mode 没有实际风险。

---

## 四、iBooks 与 Kindle 的特殊关注

### 4.1 iBooks：rendition 元数据必须全套

混排 reflowable + FXL 单页时，iBooks 比其他阅读器严苛——`content.opf` 必须显式声明：

```xml
<package xmlns="http://www.idpf.org/2007/opf"
         version="3.0"
         unique-identifier="bookid"
         xml:lang="zh-CN"
         prefix="rendition: http://www.idpf.org/vocab/rendition/#
                 ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <!-- ... -->
    <meta property="rendition:layout">reflowable</meta>
    <meta property="rendition:orientation">auto</meta>
    <meta property="rendition:spread">auto</meta>
    <meta property="ibooks:specified-fonts">true</meta>
    <!-- ... -->
  </metadata>
```

- `rendition:layout=reflowable`：告诉 iBooks「这本书默认 reflowable，spine 里标了 pre-paginated 的才是 FXL」。少了这条 iBooks 不知道该不该启用混排模式。
- `rendition:orientation=auto` + `rendition:spread=auto`：避免 iBooks 强制横屏或强制双页。
- `ibooks:specified-fonts=true`：让 iBooks 尊重书内嵌字体，否则它会强制用系统字体。
- `prefix=...`：声明 `rendition:` 和 `ibooks:` 这两个词表的 URI，少了 iBooks 直接忽略所有 `property=...`。

### 4.2 viewport 写法（FXL 单页用）

```xml
<!-- ✅ 推荐 -->
<meta name="viewport" content="width=1640,height=2560"/>

<!-- ❌ 逗号后带空格 - 部分 iBooks 版本会丢失 -->
<meta name="viewport" content="width=1640, height=2560"/>
```

> 经验值：逗号后**不要**加空格。

### 4.3 Kindle app vs Kindle 墨水屏

不是同一个渲染器：

| 特性 | Kindle app（iOS/Android/Mac/Win） | Kindle 墨水屏（KFX 8+） | Kindle 墨水屏（KFX 老固件） |
|---|---|---|---|
| 底层 | 本地 WebView，接近现代浏览器 | KFX 8.x 渲染器 | KFX 老引擎 |
| writing-mode | ✅ | ✅ | ❌ |
| text-emphasis | ✅ | ✅ | ⚠️ 部分不显示 |
| ruby + rt | ✅ | ✅ | ⚠️ |
| @font-face | ✅ | ✅ | ✅ |
| background-image | ✅ 彩色 | ✅ 灰阶 | ✅ 灰阶 |
| text-shadow | ✅ | ⚠️ 灰阶下不明显 | ❌ |
| 颜色 | ✅ 全彩 | ⚠️ 全灰阶 | ⚠️ 全灰阶 |

**对照本 demo**：

- Kindle app：所有效果（着重号、波浪线、拼音、楷体引用）都正常。
- Kindle 墨水屏（如果 KFX 8+）：竖排、着重号、ruby 都正常；颜色变灰阶（写「红色波浪线」时灰阶下会变成「灰色波浪线」），文字本身可读。
- Kindle 墨水屏老固件：竖排可能横着显示；着重号可能不显示——但文字内容仍可读。

### 4.4 Kindle Previewer

Amazon 官方桌面工具，可以切换设备 profile 模拟 Kindle Paperwhite / Oasis / Fire 等：

- 用来检查墨水屏渲染最方便，**比真机插线传输快得多**
- 但**对 `vh` / `vw` 处理不一致**——某些版本含 vh 的 EPUB 直接打不开，所以你之前观察到「打不开」十有八九是这个
- A-lite 不依赖 vh/vw，Kindle Previewer 全程能开

> 实战建议：每一版 EPUB 在 Apple Books、Kindle app、Kindle Previewer 各跑一次，能覆盖 90% 用户的真实体验。

---

## 五、正文文字效果速查

本 demo `body.xhtml` 用 5 种效果做了样张。每条都附「在哪个阅读器降级到什么」。

### 5.1 着重号 `.emp`（正文最常用）

```css
span.emp {
  text-emphasis: filled dot;
  -webkit-text-emphasis-style: filled dot;
  -epub-text-emphasis-style: filled dot;
  text-emphasis-position: under;
  -webkit-text-emphasis-position: under;
  -epub-text-emphasis-position: under;
}
```

```xml
<p>这剑必须饮我底<span class="emp">仇人</span>的血！</p>
```

| 阅读器 | 显示 |
|---|---|
| Apple Books / iBooks | ✅ 字下实心点 |
| 多看 | ✅ |
| Thorium / Readium | ✅ |
| Kindle app | ✅ |
| Kindle 墨水屏（KFX 8+） | ✅ 灰阶 |
| Kindle 老固件 | ❌ 不显示，但文字本身正常 |
| KOReader | ⚠️ 部分版本 |

`text-emphasis-position: under` 是中文习惯（点在字下方）；日文习惯是 `over`（点在字上方）。

### 5.2 波浪线 `.wavy`

```css
span.wavy {
  /* 多看私有 + 通用 underline 兜底 */
  text-decoration: duokan-wavyline underline;
  duokan-text-decoration-color: #C03030;
  text-decoration-color: #C03030;
  duokan-text-decoration-width: 1px;
}
```

```xml
<p>山上多的是<span class="wavy">松鸡野兔子</span>。</p>
```

| 阅读器 | 显示 |
|---|---|
| 多看 | ✅ 红色波浪线 |
| 其他阅读器 | ⚠️ 退化为下划线（颜色可能保留） |
| Kindle 墨水屏 | ⚠️ 退化为下划线，灰阶 |

`duokan-wavyline` 是多看私有；写在 `text-decoration` 的多值里，多看识别为波浪线，其他阅读器忽略不识别的值只保留 `underline`——这是个干净的渐进增强写法。

### 5.3 拼音注音 `ruby + rt`

```css
ruby { ruby-align: center; }
ruby > rt {
  font-family: "qfs", "kt", sans-serif;
  font-size: 0.5em;
  color: #666;
}
```

```xml
<p>那一天他<ruby>跋<rt>bá</rt></ruby><ruby>涉<rt>shè</rt></ruby>千里。</p>
```

| 阅读器 | 显示 |
|---|---|
| 几乎所有现代阅读器 | ✅ 拼音浮在汉字上方 |
| Kindle 老固件 | ⚠️ 拼音作为普通文字显示在汉字旁边 |

`ruby-align: center` 在汉字宽度大于注音时居中；如果用于多个汉字共享一个注音的话改 `space-between`。

### 5.4 行内引用 `q` + 块引用 `blockquote` + 内联 `.kaiti`

```css
q { font-family: "kt", serif; color: #101010; }

blockquote {
  margin: 0.8em 0.5em;
  padding: 0.6em 1em;
  font-family: "kt", serif;
  background: rgba(180, 180, 160, 0.12);
  border-left: 3px solid #A2906A;
}
blockquote p {
  font-family: "kt", serif;
  color: #343;
  text-indent: 0;
}

.kaiti { font-family: "kt", serif; color: #444; }
```

```xml
<p><q>今夜竟挂了单呢</q>，年青人想想颇自好笑。</p>

<blockquote>
  <p>这剑必须饮我底仇人的血！</p>
</blockquote>

<p>那些话是他<span class="kaiti">已经背得烂熟</span>了的。</p>
```

| 元素 | 语义 | 视觉 |
|---|---|---|
| `<q>` | 行内引用 | 楷体 |
| `<blockquote>` | 段落引用 | 楷体 + 灰底色块 + 左侧装饰线 |
| `<span class="kaiti">` | 内联手动切楷体（无引用语义） | 楷体 |

三者**语义不同**：`q` 表「这是引用别人说的话」，`blockquote` 表「这是一段引用」，`.kaiti` 表「这段字风格上不同但不引用」。这关系到无障碍读屏、TTS 朗读、机器索引——所以宁可多一个标签，也不要全用 `.kaiti` 套到所有「想用楷体」的地方。

### 5.5 文字效果跨阅读器降级矩阵

| 效果 | Apple Books | 多看 | Thorium | Kindle app | Kindle KFX 8+ | Kindle 老 | KOReader |
|---|---|---|---|---|---|---|---|
| `text-emphasis` 着重号 | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| `duokan-wavyline` 波浪线 | underline | ✅ | underline | underline | underline | underline | underline |
| `ruby + rt` 拼音 | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| `q` / `blockquote` 引用 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `@font-face` 嵌字 | ✅* | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 内联字号 / 颜色 | ✅ | ✅ | ✅ | ✅ | ✅ 灰阶 | ✅ 灰阶 | ✅ |

\* Apple Books 需 `<meta property="ibooks:specified-fonts">true</meta>`，否则忽略书内嵌字体。

---

## 附录：本篇与其他补充篇的关系

- §03 图片与海报页：A、B 的完整实现细节、padding-bottom 比例锁、整页 JPG fallback——本篇假设你已经读过 §03，只补「整页扉页选 A-lite」这个具体取舍和实测结论
- §01 列表与字体：嵌入字体声明、本篇文字效果中的字体回退链条
- §02 弹注与 Ruby：ruby 标签的完整用法、duokan-footnote 弹注规范
- §04 其他 CSS：夜间模式、首字下沉等