# 多看弹注 fallback 修正说明

> 完整结构定义以 `docs/final/SPEC-实现约束.md` §1 为准。本指南只说明多看旧版 fallback 相对标准 EPUB3 弹注的增量、常见故障和实测方式。

## 1. 适用边界

只有目标书必须兼容多看旧版 popup note 时才叠加本 fallback。标准 EPUB3 弹注始终是主路径；不要为多看复制第二份注释正文，也不要把私有类反向写成通用结构。

开始前先确认：

- 标准弹注已经通过 `epub-popup-footnote-converter` 整理；
- noteref、注释目标和回跳位于同一 XHTML；
- 同一 XHTML 只有一个 grouped note body；
- 目标阅读器与版本可用于实测。

## 2. 多看增量

在符合 SPEC §1 的标准结构上只增加以下内容：

1. noteref 锚增加 `duokan-footnote`，并保留图片图标作为可见触发热区；
2. grouped `ol.footnote-list` 增加 `duokan-footnote-content`；
3. 每个 `li.footnote-item` 增加 `duokan-footnote-item`；
4. 已有本地图标资源时保留原 `img src`；缺少图标资源时加入 `Images/note.png`，并同步 OPF manifest；
5. 相关 CSS 写入 `Styles/notes.css`，不写入 `base.css` 或 `effects.css`。

最小差异应类似：

```html
<a class="noteref-icon duokan-footnote" epub:type="noteref" href="#footnote-1">
  <img alt="注" src="../Images/note.png"/>
</a>

<ol class="footnote-list duokan-footnote-content">
  <li class="footnote-item duokan-footnote-item" id="footnote-1">…</li>
</ol>
```

样式增量：

```css
ol.duokan-footnote-content {
  margin: 0;
  padding: 0;
  list-style-type: none;
}

.footnote-item.duokan-footnote-item {
  list-style-type: none;
}
```

## 3. 常见故障

### 点击无反应

检查 noteref 是否仍有 `epub:type="noteref"`、有效 `href` 和图片图标，以及 OPF 是否声明图标资源。

### 弹出空白、整页或整个列表

优先检查私有类位置。`duokan-footnote-content` 只标记 grouped `<ol>`；单条 `<li>` 只增加 `duokan-footnote-item`。不得让一个节点同时承担容器与单条目标角色。

### 标准阅读器退化

检查是否误删 `footnote-list`、`footnote-item`、`epub:type`、`role`、id 或 backlink。fallback 只能叠加，不得替换标准语义。

## 4. 仓库参考

- `templates/epub-style-demo/OEBPS/Text/05-legacy-note-fallback.xhtml`：单条 fallback；
- `templates/epub-style-demo/OEBPS/Text/06-multi-legacy-note-fallback.xhtml`：多条注释共享一个 grouped list；
- `skills/epub-legacy-footnote-fallback/SKILL.md`：可执行行为契约；
- `docs/final/reader-matrix.yaml`：阅读器实测记录。

## 5. 验证

先做静态和打包验证：

```sh
bash templates/epub-style-demo/build.sh
EPUB="$(ls -t templates/epub-style-demo/dist/*.epub | head -1)"
bash scripts/validate-epub-style-demo.sh --epub "$EPUB"
bash scripts/validate-popup-notes.sh --epub "$EPUB"
```

再在目标多看版本中检查：

1. 单条样例只弹出被命中的注释；
2. 多条样例逐个点击时内容互不串联；
3. 关闭弹层后正文位置稳定；
4. 普通跳转和 `◎` 回跳仍可使用。

实测后立即在 `docs/final/reader-matrix.yaml` 记录 artifact、阅读器名称和版本、现象、状态与待复测项。信息不足时只能记为待验证假设。
