# fonts.css 扩展与字体回退模板

> 状态：仅文档；待执行模型按本文落地到 `templates/epub-style-demo/OEBPS/Styles/fonts.css`（以及真实书的 `Styles/fonts.css`）。  
> 范围：补全系统字体回退清单 + `@font-face` 注释骨架 + 字体工具类 + 与 OPF 配合的检查清单。  
> 上位约束：`docs/final/SPEC-实现约束.md §7 / §8`、终极手册 §四。  
> **本次新增**：策略反转（系统字体优先、嵌入字体仅作特定需求专用），同步给出 SPEC §8 与终极手册 §四的改写清单（见末节 §6）。

---

## 1. 策略与适用场景

目标：**一份 EPUB 在 iOS / macOS / Apple Books / Kindle / Windows / Android / Linux / KOReader / 多看 / Thorium 上同时可用，且尽量减少包体与权属负担。**

由此推出的字体策略：

1. **默认正文 / 标题 / 引用 / 等宽不嵌字体**。  
   `font-family` 链直接由"各平台系统字体 → 跨平台开源 CJK 字体 → generic family"组成。**默认链 ≤ 4 段**，按 SPEC §8 顺序。
2. **嵌入字体仅在以下三种场景出现**：  
   (a) 大量生僻字（系统字体缺字、需子集字库托底）；  
   (b) 设计上必须使用某款特定字体（题签、卷头题字、签名档等）；  
   (c) **(a) 和 (b) 同时存在**——既要保持设计字形又要兜底生僻字。
3. **嵌入字体的三种排布模式**（按场景选其一，不要在默认 body / h* 链里嵌字体）：

   | 模式 | 适用 | 链长 | 链结构 | 示例 |
   |---|---|---|---|---|
   | **A 设计专用** | 题签 / 卷头题字 / 签名档 | ≤ 2 段 | 嵌入字体 + generic | `.title-special { font-family: "BookTitleSpecial", serif; }` |
   | **B 生僻字专用** | 生僻字子集字库 | ≤ 2 段 | 嵌入字体 + generic | `.rare { font-family: "RareSongSubset", serif; }` |
   | **C 嵌入 + 系统字体复合** | 正文/标题需要嵌入字体支援，同时保留系统字体兜底 | **≤ 5 段** | 4 段系统字体链中**只**插入 1 段嵌入字体，位置为**第 1 位**或**倒数第 2 位**（二选一） | C1：`"BookSongDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", serif`<br/>C2：`"Songti SC", "SimSun", "Noto Serif CJK SC", "RareSongSubset", serif` |

   模式 C 的两种变体（按需求二选一）：

   - **C1 设计前置**：嵌入"设计字体"放第 1 位。Apple Books / Kindle Previewer（开启 Publisher Font）等支持嵌入字体的阅读器会优先呈现设计字形；其他阅读器嵌字加载失败时退回系统字体仍可读。**适用**：以设计字形为主，对生僻字不敏感。
   - **C2 嵌入兜底**：嵌入"生僻字子集字库"或"补字字库"放倒数第 2 位。绝大多数字符走系统字体；系统缺字的字符落到嵌入字库，避免豆腐 / .notdef。**适用**：以可读为主，仅个别生僻字需要补字。

   **整条链里嵌入字体只出现一次**——要么放第 1 位（C1），要么放倒数第 2 位（C2），不能两处都放。如果一本书既要设计字形又要兜底生僻字，请用两个不同的类（C1 挂在 `body` / 章节标题，B 模式 `.rare` 单独 span 包住生僻字），不要把两段嵌入塞进同一条链。

   不要把模式 C 用在默认 `body` / `h*` 元素选择器上；它应挂在专用类（`.book-song-deluxe` / `.book-kai-deluxe` 等），由 XHTML 显式选用。

**例外：模式 C1-body**。当书内存在生僻字、且嵌入"全字符集"字体时，允许把模式 C1
链结构直接挂在 `body` / `h*` 元素选择器（不再要求挂专用类）。这是本节默认禁止
"嵌入字体进 body / h*" 的唯一例外。子集字库不可走本路径——只有全字符集字体才
有资格直接挂 body。详见 SPEC §8 与终极手册 §四"含生僻字的全字符集方案"。
4. **`@font-face` 的存在与启用条件一致**：没有专用类用到它时，就**不**在 `fonts.css` 里写它，OPF 也不挂字体 item。但 `<meta property="ibooks:specified-fonts">true</meta>` 作为通用预防默认始终保留（理由见 §4）。

> 反例（必须避免）：
> - `body { font-family: "BookBodySong", "Songti SC", "Source Han Serif SC", serif; }`  
>   → 默认场景下，body 链塞嵌入字体导致整本书强制携带 6–10 MB 字库；若只是设计需求，应改用模式 C 并挂专用类。**例外**：当书内确有生僻字、且嵌入"全字符集"字体（非子集，`fontspec=forceAll`）时，这种写法属于模式 C1-body，详见 SPEC §8 与终极手册 §四。本反例针对"无生僻字、不需要嵌入也硬挂 body"的常见错误。
> - `.rare { font-family: "RareSong", "BookBodySong", "Songti SC", serif; }`  
>   → 生僻字字体后面又挂正文宋体，缺字时落到系统宋体的豆腐——这是没有解决问题的链。
> - `.book-song-deluxe { font-family: "BookSongDesign", "Songti SC", "SimSun", "RareSongSubset", serif; }`  
>   → 同一条链里嵌入字体出现两次（"BookSongDesign" 与 "RareSongSubset"），违反"嵌入字体只出现 1 次"。要嵌设计 + 补字，请拆成两个类：`.book-song-deluxe` 用 C1 + `.rare` 用模式 B，配合 XHTML 显式标记生僻字。
> - `.book-deluxe { font-family: "BookDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", "OpenSourceCJK", serif; }`  
>   → 6 段超长，违反 ≤ 5 段。模式 C 的系统字体部分**最多 3 段**（Apple + Windows + Android/开源），不允许再加。

---

## 2. 改写后的完整文件（直接替换 fonts.css）

```css
@charset "utf-8";

/* ============================================================
 * fonts.css —— 字体声明与字体工具类
 *
 * 仅放：@font-face、字体工具类（.rare / .title-special 等）。
 * 不放：排版、颜色、分页、布局（这些在 base.css / poster.css）。
 *
 * 策略（同一份 EPUB 跑遍所有阅读器）：
 *   1. 默认正文、标题、引用、等宽 **不嵌字体**，直接用各平台系统字体链
 *      （默认链 ≤ 4 段：Apple + Windows + Android/开源 + generic）。
 *   2. 嵌入字体只用于：(a) 大量生僻字；(b) 设计必须特定字体；(c) 二者同时存在。
 *   3. 嵌入字体单独挂专用类（.rare / .title-special / .book-*-deluxe …），
 *      按场景选模式 A / B / C（详见配套文档 §1）：
 *        - 模式 A 设计专用：链 ≤ 2 段，嵌入 + generic；
 *        - 模式 B 生僻字专用：链 ≤ 2 段，嵌入 + generic；
 *        - 模式 C 嵌入 + 系统字体复合：链 ≤ 5 段，嵌入只出现 1 次
 *          （第 1 位或倒数第 2 位，二选一）。
 *   4. 没有专用类用到 @font-face 时，整段 @font-face 必须保持注释。
 *
 * 详见 SPEC §7 CSS 分层、SPEC §8 字体链规则、终极手册 §四。
 * 嵌入字体启用后，记得同步 OPF manifest item。
 * OPF metadata 的 <meta property="ibooks:specified-fonts">true</meta>
 * 作为通用预防默认始终保留（未嵌字体也保留，详见配套 fonts-css-expansion-plan.md §4）。
 * ============================================================ */


/* ---------- 一、@font-face 骨架（默认全部注释，按需取消） ----------
 *
 * 仅在有专用类（§ 三 B）真正引用某字体时，才取消对应 @font-face 的注释。
 * 不要"先放着以后再说"——未引用的 @font-face 会让 OPF 必须打包对应字体文件
 * （否则 EPUBCheck 失败），多余的字体打包也会让 Apple Books 加载更慢。
 */

/*
@font-face {
  font-family: "RareSongSubset";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/RareSongSubset.ttf") format("truetype");
}

@font-face {
  font-family: "BookTitleSpecial";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/BookTitleSpecial.ttf") format("truetype");
}

@font-face {
  font-family: "BookSignature";
  font-style: normal;
  font-weight: 400;
  src: url("../Fonts/BookSignature.ttf") format("truetype");
}
*/


/* ---------- 二、系统字体参考清单（组装跨平台链时挑选） ----------
 *
 * **默认链 ≤ 4 段**；按 SPEC §8 顺序取一即可，不要全部堆。
 * 推荐顺序：1 个 Apple 系统字体 → 1 个 Windows 系统字体
 *           → 1 个 Android / 跨平台开源 CJK 字体 → 1 个 generic family。
 * （嵌入模式 C 复合链 ≤ 5 段，结构详见配套文档 §1 / SPEC §8。）
 *
 * 简体宋体 / serif（衬线，正文）
 *   Apple (macOS / iOS / Apple Books):   "Songti SC"
 *   Windows:                              "SimSun", "宋体", "NSimSun"
 *   Android (Noto 已预装):                 "Noto Serif CJK SC"
 *   跨平台开源:                            "Source Han Serif SC", "Noto Serif CJK SC"
 *   反例（同家族堆叠）:                     "Songti SC", "STSongti-SC-Regular", "SimSun", "宋体"
 *
 * 简体黑体 / sans-serif（无衬线，标题/UI）
 *   Apple:                                "PingFang SC"
 *   Windows:                              "Microsoft YaHei", "微软雅黑", "SimHei", "黑体"
 *   Android:                              "Noto Sans CJK SC"
 *   跨平台开源:                            "Source Han Sans SC", "Noto Sans CJK SC"
 *
 * 简体楷体（题签 / 引用 / 楷体段落）
 *   Apple:                                "Kaiti SC"（兼容旧名 "STKaiti"）
 *   Windows:                              "KaiTi"（兼容旧名 "楷体"）
 *   跨平台开源:                            "AR PL UKai CN"
 *   Android:                              系统通常不预装楷体；落到 serif
 *
 * 简体仿宋（少用，引文 / 古籍）
 *   Apple:                                "STFangsong"
 *   Windows:                              "FangSong", "仿宋"
 *
 * 繁体宋体（zh-TW serif）
 *   Apple:                                "Songti TC"
 *   Windows:                              "PMingLiU", "新細明體"
 *   Android:                              "Noto Serif CJK TC"
 *   跨平台开源:                            "Source Han Serif TC"
 *
 * 繁体黑体（zh-TW sans-serif）
 *   Apple:                                "PingFang TC"
 *   Windows:                              "Microsoft JhengHei", "微軟正黑體"
 *   跨平台开源:                            "Source Han Sans TC", "Noto Sans CJK TC"
 *
 * 日文 serif（明朝体）
 *   Apple:                                "Hiragino Mincho ProN", "YuMincho"
 *   Windows:                              "Yu Mincho", "MS Mincho"
 *   跨平台开源:                            "Source Han Serif JP", "Noto Serif CJK JP"
 *
 * 日文 sans-serif（黑体 / Gothic）
 *   Apple:                                "Hiragino Sans", "Hiragino Kaku Gothic ProN"
 *   Windows:                              "Yu Gothic", "YuGothic", "Meiryo"
 *   跨平台开源:                            "Source Han Sans JP", "Noto Sans CJK JP"
 *
 * 韩文
 *   Apple:                                "Apple SD Gothic Neo"
 *   Windows:                              "Malgun Gothic"
 *   跨平台开源:                            "Source Han Sans KR", "Noto Sans CJK KR"
 *
 * 西文 serif
 *   Apple Books:                          "Iowan Old Style"
 *   macOS:                                "Palatino", "Palatino Linotype", "Georgia"
 *   Windows:                              "Georgia", "Cambria", "Times New Roman"
 *
 * 西文 sans-serif
 *   Apple:                                "SF Pro Text", "Helvetica Neue", "Helvetica"
 *   Windows:                              "Segoe UI", "Tahoma", "Arial"
 *
 * 等宽
 *   Apple:                                "SF Mono", "Menlo"
 *   Windows:                              "Consolas", "Courier New"
 *   跨平台开源:                            "Source Code Pro", "DejaVu Sans Mono"
 *
 * ---------- 链组装规则（SPEC §8）：
 *   - 链尾必须是 generic family（serif / sans-serif / monospace）。
 *   - 默认链 ≤ 4 段；嵌入模式 C 复合链 ≤ 5 段。
 *   - 不堆叠同一平台的多个别名（Songti SC 与 STSongti-SC-Regular，
 *     或 SimSun 与 宋体，不要同时出现）；"一平台一字体名" 是允许的。
 *   - 嵌入字体 **不进默认链**；要用嵌入字体请去 § 三 B 的专用类。
 * ----------
 */


/* ---------- 三 A、默认跨平台字体工具类（不嵌字体；推荐直接用） ----------
 *
 * 这一组类**不依赖** @font-face，可以直接挂在 XHTML 上。base.css 的 body /
 * 标题 / 等宽规则也建议直接使用同等结构的链（详见 base.css）。
 *
 * 没有 .rare / .title-special / .signature 需求的书，整本书只需要这一组类。
 *
 * 链组装原则：每条链 **≤ 4 段**，覆盖
 *   Apple（macOS / iOS / Apple Books）→ Windows → Android / 开源 → generic family。
 *   特例：`.book-fangsong` 只有 3 段（Android 通常缺仿宋，直接落 serif）。
 *
 * 注意：q / blockquote 等元素选择器属"排版规则"，应在 base.css / literary.css
 * 通过 `font-family: inherit` 与 class 组合来控制，不写在 fonts.css 里。
 * 如需把 blockquote 默认排成楷体，在该书的页面上加 class="book-kai" 即可。
 */

/* ===== 宋体 / serif（正文衬线） ===== */
.book-song,
.song,
.book-body {                            /* .book-body 保留为向后兼容别名 */
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

/* ===== 黑体 / sans-serif（标题、UI） ===== */
.book-hei,
.hei,
.book-heading {                         /* .book-heading 保留为向后兼容别名 */
  font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
}

/* ===== 楷体（引文、题签、文学体） ===== */
.book-kai,
.kai,
.book-kaiti,                            /* .book-kaiti / .kaiti 保留为向后兼容别名 */
.kaiti {
  font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
}

/* ===== 仿宋（古籍、引文） ===== */
.book-fangsong,
.fangsong {
  /* Android 通常无内置仿宋；缺失时退到 serif（多为系统宋体兜底） */
  font-family: "STFangsong", "FangSong", serif;
}

/* ===== 等宽 ===== */
.book-mono,
.mono {
  /* Apple → Windows → 跨平台开源 → generic，恰好 4 段 */
  font-family: "SF Mono", Consolas, "Source Code Pro", monospace;
}

/* ===== 西文衬线（中英混排时给西文显式衬线，可选） ===== */
.book-latin-serif {
  font-family: "Iowan Old Style", "Palatino", Georgia, serif;
}


/* ---------- 三 B、嵌入字体专用类（按 §1 模式 A / B / C 选用） ----------
 *
 * 只在有嵌入字体 (@font-face 已取消注释) 且 OPF manifest 已加 font item 时，
 * 才取消下面对应类的注释。（OPF metadata 的 ibooks:specified-fonts=true
 * 是通用预防默认，与是否嵌字体无关，参见配套文档 §4。）
 */

/* —— 模式 A：设计字体专用（≤ 2 段，嵌入 + generic） ——
 * 卷头题字 / 题签 / 签名档：必须呈现设计字形；链里不挂任何系统字体。
 */
/*
.title-special {
  font-family: "BookTitleSpecial", serif;
}

.signature {
  font-family: "BookSignature", serif;
}
*/

/* —— 模式 B：生僻字子集字库专用（≤ 2 段，嵌入 + generic） ——
 * 嵌入字体只承担 .rare 命中的字符；缺字时落 serif（多为豆腐），方便定位补字。
 */
/*
.rare {
  font-family: "RareSongSubset", serif;
}
*/

/* —— 模式 C：嵌入 + 系统字体复合（≤ 5 段） ——
 * 嵌入字体在链里**只出现 1 次**，位置为第 1 位（C1）或倒数第 2 位（C2）。
 * 二选一，不能两处都放。
 */

/* C1 设计前置：嵌入设计字体放第 1 位 + 跨平台系统字体 + generic
 * 适用：以设计字形为主，对生僻字不敏感
 */
/*
.book-song-deluxe {
  font-family: "BookSongDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

.book-kai-deluxe {
  font-family: "BookKaiDesign", "Kaiti SC", "KaiTi", "AR PL UKai CN", serif;
}
*/

/* C2 嵌入兜底：跨平台系统字体 + 嵌入字库放倒数第 2 位 + generic
 * 适用：以可读为主，仅个别生僻字需要补字字库
 */
/*
.book-song-with-rare {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", "RareSongSubset", serif;
}
*/

/* 既要设计字形又要补字时：不要塞同一条链，请拆成两个类
 * - body / 章节标题挂 .book-song-deluxe（C1）
 * - 生僻字单独 <span class="rare">…</span>（模式 B）
 */


/* ---------- 四、demo 字体顺序对照（07 号样页用） ----------
 *
 * 这一组类**仅用于** 07 号 demo 演示在不同阅读器下"系统优先 / 嵌入优先 /
 * 混合链"三种字体链的实际命中差异，是教学样本，**不是**生产路径。
 * 生产请按 §三 A（默认）/ §三 B（专用类）的命名挂类，例如 .book-song /
 * .book-kai / .rare / .book-song-deluxe，不要直接挂这里的 demo-* 类。
 */

.demo-system-first {
  /* 系统优先（与 .book-song 同链）：Apple + Windows + Android/开源 + generic */
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

.demo-embedded-first {
  /* 嵌入优先：演示嵌入字体放第 1 位时的命中差异。
   * 与模式 C1 结构一致；生产挂载位置取决于场景：
   *   - 仅设计需求 → 模式 C1 挂专用类（如 .book-song-deluxe）；
   *   - 含生僻字 + 全字符集嵌入 → C1-body 例外，可直接挂 body / h*。 */
  font-family: "BookBodySong", "Songti SC", "Source Han Serif SC", serif;
}

.demo-mixed {
  /* 混合链：演示"嵌入字体 + 系统字体"混合时的命中差异。
   * 生产请按模式 C 挂专用类（或 C1-body 例外路径直接挂 body）；
   * 不要直接用本 demo-* 类做生产路径。 */
  font-family: "BookTitleKai", "Kaiti SC", "Source Han Serif SC", serif;
}
```

> 与现有 demo 的差异提示：
> - `.kaiti` 已迁到 § 三 A 默认跨平台链，不再依赖嵌入楷体。
> - `.rare` 仍然存在，但默认**保持注释**（demo 未真正嵌入 RareSongSubset，就不能命中）。
> - `.demo-embedded-first` / `.demo-mixed` 是 07 页教学样本，不是反模式；生产请按模式 C 挂在专用类。

---

## 3. base.css 的正文 / 标题 / 等宽规则（默认就走系统字体）

`base.css` 的 `body`、`h1–h6`、`code/pre/kbd` 三段字体声明也按系统字体链组装。它们**不**写在 fonts.css 里（属于排版规则，归 base.css 管）。

```css
/* base.css 节选 —— 默认字体链，无嵌入字体；与 §三 A 工具类同源 */

body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

h1, h2, h3, h4, h5, h6 {
  font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
}

code, pre, kbd, samp {
  font-family: "SF Mono", Consolas, "Source Code Pro", monospace;
}
```

> 现有 `base.css` 已经接近这个结构；执行模型落地时核对："body 链不要含嵌入字体名"、"标题链不要含嵌入字体名"、"链 ≤ 4 段"、"每段对应不同平台或开源 / generic"。

> 含生僻字、采用 C1-body 模式的真书，`base.css` 的 body / h* 链改写为：
>
> ```css
> body {
>   font-family: "BookSongFull", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
> }
>
> h1, h2, h3, h4, h5, h6 {
>   font-family: "BookHeiFull", "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
> }
> ```
>
> 这是 demo 不演示的场景（demo 不含生僻字），但真书可直接复用。嵌入字体必须是全字符集，OPF manifest 挂字体 item，`fontspec=forceAll`。

---

## 4. 与 OPF 的配合

| 是否启用嵌入字体（§ 三 B 任一类） | OPF manifest 字体 item | `ibooks:specified-fonts` | `fonts.css` 的 @font-face |
|---|---|---|---|
| 否（仅 § 三 A 默认链） | **不**声明 | **保留**（通用预防默认） | 全部保持注释 |
| 是 | 声明对应 `font/ttf` / `font/otf` item | **保留** | 仅取消被引用的 @font-face |

`<meta property="ibooks:specified-fonts">true</meta>` 作为**通用预防默认**始终保留：

- 未嵌入字体时，它告诉 Apple Books"使用我指定的字体链"，避免用户在 Books 里安装的第三方字体覆盖书内排版；此时"指定"的就是 § 三 A 的系统字体链，行为正确无副作用。
- 嵌入字体时，它是 Apple Books 启用书内字体的开关，必须保留。
- 两种场景都保留，OPF 不需要根据是否嵌字体来切换，减少未来切换字体策略时的遗漏点。

> 因此 demo 的现有 `<meta property="ibooks:specified-fonts">true</meta>` 不要删；新书制作时也直接抄进 OPF。

---

## 5. fontspec 三态对齐（SPEC §3 / §4，需要随之更新）

新策略下，三态语义改写为：

| 模式 | 含义 | 嵌入字体打包 | OPF `ibooks:specified-fonts` |
|---|---|---|---|
| `none`（默认） | 全书走系统字体链 | 否 | `true`（通用预防默认，始终保留）|
| `auto` | 仅 .rare / .title-special 等专用类对应的字体打包，按用字裁切 | 是（仅子集） | `true` |
| `forceAll` | 嵌入字体不裁切（整字库）；含生僻字时可走模式 C1-body 直接挂 body / h* | 是 | `true` |

> 与现 SPEC §4 "auto 模式 = 全书 XHTML 实际用字" 不冲突；只是默认推荐从 `auto` 改为 `none`。在 SPEC §4 末尾加一句："默认模式为 `none`；仅当书内出现 § 三 B 专用类时切到 `auto`/`forceAll`。"
> `forceAll` 是模式 C1-body 的必备条件——子集模式（`auto`）不允许 body / h* 直接挂嵌入字体；子集字库一律走 `.rare` 类（模式 B）。

---

## 6. SPEC §8 / 终极手册 §四 同步改写清单

> 状态：本节列出的所有改写已在分支 `codex/modify-files-to-execute-in-order` 落地：
> - SPEC §8 由 `c7f8617` 完成；
> - 终极手册 §一 / §四 / §十二 由 `c7f8617` + `a6f1902` + `f848117` 完成；
> - 速查表 §四 由 `c7f8617` + `f848117` 完成；
> - C1-body 例外的同步追加由 `a6f1902` + `f848117` 完成。
>
> 保留本节作为变更记录，新项目落地参考。

> 本次策略反转之后，SPEC §8 与终极手册 §四的现有措辞与本文档不一致，需要执行模型按下列改写一次性同步。

### 6.1 `docs/final/SPEC-实现约束.md §8` 改写

原文：

```
## 8) 字体链规则

- `font-family` 链最长 4 段：嵌入字体 → 1 个系统中文字体 → 1 个跨平台开源中文字体 → generic family。
- 嵌入字体必须放链首；系统字体仅做未嵌入时兜底。
- 不建议在同一条链里堆叠多个同家族别名（如 `Songti SC` / `STSongti-*`）。
- 生僻字回退建议使用独立类（如 `.rare`）与专用字体，不要塞进 `body` 主链。
```

替换为：

```
## 8) 字体链规则

- 同一份 EPUB 默认走跨平台系统字体链，不嵌入字体；嵌入字体仅用于
  (a) 大量生僻字、(b) 设计上必须的特定字体、(c) (a) 与 (b) 同时存在。
- **默认 `font-family` 链 ≤ 4 段**：1 个 Apple 系统字体 → 1 个 Windows 系统字体
  → 1 个 Android / 跨平台开源 CJK 字体 → generic family（serif/sans-serif/monospace）。
- 嵌入字体不允许出现在默认 `body` / `h*` / `code` 等元素选择器链中，必须挂在
  专用类（`.rare` / `.title-special` / `.book-song-deluxe` 等）上。
- 专用类按场景使用以下三种模式：
  - **模式 A 设计字体专用**（题签 / 卷头题字 / 签名档）：链 ≤ 2 段，
    嵌入字体 + generic family；
  - **模式 B 生僻字子集专用**（`.rare` 类）：链 ≤ 2 段，
    嵌入字体 + generic family；
  - **模式 C 嵌入 + 系统字体复合**：**链 ≤ 5 段**，嵌入字体在链里
    **只出现 1 次**，位置为第 1 位（C1 设计前置）或倒数第 2 位
    （C2 嵌入兜底），二选一；中间为 3 段系统字体（Apple + Windows + Android / 开源 CJK），链尾 generic family。
    - C1 示例：`"BookSongDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", serif`
    - C2 示例：`"Songti SC", "SimSun", "Noto Serif CJK SC", "RareSongSubset", serif`
- 同一条链里嵌入字体出现 ≥ 2 次属反模式；若需"设计字形 + 生僻字兜底"双重
  支援，应拆成两个类（C1 类挂在正文 / 章节，模式 B `.rare` 类用 span 包住
  生僻字），不要塞进同一条链。
- "一平台一字体名" 允许：Apple `Songti SC` + Windows `SimSun` + Android
  `Noto Serif CJK SC` 是跨平台覆盖，不算堆叠。
- 不在同一条链里堆叠**同一平台的多个别名**（如 `Songti SC` + `STSongti-SC-Regular`，
  或 `SimSun` + `宋体`，或 `Microsoft YaHei` + `微软雅黑`）；只保留各平台最常用的英文名。
- 没有专用类引用的 @font-face 必须从 `fonts.css` 删除或保持注释；OPF 不挂
  对应字体 item。
- `<meta property="ibooks:specified-fonts">true</meta>` 作为通用预防默认始终
  保留，与是否嵌入字体无关——未嵌字体时它表示"用我指定的系统字体链"，避免
  Apple Books 里用户的第三方字体覆盖书内排版。
```

### 6.2 `docs/final/EPUB 3 终极实践手册.md §四 字体方案` 改写要点

- §一 总览表里两行：
  - `正文` 行的"最终方案"列改为：`EPUB 3.3 可重排，正文使用各平台系统中文字体链；不嵌入正文字体`。
  - `标题 / 题签 / 特殊排版` 行改为：`仅"必须特定字体"的题签/卷头题字嵌入；其他标题默认走系统黑体链`。
- §4.1 `fonts.css`：把示例 `@font-face` 全部包成"按需启用"注释段（参考本文档 §2）。
- §4.2 正文字体：把 `body { font-family: "BookBodySong", "Songti SC", "Source Han Serif SC", serif; }` 改为：
  ```css
  body {
    font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
  }
  ```
  反例段（"反例：上面的长链别名堆叠…"）保留并扩充一条："反例：把嵌入字体塞进默认正文链——一份 EPUB 跨设备发布时这是常见反模式；若真的需要嵌入设计字体，按本文档 §1 模式 C 复合链挂在专用类上，不要写进 `body`。"
- §4.3 特殊标题字体：保留 `.poster-title { font-family: "BookTitleKai", serif; }` 形态，但**显式说明**"仅当本书启用了 `BookTitleKai` 嵌入字体时才出现该规则（模式 A，链 ≤ 2 段）；否则改用系统楷体链 `.book-kai { font-family: "Kaiti SC", "KaiTi", "AR PL UKai CN", serif; }`"。
- §4.4 生僻字：保留 `.rare { font-family: "RareSong", "BookBodySong", serif; }` 这段，但**改为反例提示**："旧写法把 RareSong 后面挂正文宋体，缺字时落系统宋体的豆腐——这是反模式。三种推荐写法（按需求选一）：(模式 B 纯生僻字) `.rare { font-family: "RareSongSubset", serif; }`；(模式 C1 设计前置) `.book-song-deluxe { font-family: "BookSongDesign", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }`；(模式 C2 嵌入兜底) `.book-song-with-rare { font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", "RareSongSubset", serif; }`。"
- §二 / §三 / §四 模板示例：`Fonts/` 目录、OPF `Fonts/*` item、`@font-face` 示例都必须标明"仅在嵌入字体场景下需要 / 保留 / 启用"。默认路径保留 `ibooks:specified-fonts=true`，删除字体 item，并让 `@font-face` 保持注释。
- §七 / §八 示例：弹注 `.footnote` 与 `rt` 默认继承 body 字体链；`q, blockquote` 如需楷体，使用系统楷体链 `"Kaiti SC", "KaiTi", "AR PL UKai CN", serif`。这些基础组件不应出现 `BookBodySong`，也不应把两段嵌入字体塞进同一条链。
- §十二 检查清单"字体"段：
  - 删除 `- [ ] 正文主字体为书内授权字体。`
  - 新增 `- [ ] 默认正文 / 标题 / 等宽走系统字体链，不嵌字体。`
  - 新增 `- [ ] 嵌入字体仅出现在专用类（模式 A / B / C），不进 body / h* 等元素选择器。`
  - 新增 `- [ ] 任一字体链的链尾必须是 generic family（serif / sans-serif / monospace）。`
  - 保留 `- [ ] 生僻字使用子集字体。`
  - 保留 `- [ ] Kindle Previewer 测试 Publisher Font 开关。`

### 6.3 `skills/*/SKILL.md` 顺带核对

- 若 skill 中出现"嵌入字体放链首"或"正文 font-family 包含 BookBodySong"等示例，请按 §6.1 / §6.2 同步反过来；目前已知 `epub-popup-footnote-converter` / `epub-legacy-footnote-fallback` 不涉及字体策略，无需改。

---

## 7. 验证清单（执行模型落地后跑一遍）

- [ ] `fonts.css` 内**没有**排版/颜色/分页/布局规则（SPEC §7）。
- [ ] **默认链**（`.book-song` / `.book-hei` / `.book-kai` / `.book-fangsong` / `.book-mono` 与短别名 / `body` / `h1–h6` / `code` 等）：链 ≤ 4 段，里头**没有**任何嵌入字体名。
- [ ] **专用类 模式 A / B**（`.title-special` / `.signature` / `.rare`）：链 ≤ 2 段，仅嵌入字体 + 1 个 generic family，不含任何系统字体或开源 CJK 字体。
- [ ] **专用类 模式 C 复合**（`.book-*-deluxe` / `.book-*-with-rare` 等）：链 ≤ 5 段；**嵌入字体在链里只出现 1 次**，位置为第 1 位（C1）或倒数第 2 位（C2），二选一；其余为 3 段系统字体（Apple + Windows + Android / 开源）+ generic family。
- [ ] `fonts.css` 中所有未被引用的 `@font-face` 保持**注释**状态。
- [ ] 启用 `@font-face` 时，OPF manifest 含对应 `font/ttf` 或 `font/otf` item。
- [ ] OPF metadata **始终**含 `<meta property="ibooks:specified-fonts">true</meta>`（通用预防默认，是否嵌字体都保留）。
- [ ] 同一条链里没有同家族别名堆叠。
- [ ] `.rare` 在没有 `RareSongSubset` 时**保持注释**或显式说明"未启用"。
- [ ] 跑 `sh templates/epub-style-demo/build.sh`，dist 包能在 Apple Books / Kindle Previewer / Thorium / 多看 / KOReader 任意环境打开。

## 8. 与 demo 新增页面的关系（与 demo-scene-expansion-plan.md 协同）

- 新增 demo 页面里出现的字体语义类（`.kaiti` / `.song` / `.hei` / `.fangsong` / `.book-*` 系列）都使用 § 三 A 默认链；
- 13 号"多看 fallback 富文本一体页"里 `.kaiti` 走系统楷体链，**不**依赖嵌入字体；
- 07 号"font-family 顺序验证"页保留 `.demo-system-first` / `.demo-embedded-first` / `.demo-mixed` 三类用于演示链顺序差异，文案需要标明后两者是反例；
- 非字体相关的视觉类（`.emp` / `.wavy` / `.dropcap` / `.scene-break` / `.poetry` / `.dialog` / `.letter` / `.chapter-head` / `.sptxt` 等）**不放在 fonts.css**，按 [css-layering-plan.md §2.1](./css-layering-plan.md) 分到 `effects.css` / `literary.css` / `media.css` / `vertical.css`；详见 [demo-scene-expansion-plan.md §4](./demo-scene-expansion-plan.md)。
- 文学结构类（`.epigraph` / `.letter` / `.dedication-text` 等）若需要楷体语义，**字体链必须与 §三 A `.book-kai` 保持一致**（即 `"Kaiti SC", "KaiTi", "AR PL UKai CN", serif`）。两种等价做法：
  - 在 `literary.css` 里把 `.book-kai` 写进选择器组（`font-family: inherit` 思路：`.epigraph, .letter { /* 无 font-family */ }`，并在 XHTML 上加 `class="epigraph book-kai"`）；
  - 或者在 `literary.css` 里显式重写同一条 4 段链。  
  禁止再出现 Apple-only 的旧链（如 `"Kaiti SC", "STKaiti", ...`），同家族别名堆叠也禁止。
