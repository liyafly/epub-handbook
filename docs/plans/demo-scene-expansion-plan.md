# demo 场景扩展计划

> 状态：仅文档；待执行模型按本文落地到 `templates/epub-style-demo/`。  
> 范围：在现有 00–09 之上，新增 10–17 共 8 个 demo 页，并补 `base.css` 的视觉类、`fonts.css`（见 [fonts-css-expansion-plan.md](./fonts-css-expansion-plan.md)）、`nav.xhtml`、`package.opf`、`toc.ncx`、`SCENE_MATRIX.md`、`docs/final/reader-matrix.yaml`。  
> 协同文档：[duokan-footnote-fallback-fix.md](./duokan-footnote-fallback-fix.md)（13 号页直接采用新 fallback 结构）、[fonts-css-expansion-plan.md](./fonts-css-expansion-plan.md)（字体类放在 fonts.css，本文档不重复字体声明）。

---

## 1. 现状与缺口

现有 10 页已覆盖：标题页、普通正文、Ruby+标准弹注、A-lite 海报、列表/表格/代码、多看 fallback（单/多）、font-family、长段中英混排、Kindle 风险。

仍未覆盖（且终极手册 §8 / 用户实测样本里频繁出现）：

| 缺口 | 出现频率 | 备注 |
|---|---|---|
| 着重号 `.emp`、波浪线 `.wavy` 独立类 | 高 | 终极手册 §8.1/8.2 已写规则，但 `base.css` 没有对应类 |
| 行内楷体 `.kaiti` | 高 | 用户样本 `<span class="kaiti">` |
| 首字下沉 `.dropcap` | 中 | 中文文学正文常见 |
| 场景分隔 `.scene-break`（星号 / 装饰符） | 中 | 章内分场常用 |
| 诗节 `.poetry` / `.stanza` | 中 | 中英诗排 |
| 对话格式 `.dialog` | 中 | 小说 |
| 信件块 `.letter` | 低 | 文学体裁 |
| 章首图 + 章节副标题 `.chapter-head` / `.subtitle` | 高 | 用户样本 `class="r imgh"` / `class="r title"` |
| 全页竖排正文（非 A-lite 海报） | 中 | 终极手册 §十 局部竖排页 |
| 版权页 / 题献 / 题记 frontmatter | 中 | EPUB 3 `epub:type` 完整覆盖 |
| 多看 fallback 富文本一体页 | 高 | 用户样本就是这种结构，需作为 fallback 主回归 |
| 简单数学公式 | 中 | 化学下标、幂次、勾股、∫、∑、希腊字母；HTML+sup/sub 为主，MathML 为补充 |
| 图文混排（九宫格）| 高 | 一段一图，覆盖上/下/中 + 左/右浮动 + 四角浮动 9 个位置 |

## 2. 新增页面清单

| 序号 | 文件 | 主题 | 关键检查点 |
|---|---|---|---|
| 10 | `Text/10-text-effects.xhtml` | 文字效果合集 | `.emp` / `.wavy` / `.kaiti` / `.dropcap` / 多种 Ruby 用法 / 嵌套 q |
| 11 | `Text/11-chapter-opening.xhtml` | 章首结构 | 章首图、章节大标题、副标题 `<span class="sptxt">`、卷头引文 |
| 12 | `Text/12-literary-fiction.xhtml` | 小说体综合 | 对话、诗节、场景分隔、信件块、首字下沉 |
| 13 | `Text/13-duokan-rich-fallback.xhtml` | 多看 fallback 富文本一体页 | 完整结构 + 正确的 duokan 类位置（见 fix 文档） |
| 14 | `Text/14-vertical-body.xhtml` | 整页正文竖排（非 A-lite） | `writing-mode: vertical-rl` 段落、`text-orientation:mixed`、注释跳转 |
| 15 | `Text/15-frontmatter.xhtml` | 版权 / 题献 / 题记 | `epub:type="frontmatter"`、`copyright-page`、`dedication`、`epigraph` |
| 16 | `Text/16-math.xhtml` | 简单数学公式 | 行内 sup/sub、化学式、勾股、∫/∑、希腊字母、`.math-block`、可选 MathML |
| 17 | `Text/17-image-layout.xhtml` | 图文九宫格 | `.img-top` / `.img-bottom` / `.img-center` / `.img-left` / `.img-right` / `.img-tl` / `.img-tr` / `.img-bl` / `.img-br` + `.clear-both` |

序号 10–17 紧接当前 09 之后；不打散已有编号，避免破坏 reader-matrix 历史 artifact。

---

## 3. 各页面 XHTML（直接落库）

> 通用规则：所有页面声明 `<?xml version="1.0" encoding="UTF-8"?>` + `<!DOCTYPE html>` + `xmlns:epub`；`<head>` 至少引 `../Styles/fonts.css` + `../Styles/base.css`，其余按 [css-layering-plan.md §2.3 矩阵](./css-layering-plan.md) 选用对应的 `notes / effects / literary / media / vertical` 层。下面各页 XHTML 的 `<link>` 已按矩阵填齐。

### 3.1 `Text/10-text-effects.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>文字效果合集</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>十、文字效果合集</h1>

    <h2>着重号</h2>
    <p>普通正文里出现<span class="emp">着重</span>字样：旧式着重号采用 `text-emphasis: filled dot`，并在 `text-emphasis-position: under` 时落在汉字下方。和 <em>HTML em</em> 的语义区分：`em` 表语义强调，`.emp` 表视觉着重。</p>

    <h2>波浪线</h2>
    <p>专名号 / 书名号的替代视觉：<span class="wavy">波浪线下划线</span>用于人名、地名或强调。不支持 `text-decoration-style: wavy` 的阅读器会退化为普通下划线，可读性不变。</p>

    <h2>行内楷体</h2>
    <p>普通宋体正文中夹带<span class="kaiti">行内楷体</span>：常见于引文、人物心声、信函片段。整段楷体请给容器加 `class="book-kai"`（或短别名 `class="kai"`），避免单段重复 span。</p>

    <h2>首字下沉</h2>
    <p class="dropcap-host"><span class="dropcap">汪</span>曾祺写草木与食物，常以平实的句子起头，再让句子自己散开。首字下沉用 `.dropcap` 单独包裹首字，浮动到段落左侧；阅读器若不支持浮动，自然退化为大字号首字，不影响阅读。</p>

    <h2>Ruby 注音的两种写法</h2>
    <p class="has-ruby">
      简写型（无 rp 兜底）：<ruby>燕<rt>yān</rt></ruby><ruby>京<rt>jīng</rt></ruby>。
      完整型（带 rp 括号兜底）：<ruby><rb>燕</rb><rp>（</rp><rt>yān</rt><rp>）</rp></ruby><ruby><rb>京</rb><rp>（</rp><rt>jīng</rt><rp>）</rp></ruby>。
      多字一注：<ruby>北京<rt>Běijīng</rt></ruby>。
    </p>

    <h2>行内引用与嵌套</h2>
    <p>他抬头说：<q>所谓<q>家乡味</q>，不过是少年的牙口。</q>说完低下头。嵌套 `q` 应自动切换为内层引号；不支持 `quotes` 的阅读器会显示同一对外引号，可读性不变。</p>

    <h2>组合压力</h2>
    <p class="has-ruby">人名注音与着重号叠加：<span class="emp"><ruby>沈<rt>shěn</rt></ruby><ruby>从<rt>cóng</rt></ruby><ruby>文<rt>wén</rt></ruby></span>。Ruby + emphasis 的行间距由 `.has-ruby` 段落整体 `line-height` 保证，不要单独给 `ruby` 加 margin。</p>
  </section>
</body>
</html>
```

### 3.2 `Text/11-chapter-opening.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>章首结构</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>
</head>
<body>
  <section epub:type="chapter">
    <figure class="chapter-head">
      <img src="../Images/poster.png" alt="章首装饰"/>
    </figure>

    <h1 class="chapter-title">十一、章首结构<br/><span class="sptxt">——一个示例的副标题</span></h1>

    <p class="epigraph">"凡所写，先要自己读得下去。"<span class="epigraph-source">——卷首引语</span></p>

    <p class="dropcap-host"><span class="dropcap">那</span>是一个秋天的下午，他第一次走进这座小城。城里只有一条街，街上只有一家店。门口挂的招牌字迹已经褪色，但还是<span class="emp">认得出</span>。</p>

    <p>他在店门口停了一会儿，看见两个老人在下棋。<span class="wavy">小城</span>的午后总是安静的，连蝉鸣都低了一调。</p>
  </section>
</body>
</html>
```

### 3.3 `Text/12-literary-fiction.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>小说体综合</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>十二、小说体综合</h1>

    <h2>对话</h2>
    <div class="dialog">
      <p><span class="dialog-speaker">老周：</span>"你来了。"</p>
      <p><span class="dialog-speaker">客　：</span>"我来了。"</p>
      <p><span class="dialog-speaker">老周：</span>"坐吧。"</p>
    </div>
    <p>对话块用 `.dialog` 包裹一组 `<p>`，每段开头放说话人；不依赖 table、不依赖固定缩进，重排时仍可对齐。</p>

    <h2>诗节</h2>
    <div class="poetry">
      <p class="stanza">
        山有木兮木有枝，<br/>
        心悦君兮君不知。
      </p>
      <p class="stanza">
        采采卷耳，不盈顷筐。<br/>
        嗟我怀人，寘彼周行。
      </p>
    </div>
    <p>诗节用 `.poetry &gt; p.stanza`，每段段首不缩进，行末 `<br/>` 换行；阅读器调大字号时整节仍可换行不裁切。</p>

    <h2>场景分隔</h2>
    <p>上一场景结束。</p>
    <hr class="scene-break"/>
    <p>下一场景开始：那天傍晚他去了码头。</p>
    <p class="scene-break-text">＊　＊　＊</p>
    <p>也可以用文字版分隔符（适合不支持 `border` 的旧阅读器）。</p>

    <h2>信件块</h2>
    <div class="letter">
      <p class="letter-salutation">先生台鉴：</p>
      <p>久未通问，近况想必安泰。前次所赠书已读毕，受益匪浅。</p>
      <p>专此奉复，顺颂</p>
      <p class="letter-close">秋祺</p>
      <p class="letter-signature">某某　顿首<br/>九月廿三日</p>
    </div>

    <h2>首字下沉</h2>
    <p class="dropcap-host"><span class="dropcap">他</span>把信折好，放进抽屉最底层。第二天清早起身，去码头看船。船上没有他想等的人，只有几只海鸥。</p>
  </section>
</body>
</html>
```

### 3.4 `Text/13-duokan-rich-fallback.xhtml`

> 本页采用 [duokan-footnote-fallback-fix.md](./duokan-footnote-fallback-fix.md) §3 的目标结构（`duokan-footnote-content` 挂 `<ol>`，不挂 `<li>`）。是多看 fallback 的回归 fixture。

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>多看 fallback 富文本一体页</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/notes.css"/>
</head>
<body>
  <section epub:type="chapter">
    <figure class="chapter-head">
      <img src="../Images/poster.png" alt="章首装饰"/>
    </figure>

    <h1 class="chapter-title">十三、复仇（节选）<sup><a id="note-rich-1" class="noteref-icon duokan-footnote" epub:type="noteref" role="doc-noteref" href="#footnote-rich-1"><img alt="注" src="../Images/note.png"/></a></sup><br/><span class="sptxt">——给一个孩子讲的故事</span></h1>

    <p>一缶客茶，半支素烛，主人的<span class="emp">深情</span>。</p>

    <p><q>今夜竟挂了单呢</q>，年青人想想颇自好笑。</p>

    <p>他的周身结束告诉曾经长途行脚的人，这样的一个人，走到这样冷僻的地方，即使身上没有带着钱粮，也会自己设法寻找一点东西来慰劳一天的<ruby>跋<rt>bá</rt></ruby><ruby>涉<rt>shè</rt></ruby>，山上多的是<span class="wavy">松鸡野兔子</span>。</p>

    <p>他记起离家的前夕，母亲替他<ruby>裹<rt>guǒ</rt></ruby>了行囊，抽出这剑跟他说了许多话，那些话是他<span class="kaiti">已经背得烂熟</span>了的<sup><a id="note-rich-2" class="noteref-icon duokan-footnote" epub:type="noteref" role="doc-noteref" href="#footnote-rich-2"><img alt="注" src="../Images/note.png"/></a></sup>。</p>

    <blockquote>
      <p>这剑必须饮我底<span class="emp">仇人</span>的血！</p>
    </blockquote>

    <p>当他还在母亲的肚里的时候，父亲死了，滴尽了最后一滴血，只吐出这一句话。</p>
  </section>

  <aside epub:type="footnote" role="doc-footnote">
    <div><hr class="footnote-line"/></div>
    <ol class="footnote-list duokan-footnote-content">
      <li class="footnote-item duokan-footnote-item" id="footnote-rich-1">
        <p class="footnote">
          <a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#note-rich-1">◎</a>
          本页演示四种正文文字效果：<span class="emp">着重号</span>（filled dot under）、<span class="wavy">波浪线</span>（不支持时退化为下划线）、拼音注音 <ruby>字<rt>zì</rt></ruby>（ruby+rt）、行内引用 <q>楷体引用</q> 与<span class="kaiti">内联楷体</span>（`.kaiti`）；块引用见正文剑句。
        </p>
      </li>
      <li class="footnote-item duokan-footnote-item" id="footnote-rich-2">
        <p class="footnote">
          <a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#note-rich-2">◎</a>
          多看 fallback 关键点：`duokan-footnote-content` 挂在 `<ol>` 上，`duokan-footnote-item` 挂在 `<li>` 上；`<li>` 不再持有 `duokan-footnote-content`。详见 docs/guides/duokan-footnote-fallback-fix.md §3。
        </p>
      </li>
    </ol>
  </aside>
</body>
</html>
```

### 3.5 `Text/14-vertical-body.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>整页正文竖排</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/effects.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/vertical.css"/>
</head>
<body class="page-vrl">
  <section class="vrl-section" epub:type="chapter">
    <h1 class="vrl-title">十四、竖排正文</h1>
    <p>本页演示整页正文竖排（writing-mode: vertical-rl），而不是 A-lite 海报页。整本书的 spine 仍保持 ltr，仅在本页局部启用竖排。</p>
    <p>竖排段落里仍可使用 Ruby <ruby>字<rt>zì</rt></ruby>、着重号<span class="emp">深情</span>与行内引用 <q>引文</q>；西文 ABC 123 在 `text-orientation: mixed` 下保持横向，符合中日韩竖排习惯。</p>
    <p>不支持竖排的阅读器会自动退化为横排，正文仍可读。</p>
  </section>
</body>
</html>
```

### 3.6 `Text/15-frontmatter.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>版权 / 题献 / 题记</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/literary.css"/>
</head>
<body>
  <section epub:type="frontmatter">
    <section class="copyright-page" epub:type="copyright-page">
      <h1>版权信息</h1>
      <p>书　　名：EPUB Style Demo</p>
      <p>作　　者：EPUB Handbook</p>
      <p>出　　版：示例</p>
      <p>版　　次：2026 年 5 月第 1 版</p>
      <p>ISBN：000-0-00000-000-0</p>
      <p class="copyright-statement">版权所有，未经许可不得以任何形式复制或转载本书内容。</p>
    </section>

    <section class="dedication" epub:type="dedication">
      <h1>题献</h1>
      <p class="dedication-text">谨以此书献给所有耐心读完中文排版规范的人。</p>
    </section>

    <section class="epigraph-page" epub:type="epigraph">
      <h1>题记</h1>
      <blockquote>
        <p>"文字要经得起换设备。"</p>
        <p class="epigraph-source">——本项目卷首语</p>
      </blockquote>
    </section>
  </section>
</body>
</html>
```

### 3.7 `Text/16-math.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>简单数学公式</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/media.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>十六、简单数学公式</h1>
    <p>本页用最朴素的 HTML 元素覆盖中学到大学常见的数学/化学公式；不依赖 JavaScript、不依赖外部公式渲染服务。所有阅读器都能至少回退到「带上下标的纯文本」。</p>

    <h2>行内公式（sup / sub）</h2>
    <p>勾股定理：<i>a</i><sup>2</sup> + <i>b</i><sup>2</sup> = <i>c</i><sup>2</sup>。</p>
    <p>化学式：H<sub>2</sub>O、CO<sub>2</sub>、SO<sub>4</sub><sup>2−</sup>、Fe<sup>3+</sup>。</p>
    <p>幂运算：2<sup>10</sup> = 1024；Euler 恒等式 e<sup>i&#960;</sup> + 1 = 0。</p>

    <h2>希腊字母与运算符（Unicode）</h2>
    <p>圆周率 &#960; ≈ 3.14159；自然底数 e ≈ 2.71828；黄金比 &#966; ≈ 1.618。</p>
    <p>常用希腊：&#945; &#946; &#947; &#948; &#949; &#952; &#955; &#956; &#960; &#961; &#963; &#964; &#966; &#967; &#968; &#969;。</p>
    <p>常用运算符：&#177; &#215; &#247; &#8800; &#8804; &#8805; &#8776; &#8801; &#8734; &#8733; &#8712; &#8713; &#8834; &#8835; &#8746; &#8745; &#8730; &#8721; &#8747;。</p>

    <h2>块级公式（.math-block）</h2>
    <div class="math-block"><i>E</i> = <i>m</i><i>c</i><sup>2</sup></div>
    <div class="math-block"><i>a</i><sup>2</sup> + <i>b</i><sup>2</sup> = <i>c</i><sup>2</sup></div>
    <div class="math-block">&#8747;<sub>0</sub><sup>1</sup> <i>x</i><sup>2</sup> d<i>x</i> = 1 / 3</div>
    <div class="math-block">&#8721;<sub><i>n</i>=1</sub><sup>&#8734;</sup> 1 / <i>n</i><sup>2</sup> = &#960;<sup>2</sup> / 6</div>

    <h2>分数（HTML 模拟）</h2>
    <p>简单分数用 Unicode 字符：&#189; &#8531; &#188; &#8532; &#190;。</p>
    <p>复杂分数用 <span class="math-fraction"><span class="num">a + b</span><span class="den">c + d</span></span>；不支持时退化为「(a + b) / (c + d)」可读结构。</p>

    <h2>开方（HTML 模拟）</h2>
    <p>简单开方用 Unicode：&#8730;2 ≈ 1.414。</p>
    <p>带上划线：<span class="math-sqrt"><i>x</i><sup>2</sup> + <i>y</i><sup>2</sup></span>。不支持时退化为「√(x² + y²)」。</p>

    <h2>可选：MathML（支持的阅读器会渲染，其他阅读器留空白）</h2>
    <p>下式用 MathML 表达；Apple Books / Thorium 渲染良好，Kindle Previewer / 多看 / KOReader 支持有限。若必须保证视觉一致请改回上面的 HTML + sup/sub。</p>
    <div class="math-block">
      <math xmlns="http://www.w3.org/1998/Math/MathML" display="block">
        <msup><mi>a</mi><mn>2</mn></msup>
        <mo>+</mo>
        <msup><mi>b</mi><mn>2</mn></msup>
        <mo>=</mo>
        <msup><mi>c</mi><mn>2</mn></msup>
      </math>
    </div>
    <div class="math-block">
      <math xmlns="http://www.w3.org/1998/Math/MathML" display="block">
        <mfrac>
          <mrow><mi>a</mi><mo>+</mo><mi>b</mi></mrow>
          <mrow><mi>c</mi><mo>+</mo><mi>d</mi></mrow>
        </mfrac>
      </math>
    </div>
  </section>
</body>
</html>
```

### 3.8 `Text/17-image-layout.xhtml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN"
      lang="zh-CN">
<head>
  <title>图文九宫格</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/media.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>十七、图文九宫格</h1>
    <p>本页演示一段一图、图片在九个位置时的排版。每段开头放图片，靠 float / 块级居中 / clear 控制视觉位置。每段只放一张图，避免段内多图导致阅读器换行不稳定。</p>

    <h2>1. 上方（block-top）</h2>
    <figure class="img-top">
      <img src="../Images/poster.png" alt="上方图"/>
    </figure>
    <p>图片放在段落上方，作为独立 figure 块。这种写法在所有阅读器都稳定，常用于章节插图、示意图。</p>

    <h2>2. 下方（block-bottom）</h2>
    <p>图片放在段落下方，结构与上方相同，仅 DOM 顺序不同。常用于「说明 + 配图」的图注式排版。</p>
    <figure class="img-bottom">
      <img src="../Images/poster.png" alt="下方图"/>
      <figcaption>下方配图。</figcaption>
    </figure>

    <h2>3. 居中（center，无环绕）</h2>
    <p>图片整段居中，前后文字不环绕。适合突出展示插图、地图、海报；依赖 `display: block; margin: 1em auto`，跨阅读器稳定。</p>
    <figure class="img-center">
      <img src="../Images/poster.png" alt="居中图"/>
    </figure>
    <p>居中段后再写一段普通文字，可观察图片不会影响前后段的字号、行高。</p>

    <h2>4. 左浮动（left，文字右侧环绕）</h2>
    <figure class="img-left">
      <img src="../Images/poster.png" alt="左浮动图"/>
    </figure>
    <p>图片左浮动，文字从右侧环绕。这一段连续写若干字以观察环绕：段落里塞入中英文混排 Mixed text together with Chinese 来确认 float 在 Apple Books、Thorium、KOReader、Kindle Previewer 上的稳定性。当段落短于图片高度时，下一段会继续环绕，直到出现 clear。</p>
    <p class="clear-both">用 `.clear-both` 强制结束环绕；后续段落回到全宽布局。</p>

    <h2>5. 右浮动（right，文字左侧环绕）</h2>
    <figure class="img-right">
      <img src="../Images/poster.png" alt="右浮动图"/>
    </figure>
    <p>图片右浮动，文字从左侧环绕。中文阅读习惯里，右浮动用于把图片靠版心右边缘，与左侧文字主轴呼应。这一段也连续写些字来观察实际换行：当窄屏阅读器把图片缩到一行级别时，环绕会退化为图片置顶 + 文字另起一段，仍然可读。</p>
    <p class="clear-both">结束右浮动环绕。</p>

    <h2>6–9. 角落定位（top-left / top-right / bottom-left / bottom-right）</h2>
    <p>九宫格的四角位置在 reflowable EPUB 里只能用 float + DOM 顺序模拟，不能精确定位到「页面四角」。`<figure>` 必须放在 `<p>` 之外（XHTML 5 不允许块级元素嵌进 `<p>`，否则解析器会强制闭合），靠 float 让正文从图片旁边绕过去。</p>

    <h3>6. 左上角（img-tl）：figure 放在段落**之前**，float-left</h3>
    <figure class="img-tl">
      <img src="../Images/poster.png" alt="左上角图"/>
    </figure>
    <p>「左上角」等价于段首 float-left；这里 figure 是 `<p>` 的兄弟节点，紧贴下一段段首。继续写一段长文字以包出环绕区域，观察图片占据段首左上方、文字从段首右侧开始绕图。窄屏会退化为图片在段顶、文字在下方。</p>
    <p class="clear-both"></p>

    <h3>7. 右上角（img-tr）：figure 放在段落**之前**，float-right</h3>
    <figure class="img-tr">
      <img src="../Images/poster.png" alt="右上角图"/>
    </figure>
    <p>「右上角」等价于段首 float-right；同样 figure 在 `<p>` 之前。写一段长文字观察文字从段首左侧开始绕图。同样窄屏退化为图片置顶。</p>
    <p class="clear-both"></p>

    <h3>8. 左下角（img-bl）：figure 插在**两段之间**，float-left</h3>
    <p>「左下角」无法在 reflowable 里精确实现；技术上是把 figure 放在两段之间，由于它向左浮动并向后延伸，会落在上一段的"段尾左下"。下面这一段是上文：连续写若干字让段落足够长。</p>
    <figure class="img-bl">
      <img src="../Images/poster.png" alt="左下角图"/>
    </figure>
    <p>这一段是下文，会从图片右侧开始环绕，于是图片视觉上停在"上一段段尾、下一段左侧"的位置——也就是九宫格里的左下区。继续中英混排 keep the paragraph long enough to wrap properly around the float。</p>
    <p class="clear-both"></p>

    <h3>9. 右下角（img-br）：figure 插在**两段之间**，float-right</h3>
    <p>「右下角」同理：figure 插在两段之间，使用 float-right。这一段也写较长文字给上下文做铺垫。</p>
    <figure class="img-br">
      <img src="../Images/poster.png" alt="右下角图"/>
    </figure>
    <p>下文从图片左侧开始环绕，让图片靠在视觉上的右下位置。继续中英混排 keep paragraph long enough so the trailing right float can sit at the bottom-right。</p>
    <p class="clear-both"></p>

    <h2>注意事项</h2>
    <ul>
      <li>`<figure>` 是块级元素，**必须**作为 `<p>` 的兄弟节点出现，不允许嵌在 `<p>` 内（XHTML 5 解析器会强制闭合 `<p>` 导致结构错位）。</li>
      <li>每段只放一张图；段内多图会导致阅读器换行不稳定。</li>
      <li>所有 float 段落组后建议跟一个 `.clear-both` 段，避免下一标题或段落仍被环绕。</li>
      <li>四角位置不是「页面坐标」，而是「DOM 流中的浮动位置」，受阅读器宽度、字号、`text-align: justify` 影响；窄屏会退化为块级图片。</li>
      <li>图片 `max-width` 在 `.img-left / .img-right / .img-tl / .img-tr / .img-bl / .img-br` 都建议设为 40–50%，避免吃掉整行。</li>
      <li>`figcaption` 仅在 `.img-top / .img-bottom / .img-center` 等块级位置可靠居中；浮动 figure 里写 figcaption 容易被环绕文字打断，建议改用段尾文字承载图注。</li>
    </ul>
  </section>
</body>
</html>
```

---

## 4. CSS 样式落点（按 [css-layering-plan.md](./css-layering-plan.md) §2 的 8 文件分层）

> 本节按 CSS 文件分组列出 10–17 各页需要的视觉类。所有"哪个类放哪个文件"的归属决策遵循 `css-layering-plan.md §2.1` 的契约表，不要再把这些类堆回 `base.css`。

### 4.1 `base.css` 清理

`base.css` 后半部分（约 line 234–414）出现整段重复（`.demo-list / table / pre / .title-page / @media` 块被复制了一遍），落本计划前**先删除重复段**。删除后 `base.css` 应只包含：reset / `html/body` / `h1–h6` / `p` / `ul/ol/dl` / `table` / `pre/code/kbd/samp` / `figure/img/figcaption` / `a` / `em/strong/q/blockquote` / `ruby/rt/rp` 默认样式、`.has-ruby` 行距兜底 + 必要的 `@page` 与窄屏 `@media`。不允许再出现弹注、文字效果、文学结构、图文浮动、海报、竖排相关规则——它们分别迁到 `notes.css / effects.css / literary.css / media.css / poster.css / vertical.css`。

验证：

```bash
# 重复块自检
awk '/^\.demo-list li \{$/{c++} END{print c}' templates/epub-style-demo/OEBPS/Styles/base.css
# 应输出 1，不是 2
# base.css 不应出现这些组件类
grep -E '^\.(emp|wavy|dropcap|chapter-head|dialog|poetry|letter|copyright-page|dedication|epigraph|img-(left|right|center|top|bottom|tl|tr|bl|br)|math-(block|inline|fraction|sqrt)|footnote-(line|list|item|back)|noteref-icon|duokan-footnote|vrl-|page-vrl|poster-|fullpage|fullframe)' \
  templates/epub-style-demo/OEBPS/Styles/base.css
# 应无输出
```

### 4.2 新建 `OEBPS/Styles/notes.css`

把现 `base.css` 里所有弹注相关规则**剪切**到这里，并保留 `sup` 的微调；02/05/06/13 号页 link 这一层。

```css
@charset "utf-8";

/* 弹注（含多看 fallback；与 docs/guides/duokan-footnote-fallback-fix.md §3 一致） */

sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon,
a.duokan-footnote {
  text-decoration: none;
}

.noteref-icon img,
a.duokan-footnote img {
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

ol.duokan-footnote-content {
  list-style-type: none;
  padding: 0;
  margin: 0;
}

.footnote-item,
.duokan-footnote-item {
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
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
```

### 4.3 新建 `OEBPS/Styles/effects.css`

10/11/12/13/14/16 号页 link 这一层。

```css
@charset "utf-8";

/* 文字效果（终极手册 §8） */

.emp {
  font-style: normal;
  text-emphasis: filled dot;
  text-emphasis-position: under;
  -webkit-text-emphasis-style: filled dot;
  -webkit-text-emphasis-position: under;
  -epub-text-emphasis-style: filled dot;
  -epub-text-emphasis-position: under;
}

.wavy {
  text-decoration-line: underline;
  text-decoration-style: wavy;
  text-decoration-color: #c03030;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.12em;
}

/* `.kaiti` 字体声明在 fonts.css；此处不重复。 */

/* 首字下沉 */
.dropcap-host {
  text-indent: 0;
}

.dropcap {
  float: left;
  margin: 0.05em 0.1em 0 0;
  padding: 0;
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
  font-size: 3em;
  line-height: 0.9;
}

```

### 4.4 新建 `OEBPS/Styles/literary.css`

11/12/13/15 号页 link 这一层。

```css
@charset "utf-8";

/* 章首图与章节标题 */
.chapter-head {
  margin: 0 0 1em;
  text-align: center;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}

.chapter-head img {
  max-width: 70%;
  height: auto;
}

.chapter-title {
  margin: 0.6em 0 0.5em;
  text-align: center;
  line-height: 1.3;
}

.chapter-title .sptxt {
  display: inline-block;
  margin-top: 0.3em;
  font-size: 0.6em;
  font-weight: normal;
}

.epigraph {
  margin: 1.2em 1.5em;
  text-indent: 0;
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
  text-align: center;
}

.epigraph-source {
  display: block;
  margin-top: 0.4em;
  font-size: 0.85em;
}

/* 对话 */
.dialog p {
  margin: 0.35em 0;
  text-indent: 0;
}

.dialog-speaker {
  display: inline-block;
  min-width: 4em;
  margin-right: 0.25em;
  font-weight: 700;
}

/* 诗节 */
.poetry {
  margin: 1em 0;
}

.poetry .stanza {
  margin: 0.6em 0;
  padding-left: 1em;
  text-indent: 0;
  line-height: 1.9;
}

/* 场景分隔 */
.scene-break {
  width: 30%;
  margin: 1.4em auto;
  border: none;
  border-top: 1px solid #9b9488;
}

.scene-break-text {
  margin: 1em 0;
  text-align: center;
  text-indent: 0;
  letter-spacing: 0.5em;
}

/* 信件块 */
.letter {
  margin: 1em 1.5em;
  padding: 0.6em 1em;
  border-left: 0.18em solid #c8b89b;
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
}

.letter p {
  margin: 0.3em 0;
}

.letter-salutation {
  text-indent: 0;
  font-weight: 700;
}

.letter-close,
.letter-signature {
  text-indent: 0;
  text-align: right;
}

/* 版权 / 题献 / 题记前置页 */
.copyright-page,
.dedication,
.epigraph-page {
  margin: 1.5em 0;
  page-break-after: always;
  -webkit-page-break-after: always;
}

.copyright-page p {
  margin: 0.3em 0;
  text-indent: 0;
}

.copyright-statement {
  margin-top: 1em;
  font-size: 0.88em;
}

.dedication-text {
  margin: 2em 1em;
  text-align: center;
  text-indent: 0;
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
  font-size: 1.05em;
}
```

### 4.5 新建 `OEBPS/Styles/media.css`

16（math）/ 17（图文九宫格）号页 link 这一层。包含两部分：图片浮动九宫格、数学公式。

```css
@charset "utf-8";

/* ===== 图文九宫格 ===== */

/* 1) 上 / 下 / 中：块级，不环绕 */
.img-top,
.img-bottom,
.img-center {
  display: block;
  margin: 1em auto;
  text-align: center;
  page-break-inside: avoid;
  -webkit-page-break-inside: avoid;
}

.img-top img,
.img-bottom img,
.img-center img {
  max-width: 80%;
  height: auto;
}

/* 2) 左 / 右浮动 */
.img-left {
  float: left;
  margin: 0.3em 1em 0.5em 0;
  max-width: 45%;
}

.img-right {
  float: right;
  margin: 0.3em 0 0.5em 1em;
  max-width: 45%;
}

.img-left img,
.img-right img {
  max-width: 100%;
  height: auto;
}

/* 3) 四角：段首浮动 = 上左/上右；段尾浮动 = 下左/下右 */
.img-tl {
  float: left;
  margin: 0 1em 0.5em 0;
  max-width: 40%;
}

.img-tr {
  float: right;
  margin: 0 0 0.5em 1em;
  max-width: 40%;
}

.img-bl {
  float: left;
  margin: 0.5em 1em 0 0;
  max-width: 40%;
}

.img-br {
  float: right;
  margin: 0.5em 0 0 1em;
  max-width: 40%;
}

.img-tl img,
.img-tr img,
.img-bl img,
.img-br img {
  max-width: 100%;
  height: auto;
}

/* 4) 结束浮动 */
.clear-both {
  clear: both;
  margin: 0;
  padding: 0;
  text-indent: 0;
}

/* 5) 窄屏退化：< 420px 时所有浮动退化为块级 */
@media (max-width: 420px) {
  .img-left,
  .img-right,
  .img-tl,
  .img-tr,
  .img-bl,
  .img-br {
    float: none;
    margin: 1em auto;
    max-width: 90%;
    display: block;
    text-align: center;
  }
}

/* ===== 数学公式 ===== */

.math-block {
  margin: 1em 0;
  text-align: center;
  text-indent: 0;
  line-height: 1.6;
  /* 数学公式以西文衬线为主；中文字符走系统 CJK fallback。链 ≤ 4 段。 */
  font-family: "Iowan Old Style", "Palatino", Georgia, serif;
}

.math-block math {
  font-size: 1.05em;
}

.math-inline {
  font-style: normal;
  white-space: nowrap;
}

/* 模拟分数 a/b：上下两行 */
.math-fraction {
  display: inline-block;
  vertical-align: middle;
  text-align: center;
  line-height: 1;
  margin: 0 0.15em;
}

.math-fraction .num {
  display: block;
  border-bottom: 1px solid currentColor;
  padding: 0 0.25em;
}

.math-fraction .den {
  display: block;
  padding: 0 0.25em;
}

/* 模拟开方：根号在前 + 上划线 */
.math-sqrt {
  display: inline-block;
  border-top: 1px solid currentColor;
  padding: 0 0.15em;
  margin-left: 0.05em;
}

.math-sqrt::before {
  content: "\221A";          /* √ */
  margin-right: 0.1em;
  font-family: serif;
}

/* sup / sub 在公式块里的微调（不影响正文 sup/sub） */
.math-block sup,
.math-block sub,
.math-inline sup,
.math-inline sub {
  font-size: 0.72em;
  line-height: 0;
}
```

### 4.6 新建 `OEBPS/Styles/vertical.css`

14 号页 link 这一层。

```css
@charset "utf-8";

/* 整页正文竖排（非 A-lite） */

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
  height: 100%;
  line-height: 1.85;
}

.vrl-section p {
  margin: 0 0 0 1em;
  text-indent: 2em;
}

.vrl-title {
  margin: 0 0 0 1.5em;
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
  font-weight: normal;
  line-height: 1.25;
}
```

### 4.7 `fonts.css`

按 [fonts-css-expansion-plan.md §2](./fonts-css-expansion-plan.md) 整体替换，不在此重复。

### 4.8 拆分后整体自检

```bash
# 1) 每个 CSS 文件 400 行预警、500 行硬上限
for f in templates/epub-style-demo/OEBPS/Styles/*.css; do
  lines=$(wc -l < "$f")
  [ "$lines" -gt 400 ] && echo "WARN $f ($lines lines)"
  [ "$lines" -gt 500 ] && echo "OVERSIZE $f ($lines lines)"
done
# 2) 每个 XHTML 里 link 的所有 CSS 都在 manifest
# 3) 每个 manifest 里的 CSS 都真实存在
ls templates/epub-style-demo/OEBPS/Styles/
# 应出现：fonts.css base.css notes.css effects.css literary.css media.css vertical.css poster.css
```

---

## 5. `OEBPS/nav.xhtml` 更新

在现有 `<ol>` 末尾追加：

```xml
      <li><a href="Text/10-text-effects.xhtml">文字效果合集</a></li>
      <li><a href="Text/11-chapter-opening.xhtml">章首结构</a></li>
      <li><a href="Text/12-literary-fiction.xhtml">小说体综合</a></li>
      <li><a href="Text/13-duokan-rich-fallback.xhtml">多看 fallback 富文本一体页</a></li>
      <li><a href="Text/14-vertical-body.xhtml">整页正文竖排</a></li>
      <li><a href="Text/15-frontmatter.xhtml">版权 / 题献 / 题记</a></li>
      <li><a href="Text/16-math.xhtml">简单数学公式</a></li>
      <li><a href="Text/17-image-layout.xhtml">图文九宫格</a></li>
```

landmarks 段无需变动（封面与正文开始已声明）。

---

## 6. `OEBPS/package.opf` 更新

manifest 段追加 8 条 XHTML item：

```xml
    <item id="text-effects"        href="Text/10-text-effects.xhtml"        media-type="application/xhtml+xml"/>
    <item id="chapter-opening"     href="Text/11-chapter-opening.xhtml"     media-type="application/xhtml+xml"/>
    <item id="literary-fiction"    href="Text/12-literary-fiction.xhtml"    media-type="application/xhtml+xml"/>
    <item id="duokan-rich"         href="Text/13-duokan-rich-fallback.xhtml" media-type="application/xhtml+xml"/>
    <item id="vertical-body"       href="Text/14-vertical-body.xhtml"       media-type="application/xhtml+xml"/>
    <item id="frontmatter"         href="Text/15-frontmatter.xhtml"         media-type="application/xhtml+xml"/>
    <item id="math"                href="Text/16-math.xhtml"                media-type="application/xhtml+xml"/>
    <item id="image-layout"        href="Text/17-image-layout.xhtml"        media-type="application/xhtml+xml"/>
```

manifest 段同时追加 5 条新 CSS item（按 [css-layering-plan.md §4.1](./css-layering-plan.md) 的 8 文件分层）：

```xml
    <item id="css-notes"           href="Styles/notes.css"                   media-type="text/css"/>
    <item id="css-effects"         href="Styles/effects.css"                 media-type="text/css"/>
    <item id="css-literary"        href="Styles/literary.css"                media-type="text/css"/>
    <item id="css-media"           href="Styles/media.css"                   media-type="text/css"/>
    <item id="css-vertical"        href="Styles/vertical.css"                media-type="text/css"/>
```

spine 段追加（顺序：放在 `kindle-risk` 之后）：

```xml
    <itemref idref="text-effects"/>
    <itemref idref="chapter-opening"/>
    <itemref idref="literary-fiction"/>
    <itemref idref="duokan-rich"/>
    <itemref idref="vertical-body"/>
    <itemref idref="frontmatter"/>
    <itemref idref="math"/>
    <itemref idref="image-layout"/>
```

> 备注：当前 OPF 的 `<meta property="ibooks:specified-fonts">true</meta>` 作为通用预防默认保留（理由见 [fonts-css-expansion-plan.md §4](./fonts-css-expansion-plan.md)）。即便 demo 未实际嵌字体，也不要删它。

---

## 7. `OEBPS/toc.ncx` 更新

`navMap` 末尾追加（playOrder 递增，从 11 起到 18）：

```xml
    <navPoint id="navPoint-11" playOrder="11">
      <navLabel><text>文字效果合集</text></navLabel>
      <content src="Text/10-text-effects.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-12" playOrder="12">
      <navLabel><text>章首结构</text></navLabel>
      <content src="Text/11-chapter-opening.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-13" playOrder="13">
      <navLabel><text>小说体综合</text></navLabel>
      <content src="Text/12-literary-fiction.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-14" playOrder="14">
      <navLabel><text>多看 fallback 富文本一体页</text></navLabel>
      <content src="Text/13-duokan-rich-fallback.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-15" playOrder="15">
      <navLabel><text>整页正文竖排</text></navLabel>
      <content src="Text/14-vertical-body.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-16" playOrder="16">
      <navLabel><text>版权 / 题献 / 题记</text></navLabel>
      <content src="Text/15-frontmatter.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-17" playOrder="17">
      <navLabel><text>简单数学公式</text></navLabel>
      <content src="Text/16-math.xhtml"/>
    </navPoint>
    <navPoint id="navPoint-18" playOrder="18">
      <navLabel><text>图文九宫格</text></navLabel>
      <content src="Text/17-image-layout.xhtml"/>
    </navPoint>
```

> 若现有 ncx playOrder 起点不是 1（请先读现有文件），按现状续接即可。

---

## 8. `templates/epub-style-demo/SCENE_MATRIX.md` 更新

在现有表格末尾追加：

```markdown
| 文字效果合集 | `Text/10-text-effects.xhtml` | `.emp` / `.wavy` / `.kaiti` / `.dropcap` / Ruby 两种写法 / 嵌套 q | 全部 |
| 章首结构 | `Text/11-chapter-opening.xhtml` | 章首图、章节大标题、副标题 `.sptxt`、卷头引文 | Apple Books / Kindle Previewer |
| 小说体综合 | `Text/12-literary-fiction.xhtml` | 对话、诗节、场景分隔、信件块、首字下沉 | Apple Books / Thorium / KOReader |
| 多看 fallback 富文本一体页 | `Text/13-duokan-rich-fallback.xhtml` | 章首图 + 标题 + emp/wavy/kaiti/ruby + 多看 fallback（content 在 ol） | 多看 / 标准阅读器回退 |
| 整页正文竖排 | `Text/14-vertical-body.xhtml` | `writing-mode: vertical-rl`、`text-orientation: mixed`、Ruby 与 emp 竖排叠加 | Apple Books / Thorium / KOReader |
| 版权 / 题献 / 题记 | `Text/15-frontmatter.xhtml` | `epub:type="copyright-page" / dedication / epigraph` 语义页 | 全部 |
| 简单数学公式 | `Text/16-math.xhtml` | `sup/sub`、`.math-block`、希腊字母 / 运算符 Unicode、可选 MathML | Apple Books / Thorium（MathML）/ 其他阅读器（HTML 退化）|
| 图文九宫格 | `Text/17-image-layout.xhtml` | 9 位置：top/bottom/center/left/right/tl/tr/bl/br + `.clear-both` + 窄屏退化 | 全部 |
```

---

## 9. `docs/final/reader-matrix.yaml` 更新

`cases:` 段追加：

```yaml
  - id: 10-text-effects
    title: 文字效果合集
    fixture: templates/epub-style-demo/OEBPS/Text/10-text-effects.xhtml
  - id: 11-chapter-opening
    title: 章首结构
    fixture: templates/epub-style-demo/OEBPS/Text/11-chapter-opening.xhtml
  - id: 12-literary-fiction
    title: 小说体综合
    fixture: templates/epub-style-demo/OEBPS/Text/12-literary-fiction.xhtml
  - id: 13-duokan-rich
    title: 多看 fallback 富文本一体页
    fixture: templates/epub-style-demo/OEBPS/Text/13-duokan-rich-fallback.xhtml
  - id: 14-vertical-body
    title: 整页正文竖排
    fixture: templates/epub-style-demo/OEBPS/Text/14-vertical-body.xhtml
  - id: 15-frontmatter
    title: 版权 / 题献 / 题记
    fixture: templates/epub-style-demo/OEBPS/Text/15-frontmatter.xhtml
  - id: 16-math
    title: 简单数学公式
    fixture: templates/epub-style-demo/OEBPS/Text/16-math.xhtml
  - id: 17-image-layout
    title: 图文九宫格
    fixture: templates/epub-style-demo/OEBPS/Text/17-image-layout.xhtml
```

`readers:` 段追加（多看 fallback 验证用）：

```yaml
  - id: duokan
    name: 多看阅读（含 Duokan 协议复刻版本）
```

`expectations:` 段：本次仅新增 fixture，不要预填 `status`；执行模型按"demo 先行"流程跑构建+目标阅读器实测，再回写 status / artifact / 现象。所有 13 号页相关条目要带 `pending duokan verification` 备注，方便后续追溯。

---

## 10. 落地顺序（给执行模型）

> 与项目 CLAUDE.md "demo 先行、文档后补"一致。

1. 先按 [css-layering-plan.md §6](./css-layering-plan.md) 在 `OEBPS/Styles/` 下创建 `notes.css / effects.css / literary.css / media.css / vertical.css` 五个新文件，按 §4.1 清理 `base.css` 重复段并把弹注规则迁出；OPF manifest 同步加 5 条 CSS item（见本文 §6）。
2. 按 §3 写入 8 个 XHTML（10–17；**不动** 05/06，由 [duokan-footnote-fallback-fix.md](./duokan-footnote-fallback-fix.md) §4.4 单独处理）。
3. 按 §4.2–§4.6 把 demo 视觉类填入对应 CSS 层；按 [fonts-css-expansion-plan.md §2](./fonts-css-expansion-plan.md) 整体替换 `fonts.css`；OPF 的 `<meta property="ibooks:specified-fonts">true</meta>` **保留**为通用预防默认（见 fonts-css-expansion-plan.md §4）。
4. 同步 §5–§9 的 nav / opf / ncx / SCENE_MATRIX / reader-matrix（**仅 cases / readers**，不预填 expectations）。
5. 跑 `sh templates/epub-style-demo/build.sh`，产物落 dist。
6. 执行：
   ```bash
   # XHTML 良构（每个文件）
   xmllint --noout templates/epub-style-demo/OEBPS/Text/1*.xhtml
   # OPF 中所有 item 都能在磁盘找到
   rg -oP 'href="[^"]+"' templates/epub-style-demo/OEBPS/package.opf
   # ZIP 检查 mimetype 是首条
   unzip -p templates/epub-style-demo/dist/*.epub mimetype
   # 链接、id 不重复
   for f in templates/epub-style-demo/OEBPS/Text/1*.xhtml; do
     rg -oP 'id="[^"]+"' "$f" | sort | uniq -d
   done
   # 每个 CSS 文件 400 行预警、500 行硬上限
   for f in templates/epub-style-demo/OEBPS/Styles/*.css; do
     lines=$(wc -l < "$f")
     [ "$lines" -gt 400 ] && echo "WARN $f ($lines lines)"
     [ "$lines" -gt 500 ] && echo "OVERSIZE $f ($lines lines)"
   done
   ```
7. Kindle Previewer / Apple Books / Thorium / 多看 / KOReader 打开 dist EPUB；按 SCENE_MATRIX 每页过一遍。13/16/17 是本次新增的高优先级回归页。
8. 把每次实测结果写入 reader-matrix.yaml 的 expectations 段，附 reader version、artifact、现象、状态。

落地后请勾选：

- [ ] 8 个新 XHTML（10–17）已加入 OPF manifest 与 spine、nav.xhtml、toc.ncx。
- [ ] `OEBPS/Styles/` 下出现 8 个 CSS 文件，且各文件 ≤ 500 行（400 行提示规划拆分）。
- [ ] base.css 重复块已删除，弹注规则已迁到 notes.css，未保留任何组件类。
- [ ] fonts.css 已整体替换为新模板，且 demo 仍能不依赖第三方字体打包。
- [ ] 每个 XHTML 的 `<link>` 顺序符合 [css-layering-plan.md §2.2](./css-layering-plan.md)，且只 link 自己用到的层。
- [ ] SCENE_MATRIX.md 表格扩展（含 16 / 17 行）。
- [ ] reader-matrix.yaml 增加 cases（10–17），readers 增加 duokan，expectations 留空待实测。
- [ ] dist 包能在 Apple Books / Kindle Previewer 打开；13 号页待多看实测确认；16 号页 MathML 部分在 Kindle / 多看 / KOReader 可允许空白；17 号页四角浮动在窄屏下退化为块级。
- [ ] 多看实测通过后，再触发 duokan-footnote-fallback-fix.md §4.1–4.3 的 SPEC/手册/skill 同步。
- [ ] SPEC §7 已按 [css-layering-plan.md §5](./css-layering-plan.md) 替换为 8 行新表。
- [ ] SPEC §8 与终极手册 §四 已按 [fonts-css-expansion-plan.md §6](./fonts-css-expansion-plan.md) 改写。
