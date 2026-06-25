# 大合集与分卷导航

> 状态：基础规则；用于短篇全集、作品合集、分卷文集和多部合订 EPUB。英文正文排版见 `english-fiction-layout.md`。

## 大合集结构模式

短篇全集、作品合集和分卷文集不适合做成“整本一个 HTML”。更稳的结构是按卷、作品集或主题拆成多个 XHTML；每个卷或作品集开头有局部目录，正文每篇后面可用短链接回当前作品集目录，也可以在局部目录页提供回总目录链接。

这个逻辑适合大合集，但它必须是“辅助导航”，不能替代 EPUB 的主目录和 spine。

## 推荐结构

- 主导航：EPUB 3 `nav.xhtml` + Kindle/legacy `toc.ncx`，覆盖总目录、卷、作品集和主要作品。
- 局部目录：每个卷或作品集开头保留一个本卷目录，如 `id="collection-toc"`。
- 回本卷目录：每篇短文末尾可以放一个短链接，帮助读者读完一篇后回到本卷目录。
- 回总目录：放在本卷目录页，不必在每篇末尾都重复。
- 文件拆分：大型合集按卷或作品集拆 XHTML，避免单文件过长导致编辑、转换和阅读器定位变慢。

## 超大古籍 / 工具书

检索型大部头可以比普通合集保留更细的主目录粒度。五百卷级别的古籍样本证明，`nav.xhtml` 和 `toc.ncx` 可以同时覆盖卷级入口和条目级锚点，但前提是这些链接由结构化 source 自动生成。

推荐约束：

- 按卷、篇、部类或作品集拆 XHTML；不要把全书压成一个超长文件。
- 每卷内每个条目有真实标题元素和稳定 `id`，供 nav / NCX / 搜索结果定位。
- 条目级主目录只用于工具书、古籍、短篇索引型合集；普通小说/散文不要滥用。
- 条目级 nav 必须自动生成，并在构建时检查链接目标存在。
- 生成器临时 id 可以用于一次性样本；长期维护版优先使用语义化稳定 id。
- 如果主 nav 超过数千条，同时保留卷级局部目录或按卷目录页，避免阅读器目录面板过深时难以浏览。

## 关于 `(•)` 回目录点

`(•)` 可以保留为视觉形式，尤其适合复古短篇集。但生产版不要只放一个无说明的点号：

- 最稳：可见文案，如“回本卷目录”。
- 复古视觉：保留 `(•)`，同时加 `title` 和 `aria-label`。
- 不推荐：每篇末尾只有裸 `(•)`，没有任何说明。

```html
<p class="local-nav">
  <a href="#collection-toc" title="Back to this collection contents" aria-label="Back to this collection contents">(•)</a>
</p>
```

中文项目可以用：

```html
<p class="local-nav">
  <a href="#collection-toc" title="回本卷目录" aria-label="回本卷目录">(•)</a>
</p>
```

## 推荐 XHTML 形态

```html
<section epub:type="part" id="collection-a">
  <nav epub:type="toc" id="collection-toc">
    <h2>Collection Title</h2>
    <ol>
      <li><a href="#story-01">Story One</a></li>
      <li><a href="#story-02">Story Two</a></li>
    </ol>
    <p class="local-nav"><a href="../nav.xhtml">Back to main contents</a></p>
  </nav>

  <section epub:type="chapter" id="story-01">
    <h3>Story One</h3>
    <p>...</p>
    <p class="local-nav">
      <a href="#collection-toc" title="Back to this collection contents" aria-label="Back to this collection contents">(•)</a>
    </p>
  </section>
</section>
```

## 检查清单

- `nav.xhtml` 是否能从总目录进入每个卷或作品集。
- `toc.ncx` 是否覆盖 Kindle/旧阅读器需要的层级。
- 每个局部目录是否有稳定 `id`。
- 每个回目录链接是否指向当前卷目录，而不是误回总目录。
- 符号型链接是否有 `title` / `aria-label` 或可见文案。
- 是否避免把上百篇短文塞进一个超长 XHTML。
