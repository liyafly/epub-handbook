# Review — `origin/codex/modify-files-to-execute-in-order`

> Base: `develop` (10 commits ahead, 71 files, +3906 / −445).
> Reviewer: Claude (read-only, no fixture rebuild executed locally).
> Date: 2026-05-25.

## 1. 范围概览

这条分支落地的是「cleanup-plan §1–§7 收尾 + 后续四个专题扩展」。十次 commit 可以归为三条主线：

1. **字体策略反转**（`c7f8617` / `a6f1902` / `f8481177` / `b8e631c` / `974e2f0`）
   - 默认改为系统字体链，嵌入字体只挂模式 A / B / C 专用类。
   - 新增「模式 C1-body」例外：含生僻字 + 全字符集字体可直接挂 `body` / `h*`，链 ≤ 5 段，`fontspec=forceAll`。
   - 同步面：`SPEC §8`、`终极手册 §一 / §四 / §七 / §八 / §十二`、`速查表 §四`、`fonts.css`、`base.css`、`fonts-css-expansion-plan.md` 全部一次性改完。
2. **CSS 分层硬约束**（`c7f8617` / `a6f1902`）
   - 单文件阈值 200 → 400 行预警、500 行硬上限（手册、`css-layering-plan.md`、`demo-scene-expansion-plan.md` 同步）。
   - `base.css` 删除重复块和 `color/background` 页面级声明；`effects.css` 接管 `.note-box` 容器视觉；`literary.css` 接管 `.scene-break` / `.chapter-head-*` / `.english-fiction`。
   - SPEC §7 八个 CSS 文件的职责表全部重写。
3. **三个新场景 + 一套 AI 入口**（`7f4c76a` / `d81f4bc` / `1ce666b` / `f447574` / `de3b493`）
   - 新 fixture `18-english-fiction.xhtml` / `19-border-shadow-notes.xhtml` / `20-chapter-head-image.xhtml`，并补 `Images/chapter-banner.png`。
   - 新 skills：`epub-layout-auditor` / `epub-source-intake` / `epub-css-layering-optimizer` / `epub-english-typography-optimizer` / `epub-literary-structure-formatter` / `epub-package-nav-auditor` / `epub-typography-optimizer` / `epub-image-layout-optimizer` / `epub-kindle-compatibility-checker` / `epub-vertical-ruby-optimizer`（含 `agents/openai.yaml`）。
   - 新脚本：`scripts/epub_ai_harness.py`、`scripts/validate_skills_basic.py`、`hooks/pre-commit.epub-handbook`；`scripts/validate_epub_style_demo.py` 显著扩张。
   - 四份新指南：`english-fiction-layout.md` / `anthology-navigation.md` / `chapter-head-image.md` / `note-box-border-styles.md`。

## 2. 总体评价

工作量大，但 demo-first 闭环没有被打破：

- 每个新增规则都先落 fixture（`Text/18–20`）→ 进 `reader-matrix.yaml`（status 一律 `warn`，未虚构 pass/fail）→ 再回写 SPEC / 手册 / 速查表 / skill。`reader-matrix.yaml` 的三条 fixture 18 条目和三条 fixture 19 条目都带 `reader_version`、`artifact`、`issue`、`action`、`workaround`，traceability 合格。
- 改回写文件时保持了项目契约：`skills/*/SKILL.md` frontmatter 仍是 `name` + `description`，未引入额外 key；`agents/openai.yaml` 全部带 `display_name` / `short_description` / `default_prompt` 三项；OPF manifest 新 item 都带正确 properties（`mathml` 已有，`svg` 这次新增 `19-border-shadow-notes.xhtml`）。
- 颜色/底色的清理思路是对的：`body` / `body.fullpage` / `body.poster-bg` / `figcaption` / `blockquote` / `.poster-title` / `.poster-subtitle` 都不再写 `color` / `background-color`，让用户的夜间模式和原版字体设置生效。局部组件（`pre`、`table thead/tbody`、`kbd`、`.note-*`）保留必要装饰底色，与 SPEC §7 的措辞一致。
- 验证脚本质量是这条分支最稳的部分。`scripts/validate_epub_style_demo.py` 已能：
  - 把 `properties="mathml"` 检查推广为「namespace 探测 → properties 推断」，覆盖到 inline SVG（`SVG_URI` 探测后强制 `properties="svg"`）；
  - 主动验证 `fonts.css` 没有泄漏 `../Fonts/` 引用（覆盖「未启用嵌入字体却挂着 OPF item」的常见反模式）；
  - 在 `effects.css` 上禁掉 `transform:` / `-webkit-transform:`（与 SPEC §5.10 的 Kindle Previewer 3.104 实测结论闭环）；
  - 对三个新 fixture 做关键 token 断言（class、SVG path、`chapter-banner.png` 引用）。
- `scripts/validate_skills_basic.py` 引入跨文件契约（CONTRACTS 表）：四个核心 skill 会显式检查它们声称的 guide / fixture token 实际存在。这是把「skill 文档保持与 fixture 同步」的承诺转成可执行检查的好做法。
- `hooks/pre-commit.epub-handbook` 的条件构建合理：只在 `templates/epub-style-demo/` / `scripts/validate_*` / `scripts/epub_ai_harness.py` / `docs/final/SPEC-` 触发时再 build + 校验 demo；其它时机仍跑 `validate_skills_basic.py` + `git diff --check`。

## 3. 需要补充或修正的事项（按优先级）

### P1：需要在合并前处理

1. **`docs/guides/note-box-border-styles.md` 展示了 `.note-handcut`，但 `effects.css` 没有这个类。**
   guide 给的 CSS 看起来是实现示例，但读者会以为这是仓库内已存在的类。
   - 建议：要么把 `.note-handcut` 加入 `effects.css` 并补一个对应 fixture 段（最好挂在 `19-border-shadow-notes.xhtml` 末尾），要么在 guide 的代码块上方加一行「以下是建议落地模式，仓库当前 fixture 暂未使用」。

2. **`pre-commit.epub-handbook` 没有安装路径说明。**
   仓库根有 `hooks/pre-commit.epub-handbook`，但没有 `scripts/install-hooks.sh`、`README.md` 也没有说明应该如何把它链接到 `.git/hooks/pre-commit`。
   - 建议：在 `README.md` 或 `docs/guides/skills-and-templates.md` 末尾补一句 install 指令（`ln -sf ../../hooks/pre-commit.epub-handbook .git/hooks/pre-commit`），或者干脆加一个一行的 `scripts/install-hooks.sh`。否则团队/外部贡献者拿到分支只能看 hook，没有触发它的办法。

3. **`docs/guides/fonts-css-expansion-plan.md §6` 自引用分支名 `codex/modify-files-to-execute-in-order`。**
   合并后这个分支名会消失，引用会成为悬空指针。
   - 建议：去掉分支名，只保留 commit hash（`c7f8617` / `a6f1902` / `f8481177`），或者改成「已合并入 main」。

### P2：建议但非阻塞

4. **`scripts/validate_epub_style_demo.py` 对 `transform:` 的禁用过于宽松。**
   现状：`"transform:" not in effects_css and "-webkit-transform:" not in effects_css`。这条规则源自 Kindle Previewer 3.104 对 `transform: rotate()` 的 KFX 转换 bug。
   - 风险：未来如果 `effects.css` 引入 `translateY` / `scale` 等无害变换（例如做 inline icon 的微调），脚本会误报。
   - 建议：把检查窄化为 `re.search(r"transform:\s*rotate", css)`（或同时 `-webkit-transform:\s*rotate`），并在脚本注释里链回 `docs/final/SPEC-实现约束.md §5.10`。

5. **`effects.css` 同时承载「文字效果」和「便签视觉容器」两种职责。**
   现在 162 行还在 400 行预警下方，但 `.note-*` 系列再加 1–2 个变体就会顶到预警线。SPEC §7 已经把两类挂在同一层，css-layering-plan 也写了「超过 400 行评估拆分」，但没有给出拆分后的目标文件名。
   - 建议：要么在 SPEC §7 注释里预留一个未来文件名（如 `decoration.css`），要么在 `effects.css` 顶部留一段 maintenance note 说明拆分判据，否则未来某个 PR 不得不在合并节点上临时讨论命名。

6. **`pre-commit.epub-handbook` 的触发路径列表不覆盖手册/速查表/reader-matrix。**
   只匹配 `templates/epub-style-demo/`、`scripts/validate_`、`scripts/epub_ai_harness.py`、`docs/final/SPEC-`。如果有人只改了 `docs/final/EPUB 3 终极实践手册.md`、`reader-matrix.yaml` 或速查表，并未触动 SPEC 或 fixture，hook 不会重 build demo / 重跑 popup 校验。
   - 现状一般没问题（这种 PR 只是文档），但「demo 先行」契约要求实测结果有 artifact 支撑，最好让 reader-matrix 的修改也触发一次构建。
   - 建议：把 grep 模式扩成 `^(templates/epub-style-demo/|scripts/validate_|scripts/epub_ai_harness.py|docs/final/(SPEC-|reader-matrix\.yaml))`，或者把 `docs/final/` 整体纳入。

7. **`base.css` 仍写了 `pre / table thead / tbody / kbd` 的浅底色 + 边框颜色（`#f1eee8` / `#eee7dc` / `#f6f1e8` / `#f4efe6`）。**
   SPEC §7 允许「局部组件保留必要的边框、阴影和背景装饰」，所以这些不算违规。但夜间模式下浅奶白色的 code block 容易刺眼。
   - 建议：要么留个 TODO 注释说明这是有意保留，要么把 `pre` / `table` 的底色挪到 `effects.css` 的局部 modifier 类（如 `.code-block` / `.table-shaded`），让默认 `pre` / `table` 保持透明。这是后续 polish，不阻塞当前合并。

8. **三个新 fixture 在 `reader-matrix.yaml` 中全部 `warn`，缺 Thorium / KOReader 实测。**
   `SCENE_MATRIX.md` 第 22 行声明 fixture 20 的目标读者是「Kindle Previewer / Apple Books / Thorium」，但 reader-matrix 只录了 Kindle Previewer / Apple Books / Readest（fixture 18 / 19）和**完全缺**（fixture 20）的实测条目。`reader-matrix.yaml` 里没有任何 `case: 20-chapter-head-image` 的 `expectations` 条目。
   - 建议：要么补一个 `case: 20-chapter-head-image` 的 `warn` 条目（带 `artifact` 与「待复测」action），要么在 SCENE_MATRIX 备注「reader-matrix 实测条目待补」。否则光看 reader-matrix 会以为这页没人测过。

9. **`scripts/validate_skills_basic.py` 的 YAML 解析过于朴素。**
   `parse_simple_yaml_strings` 只接受单行 `key: value`，不支持多行 / 块 scalar / 嵌套。当前 `agents/openai.yaml` 模板都很简单，但脚本注释里没有写约束。
   - 建议：在脚本顶部 docstring 加一行「agents/openai.yaml only supports flat string keys; do not introduce lists or block scalars」，或者切到 `tomllib`/PyYAML（PyYAML 不是 stdlib，目前刻意避开）。

10. **英文小说 fixture `Text/18-english-fiction.xhtml` 复用 `Images/poster.png` 作为章首插图。**
    `Images/poster.png` 是 03 / 03b 海报页的素材，做英文 prose 插图视觉上不切题。功能上没问题（只是占位）。
    - 建议：要么换一张合适的小型黑白线稿插图，要么在 fixture 顶部加 `<!-- placeholder illustration -->` 注释，让维护者知道这里需要替换。

### P3：润色 / 长期项

11. **`docs/final/EPUB 3 HTML CSS 属性速查表.md` 出现重复行键。**
    `line-height` 行有两条（一条 1.6–1.9 中文、一条 1.45–1.65 英文），`text-indent` 同理。表格读起来仍清晰，但 markdown 渲染后没有视觉分组。
    - 建议：把英文相关行集中到「英文小说」专题表，或在第二行的「属性 / 规则」列改写为 `line-height (英文)`，避免 grep 出两条同名行。

12. **`epub_ai_harness.py` 与新 fixture/skill 的关系应被 `scripts/validate_*` 触发，但没有 self-test。**
    harness 输出格式（findings / recommended_skills / commands）目前没有 unit/fixture 测试。改它时容易引入静默回归。
    - 建议：加一个 `scripts/test_epub_ai_harness.py` 或 doctest，至少跑一次 `epub_ai_harness.py templates/epub-style-demo` 并断言关键 skill 出现（如 `$epub-popup-footnote-converter` / `$epub-vertical-ruby-optimizer`）。

13. **`docs/guides/note-box-border-styles.md` 没列「字号/夜间模式回归」检查项。**
    SPEC §5.10 已经强调便签必须在大字号、夜间模式下保持可读，但 guide 末尾的「Demo 补充建议」段没明确列出回归项。
    - 建议：在 guide 末尾补一小段「验证清单」：默认字号 / 大字号 / 夜间模式 / Kindle KFX 转换 / Apple Books 原版字体。

14. **`skills/README.md` 已经列出 14 个 skills，但 `epub-typography-optimizer/SKILL.md` 在这条分支里只是被 lint 处理（中文化）了。**
    内容上没有跟 fonts.css 的策略反转完全同步检查；建议在合并前对它再过一遍，确保它的「正文字体链」描述与新 SPEC §8 / fonts.css §三 A 一致。

## 4. 风险评估

- **回归面**：fixture 10–15 这次被 reformat 成多行 XHTML（仅排版变化），但 link 顺序未动，对阅读器表现应无影响。值得在 merge 前再 build 一次确认。
- **回滚成本**：分支以 SPEC / 手册 / 速查表 三处一体的方式落字体策略反转，单独 revert 任何一条 commit 都会让三处不再自洽。如果需要回滚字体策略，应当整段 revert `c7f8617` + `a6f1902` + `f8481177` + `b8e631c` + `974e2f0`。
- **下游 skill 漂移**：新增 6 个 skills + 修改 5 个 skills，但 `validate_skills_basic.py` 的 CONTRACTS 表只覆盖 4 个 skill 的跨文件指针。剩余 10 个 skill 的「描述与 fixture 同步」依赖人工。
- **CI 缺失**：仓库似乎没有 GitHub Actions（develop 上有 `ci: add GitHub Actions workflow to build demo EPUB artifact` 这条 commit，但本分支 review 未检查 workflow 实际状态）。建议在合并前确认 GH Actions 仍在跑，并把 `validate_skills_basic.py` 加进 CI（如果尚未加入）。

## 5. 合并建议

- **可以合并**，但建议先处理 P1 三项（`.note-handcut` 引用、hook 安装说明、guide 中的分支名自引用）。
- P2 几项可以拆成 follow-up PR，但「`transform:` 检查窄化」和「reader-matrix 补 case 20 实测条目」最好在合并前同期处理，避免把硬规则建立在「以后会补」的承诺上。
- 字体策略反转 + CSS 分层升级两个核心面已经走完「demo → matrix → SPEC → 手册 → 速查表 → skills」的闭环，且 fixture 的实测条目带 reader_version 与 artifact，符合 `CLAUDE.md` 的实测回写契约。

---

附：本 review 基于 `git diff develop...origin/codex/modify-files-to-execute-in-order` 静态阅读完成，未执行 `sh templates/epub-style-demo/build.sh`、未跑 `scripts/validate-epub-style-demo.sh`、未在任何真实阅读器中打开 fixture。合并前应至少执行一次 demo build + popup validator + `validate_skills_basic.py`。
