---
name: epub-package-nav-auditor
description: 审核 EPUB package 结构、OPF metadata、manifest、spine、nav.xhtml、toc.ncx、封面声明、CSS/资源引用、MathML properties 和 EPUB zip 规则。用于新增、重命名、删除文件后，或 EPUB 本地构建通过但阅读器/Kindle 工具失败时；只读体检，不处理视觉样式。
---

# EPUB Package 与导航审核

## 何时用

- 结构或打包可能出错的场景：新增/重命名/删除资源后、EPUB3 迁移后、本地构建通过但阅读器或 Kindle 工具失败时；它也是已有 EPUB 清洗流程的第一步 preflight。处理 package 正确性，不处理视觉样式。
- 一个合格 EPUB package 应当具备（审核与修复都以此为基准）：
  - `mimetype` 是 zip 第一项且不压缩；`META-INF/container.xml` 指向 OPF。
  - OPF metadata 有稳定 identifier 和必要阅读器提示；XHTML 根同时声明 `lang` 和 `xml:lang`。
  - OPF manifest 声明所有被使用的 XHTML、CSS、图片、字体、nav、NCX 文件；只有一个带 `properties="nav"` 的 nav item。
  - Kindle/legacy 交付包包含 `toc.ncx` 和 `spine toc="ncx"`；spine 顺序匹配预期阅读顺序。
  - 封面图片同时声明 EPUB 3（`properties="cover-image"`）与旧 Kindle `<meta name="cover" content="..."/>`。
  - 含 MathML 的 manifest item 带 `properties="mathml"`；含内联 SVG 的 XHTML item 带 `properties="svg"`。
  - 大合集、分卷文集和短篇全集的局部目录按 `docs/how-to/anthology-navigation.md` 处理，局部目录只能作为辅助导航。
- 本能力只读：解析 package.opf、nav.xhtml、toc.ncx、container.xml，建立 manifest id→href、href→磁盘文件、spine idref→manifest item、nav/NCX link→XHTML、XHTML link→CSS/image/font/note target 的映射并逐项核对；它不写输出、不做修复。修复由 AI 按第四段判据执行后再重跑本能力复核。
- 禁止事项（修复时同样遵守）：
  - 不通过从 spine 删除页面来掩盖 package 错误，除非该页面确实废弃。
  - 不重命名文件，除非用户要求或当前命名破坏打包规则；重命名/目录整理属于 `epub-structure-normalizer`。
  - 清理时不自动删除 `ibooks:specified-fonts` metadata；自由模式书是否移除交人工判断，见 `docs/final/SPEC-实现约束.md` §8。
  - 不依赖浏览器 HTML 容错；XHTML 必须 XML-valid。不让 nav/NCX 指向已删除或重命名文件。
  - 不以文件名、正文语言外观或阅读器猜测填充语言；OPF 缺失 `dc:language` 时保留 XHTML 现状并报告。
  - 不因为文件在磁盘上存在，就把未使用文件加入 OPF。
  - 有明确删除授权时，同时从 ZIP、manifest、spine、nav 和 NCX 删除同一资源，把精确删除列表写入报告，并只对列表内 XHTML 和重新生成的 nav 使用 allow-list。

## 调什么

```sh
epub run epub.package.nav.audit --input <书> --json
```

无额外 KEY=VALUE 参数；只读能力，不需要 `--output`。需要旧报告形状明细（`recommended_skills`、`suggested_commands`、`findings_by_level`、`actionable_findings`）时加 `legacy_report=true`（迁移期脚手架）：

```sh
epub run epub.package.nav.audit --input <书> --json legacy_report=true
```

修复后重跑同一命令复核，直到 error 清零。

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；`nextCommands[]` 给出建议的下一步命令（迁移期可能仍带旧执行面命令形态，仅供人参考，AI 一律按各 skill 的 `epub run` 形态执行）。
- 退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误（参数非法、文件不存在）。
- facts 键前缀 `epub.package.nav.audit.`：
  - `summary`：`zip_entries`、`manifest_items`、`spine_items`、`media_counts`（xhtml/css/images/fonts/other）、`opf`，以及存在时的 `obfuscated_filenames`、`package_version`、`language`。
  - `input_kind`：`existing-epub`。
- findings：ID 形如 `audit.<序号>`，`title` 是检查结论，`location` 是相关资源路径，`detail` 是问题类别。典型类别：manifest href 缺失、spine idref 不可解析、nav 数量不为 1、缺 NCX/spine toc、封面声明不全、CSS url() 目标缺失、MathML/SVG properties 缺失、文件名混淆、EPUB2 版本、`META-INF/encryption.xml` 存在。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（preflight JSON：findings、findings_by_level、recommended_skills、suggested_commands、tool_availability 等）。

## 依据返回怎么判断

- 无 error findings（`status == complete`）→ package 基线合格，按 findings 的 warn/info 与下列映射分派专项 skill；`epub.layout.audit` 做排版层入口审核。
- 出现 `error` → 先修最小结构问题再进入任何写型能力：
  - manifest href / spine idref 缺失、CSS url() 目标缺失（非字体）→ 修 OPF 或补资源；尽量保留现有 id，新增 manifest item 用稳定描述性 id，manifest/spine 排序保持可复现，assets 不进 spine，CSS 不进 nav landmarks，重打包排除任意路径中的 `.DS_Store`。
  - `mathml` / `svg` properties 缺失 → 在对应 manifest item 补 properties（模式：`<item id="math" href="Text/16-math.xhtml" media-type="application/xhtml+xml" properties="mathml"/>`；内联 SVG 同理补 `properties="svg"`）。
  - `META-INF/encryption.xml` → 停止：真实存在的未知加密资源不得猜测或绕过；只有确认是标准字体混淆且任务得到明确授权时才继续。
  - 修复后涉及 XHTML 重写时，输出保持 XML-valid 且可人工 diff：保留 XML 声明与 HTML doctype，两空格缩进，块级元素分行；不把整个文档压成单行，不拆散 mixed-content 文本节点。
- warn 类分派：
  - `filename-obfuscation`（`obfuscated_filenames` > 0）→ `epub-structure-normalizer`（先 dry-run 审查映射）。
  - `epub3-migration`（package_version 非 3）→ `epub3-migrator`。
  - 封面声明不全 → `epub-image-layout-optimizer`；封面模式：`<meta name="cover" content="cover-image"/>` + `<item id="cover-image" href="Images/cover.png" media-type="image/png" properties="cover-image"/>`。
  - 缺 `toc.ncx` / `spine toc="ncx"` → `epub-kindle-compatibility-checker` 复核交付面；没有明确范围变化时不从 Kindle/legacy fixture 删除 NCX。
  - CSS 字体 url() 缺失 → 保留声明与 `local()` fallback，交 `epub-css-layering-optimizer` / `epub-typography-optimizer` 人工复核。
  - noteref 无同文件 footnote aside → `epub-popup-footnote-converter`。
  - 语言以 en 开头 → `epub-english-typography-optimizer`；否则中文排版入口 `epub-typography-optimizer`。
  - 疑似扫描书（图多字少）→ 先回到 `epub-source-intake`，清洗难以奏效。
- 目录加入正文内真实标题（如 `h2`）时，为标题补稳定 fragment，并在 nav 和 NCX 中使用同一树与相同目标。
- 重跑本能力直至 error 清零，再做人工 diff review；修复清单与理由记入书级 `制作说明.md`。
