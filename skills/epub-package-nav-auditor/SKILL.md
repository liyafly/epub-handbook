---
name: epub-package-nav-auditor
description: 审核和修复 EPUB package 结构、OPF metadata、manifest、spine、nav.xhtml、toc.ncx、封面声明、CSS/资源引用、MathML properties 和 EPUB zip 规则。用于新增、重命名、删除文件后，或 EPUB 本地构建通过但阅读器/Kindle 工具失败时。
---

# EPUB Package 与导航审核

这个 skill 用于结构或打包可能出错的场景。它处理 package 正确性，不处理视觉样式。

## 固定目标

一个 EPUB package 应当具备：

- `mimetype` 是 zip 第一项，且不压缩。
- `META-INF/container.xml` 指向 OPF。
- OPF metadata 有稳定 identifier 和必要阅读器提示。
- XHTML 根同时声明 `lang` 和 `xml:lang`；两者都缺失时只能复制 OPF `dc:language`，不得猜测或覆盖已有值。
- OPF manifest 声明所有被使用的 XHTML、CSS、图片、字体、nav、NCX 文件。
- 只有一个带 `properties="nav"` 的 nav item。
- Kindle/legacy 交付包包含 `toc.ncx` 和 `spine toc="ncx"`。
- spine 顺序匹配预期阅读顺序。
- 大合集、分卷文集和短篇全集的局部目录按 `docs/guides/anthology-navigation.md` 处理，局部目录只能作为辅助导航。
- 封面图片在需要时同时声明 EPUB 3 与旧 Kindle metadata。
- 含 MathML 的 manifest item 带 `properties="mathml"`。
- 含内联 SVG 的 XHTML manifest item 带 `properties="svg"`。

## 审核流程

1. 解析 `package.opf`、`nav.xhtml`、`toc.ncx` 和 `META-INF/container.xml`。
2. 建立映射：
   - manifest id -> href
   - href -> 磁盘文件
   - spine idref -> manifest item
   - nav/toc link -> 目标 XHTML
   - XHTML link -> CSS/image/font/note target
3. 检查每个 manifest href 存在，每个 spine idref 可解析。
4. 检查 nav/NCX 中每个 XHTML 存在，并符合预期阅读顺序。
   如目录加入正文内真实标题（例如 `h2`），为标题补稳定 fragment，并在 nav 和 NCX 中使用同一树与相同目标。
5. 检查每个 XHTML 根的 `lang` / `xml:lang` 与 OPF `dc:language`；缺一项时补齐另一项，缺两项且 OPF 无值时报告人工处理。
6. 检查每个被 XHTML 引用的 CSS/image/font 在打包要求下进入 OPF。
7. 检查特殊 properties：
   - `nav`
   - `cover-image`
   - `mathml`
   - `svg`
8. 只修最小结构问题，然后重新验证。

## 源文件可读性

涉及 XHTML 重写时，输出保持 XML-valid 且可人工 diff：保留 XML 声明与 HTML doctype，使用两空格结构缩进；块级元素分行，不把整个文档压成单行。不得为了格式化拆分段落、标题或其他 mixed-content 中的实际文本节点。

## 修复规则

- 尽量保留现有 id，避免无意义 churn。
- 新增 manifest item 使用稳定、描述性 id。
- manifest/spine 排序保持可复现。
- assets 不进入 spine。
- CSS 不进入 nav landmarks。
- 没有明确范围变化时，不从 Kindle/legacy fixture 删除 NCX。
- 不因为文件在磁盘上存在，就把未使用文件加入 OPF。
- 不以文件名、正文语言外观或阅读器猜测填充语言；OPF 缺失 `dc:language` 时保留 XHTML 现状并报告。

## 封面模式

```xml
<meta name="cover" content="cover-image"/>
...
<item id="cover-image"
      href="Images/cover.png"
      media-type="image/png"
      properties="cover-image"/>
```

## MathML 模式

```xml
<item id="math"
      href="Text/16-math.xhtml"
      media-type="application/xhtml+xml"
      properties="mathml"/>
```

## 内联 SVG 模式

```xml
<item id="border-shadow-notes"
      href="Text/19-border-shadow-notes.xhtml"
      media-type="application/xhtml+xml"
      properties="svg"/>
```

## 禁止事项

- 不通过从 spine 删除页面来掩盖 package 错误，除非该页面确实废弃。
- 不重命名文件，除非用户要求或当前命名破坏打包规则。
- 清理时不自动删除 `ibooks:specified-fonts` metadata；自由模式书是否移除交人工判断，见 `docs/final/SPEC-实现约束.md` §8。
- 不依赖浏览器 HTML 容错；XHTML 必须 XML-valid。
- 不让 nav/NCX 指向已删除或重命名文件。
- 有明确删除授权时，同时从 ZIP、manifest、spine、nav 和 NCX 删除同一资源；把精确删除列表写入报告，并在文本 gate 只 allow-list 这些 XHTML 和重新生成的 nav。

## 验证

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
```

如果本机没有 `xmllint`，仍要运行 Python validator；它会用标准库解析 XML，并检查 package invariants。

## Dry-run 约定

本 skill 默认 dry-run。直接调用只输出预期改动 JSON；加 `--commit` 才真正改。

调用示例：

```sh
# 预览
<skill-invocation> work/before/source.epub > work/dry-run.json

# 审查
cat work/dry-run.json | jq

# 确认后执行
<skill-invocation> --commit work/before/source.epub
```

dry-run 输出格式见 [docs/pipeline/cleanup-flow.md](../../docs/pipeline/cleanup-flow.md)。
