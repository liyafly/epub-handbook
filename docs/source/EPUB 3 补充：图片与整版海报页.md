# EPUB 3 补充：图片与整版海报页

> 主手册（`EPUB3_制作完全参考手册.md`）的速查式补充（三）。
> 适用范围与主手册一致：可重排 EPUB 3.3，固定版式 (FXL) 不在内（§二 整版海报页中 Fallback B "Pre-paginated 单页"是 EPUB 3.3 允许的「混排 FXL 单页」例外）。
> 目标阅读器：Apple Books 4.x、Thorium 3.x、Calibre 7.x、Kindle Previewer 3 / KFX、Kobo、KOReader 2024+。
>
> 系列其他文档：
> - 列表与字体 → `EPUB3_补充_列表与字体.md`
> - 弹注与 Ruby → `EPUB3_补充_弹注与Ruby.md`
> - 其他 CSS（夜间模式、首字下沉、竖排等）→ `EPUB3_补充_其他CSS.md`

---

## 目录

1. [图片清单（普通图、绕排、行内、夜间反相、封面）](#一图片清单)
2. [整版海报页（背景、四角贴图、横竖排文字、堆叠 + vw/vh 主方案 + 三套 fallback）](#二整版海报页全屏背景--角落贴图--横竖排文字)
3. [对主手册的补丁建议（本主题相关）](#三对主手册的补丁建议本主题相关)
4. [阅读器兼容矩阵（节选）](#四阅读器兼容矩阵节选)

---

## 一、图片清单

### 1.1 HTML 模板（按用途分）

```xml
<!-- 单图（图注居中） -->
<figure>
  <img src="../images/diagram.png" alt="系统架构示意"/>
  <figcaption>图 1 系统架构</figcaption>
</figure>

<!-- 两图并排 -->
<div class="figure-pair">
  <figure>
    <img src="../images/before.png" alt="修改前"/>
    <figcaption>修改前</figcaption>
  </figure>
  <figure>
    <img src="../images/after.png" alt="修改后"/>
    <figcaption>修改后</figcaption>
  </figure>
</div>

<!-- 左 / 右绕排（文字环绕图片） -->
<figure class="float-right">
  <img src="../images/portrait.jpg" alt="作者肖像"/>
  <figcaption>作者肖像</figcaption>
</figure>

<!-- 行内图（生僻字、签名） -->
<p>这个字 <img class="inline-glyph" src="../images/glyph-x.svg" alt="生僻字"/> 读作……</p>

<!-- 透明 PNG（夜间自适应反相，见 §8.4） -->
<img class="dark-invert" src="../images/diagram.png" alt="原理图"/>
```

### 1.2 图片 CSS 模块

```css
/* 基础：不溢出、保持比例、块级居中 */
img {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em auto;
}

/* 图片容器 */
figure {
  margin: 1.5em 0;
  text-align: center;
  page-break-inside: avoid;
}
figcaption {
  font-size: 0.9em;
  margin-top: 0.5em;
  text-indent: 0;
  color: inherit;
}

/* 两图并排 */
.figure-pair { text-align: center; margin: 1.5em 0; }
.figure-pair figure {
  display: inline-block;
  width: 48%;
  vertical-align: top;
  margin: 0 0.5%;
}

/* 左绕排 / 右绕排 */
.float-left {
  float: left;
  width: 40%;
  margin: 0.3em 1em 0.5em 0;
}
.float-right {
  float: right;
  width: 40%;
  margin: 0.3em 0 0.5em 1em;
}
.float-left img,
.float-right img { width: 100%; margin: 0; }

/* 行内图（不脱离文字基线） */
.inline-glyph {
  display: inline;
  height: 1em;
  width: auto;
  vertical-align: -0.1em;
  margin: 0;
}

/* 绕排清浮动 */
.clearfix::after {
  content: "";
  display: table;
  clear: both;
}
```

### 1.3 图片格式选择

| 用途 | 首选 | 备注 |
|---|---|---|
| 照片 / 截图 | `.jpg` | 质量 80–85，sRGB |
| 线条图 / Logo / 截图含文字 | `.png` | 8 位透明优先 |
| 矢量图 / 图标 | `.svg` | 文字转路径，避免依赖字体 |
| 图片底色透明 | `.png`（透明） | 见 §8.4 夜间适配 |
| 动图 | `.gif` | 主流阅读器只播放一次；不要依赖动效传递必要信息 |
| WebP / AVIF / HEIC | ❌ | EPUB 3.3 不在 Core Media；KFX 全拒收 |

**分辨率**：单图最长边 ≤ 2000 px；屏幕宽方向通常 1200–1600 px 已经足够。**总图片体积控制在 30 MB 以内**，老设备打开才不卡。

### 1.4 透明图夜间自适应（替代多看私有方案）

赤霓书 §10.4.7 给的是多看 `duokan-image-static` 私有类名。**2025 通用做法**用 `prefers-color-scheme`：

```css
/* 默认：日间，透明 PNG 黑色线条在浅底上正常显示 */
.dark-invert { filter: none; }

/* 阅读器夜间模式下，反相让黑线条变白线条 */
@media (prefers-color-scheme: dark) {
  .dark-invert { filter: invert(1) hue-rotate(180deg); }
}
```

| 阅读器 | `prefers-color-scheme: dark` 是否触发 |
|---|---|
| Apple Books（夜间模式） | ✅ |
| Thorium（Night 主题） | ✅ |
| Calibre（Dark 主题） | ✅ |
| Kindle KFX（夜间模式） | ⚠️ 仅 Kindle Colorsoft / 较新固件；旧固件忽略 |
| KOReader | ✅ |

**Kindle 老固件不触发时**，画面退化为正常显示——黑线条在黑底上看不清，但仍可读。这是可接受的"优雅降级"。

### 1.5 alt 文本写作规范

| 图片类型 | alt 应该写什么 |
|---|---|
| 信息图 / 图表 | 用文字描述图的关键结论，不复述数字 |
| 装饰图 / 分隔线 | `alt=""`（空字符串，不是没写） |
| 章前插图 | 描述画面主体，简短 |
| 生僻字 / 公式图 | 写出该字 / 公式的纯文本形式 |
| Logo | 写品牌名 |

**`alt` 永远要写**，EPUBCheck 会强制检查。

### 1.6 封面 OPF 写法（EPUB 3 标准）

`content.opf` 内的封面声明（**两套元数据同时写，兼容老阅读器**）：

```xml
<metadata>
  <!-- EPUB 2 风格：兼容 ADE 等老阅读器 -->
  <meta name="cover" content="cover-image"/>
  ...
</metadata>

<manifest>
  <!-- EPUB 3 风格：properties="cover-image" -->
  <item id="cover-image"
        href="images/cover.jpg"
        media-type="image/jpeg"
        properties="cover-image"/>
  <item id="cover-page"
        href="text/cover.xhtml"
        media-type="application/xhtml+xml"/>
  ...
</manifest>

<spine>
  <itemref idref="cover-page"/>
  ...
</spine>
```

封面 `cover.xhtml` 内只放一个 `<img>`，不要加文字 / 边框：

```xml
<body epub:type="cover">
  <div class="cover">
    <img src="../images/cover.jpg" alt="封面"/>
  </div>
</body>
```

```css
.cover { margin: 0; padding: 0; text-align: center; }
.cover img { max-width: 100%; max-height: 100vh; height: auto; }
```

**封面图尺寸建议**：宽 1600 × 高 2560 px（约 1:1.6），JPEG 质量 85，文件 ≤ 500 KB。

---
## 二、整版海报页（全屏背景 + 角落贴图 + 横竖排文字）

适用：章扉页、卷首图、题献页、画册式插图页。要求：

- 背景图铺满当前渲染页，可压缩或轻微裁剪
- 整版**不随字号变化**
- 整版**不跨页**
- 横排书里也能局部嵌竖排文字
- 角落 / 中央可叠多张图与文字层
- 跨阅读器（Apple Books / Thorium / Calibre / KFX / KOReader / 多看）

**前提**：放在**独立 XHTML 文件**里（如 `poster-ch01.xhtml`），列入 `spine`。不要塞进正文章节中——海报会被正文样式污染。

#### HTML 模板

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      lang="zh-CN" xml:lang="zh-CN">
<head>
  <title>第一章 扉页</title>
  <link rel="stylesheet" type="text/css" href="../styles/main.css"/>
  <link rel="stylesheet" type="text/css" href="../styles/poster.css"/>
</head>
<body class="poster-body" epub:type="frontmatter">
  <section class="poster">
    <!-- 第 1 层：背景图（必有，永远在最底） -->
    <img class="poster-bg" src="../images/poster-bg.jpg" alt="封面背景"/>

    <!-- 第 2 层：四角贴图 -->
    <img class="poster-deco tl" src="../images/seal.png" alt=""/>
    <img class="poster-deco tr" src="../images/icon.png" alt=""/>
    <img class="poster-deco br" src="../images/sign.png" alt=""/>

    <!-- 第 3 层：堆叠图（z-index 显式控制顺序） -->
    <img class="poster-stack" style="left: 30%; top: 35%; width: 25%; z-index: 5;"
         src="../images/portrait-back.png" alt=""/>
    <img class="poster-stack" style="left: 38%; top: 40%; width: 25%; z-index: 6;"
         src="../images/portrait-front.png" alt=""/>

    <!-- 第 4 层：文字 —— 横排标题 -->
    <p class="poster-title-h">第一章　江南旧事</p>

    <!-- 第 5 层：文字 —— 竖排副题（书整体横排时也可用） -->
    <p class="poster-title-v">一九〇五年·苏州</p>
  </section>
</body>
</html>
```

#### CSS 模块（`poster.css`，独立文件）

```css
/* —— 海报页 body：抹掉主样式表里 body 的 padding —— */
body.poster-body {
  margin: 0;
  padding: 0;
  font-size: 16px;           /* 锁定字号，不跟随用户字号设置 */
}

/* —— 海报容器：占满 viewport，不跨页 —— */
.poster {
  position: relative;
  width: 100vw;
  height: 100vh;
  margin: 0;
  padding: 0;
  overflow: hidden;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
  break-inside: avoid;       /* 新规范名（CSS Fragmentation 3） */
}

/* —— 背景图：覆盖全屏，可裁剪 —— */
.poster-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;         /* 铺满，多余部分裁掉 */
  object-position: center;
  margin: 0;
  z-index: 1;
}

/* —— 四角贴图基础样式 —— */
.poster-deco {
  position: absolute;
  width: 18%;                /* 用百分比，跟 viewport 而非字号 */
  height: auto;
  margin: 0;
  z-index: 2;
}
.poster-deco.tl { top: 4%;    left: 4%; }
.poster-deco.tr { top: 4%;    right: 4%; }
.poster-deco.bl { bottom: 4%; left: 4%; }
.poster-deco.br { bottom: 4%; right: 4%; }

/* —— 可堆叠图（用 inline style 控制位置与层级） —— */
.poster-stack {
  position: absolute;
  height: auto;
  margin: 0;
  /* left/top/width/z-index 在 HTML 里写 inline，更直观 */
}

/* —— 横排标题层 —— */
.poster-title-h {
  position: absolute;
  top: 45%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 80%;
  margin: 0;
  text-align: center;
  text-indent: 0;
  font-size: 2.2em;          /* em 基于 .poster-body 的 16px，可控 */
  font-weight: bold;
  font-family: "Songti SC", "STSong", "Source Han Serif SC", serif;
  letter-spacing: 0.1em;
  color: #fff;
  text-shadow: 0 2px 6px rgba(0, 0, 0, 0.6);
  z-index: 10;
}

/* —— 竖排副题层（横排书里局部竖排） —— */
.poster-title-v {
  position: absolute;
  top: 8%;
  right: 8%;
  margin: 0;
  writing-mode: vertical-rl;
  -webkit-writing-mode: vertical-rl;
  -epub-writing-mode: vertical-rl;
  text-orientation: mixed;
  -webkit-text-orientation: mixed;
  -epub-text-orientation: mixed;
  font-size: 1.4em;
  font-family: "Kaiti SC", "STKaiti", "LXGW WenKai", serif;
  color: #f0e0c0;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.7);
  letter-spacing: 0.15em;
  z-index: 10;
}
```

#### 关键规则（不要违反）

1. **独立 XHTML 文件**：在 `content.opf` 的 `<spine>` 单独列 `<itemref idref="poster-ch01"/>`，让 body 样式不与正文冲突
2. **尺寸单位用 `vw / vh / %`**，不用 `em / rem`——这是"不随字号变"的根本
3. **`.poster-body` 锁 `font-size: 16px`**，给标题用 `em` 时基数固定
4. **背景图用 `<img>` + `object-fit: cover`**，比 `background-image` 在 KFX 上更稳
5. **`page-break-*` 三条都写**：`before always` / `after always` / `inside avoid`，把海报隔成独立一页
6. **背景图分辨率**：宽 ≥ 1600 px，长边 ≤ 2400 px；JPEG 质量 80–85；单张文件 ≤ 800 KB
7. **`writing-mode` 加 `-webkit-` 与 `-epub-` 前缀**：兼容老 KFX / 老 Apple Books

#### 老 KFX 的优雅降级

老 KFX 固件不识别 `vh` 时，`.poster` 高度退化为内容自然高度——背景图按自己原始比例显示，标题文字按 em 放大。**画面会变小、不再占满屏，但不会错位、不乱码**。

这是可接受的降级；如果一定要"老 Kindle 也铺满"，唯一办法是把整张海报**直接做成单张 JPG**，HTML 里只放 `<img>`（即"图片即整页"），但就失去叠加文字与角落贴图的能力了。

#### 叠加更多图层 / 文字层

把 `.poster-deco` / `.poster-stack` / `.poster-title-h` / `.poster-title-v` 复制多份，`z-index` 递增即可。`z-index` 在所有主流阅读器都支持。

```xml
<img class="poster-stack" style="left: 60%; top: 20%; width: 30%; z-index: 7;"
     src="../images/cloud.png" alt=""/>
<p style="position:absolute; left:10%; bottom:25%; z-index:8;
          writing-mode:vertical-rl; font-family:'Kaiti SC';">
  题记：往事如烟
</p>
```

#### 阅读器兼容矩阵（海报页特化）

| 特性 | Apple Books | Thorium | Calibre | KFX | KOReader |
|---|---|---|---|---|---|
| `100vw / 100vh` 单位 | ✅ | ✅ | ✅ | ⚠️ 老固件降级 | ✅ |
| `object-fit: cover` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `position: absolute` + `z-index` | ✅ | ✅ | ✅ | ✅ | ✅ |
| 元素级 `writing-mode` | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| `text-shadow` | ✅ | ✅ | ✅ | ⚠️ 墨水屏不显示 | ⚠️ |
| `transform: translate(-50%,-50%)` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `page-break-inside: avoid` | ✅ | ✅ | ✅ | ✅ | ✅ |

#### vw / vh 不被识别时的 fallback

老 KFX 固件 / 老 ADE 可能不识别 `vw`/`vh`。三种替代方案，按推荐度排序：

##### Fallback A：比例 padding hack（推荐，纯 CSS 全员兼容）

原理：`padding-bottom` 用百分比时基于**父元素的宽度**计算。`156%` ≈ Kindle Paperwhite 长宽比 2560÷1640。文字放大不影响盒子高度。

```css
.poster {
  position: relative;
  width: 100%;
  padding-bottom: 156%;          /* 锁定 1 : 1.56 长宽比 */
  height: 0;                     /* 高度由 padding 撑 */
  margin: 0;
  overflow: hidden;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
}

/* 子元素改为按"容器盒"绝对定位，不再依赖 100vh */
.poster > * {
  position: absolute;
}
.poster-bg {
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  z-index: 1;
}
/* 四角、堆叠图、标题层的 top/left/right/bottom 百分比写法不变，
   现在它们基于 .poster 的真实盒子（宽 × 1.56），效果等同 vh 方案 */
```

**适配不同设备比例**：

```css
.poster.ratio-paperwhite { padding-bottom: 156%; }  /* 2560/1640 */
.poster.ratio-oasis      { padding-bottom: 160%; }  /* 1680/1264 */
.poster.ratio-iphone     { padding-bottom: 178%; }  /* 16/9 */
.poster.ratio-ipad       { padding-bottom: 133%; }  /* 4/3 */
.poster.ratio-square     { padding-bottom: 100%; }  /* 1:1 */
```

主流书选 **156%（接近多数墨水屏）** 或 **150%（A 系列纸）** 做主版式，宽屏阅读器上方会留点空白带——这是可接受的"信封"效果。

##### Fallback B：单页 Pre-paginated（混排 FXL，官方支持）

EPUB 3.3 标准允许 **整本 reflowable，单页 FXL**——这是出版社做章前插图的主流。

`content.opf` 里给海报页单独的 properties：

```xml
<spine page-progression-direction="ltr">
  <itemref idref="ch01" linear="yes"/>
  <itemref idref="poster-ch02"
           properties="rendition:layout-pre-paginated rendition:spread-none"
           linear="yes"/>
  <itemref idref="ch02" linear="yes"/>
</spine>
```

海报 XHTML 头部加 viewport：

```xml
<head>
  <title>第二章扉页</title>
  <meta name="viewport" content="width=1640, height=2560"/>
  <link rel="stylesheet" href="../styles/poster.css"/>
</head>
```

CSS 改用绝对像素（因为现在容器是固定 1640×2560 的"画布"）：

```css
.poster { position: relative; width: 1640px; height: 2560px; margin: 0; }
.poster-bg { position: absolute; top: 0; left: 0; width: 1640px; height: 2560px; object-fit: cover; }
.poster-title-h { position: absolute; top: 1152px; left: 0; width: 1640px; text-align: center; font-size: 96px; }
.poster-deco.tl { position: absolute; top: 100px; left: 100px; width: 290px; }
/* … 等等 */
```

| 阅读器 | FXL 单页支持 |
|---|---|
| Apple Books | ✅ |
| Thorium | ✅ |
| Calibre | ✅ |
| Kindle KFX | ✅（2018+ KFX 8.x 起） |
| 多看 | ✅ |
| KOReader | ⚠️ 显示为图片但布局简化 |
| 老 ADE | ❌ 退化为普通 reflowable 页 |

**好处**：跨设备像素级一致；**坏处**：失去 reflowable 的自适应、用户字号设置被忽略。**适合"必须保证视觉一致"的章前画 / 题词页**。

##### Fallback C：整页 JPG（终极保底）

把整张海报（含背景 + 角落贴图 + 文字）在 Photoshop 里合成一张图，HTML 里只放：

```xml
<body class="poster-body" epub:type="frontmatter">
  <div class="poster-img-only">
    <img src="../images/poster-ch01-rendered.jpg" alt="第一章扉页"/>
  </div>
</body>
```

```css
.poster-body { margin: 0; padding: 0; }
.poster-img-only { text-align: center; }
.poster-img-only img {
  max-width: 100%;
  max-height: 100vh;             /* 在支持的设备上撑满 */
  height: auto;
  display: block;
  margin: 0 auto;
  page-break-before: always;
  page-break-after: always;
  page-break-inside: avoid;
}
```

**100% 兼容**，但失去"叠层 / 文字可选 / 夜间反相"能力。**适合纯装饰、不含正文意义的整版插图**。

##### 三种 fallback 怎么选

| 需求 | 选哪个 |
|---|---|
| 想保留文字层 + 角落贴图，且要老 Kindle 也能跑 | **A 比例 padding hack** |
| 需要像素级一致（设计师交稿，差一点都不行） | **B Pre-paginated 单页** |
| 整页就是装饰、文字内容不重要 | **C 整页 JPG** |
| 默认（首选） | §9.14 主体的 vw/vh 方案；再叠 A 作 fallback class |

**并存写法**：默认用 vw/vh，老阅读器自动降级到 padding hack：

```css
/* 默认：现代阅读器用 vh */
.poster { width: 100vw; height: 100vh; position: relative; }

/* 老阅读器：vh 被忽略时盒子高度=0，用 @supports 兜底 */
@supports not (height: 100vh) {
  .poster {
    width: 100%;
    padding-bottom: 156%;
    height: 0;
  }
}
```

`@supports` 在 Apple Books / Thorium / Calibre / 新 KFX 都识别；不识别的老设备走 padding hack 路径——两条路都通。

---

## 三、对主手册的补丁建议（本主题相关）

| 主手册位置 | 改动 |
|---|---|
| §六 基础样式表 / 图片 | 追加 §一.2 的绕排、行内图、clearfix |
| §七 常见组件 / 图片 | 追加绕排、行内图、夜间反相示例 |
| §九 避坑清单 / 图片与字体 | 加一行"WebP / AVIF / HEIC 全拒"；"单图 ≤ 2000 px"补理由 |
| §九 避坑清单 / 文件与结构 | 补一行"整版海报页独立 xhtml 文件，不要塞进章节中" |
| §十一 验证测试清单 | 补"切夜间模式确认 prefers-color-scheme 反相生效"；"竖排书测翻页方向是否正确" |

---

## 四、阅读器兼容矩阵（节选）

| 特性 | Apple Books 4.x | Thorium 3.x | Calibre 7.x | Kindle KFX | KOReader 2024+ |
|---|---|---|---|---|---|
| 图片绕排（`float`） | ✅ | ✅ | ✅ | ⚠️ 旧固件忽略 | ✅ |
| 行内图（`<img>` 在 `<p>` 内） | ✅ | ✅ | ✅ | ✅ | ✅ |
| `prefers-color-scheme: dark` | ✅ | ✅ | ✅ | ⚠️ Colorsoft / 新固件 | ✅ |
| `<picture>` 多分辨率 | ✅ | ✅ | ✅ | ❌ 取首张 | ✅ |
| `object-fit: cover` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `100vw / 100vh` 单位 | ✅ | ✅ | ✅ | ⚠️ 老固件降级 | ✅ |
| 比例 `padding-bottom: %` hack | ✅ | ✅ | ✅ | ✅ | ✅ |
| Pre-paginated 单页（混排 FXL） | ✅ | ✅ | ✅ | ✅ KFX 8+ | ⚠️ |
| `position: absolute` + `z-index` | ✅ | ✅ | ✅ | ✅ | ✅ |
| 元素级 `writing-mode`（局部竖排） | ✅ | ✅ | ✅ | ✅ | ⚠️ |

> ✅ 正常 ⚠️ 部分版本 / 部分场景 ❌ 不支持

---

**文档版本**：2026-05-15
**对应标准**：EPUB 3.3
**关联文档**：`EPUB3_制作完全参考手册.md`（主手册）、《EPub指南——从入门到放弃》（赤霓，2023-04-18）§10.4–10.5