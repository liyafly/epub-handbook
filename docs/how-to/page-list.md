# 页码对照（page-list）

> 状态：可照抄的写法。阅读器是否显示纸书页码还没实测，见文末。

有纸书时，可以在正文里标出纸书每页的起点，让读者按纸书页码跳转、引用。没有纸书对照就不要加。

## 三处写法

正文里，在纸书每页开始的位置放一个空的页码标记。`id` 全书唯一，`aria-label` 写纸书页码：

```xml
<p>上一页最后一段的文字，<span epub:type="pagebreak" role="doc-pagebreak" id="page-12" aria-label="12"/>下一页从这里开始。</p>
```

`nav.xhtml` 里加一个 page-list，按页码顺序列出每个标记：

```xml
<nav epub:type="page-list" id="page-list" hidden="">
  <ol>
    <li><a href="Text/01-chapter.xhtml#page-12">12</a></li>
    <li><a href="Text/01-chapter.xhtml#page-13">13</a></li>
  </ol>
</nav>
```

OPF 的 `<metadata>` 里写页码来自哪个纸书版本，用该版的 ISBN：

```xml
<meta property="pageBreakSource">urn:isbn:9780000000000</meta>
```

不要写成 `a11y:pageBreakSource`：EPUBCheck 5.4.0 报它是未定义属性（OPF-027）。不带前缀的写法和旧写法 `<dc:source id="print-source">…</dc:source>` 加 `<meta refines="#print-source" property="source-of">pagination</meta>` 都通过了 EPUBCheck 5.4.0。

## 审计会查什么

- page-list 里的链接指向不存在的文件或 `id`：`error`，构建停下。
- 正文有页码标记但 nav 没有 page-list，或有 page-list 但正文没有标记：`warn`。
- 标记认 `epub:type="pagebreak"` 或 `role="doc-pagebreak"`，两个都写最稳。

## 红线

实际跑过：在起始书第一章加一个空的页码标记、在 OPF 加页码来源，`epub redline --check all` 前后对比全部通过。`anchors` 只报被删除的 `id`，新增的不报；空 `span` 没有文字，`text` 也不报。

nav 里新加的 page-list 会被 `text` 红线算作 nav 文字变化（块数不同），对比加 page-list 前后的成品时会报 `error`。这是预期的结构变化：确认只改了 nav 后，在制作说明里记下。`epub book build` 本身比较的是源和成品，不受影响。

## 阅读器显示（待测）

Apple Books、Kindle、Reeden、Readest 是否显示纸书页码、能否按页码跳转，都还没有实测。实测前不要承诺效果。
