# EPUB2 外壳中的 Popup Note 兼容写法

> 范围：已有 OPF `version="2.0"` 的 EPUB，因为目标阅读器兼容需求，暂时不能完整迁移 EPUB3，但希望尝试 `epub:type="noteref"` / `epub:type="footnote"` 弹注识别。
>
> 这是一条需要实测的兼容路径，不是本仓默认发行路径。新书和允许迁移的旧书仍应优先交付 EPUB3 主包。

## 1. 先判断你要解决什么问题

注释至少有三种目标：

| 目标 | 推荐写法 |
| --- | --- |
| 所有阅读器都能读 | 双向超链接：正文跳到注释，注释跳回正文 |
| EPUB3 阅读器识别标准弹注 | `xmlns:epub` + `epub:type="noteref"` + `epub:type="footnote"` |
| 旧多看版本识别私有弹注 | 在项目标准结构上再叠加 `duokan-*` 类，见 [多看弹注 fallback 修正说明](duokan-footnote-fallback-fix.md) |

不要把三个目标合并成一句“支持弹注”。某个平台显示弹窗，可能来自 EPUB3 标准语义，也可能来自普通脚注链接识别，还可能来自平台私有协议。

## 2. 你需要补的 XHTML 根声明

在使用 `epub:type` 的 XHTML 文件根元素上增加：

```xml
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
```

其中：

- 默认 `xmlns` 声明 XHTML namespace；
- `xmlns:epub` 声明 `epub:` 前缀；
- 只有声明后，XML 解析器才知道 `epub:type` 的 `epub` 是什么。

这就是历史 EPUB 制作资料里常说的“给头文件加一行”。它是必要条件，但不是完整迁移步骤，也不是跨平台弹窗保证。

## 3. 严格 EPUB2 基线：先保证能跳转

如果交付要求是严格 EPUB2，先使用普通 XHTML 元素和双向链接：

```html
<p>
  正文
  <a id="note-ref-1" href="#note-1"><sup>[1]</sup></a>
</p>

<div class="footnotes">
  <p id="note-1">
    <a href="#note-ref-1">[1]</a>
    注释正文。
  </p>
</div>
```

这个结构不会依赖 popup：

- 阅读器认识脚注时，可以自行增强显示；
- 阅读器不认识时，至少可以上下跳转；
- 注释正文始终是真实文本。

## 4. 兼容模式 A：保留 EPUB2 友好元素

如果需要尝试 `epub:type`，但希望尽量保留 EPUB2 能理解的元素，可以使用 `<div>` 或 `<p>` 作为注释目标：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml"
      xmlns:epub="http://www.idpf.org/2007/ops"
      xml:lang="zh-CN">
<head>
  <title>第一章</title>
</head>
<body>
  <p>
    正文
    <a id="note-ref-1"
       epub:type="noteref"
       href="#note-1"><sup>[1]</sup></a>
  </p>

  <div class="footnotes">
    <p id="note-1" epub:type="footnote">
      <a href="#note-ref-1">◎</a>
      注释正文。
    </p>
  </div>
</body>
</html>
```

这个版本的特点：

- 使用 XHTML 1.1 中已有的 `<a>`、`<div>` 和 `<p>`；
- `epub:type` 仍属于 EPUB3 namespace 扩展；
- Apple Books 官方文档说明，在 EPUB3 包中用 `<div>` 或 `<p>` 替代 `<aside>` 时，点击仍可弹窗，但注释内容也会出现在普通阅读流中；
- 放进 EPUB2 外壳后的具体行为仍需按目标阅读器测试。

如果“注释正文显示在章末”可以接受，这通常比依赖 `<aside>` 更保守。

## 5. 兼容模式 B：使用 `<aside>` 尝试隐藏注释正文

历史制作资料常用：

```html
<p>
  正文
  <a id="note-ref-1"
     epub:type="noteref"
     href="#note-1">[1]</a>
</p>

<aside id="note-1" epub:type="footnote">
  <p><a href="#note-ref-1">◎</a>注释正文。</p>
</aside>
```

支持该行为的阅读器可以把 `<aside epub:type="footnote">` 识别成弹窗正文，并从主阅读流隐藏。不能依赖该行为的阅读器仍应能沿 `href` 找到注释。

边界必须写清楚：

- `<aside>` 是 EPUB3 / HTML5 语义路径，不是严格 EPUB2 XHTML 主路径；
- OPF 仍写 `version="2.0"` 时，这是混合兼容包；
- 目标阅读器、版本、artifact、epubcheck 输出和降级表现必须记录。

## 6. EPUB3 主包仍应使用项目标准结构

本仓 EPUB3 默认不是每条注释各放一个 `<aside>`，而是同一 XHTML 内聚合：

```html
<p>
  正文
  <sup>
    <a id="note-ref-1"
       class="noteref-icon"
       epub:type="noteref"
       role="doc-noteref"
       href="#note-1">
      <img alt="注" src="../Images/note.png"/>
    </a>
  </sup>
</p>

<aside epub:type="footnote" role="doc-footnote">
  <ol class="footnote-list">
    <li class="footnote-item" id="note-1">
      <p class="footnote">
        <a epub:type="backlink"
           role="doc-backlink"
           href="#note-ref-1">◎</a>
        注释正文。
      </p>
    </li>
  </ol>
</aside>
```

这个结构用于本仓 demo、validator 和转换 skill。EPUB2 兼容实验不要反向改变 EPUB3 主路径。

## 7. 验证清单

### 7.1 静态检查

```sh
unzip -t book.epub
python3 scripts/epub_preflight_harness.py book.epub --format json
epubcheck book.epub
```

本机未安装 epubcheck 时：

```sh
brew install epubcheck
```

### 7.2 注释行为

每个平台至少验证：

1. 点击正文 noteref 是否弹窗或跳转；
2. 注释正文是否完整；
3. 不弹窗时是否仍可跳转；
4. 是否能从注释回到原文；
5. 大字号、夜间模式和窄屏是否仍可读；
6. 注释跨 XHTML 时是否仍工作。项目标准主路径优先把 noteref 和注释放在同一 XHTML。

### 7.3 记录方式

写入 [reader-matrix.yaml](../final/reader-matrix.yaml) 时记录：

- OPF 是 `2.0` 还是 `3.0`；
- 使用 `<p>/<div>` 还是 `<aside>`；
- reader 名称和版本；
- artifact 路径；
- epubcheck 结果；
- 弹窗、跳转或失败现象；
- 回跳是否成功。

## 8. 参考依据

- 本地历史参考：`_epub_reference/epub-guide/OEBPS/Text/Chapter10-6.xhtml`。该参考书自身是 EPUB3 包，相关段落用于记录制作经验，不作为 EPUB2 样本证明。
- [Apple Books Asset Guide: Pop-up Footnotes](https://help.apple.com/itc/booksassetguide/en.lproj/itccf8ecf5c8.html)：说明 EPUB3 popup footnote、`xmlns:epub` 必要条件，以及 `<aside>` 与 `<div>/<p>` 的显示差异。
- [IDPF OPS 2.0.1](https://idpf.org/epub/20/spec/OPS_2.0.1_draft.htm)：说明 EPUB2 OPS Content Document 的 XHTML modules 基线。
- [W3C OPS Namespace](https://www.w3.org/ns/epub/2007/ops/)：说明 `http://www.idpf.org/2007/ops` namespace。
