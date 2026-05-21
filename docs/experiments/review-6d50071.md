# Review: 6d50071「ci: add GitHub Actions workflow to build demo EPUB artifact」

> 范围：commit `6d50071` 的全部 23 个文件改动（不仅是 workflow 本身）。
> 目的：交给另一个模型落地修复。每条问题都给出**文件:行号** + **建议修法**。
> 检阅人：Claude（仓库内 review，未运行阅读器实测，仅静态审阅 + 与 CLAUDE.md 约束比对）。

---

## 0. 整体结论

这次提交在 commit message 上是「加 CI workflow」，但实际把 fonts.css 全面重写、拆出 5 个新 CSS 模块、新增 8 个 XHTML 场景、改了多看 footnote 结构。属于「标题与内容不符」的复合提交，**风险点集中在 3 块**：

1. **CI workflow 只 build 不验证**，与 CLAUDE.md「demo 先行、文档后补」闭环里的「构建 → 验证 → 回写」要求不一致。
2. **`reader-matrix.yaml` 没同步**：新增了 8 个 fixture（cases 10–17）属于「兼容性判断」，按 CLAUDE.md 必须实时回写 reader-matrix，本次完全没有更新。（多看 `duokan-footnote-content` 的挂载层级调整经用户实测验证可行，仅需补登记，不需要回滚——详见 §4。）
3. **图片环绕（`.img-left` / `.img-right`）在 Kindle 上不生效**，根因不是「Amazon 不支持 float」，而是 demo 把 `float` 挂在了 `<figure>` 包裹层，加上没声明宽度、没 clear，KF8/KFX 的 CSS 子集过滤掉了。**这是用户实测发现的核心 bug，必须改 §3。**

---

## 1. CI workflow 本体 (`.github/workflows/build-epub-demo.yml`)

### 1.1 [中] 只 build 不校验，不符合「构建 → 验证 → 回写」闭环
- **位置**: `.github/workflows/build-epub-demo.yml:17-25`
- **现状**: 只跑 `sh templates/epub-style-demo/build.sh`，把产物 upload 上去就结束。CLAUDE.md「实测回写闭环」第 3 步要求「验证」环节保留错误码/日志，CI 至少要跑结构校验。
- **建议修法**: 在 `Build demo EPUB` 之后、`Upload` 之前，加一步 EPUBCheck：
  ```yaml
  - name: Setup Java
    uses: actions/setup-java@v4
    with:
      distribution: temurin
      java-version: '17'

  - name: Install EPUBCheck
    run: |
      curl -L -o epubcheck.zip https://github.com/w3c/epubcheck/releases/download/v5.1.0/epubcheck-5.1.0.zip
      unzip -q epubcheck.zip -d "$RUNNER_TEMP/epubcheck"
      echo "EPUBCHECK_JAR=$RUNNER_TEMP/epubcheck/epubcheck-5.1.0/epubcheck.jar" >> "$GITHUB_ENV"

  - name: Validate with EPUBCheck
    run: |
      epub=$(ls templates/epub-style-demo/dist/*.epub | head -n 1)
      java -jar "$EPUBCHECK_JAR" "$epub" --mode epub --profile default \
        --out "$epub.epubcheck.xml" || true
      # 不阻塞 CI：先把报告作为 artifact 保留，待 reader-matrix 用 status: warn 跟踪
  ```
  > 说明：先不让 epubcheck 报错阻塞 workflow，因为当前 demo 里有刻意保留的「老式」`duokan-*` 类，可能触发非致命 warning；上传 XML 报告以便人工对比即可。

### 1.2 [低] 缺 artifact 保留时长与去重
- **位置**: `.github/workflows/build-epub-demo.yml:20-25`
- **现状**: 没设 `retention-days`，默认 90 天；产物文件名带时间戳，但 artifact 名固定为 `epub-style-demo`，重复运行会被新版本覆盖（GitHub 的 artifact 同名行为）——和 build.sh 的「时间戳命名」初衷冲突，导致历史可追溯性不如本地。
- **建议修法**:
  ```yaml
  - name: Upload EPUB artifact
    uses: actions/upload-artifact@v4
    with:
      name: epub-style-demo-${{ github.sha }}
      path: templates/epub-style-demo/dist/*.epub
      if-no-files-found: error
      retention-days: 30
  ```
  并把 EPUBCheck 的 XML 单独再上传一份 artifact。

### 1.3 [低] `on:` 没拦 PR，也没限分支
- **位置**: `.github/workflows/build-epub-demo.yml:3-8`
- **现状**: `push` 任意分支都会触发；却没有 `pull_request`，外部贡献者 PR 在 fork 上跑不到。
- **建议修法**: 加 `pull_request` 并把 `push` 限到主干：
  ```yaml
  on:
    workflow_dispatch:
    push:
      branches: [main, develop]
      paths:
        - 'templates/epub-style-demo/**'
        - '.github/workflows/build-epub-demo.yml'
    pull_request:
      paths:
        - 'templates/epub-style-demo/**'
        - '.github/workflows/build-epub-demo.yml'
  ```

### 1.4 [低] `build.sh` 在 CI 里没自动清 dist
- **位置**: `templates/epub-style-demo/build.sh:13-17`
- **现状**: 「Output path already exists 就报错」的设计在本地是安全的，但 CI 里 checkout 是干净的，不会触发；如果将来想让 CI 复用 cache 或在同一 job 内多次 build，就会卡住。
- **建议修法**: workflow 里在 Build 之前显式 `rm -rf templates/epub-style-demo/dist`，或在 build.sh 里提供 `--force` 选项，不要默默改默认行为。

---

## 2. CSS 拆分 (`base.css` 瘦身 + `notes.css` / `effects.css` / `literary.css` / `media.css` / `vertical.css`)

### 2.1 [中] `notes.css` 的 `sup { line-height: 0; }` 与旧 base.css 行为不一致
- **位置**: `templates/epub-style-demo/OEBPS/Styles/notes.css:3`
- **现状**: 老 base.css 里是 `sup { vertical-align: middle; line-height: 1; }`，拆分后变成 `sup { line-height: 0; }`，丢了 `vertical-align: middle`，且把 `line-height` 从 `1` 改成 `0`。`line-height: 0` 在某些阅读器（尤其 Kindle / Calibre）会导致 sup 元素压扁、相邻行间距错乱。这是「拆分时顺手改了规则」，没有 demo + reader-matrix 支撑。
- **建议修法**: 恢复为
  ```css
  sup { vertical-align: middle; line-height: 1; }
  ```
  如果确实想改 sup 行为，必须先在 reader-matrix 里标 `warn` + artifact 再决定。

### 2.2 [中] 新 CSS 模块全部用「单行压缩」写法，与 `base.css` 风格冲突
- **位置**: `effects.css` / `literary.css` / `media.css` / `notes.css` / `vertical.css` 全文
- **现状**: 例如 `vertical.css:2`：
  ```css
  body.page-vrl { writing-mode: vertical-rl; -epub-writing-mode: vertical-rl; }
  ```
  而老 `base.css` 是每条规则一行、属性多行展开。本仓库 `base.css` 是「教学样本」，单行压缩对读者不友好，未来 diff 也会更难看。
- **建议修法**: 把所有新 CSS 模块改回多行、规范缩进的格式，统一与 base.css 一致。例：
  ```css
  body.page-vrl {
    writing-mode: vertical-rl;
    -epub-writing-mode: vertical-rl;
  }
  ```

### 2.3 [低] `demo-*` 字体类从 base.css 迁到 fonts.css，但 fonts.css 注释明确说「§四 demo 字体顺序对照」是教学专用
- **位置**: `templates/epub-style-demo/OEBPS/Styles/fonts.css:265-295`
- **现状**: 迁移是对的，但 `.demo-system-first` 与同文件里的 `.book-song` **链完全一样**（`"Songti SC", "SimSun", "Noto Serif CJK SC", serif`），这让 07 号样页不再具有「系统优先 vs 嵌入优先 vs 混合」三段对比的差异——读者在 Apple Books 上看到的 `.demo-system-first` 与 `.book-song` 视觉一致，对照实验失效。
- **建议修法**: 让 `.demo-system-first` 保留**老 base.css 的 3 段链**（`"Songti SC", "Source Han Serif SC", serif`），不要与 `.book-song` 同形，注释里说明「demo 链与生产链刻意不同，用来观察阅读器对长链/短链的取字差异」。

### 2.4 [低] `effects.css` 的 `text-emphasis` 缺 epub 前缀回退
- **位置**: `effects.css:2`
- **现状**: 写了 `text-emphasis` + `-webkit-text-emphasis`，但没写 `-epub-text-emphasis`。老 EPUB 引擎和 KFX 在缺 webkit 时依赖 epub 前缀。
- **建议修法**:
  ```css
  .emp {
    -epub-text-emphasis: filled dot;
    -webkit-text-emphasis: filled dot;
    text-emphasis: filled dot;
    -epub-text-emphasis-position: under;
    text-emphasis-position: under;
  }
  ```

### 2.5 [skip] `effects.css .dropcap`（`float: left`）—— 用户实测 Kindle App 可用，保留
- **位置**: `effects.css:5`
- **用户实测结论**: `.dropcap` 在 **Kindle App 实测可用**（2026-05-21 review 反馈），不需要改动。
- **要做的事（仅 reader-matrix 登记，无代码改动）**:
  - 在 `docs/final/reader-matrix.yaml` 的 `cases:` 块加 `case: 10-text-effects` 之后，给 Kindle 一条 expectation：
    ```yaml
      - reader: kindle_previewer
        case: 10-text-effects
        status: pass
        reader_version: <填用户实测时的 Kindle App 版本>
        artifact: templates/epub-style-demo/dist/<实测产物>.epub
        issue: ".dropcap (float:left) 首字下沉是否在 Kindle App 上生效。"
        action: "实测 pass，保留 effects.css 现有写法，不引入 Kindle 专用 fallback。"
    ```
  - 与 §3.1 图片环绕的「`<figure>` 上 float 被剥掉」结论**不冲突**：dropcap 的 float 挂在 `<span>` 行内元素上，Kindle 的 KF8/KFX 对行内 float 的处理路径与 `<figure>` 容器不同。

---

## 3. 【核心】图片环绕在 Kindle 不生效 (`media.css` + `17-image-layout.xhtml`)

> 用户实测：「图片环绕的逻辑在 Kindle 上没有生效，但官方文档是支持的。」结论：**Amazon 的确支持 float，但只支持挂在 `<img>` 直接元素上、且要求显式宽度与源序在前**。当前实现违反全部 3 条。

### 3.1 [高] `float` 挂在 `<figure>` 包裹层而不是 `<img>`
- **位置**:
  - `templates/epub-style-demo/OEBPS/Styles/media.css:4-5`
  - `templates/epub-style-demo/OEBPS/Text/17-image-layout.xhtml:6`
- **现状**:
  ```css
  .img-left  { float:left;  width:40%; margin:0 1em .6em 0; }
  .img-right { float:right; width:40%; margin:0 0 .6em 1em; }
  ```
  XHTML 用法：
  ```html
  <figure class="img-left"><img src="../Images/poster.png" alt="左浮动"/></figure>
  ```
  Kindle Publishing Guidelines（KPG §3.2 Image Wrapping）允许的环绕生效条件：
  1. `float` 必须直接挂在 `<img>` 或行内元素，**不挂 `<figure>`** —— Kindle 转换器（KindleGen / Kindle Create）会把 `<figure>` 当 block 容器，遇到 float 容器时多数版本直接剥掉 float 属性。
  2. 必须显式指定 `<img width="…"/>` 或在 CSS 上写明 `width: <绝对单位>`（KFX 对 `%` 宽度 + float 的组合常常忽略 float）。
  3. 源序：被环绕的文本必须在 `<img>` **之后**且同段内（兄弟节点级），不能跨 section / 跨段。
- **建议修法**：把 `.img-left` / `.img-right` 改成直接挂在 `<img>` 上，并提供一条「figure 版」+「img 版」双轨：
  ```css
  /* 兼容 Kindle KF8/KFX：float 直接挂 <img>，并给绝对宽度 */
  img.kindle-img-left  { float: left;  width: 12em; max-width: 40%; margin: 0 1em .6em 0; }
  img.kindle-img-right { float: right; width: 12em; max-width: 40%; margin: 0 0 .6em 1em; }

  /* 标准 EPUB：float 可挂 <figure> */
  .img-left  { float: left;  width: 40%; margin: 0 1em .6em 0; }
  .img-right { float: right; width: 40%; margin: 0 0 .6em 1em; }
  .img-center { display: block; margin: .8em auto; text-align: center; }
  .clear-both { clear: both; }
  ```
  XHTML 改为「兼容写法」：
  ```html
  <p>
    <img class="kindle-img-left" src="../Images/poster.png" alt="左浮动" width="200"/>
    后面这一段文字应该在图片右侧环绕显示。Kindle 把 float 剥掉时也只会变成
    上下排版，不会破版。这一段刻意写长一些，确保至少 3 行能贴住浮动图片。
  </p>
  <p class="clear-both">清除浮动后另起一段。</p>
  ```
  说明：保留 `.img-left` 是为了 Apple Books / Thorium 上的 `<figure>` 路径；同时给 Kindle 一份 `img.kindle-img-left` 的纯 img 路径。`width="200"` 这个 HTML 属性是 KF8 转换器最稳定识别的环绕宽度声明。

### 3.2 [高] `17-image-layout.xhtml` 没有可被环绕的正文
- **位置**: `templates/epub-style-demo/OEBPS/Text/17-image-layout.xhtml:6`
- **现状**: 整段是
  ```html
  <figure class="img-left"><img src=".../poster.png" alt="左浮动"/></figure>
  <p>图文混排示例，含左浮动。</p>
  <p class="clear-both">清除浮动。</p>
  ```
  问题：(a) `<figure>` 与 `<p>` 是兄弟元素，且 figure 在 p 外；KF8 即便认 float，也会因为「figure 自己撑满 width:40%」而把后续 `<p>` 顶到下一行——根本就观察不到环绕，与「Kindle 不生效」的现象一致。(b) 一段「图文混排示例，含左浮动」太短，连一行都填不满浮动图右侧。
- **建议修法**:
  - 把图改成 `<img>` 形式，放进段落首字位置；
  - 段落正文要至少 4–6 行中文，足以「绕完」图片高度；
  - 同步加左/右/居中三种场景的对比，方便阅读器矩阵填表。

  完整替换为：
  ```html
  <body>
  <section epub:type="chapter">
    <h1>十七、图文环绕</h1>

    <h2>左浮动（Kindle 兼容路径：float 挂在 img 上）</h2>
    <p>
      <img class="kindle-img-left" src="../Images/poster.png" alt="左浮动" width="200"/>
      这是用于验证 Kindle 图文环绕的演示段落，正文需要足够长才能完整环绕浮动图片。
      Kindle Publishing Guidelines 允许 float 但要求挂在 img 元素本身、并显式声明宽度，
      所以这里同时使用 width 属性与 CSS 宽度，并尽量保证段落至少四到六行，避免出现
      图片把段落顶走的假阴性。
    </p>
    <p class="clear-both">清除浮动后另起一段。</p>

    <h2>右浮动（同上，方向相反）</h2>
    <p>
      <img class="kindle-img-right" src="../Images/poster.png" alt="右浮动" width="200"/>
      右浮动用于对照左浮动；如果阅读器对 float:right 的支持弱于 float:left，
      这里能直接观察到差异。段落保持等长，方便阅读器矩阵填写。
    </p>
    <p class="clear-both">清除浮动。</p>

    <h2>figure 包裹版（Apple Books / Thorium 标准路径）</h2>
    <figure class="img-left">
      <img src="../Images/poster.png" alt="figure 左浮动"/>
      <figcaption>figure 路径：标准阅读器走这一条</figcaption>
    </figure>
    <p>
      figure 版用于验证 Apple Books / Thorium 上 figure 包裹层的 float 行为。
      Kindle 上预期这一段不会环绕（float 在 figure 上会被剥掉），是「预期失败」用例，
      用来与上面的 img 版做对照。
    </p>
    <p class="clear-both">清除浮动。</p>
  </section>
  </body>
  ```

### 3.3 [必做] 跑一次 Kindle Previewer 并回写 reader-matrix
- 改完后必须按 CLAUDE.md「实测回写闭环」跑一次：
  1. `sh templates/epub-style-demo/build.sh`
  2. 用 **Kindle Previewer 3 最新版（同时切 KFX 与 KF8 两种渲染）** 打开 17 号页面
  3. 截图保留：左浮动是否环绕、右浮动是否环绕、figure 版是否退化为块级
  4. 把结果写进 `docs/final/reader-matrix.yaml`，新增 case `17-image-layout` 与两条 expectation（`kindle_previewer` 下分别一条 `status: pass`/`warn`/`fail`）
  5. 只有跑通后，再去更新 `docs/final/EPUB 3 HTML CSS 属性速查表.md:262` 的 `float` 状态注释（目前是「推荐」，可能需要补「KF8 仅挂 img 时生效」）

---

## 4. 多看 footnote 结构改造（用户实测：新写法可用，保留）

### 4.1 [P1] 实测结论需要落到 reader-matrix 与 SPEC，否则改动没有可追溯凭据
- **位置**:
  - `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml:47-58` (diff)
  - `templates/epub-style-demo/OEBPS/Text/06-multi-legacy-note-fallback.xhtml:57-77` (diff)
  - `templates/epub-style-demo/SCENE_MATRIX.md:12` (同步把描述改成 `ol.duokan-footnote-content`)
  - `templates/epub-style-demo/OEBPS/Styles/notes.css:14` 新增 `ol.duokan-footnote-content { … }`
- **现状**: 把每个 `<li class="footnote-item duokan-footnote-item duokan-footnote-content">` 改成 `<ol class="footnote-list duokan-footnote-content"> <li class="footnote-item duokan-footnote-item">`。这是协议级语义改动（从「每条 li 单独标 content」改为「ol 容器标 content」）。
- **用户实测结论**: 新写法在多看上**可正常识别弹注**，所以**不需要回滚**，结构保留即可。
- **必须补的事**:
  1. 在 `docs/final/reader-matrix.yaml` 的 `readers:` 块加上 `duokan` 条目（详见 §6.3），并补一条 `case: 05-duokan-fallback` 和 `case: 06-multi-duokan-fallback` 在 `duokan` 下的 `status: pass` expectation，artifact 指向用户实测时使用的 epub 构建产物，`reader_version` 填实测版本。模板：
     ```yaml
       - reader: duokan
         case: 05-duokan-fallback
         status: pass
         reader_version: <填用户实测时的多看版本>
         artifact: templates/epub-style-demo/dist/<实测产物>.epub
         issue: "验证 ol.duokan-footnote-content 容器级挂载是否能被多看识别。"
         action: "实测可用，保留 ol 容器级写法，不再使用 li 级 duokan-footnote-content。"
     ```
  2. 在 `docs/final/SPEC-实现约束.md` 中把多看 fallback 的「容器级 vs 单条级」规则**确认下来**：要求作者使用 `<ol class="footnote-list duokan-footnote-content">` 容器写法，**禁止**在 `<li>` 上重复挂 `duokan-footnote-content`。
  3. 同步更新 `docs/final/EPUB 3 终极实践手册.md` 与速查表里所有提及 `duokan-footnote-content` 的位置，统一指向「挂在 `<ol>` 上」。
  4. commit message 里保留一句「多看 vX.Y 实测 pass」的可追溯说明（即便是补登记的后续 commit，也写一句来源）。

---

## 5. 新增的 8 个 fixture（10–17）质量偏低

### 5.1 [中] 每个 fixture 都是「6 行 XHTML」，覆盖面不足
- **位置**: `templates/epub-style-demo/OEBPS/Text/11-chapter-opening.xhtml` 等
- **现状**: 11–17 全部是「head + 一段 body」单行压缩，每页只有 1–3 个验证点，远不够 CLAUDE.md「demo EPUB 必须覆盖结构多样性」的要求。例如：
  - `14-vertical-body.xhtml` 整页竖排只写了 `<p>这是竖排正文示例。</p>` 一句，不足以观察换行/标点压缩/中英混排在竖排下的行为；
  - `16-math.xhtml` 缺 MathML 块、缺多行公式；
  - `15-frontmatter.xhtml` 缺版权页本身，只有题献和题记。
- **建议修法**: 每个 fixture 至少 15–25 行有效内容，覆盖正反向用例（即「正常显示」+「至少 1 个边界用例」）。具体 checklist 见 §5.2。

### 5.2 各 fixture 应补的最小内容
| fixture | 必须新增 |
|---|---|
| `10-text-effects.xhtml` | 加 `text-emphasis-style: open dot` / 边框圈点；首字下沉至少 2 段；ruby 嵌套 |
| `11-chapter-opening.xhtml` | 加 `epub:type="titlepage"` 与正文分页测试；卷首引文带作者归属 |
| `12-literary-fiction.xhtml` | 对话至少 4 个角色交替、`<blockquote>` 引文、信件含署名抬头 |
| `13-duokan-rich-fallback.xhtml` | 至少含 2 个 `noteref` + 2 个 `duokan-footnote`，并标注「这一页是同时测试两种 fallback 路径」 |
| `14-vertical-body.xhtml` | 至少 1 整屏文字，含中英混排、ruby、数字、引号；测试 `text-orientation: mixed` |
| `15-frontmatter.xhtml` | 必须包含「版权页」本身（ISBN、版次、出版社）；`epub:type` 用 `copyright-page` |
| `16-math.xhtml` | 加一段 MathML（即使包不进 KF8 也要试，结果填进 reader-matrix）；行内 + 块级各 1 |
| `17-image-layout.xhtml` | 按 §3.2 重写 |

### 5.3 [低] 单行压缩的 XHTML 不利于人工审阅
- 与 §2.2 同源问题。建议把所有新 XHTML 改回每标签一行、缩进 2 空格的写法，统一与 `00-title.xhtml` / `01-body.xhtml` 一致。

---

## 6. `reader-matrix.yaml` 完全没同步

### 6.1 [高] 8 个新 fixture 没有 case 条目
- **位置**: `docs/final/reader-matrix.yaml`
- **现状**: 改动只到 `09-kindle-risk`，10–17 全部缺失。
- **建议修法**: 在 `cases:` 块尾追加：
  ```yaml
    - id: 10-text-effects
      title: 文字效果合集（着重 / 波浪线 / 首字下沉 / Ruby）
      fixture: templates/epub-style-demo/OEBPS/Text/10-text-effects.xhtml
    - id: 11-chapter-opening
      title: 章首结构
      fixture: templates/epub-style-demo/OEBPS/Text/11-chapter-opening.xhtml
    - id: 12-literary-fiction
      title: 小说体综合
      fixture: templates/epub-style-demo/OEBPS/Text/12-literary-fiction.xhtml
    - id: 13-duokan-rich-fallback
      title: 多看富文本 fallback 一体页
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
      title: 图文环绕
      fixture: templates/epub-style-demo/OEBPS/Text/17-image-layout.xhtml
  ```

### 6.2 [高] 至少需要补一条关于 Kindle 图片环绕的 expectation
- **建议条目**（修完 §3 后再写状态）:
  ```yaml
    - reader: kindle_previewer
      case: 17-image-layout
      status: fail            # 修复前先记 fail，修完后改 pass/warn
      reader_version: <填实测版本>
      artifact: templates/epub-style-demo/dist/<新构建产物>.epub
      failed_page: 17-image-layout
      issue: "`.img-left` 挂在 <figure> 上时，KF8/KFX 剥掉 float，整段不环绕。"
      action: "改用 img.kindle-img-left 直接挂在 <img> 上，并加 width 属性；不引入 amzn 私有 media 查询。"
      workaround: "对 Kindle 路径强制走 img 版；figure 版仅 Apple Books / Thorium 走。"
  ```

### 6.3 [中] readers 列表缺 `duokan`
- **位置**: `docs/final/reader-matrix.yaml:11-21`
- **现状**: 既然 demo 改了多看 footnote 结构（§4），就必须在 readers 块里加多看；否则后续 expectation 无法登记。
- **建议修法**:
  ```yaml
    - id: duokan
      name: 多看阅读
  ```

---

## 7. OPF / 元数据小问题

### 7.1 [低] `dcterms:modified` 没随这次内容大改而更新
- **位置**: `templates/epub-style-demo/OEBPS/package.opf:13`
- **现状**: 还是 `2026-05-19T00:00:00Z`，但 commit 是 5-21，且新增了 8 个内容页。EPUB 3 规范要求 `dcterms:modified` 每次发版都更新。
- **建议修法**: 改成 commit 当天 UTC，例如 `2026-05-21T05:26:00Z`。

### 7.2 [保留] `ibooks:specified-fonts=true` 立场维持
- **位置**: `package.opf:18` + `fonts.css` 头部注释
- **结论**: 用户确认这条元数据是**正确的、必须保留**。fonts.css 头部注释已经把它定位为「通用预防默认」，与 `docs/final/fonts-css-expansion-plan.md §4` 一致——`specified-fonts=true` 是为了避免 Apple Books 把无衬线 / 衬线设定误盖到我们在 CSS 里精心选的系统字体链上，即便 demo 没嵌字体也保留。
- **要做的事（仅文档同步）**:
  - 在 `docs/final/SPEC-实现约束.md` 字体相关条目里**显式列出**：「无论是否嵌入字体，OPF 都必须保留 `<meta property="ibooks:specified-fonts">true</meta>`」，并给一句理由（防止 Apple Books 用用户偏好字体覆盖 CSS 字体链）。
  - 同步把这条规则补进 `docs/final/EPUB 3 终极实践手册.md` 字体小节，避免读者把它当成「嵌入字体时才需要」。
  - **不要**在 demo OPF 上移除它，也**不要**在 fonts.css 注释里把它降级为「可选」。

### 7.3 [低] `toc.ncx` 新增 navPoint 用单行压缩，与既有风格不一致
- **位置**: `templates/epub-style-demo/OEBPS/toc.ncx:53-60`
- **建议修法**: 与既有多行风格保持一致：
  ```xml
  <navPoint id="navpoint-11" playOrder="11">
    <navLabel><text>文字效果合集</text></navLabel>
    <content src="Text/10-text-effects.xhtml"/>
  </navPoint>
  ```

---

## 8. Commit 卫生

### 8.1 [低] commit message 不准
- **建议**: 把本次提交拆成 3–4 个独立 commit（CI workflow / CSS 模块拆分 / 新增 fixture / 多看 footnote 重构），或者至少把 commit message 改成更准确的「ci+fixtures: build workflow & expand demo with effects/literary/vertical/math/image-layout pages」。已经提交的不强求 rebase，**下一次类似 PR 必须拆分**。

---

## 修复优先级清单（给执行者用）

| 优先级 | 编号 | 文件 | 一句话 |
|---|---|---|---|
| **P0** | §3.1 §3.2 | `media.css` + `17-image-layout.xhtml` | Kindle 图片环绕：float 挂到 `<img>`、加 `width` 属性、正文加长；**不**加 `@media amzn-kf8` |
| **P0** | §3.3 | flow | 改完跑 Kindle Previewer 实测，回写 reader-matrix |
| **P0** | §6.1 §6.2 §6.3 | `docs/final/reader-matrix.yaml` | 补 8 个 case + Kindle 环绕 expectation + `duokan` reader + 多看 fallback 实测条目 |
| P1 | §1.1 §1.2 | workflow | 加 EPUBCheck + retention-days + sha 后缀 |
| P1 | §2.1 | `notes.css` | 恢复 `sup { vertical-align: middle; line-height: 1; }` |
| P1 | §4.1 | SPEC + 终极手册 + 速查表 | 多看 footnote 容器级写法落规则（用户实测 pass，保留新结构） |
| P1 | §5.1 §5.2 | 10–17 XHTML | 每页补充覆盖度 |
| P1 | §7.2 | SPEC + 终极手册 | `ibooks:specified-fonts=true` 显式落为强制规则（不要从 demo 移除） |
| P2 | §2.2 §5.3 §7.3 | 全部新文件 | 改回多行 / 标准缩进 |
| P2 | §2.3 §2.4 | `fonts.css` / `effects.css` | demo-* 链恢复差异；text-emphasis 加 epub 前缀 |
| skip | §2.5 | — | dropcap 用户实测 Kindle App pass，仅补 reader-matrix 登记，不改代码 |
| P2 | §1.3 §1.4 | workflow + build.sh | pull_request 触发；dist 清理策略 |
| P3 | §7.1 | `package.opf` | 更新 `dcterms:modified` 时间戳 |
| P3 | §8.1 | 流程 | 下次 PR 拆分 |

---

## 给执行者的提醒（来自 CLAUDE.md）

1. §3 / §4 / §7.2 的改动顺序必须遵守 CLAUDE.md「实测回写闭环」：demo / OPF 改动 → build → 阅读器实测 → `reader-matrix.yaml` → `SPEC-实现约束.md` → 终极手册 → 速查表。**不要直接改手册**。
2. 修 §3 时，跑完 Kindle Previewer 后**也要在 Apple Books / Thorium 上确认 figure 版没回归**——别为了治 Kindle 把别家路径打坏。
3. §4 多看 footnote 与 §7.2 `ibooks:specified-fonts` 都属于「用户已确认的实测结论」——补 reader-matrix / SPEC 条目时，commit message 里要带一句来源（例：「多看 v<版本> 实测 pass，结构保留」「ibooks:specified-fonts 经实测确认应在 demo 与生产 OPF 中始终保留」），让结论可追溯。
4. **不要在 media.css / effects.css 等 CSS 里加 `@media amzn-kf8` / `@media amzn-mobi` 媒体查询分支**——用户决定不走 Amazon 私有 media 路径，Kindle 兼容只能在通用 CSS 层面（如 `<img>` 直接挂 float、显式 width、合理源序）解决。
