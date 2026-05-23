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
- OPF manifest 声明所有被使用的 XHTML、CSS、图片、字体、nav、NCX 文件。
- 只有一个带 `properties="nav"` 的 nav item。
- Kindle/legacy 交付包包含 `toc.ncx` 和 `spine toc="ncx"`。
- spine 顺序匹配预期阅读顺序。
- 封面图片在需要时同时声明 EPUB 3 与旧 Kindle metadata。
- 含 MathML 的 manifest item 带 `properties="mathml"`。

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
5. 检查每个被 XHTML 引用的 CSS/image/font 在打包要求下进入 OPF。
6. 检查特殊 properties：
   - `nav`
   - `cover-image`
   - `mathml`
7. 只修最小结构问题，然后重新验证。

## 修复规则

- 尽量保留现有 id，避免无意义 churn。
- 新增 manifest item 使用稳定、描述性 id。
- manifest/spine 排序保持可复现。
- assets 不进入 spine。
- CSS 不进入 nav landmarks。
- 没有明确范围变化时，不从 Kindle/legacy fixture 删除 NCX。
- 不因为文件在磁盘上存在，就把未使用文件加入 OPF。

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

## 禁止事项

- 不通过从 spine 删除页面来掩盖 package 错误，除非该页面确实废弃。
- 不重命名文件，除非用户要求或当前命名破坏打包规则。
- 清理时不删除 `ibooks:specified-fonts` metadata。
- 不依赖浏览器 HTML 容错；XHTML 必须 XML-valid。
- 不让 nav/NCX 指向已删除或重命名文件。

## 验证

运行：

```sh
sh templates/epub-style-demo/build.sh
scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/<artifact>.epub
xmllint --noout templates/epub-style-demo/OEBPS/package.opf templates/epub-style-demo/OEBPS/nav.xhtml templates/epub-style-demo/OEBPS/toc.ncx
```

如果本机没有 `xmllint`，仍要运行 Python validator；它会用标准库解析 XML，并检查 package invariants。

