---
name: epub-popup-footnote-converter
description: 将 EPUB 普通注释、尾注、旧式注释或纯文本注释标记转换为项目标准 popup footnote 结构：图片注释图标触发、同文件 grouped note body、◎ 回跳，并保留注释内容。
---

# EPUB Popup Footnote 转换

当需要把普通脚注、尾注、旧多看注释或纯文本 noteref 标记转换为项目最终 popup footnote 模式时使用这个 skill。

## 固定目标

权威结构源是 `docs/final/SPEC-实现约束.md` §1；以下内容保持 skill 可独立执行，并不得与该节分叉。

使用这个结构：

- 任何使用 `epub:type` 的 XHTML 根 `<html>` 都声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。
- noteref 是带 `epub:type="noteref"` 和 `role="doc-noteref"` 的 `<a>`。
- noteref 内容是图片图标，通常为 `../Images/note.png`；已有本地图标资源时保留原 `img src`，本 skill 的 `assets/note.png` 只作为纯文本标记转换时的默认图标。
- 每个 XHTML 文件最多一个 grouped note body：`<aside epub:type="footnote" role="doc-footnote">`。
- 该 XHTML 文件内所有 notes 放进 `ol.footnote-list`。
- 每条 note target 是带目标 `id` 的 `li.footnote-item`。
- noteref `href` 指向对应 `li.footnote-item` id，不指向独立 per-note aside。
- 回跳符号是 `◎`。
- noteref、target `li` 和包含它的 aside 位于同一 XHTML 文件。
- 注释正文精确保留。
- 私有 note 机制不能作为主路径。

## 转换流程

1. 读取包含 note reference 和 note body 的 XHTML 文件。
2. 确保 XHTML 根 `<html>` 声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。已有声明则保留，不重复添加。
3. 尽量保留已有 note id。只有缺失或冲突时才规范化。
4. 识别 Sigil 旧结构：`section[epub:type="footnotes"]` 内含多个 `aside#footnote_N`，正文引用为 `a#noteref_N`。仅当该 section 的所有 aside 都能匹配时，保留原 ID 并合并为一个 grouped `aside/ol/li`；有任何无法识别的 aside 或附加内容时不做部分转换，改为人工 review。
5. 把 `[1]`、`*`、`注` 等文本标记替换为图片 noteref；如果原 noteref 已经包含图片图标，保留原 `img src` 和 `alt`，不替换为默认图标。`href` 必须指向最终 `li.footnote-item` target id：

```html
<sup class="note-marker">
  <a id="note-1"
     class="noteref-icon"
     epub:type="noteref"
     role="doc-noteref"
     href="#footnote-1">
    <img alt="注" src="../Images/note.png"/>
  </a>
</sup>
```

6. 把同一 XHTML 文件内所有 note body 转成一个 grouped aside：

```html
<aside epub:type="footnote" role="doc-footnote">
  <div><hr class="footnote-line xian"/></div>

  <ol class="footnote-list">
    <li class="footnote-item" id="footnote-1">
      <p class="footnote">
        <a class="footnote-back"
           epub:type="backlink"
           role="doc-backlink"
           href="#note-1">◎</a>
        注释内容。
      </p>
    </li>
  </ol>
</aside>
```

7. 源文件使用旧 `duokan-*` note 类时，保留 grouped `ol/li` 结构，但改成 `footnote-list`、`footnote-item` 等中性类。不要把 `duokan-*` 类作为主输出。
8. 校验每个 noteref 图标都在 OPF manifest 中声明且文件存在。只有 EPUB 还没有可用注释图标、且需要从纯文本标记生成图片触发器时，才把本 skill 的 `assets/note.png` 复制进 EPUB 的 `Images/` 目录并补 manifest。
9. 把下面 CSS 加入活动 stylesheet，或合并进已有 note section。
10. 验证每个 noteref `href="#footnote-x"` 都指向 `li.footnote-item`，每个 backlink 都能回跳，每个有 notes 的文件只有一个 grouped footnote aside，且每条 note 都留在同一 XHTML 文件。

## CSS

```css
sup.note-marker {
  font-size: 1em;
  line-height: 0;
  vertical-align: baseline;
}

sup.note-marker > .noteref-icon {
  display: inline-block;
  line-height: 0;
  position: relative;
  top: -0.14em;
  text-decoration: none;
}

sup.note-marker > .noteref-icon > img {
  display: block;
  width: auto;
  height: 0.72em;
  max-width: none;
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

.footnote-item {
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

## CSS 放置

- 在本仓库 layered demo 中，footnote CSS 必须写进 `Styles/notes.css`。
- 不把 footnote CSS 写进 `poster.css`。
- `@font-face` 和字体工具类属于 `Styles/fonts.css`。

## 禁止事项

- 除非没有图标资源且用户同意，不把图片图标替换为纯文本。
- 已有图片图标不得无差别替换为默认 `Images/note.png`；默认图标只用于纯文本/数字上标标记转换。
- 不使用无作用域的 `sup img` 或裸 `sup` 图标规则；只给图片 noteref 的 `sup.note-marker` 设置零行高和相对上移，普通文字上标保持原样。
- 不对 footnote body 使用 `display:none`。
- 不把 notes 移到另一个 XHTML 文件。
- 同一文件包含多条 notes 时，不输出每条一个 aside；必须用一个 aside + `ol/li` 分组。
- 不改写注释正文。
- 不把 `duokan-wavyline`、多看专属 notes 或 JS 作为主机制。
- 如果目标 EPUB 需要多看旧版兼容，先做本转换，再应用 `epub-legacy-footnote-fallback`。

## 验证 fixture

使用 `templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml` 作为本地 popup footnote 参考。转换文件应满足：

- noteref `href` 指向同一 XHTML 内的 `li.footnote-item`。
- XHTML 根 `<html>` 声明 `xmlns:epub="http://www.idpf.org/2007/ops"`。
- 每个文件用一个 grouped `aside epub:type="footnote"` 容纳所有本地 notes。
- backlinks 使用 `epub:type="backlink"` 和 `role="doc-backlink"`。
- EPUB 有或能接收图标资源时，note trigger 使用图片图标。

转换后运行：

```sh
scripts/validate-popup-notes.sh
python3 scripts/validate_text_invariance.py before.epub after.epub --check all
```

`validate_text_invariance.py` 只将 noteref/backlink 的数字、图标和 `◎` 当作表示控件；所有 `li.footnote-item` 的注释正文仍必须逐字相同。

验证已构建产物：

```sh
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```

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
