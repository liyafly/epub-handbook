# 一键 EPUB3 转换脚本

> 状态：流程文档；用于把一本旧 EPUB/EPUB2 在本地转换为 EPUB3，并套用项目的弹注与 CJK 文学排版基础层。
> 脚本：`scripts/epub3_oneclick_converter.py`

## 适用范围

适合这类输入：

- EPUB2 或缺少 `nav.xhtml` 的旧包。
- `toc.ncx` 来自 Kindle/MOBI 反解，存在 `src="Text/file.xhtml"#id` 这类坏片段引号。
- 注释是同文件 `wN -> mN` 普通尾注。
- CSS 里正文使用不存在的 `cnepub` 或过旧的宋/黑/楷字体名。

不处理：

- OCR 校对。
- 正文改写。
- 图片压缩或转码。
- 字体内嵌。脚本只写多字体使用规则，不打包字体。

## 一键命令

用脱敏工作目录承接真实文件，不把真实文件名写进报告：

```sh
mkdir -p work/before work/after
cp /path/to/input.epub work/before/source.epub

python3 scripts/epub3_oneclick_converter.py \
  work/before/source.epub \
  --output work/after/cleaned.epub \
  --format json > work/after/cleaned.report.json
```

输出 EPUB 会包含：

- EPUB3 `package version="3.0"`。
- `dcterms:modified`。
- `ibooks:specified-fonts`。
- 新建 `nav.xhtml`，保留 `toc.ncx` 和 `spine toc="ncx"`。
- 修正 `mimetype` 为 zip 第一项且 stored。
- 修正 `guide` 中可自动识别的坏相对路径。
- 新增 `Styles/epub3-enhancements.css`。
- 新增 `Images/note.png`。
- 普通尾注转为同文件 grouped popup footnote。

## 字体策略

脚本注入的覆盖层遵循系统优先，不嵌入字体：

- 正文：`"Songti SC", "SimSun", "Source Han Serif SC", serif`
- 标题：`"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`
- 楷体类、引用、注释：`"Kaiti SC", "STKaiti", "KaiTi", serif`

后续如需内嵌字体，只替换或补充显式类，不把子集字体挂到 `body`。

## 弹注结构

普通尾注：

```html
<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a>
...
<p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文。</p>
```

会转为：

```html
<sup>
  <a id="w1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#m1">
    <img alt="注" src="../Images/note.png"/>
  </a>
</sup>

<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>
  <ol class="footnote-list">
    <li class="footnote-item" id="m1">
      <p class="footnote">
        <a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#w1">◎</a>注释正文。
      </p>
    </li>
  </ol>
</aside>
```

## 验证

```sh
unzip -tqq work/after/cleaned.epub
python3 scripts/epub_preflight_harness.py work/after/cleaned.epub --format json
bash scripts/validate-popup-notes.sh --epub work/after/cleaned.epub
python3 scripts/validate_text_invariance.py \
  work/before/source.epub \
  work/after/cleaned.epub \
  --check metadata,drm,anchors \
  --allow-list '*/nav.xhtml'
```

如果启用弹注转换，普通 `--check text` 会因为 `[1]` 被图片触发器替换而失败。此时追加一条“去注释编号/回跳符后文本等价”的专用检查，确认正文与注释内容没有被改写。

Kindle Previewer 可选：

```sh
mkdir -p work/after/kindle-preview-output
'/Applications/Kindle Previewer 3.app/Contents/MacOS/Kindle Previewer 3' \
  work/after/cleaned.epub \
  -convert -qualitychecks \
  -output work/after/kindle-preview-output \
  -locale zh
```

通过标准：

- Summary log：增强排版状态为支持。
- 转码状态为成功。
- 错误数为 `0`。
- 质量问题数量为 `0`。

## 脱敏记录

提交到仓库的文档只记录：

- 输入 SHA-256。
- 转换计数，例如 nav 条目数、弹注数量、CSS 链接数量。
- 输出文件角色，例如 `work/after/cleaned.epub`。
- 工具版本和错误/质量问题数量。

不要提交：

- 原 EPUB。
- 转换后 EPUB/KPF。
- 真实书名、作者、ISBN、ASIN、水印、私有 metadata。
- Kindle Previewer 生成的完整临时路径日志，除非已替换成本地占位路径。
