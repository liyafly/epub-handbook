---
name: epub-popup-footnote-converter
description: 将 EPUB 普通注释、尾注、旧式注释或纯文本注释标记转换为项目标准 popup footnote 结构：图片注释图标触发、同文件 grouped note body、◎ 回跳，并保留注释内容。
---

# EPUB Popup Footnote 转换

当需要把普通脚注、尾注、旧多看注释或纯文本 noteref 标记转换为项目最终 popup footnote 模式时使用这个 skill。

## 固定目标

使用这个结构：

- noteref 是带 `epub:type="noteref"` 和 `role="doc-noteref"` 的 `<a>`。
- noteref 内容是图片图标，通常为 `../Images/note.png`；本 skill 的 `assets/note.png` 是默认图标。
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
2. 尽量保留已有 note id。只有缺失或冲突时才规范化。
3. 把 `[1]`、`*`、`注` 等文本标记替换为图片 noteref。`href` 必须指向最终 `li.footnote-item` target id：

```html
<sup>
  <a id="note-1"
     class="noteref-icon"
     epub:type="noteref"
     role="doc-noteref"
     href="#footnote-1">
    <img alt="注" src="../Images/note.png"/>
  </a>
</sup>
```

4. 把同一 XHTML 文件内所有 note body 转成一个 grouped aside：

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

5. 源文件使用旧 `duokan-*` note 类时，保留 grouped `ol/li` 结构，但改成 `footnote-list`、`footnote-item` 等中性类。不要把 `duokan-*` 类作为主输出。
6. 如果 OPF manifest 未列出 `Images/note.png`，补上该资源。EPUB 还没有注释图标时，把本 skill 的 `assets/note.png` 复制进 EPUB 的 `Images/` 目录。
7. 把下面 CSS 加入活动 stylesheet，或合并进已有 note section。
8. 验证每个 noteref `href="#footnote-x"` 都指向 `li.footnote-item`，每个 backlink 都能回跳，每个有 notes 的文件只有一个 grouped footnote aside，且每条 note 都留在同一 XHTML 文件。

## CSS

```css
sup {
  vertical-align: middle;
  line-height: 1;
}

.noteref-icon {
  text-decoration: none;
}

.noteref-icon img {
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
- 不对 footnote body 使用 `display:none`。
- 不把 notes 移到另一个 XHTML 文件。
- 同一文件包含多条 notes 时，不输出每条一个 aside；必须用一个 aside + `ol/li` 分组。
- 不改写注释正文。
- 不把 `duokan-wavyline`、多看专属 notes 或 JS 作为主机制。
- 如果目标 EPUB 需要多看旧版兼容，先做本转换，再应用 `epub-legacy-footnote-fallback`。

## 验证 fixture

使用 `templates/epub-style-demo/OEBPS/Text/02-ruby-note.xhtml` 作为本地 popup footnote 参考。转换文件应满足：

- noteref `href` 指向同一 XHTML 内的 `li.footnote-item`。
- 每个文件用一个 grouped `aside epub:type="footnote"` 容纳所有本地 notes。
- backlinks 使用 `epub:type="backlink"` 和 `role="doc-backlink"`。
- EPUB 有或能接收图标资源时，note trigger 使用图片图标。

转换后运行：

```sh
scripts/validate-popup-notes.sh
```

验证已构建产物：

```sh
scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/<artifact>.epub
```
