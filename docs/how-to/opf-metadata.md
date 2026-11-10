# OPF 元数据：丛书、排序名、无障碍

> 状态：可照抄的写法。阅读器怎么显示这些信息还没实测，见文末。

从零做书时，元数据直接写在 `epub/OEBPS/package.opf` 的 `<metadata>` 里。改已有的 EPUB 成品时，丛书和排序名也可以用 `epub run epub.metadata.edit` 的 `series`、`series_position`、`title_sort`、`author_sort` 字段写入，见 [包操作](../pipeline/package-operations.md)。下面每段都能原样贴进起始书，改掉值即可。`epub book build` 的导航审计会检查 `refines` 指向、丛书挂接和无障碍词表。

## 丛书

```xml
<meta property="belongs-to-collection" id="series">丛书名</meta>
<meta refines="#series" property="collection-type">series</meta>
<meta refines="#series" property="group-position">2</meta>
```

- `collection-type` 用 `series`（连续出版的系列）或 `set`（成套出版的一组）。
- `group-position` 是本书在丛书里的序号，可以是小数（如 `2.5`）。
- `collection-type` 和 `group-position` 必须用 `refines` 挂在某个 `belongs-to-collection` 上，否则审计报 `warn`。
- 不属于丛书就不写。

## 排序名

```xml
<dc:title id="title">书名</dc:title>
<meta refines="#title" property="file-as">排序用书名</meta>
<dc:creator id="creator">作者名</dc:creator>
<meta refines="#creator" property="file-as">姓, 名</meta>
<meta refines="#creator" property="role" scheme="marc:relators">aut</meta>
```

- 给 `dc:title`、`dc:creator` 加 `id`，再用 `refines` 挂 `file-as`。起始书原有的 `dc:title`、`dc:creator` 没有 `id`，照抄时一起替换。
- `file-as` 只在书架排序和显示名不同时写，例如西文名“姓, 名”、书名去掉开头的冠词。
- `role` 可选：作者 `aut`，译者 `trl`，编者 `edt`，插图 `ill`。
- `refines` 指向不存在的 `id` 时审计报 `error`，构建会停下。

## 无障碍声明

声明必须属实。看过全书再写，拿不准的项不写。

```xml
<meta property="schema:accessMode">textual</meta>
<meta property="schema:accessModeSufficient">textual</meta>
<meta property="schema:accessibilityFeature">structuralNavigation</meta>
<meta property="schema:accessibilityFeature">tableOfContents</meta>
<meta property="schema:accessibilityFeature">readingOrder</meta>
<meta property="schema:accessibilityHazard">none</meta>
<meta property="schema:accessibilitySummary">正文为可重排文字，章节有标题与目录，无闪烁、动效或声音。</meta>
```

| 属性 | 什么时候能这样写 |
| --- | --- |
| `accessMode` = `textual` | 书里有文字内容（几乎所有书） |
| `accessMode` = `visual` | 书里有承载信息的图片、图表；有就要另写一行 |
| `accessModeSufficient` = `textual` | 所有承载信息的图片都有替代文字或正文说明，只读文字不丢信息 |
| `accessibilityFeature` = `structuralNavigation` | 章节都用真实的 `h1`–`h6` 标题 |
| `accessibilityFeature` = `tableOfContents` | 有 `nav` 目录且链接可用 |
| `accessibilityFeature` = `readingOrder` | spine 和页面源码顺序就是阅读顺序 |
| `accessibilityFeature` = `alternativeText` | 每张承载信息的图都写了 `alt` |
| `accessibilityFeature` = `pageNavigation` | nav 里有 page-list（见 [page-list.md](page-list.md)） |
| `accessibilityHazard` = `none` | 确认没有闪烁、动效模拟和声音；不确定就不写，或写 `unknown` |
| `accessibilitySummary` | 一两句话，说明上面这些声明的实际情况 |

`schema:` 是 EPUB 3 的保留前缀，不用在 `prefix` 里声明。`accessMode` 和 `accessibilityHazard` 的值不在标准词表里时审计报 `warn`：

- `accessMode`：`auditory`、`chartOnVisual`、`chemOnVisual`、`colorDependent`、`diagramOnTactile`、`mathOnVisual`、`musicOnVisual`、`tactile`、`textOnVisual`、`textual`、`visual`。
- `accessibilityHazard`：`flashing`、`noFlashingHazard`、`unknownFlashingHazard`、`motionSimulation`、`noMotionSimulationHazard`、`unknownMotionSimulationHazard`、`sound`、`noSoundHazard`、`unknownSoundHazard`、`unknown`、`none`。

起始书不预填无障碍声明，也不放注释示例：要作者看过这一页后自己写。

## 红线与验证

`epub redline --check metadata` 只比较 `dc:title`、`dc:creator`、`dc:identifier`、`dc:language`，加这些 `meta` 不会触发红线。给 `dc:title`、`dc:creator` 加 `id` 也不改它们的文字。

本页每段都贴进过起始书，`epub book build` 通过、`xmllint --noout` 通过，EPUBCheck 5.4.0 没有报这些元数据。

## 阅读器显示（待测）

Apple Books、Kindle、Reeden、Readest 是否显示丛书名和序号、是否按 `file-as` 排序，都还没有实测，也不在 `reader-matrix.yaml` 里。实测前不要据此承诺显示效果。
