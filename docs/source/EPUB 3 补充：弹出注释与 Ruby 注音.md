# EPUB 3 补充：弹出注释与 Ruby 注音

> 主手册（`EPUB3_制作完全参考手册.md`）的速查式补充（二）。
> 适用范围与主手册一致：可重排 EPUB 3.3，固定版式 (FXL) 不在内。
> 目标阅读器：Apple Books 4.x、Thorium 3.x、Calibre 7.x、Kindle Previewer 3 / KFX、Kobo、KOReader 2024+。
> 国内主流（多看 5.x+、微信读书、京东读书）兼容方案见 §一.6。
>
> 系列其他文档：
> - 列表与字体 → `EPUB3_补充_列表与字体.md`
> - 图片与海报页 → `EPUB3_补充_图片与海报页.md`
> - 其他 CSS → `EPUB3_补充_其他CSS.md`

---

## 目录

1. [弹出注释（唯一推荐规则）](#一弹出注释唯一推荐规则)
2. [Ruby 注音（行间注）](#二ruby-注音行间注)
3. [对主手册的补丁建议（本主题相关）](#三对主手册的补丁建议本主题相关)
4. [阅读器兼容矩阵（节选）](#四阅读器兼容矩阵节选)

---

## 一、弹出注释（唯一推荐规则）

一份代码，跨所有 Kindle 固件（KF8 + KFX）、Apple Books、Thorium、Kobo、Calibre、KOReader 都能弹出。

### 1.1 HTML 模板

```xml
<!-- 正文：noteref + sup -->
<p>……提到某概念<sup><a epub:type="noteref"
       role="doc-noteref"
       id="fnref1"
       href="#fn1">[1]</a></sup>……</p>

<!-- 章末：footnotes 容器（与上面 noteref 同一 xhtml 文件） -->
<section epub:type="footnotes">
  <aside epub:type="footnote" role="doc-footnote" id="fn1">
    <p><a id="fn1back" href="#fnref1">[1]</a>
       这是注释内容，可以包含<em>多个段落</em>、链接、图片。</p>
  </aside>
</section>
```

**为什么这样写**：

- `<a>` 上同时有 `id` 和 `href`，aside 内有反向 `<a id href>` → 命中 KF8 双向锚点启发式，老 Kindle 弹
- `epub:type="noteref"` + `<aside epub:type="footnote">` → 命中 EPUB 3 / KFX / Apple Books / Thorium 标准
- `role="doc-noteref"` / `role="doc-footnote"` → 部分阅读器只认 ARIA 时兜底
- `<sup>` 包在外面，标记视觉一致

### 1.2 CSS 模块

```css
a[epub|type~="noteref"],
a[role~="doc-noteref"] {
  text-decoration: none;
}

/* sup 内嵌时让 sup 控制上抬，<a> 保持基线 */
sup a[epub|type~="noteref"],
sup a[role~="doc-noteref"] {
  font-size: 0.75em;
  vertical-align: baseline;
}

aside[epub|type~="footnote"],
aside[role~="doc-footnote"] {
  font-size: 0.9em;
  margin: 1em 0;
  padding: 0.5em;
  border-top: 1px solid #888;
}

section[epub|type~="footnotes"] {
  margin-top: 2em;
  padding-top: 1em;
  border-top: 1px solid #ccc;
  page-break-before: always;
}

/* 绝对不要写 aside[epub|type~="footnote"] { display: none; }
   阅读器自己接管弹注的显隐 */
```

### 1.3 七条硬规则

1. `<a>` 必须有 `epub:type="noteref"`，建议加 `role="doc-noteref"`
2. 每个含注释的 XHTML 文件只放一个 `<aside epub:type="footnote">`，建议加 `role="doc-footnote"`
3. 多条注释放在同一个 `aside` 内的 `ol.footnote-list > li.footnote-item`
4. 每条 `li.footnote-item` 带全文档唯一 `id`，正文 `noteref` 直接指向这个 `li`
5. `li.footnote-item` 必须包**整段**注释内容（KFX 检查最严）
6. **`<aside>` 与目标 `li` 必须和 `<a noteref>` 在同一 xhtml 文件**（Apple Books 限制；跨文件强制退化为跳转）
7. 不要给 `<aside>` 写 `display: none`，阅读器接管显隐

### 1.4 各阅读器实际行为

| 阅读器 | 触发 | 弹出形式 | 内容长度上限 |
|---|---|---|---|
| Apple Books | 单击 | iPhone 底部 sheet / iPad-Mac 浮卡 | ~3 屏后加"继续阅读" |
| Thorium | 单击 | 居中浮窗 | 无硬上限，可滚动 |
| Calibre | 悬停 / 单击 | 工具提示卡 | 无硬上限 |
| Kindle KFX | 单击 | 底部抽屉，可上拖 | ~2000 字 |
| Kindle KF8（老固件） | 单击 | 底部 1/3 抽屉 | ~500 字硬截 |
| Kobo / Moon+ | 单击 | 弹卡 | 同 Thorium |
| KOReader | 单击 | 弹卡 | 同 Thorium |

### 1.5 KF8 / KFX / Apple Books 关键差异（速查）

| 维度 | KF8 | KFX | Apple Books |
|---|---|---|---|
| 触发机制 | 双向 `<a>` 启发式 | `epub:type` + 双向兼容 | `epub:type` / `role` |
| 注释最大长度 | ~500 字硬截 | ~2000 字可滚动 | ~3 屏后截 |
| 跨文件 `aside` | 不支持弹 | 支持 | **不弹**，退化跳转 |
| 图标源（`<a><img></a>`） | 多不识别 | 识别 | 识别 |
| CSS 作用于弹层 | 多忽略 | 完整 | 完整（夜间模式覆盖颜色） |
| 多段注释 | 截到首段 | 完整 | 完整 |

### 1.6 多看兼容（历史探索，不作为最终方案）

国内流通量较大的多看有私有扩展。以下内容仅保留早期兼容探索，最终手册和 skill 不采用 `duokan-*` 作为主路径。正式制作时使用 `aside + ol.footnote-list + li.footnote-item` 的中性结构。

```xml
<!-- 正文：在 <a> 上叠加多看类名 -->
<p>……提到某概念<sup><a epub:type="noteref"
       role="doc-noteref"
       class="duokan-footnote"
       id="fnref1"
       href="#fn1">[1]</a></sup>……</p>

<!-- 章末：在 <aside> 上叠加多看注释项类名 -->
<section epub:type="footnotes">
  <aside epub:type="footnote" role="doc-footnote"
         class="duokan-footnote-item"
         id="fn1">
    <p><a id="fn1back" href="#fnref1">[1]</a>
       这是注释内容，可包含<em>多个段落</em>、链接、图片。</p>
  </aside>
</section>
```

差异说明（相对 §6.1 仅两处）：

| 元素 | §6.1 | §6.6 增加 | 作用 |
|---|---|---|---|
| `<a>` | `epub:type` + `role` + `id` + `href` | `class="duokan-footnote"` | 多看识别为弹注触发 |
| `<aside>` | `epub:type` + `role` + `id` | `class="duokan-footnote-item"` | 多看识别为注释内容（替代独立 `<ol>`） |

CSS 沿用 §6.2 不改（多看在自有阅读器内强制套用自己的弹层样式，作者 CSS 多被忽略）。

**各平台兼容情况**：

| 平台 | 触发依据 | 结果 |
|---|---|---|
| 多看 5.x+（当前主流） | `<a class="duokan-footnote">` + `href` 指向带 id 的元素 | 弹出 aside 内容 |
| 多看 4.x 及更早 | 需独立私有注释列表结构 | 回退跳转（结构限制，不覆盖） |
| 微信读书 | 仅识别 EPUB 3 标准 | §6.1 即支持 |
| 京东读书 / 网易蜗牛 | 识别 EPUB 3 标准 | §6.1 即支持 |

**接受的妥协**：要让多看 4.x 完全弹出，必须额外写一份独立私有结构——这会破坏"单一结构"原则，本手册不覆盖。在该老版本上，§6.6 会退化为页面跳转，**仍可读，只是不弹**。

---

## 二、Ruby 注音（行间注）

### 2.1 HTML 模板

```xml
<!-- 按字注（一字一拼音）：建议带 <rb> 与 <rp> 后备 -->
<p>
  <ruby><rb>明</rb><rp>(</rp><rt>míng</rt><rp>)</rp></ruby>
  <ruby><rb>月</rb><rp>(</rp><rt>yuè</rt><rp>)</rp></ruby>
  几时有？
</p>

<!-- 按词注（多字注一词，整词读音不可拆） -->
<p>
  <ruby><rb>葡萄</rb><rp>(</rp><rt>pútáo</rt><rp>)</rp></ruby>
  美酒夜光杯。
</p>

<!-- 外文注（人名、术语） -->
<p>
  <ruby><rb>夏洛克·福尔摩斯</rb><rp>(</rp><rt>Sherlock Holmes</rt><rp>)</rp></ruby>
</p>
```

**规则**：

- `<rb>` 是基字（"ruby base"），EPUB 3 推荐显式使用；不写也合法但混排时阅读器对齐不一致
- `<rt>` 是注音；可放在 `<rb>` 后或包多字
- `<rp>` 是括号后备：阅读器不支持 ruby 时，读者看到的就是 `明(míng)`；支持的阅读器自动隐藏
- **按字 vs 按词**：人名、外文、连读词用按词；普通汉字用按字

### 2.2 CSS 模块

```css
ruby {
  ruby-position: over;          /* 横排时注音在上方；竖排自动改右侧 */
  ruby-align: center;           /* 注音与基字居中对齐 */
}

rt {
  font-size: 0.5em;             /* 注音字号约为基字一半 */
  line-height: 1;               /* 防止行高被撑大 */
  font-family:
    "Songti SC", "Source Han Serif SC",
    "Times New Roman",
    serif;
  font-weight: normal;
}

rp {
  display: none;                /* 支持 ruby 的阅读器隐藏括号；写不写都行，浏览器默认就这样 */
}

/* 带 ruby 的段落行距调宽，避免上下行注音相碰 */
p:has(ruby) { line-height: 2; }   /* 渐进增强：:has() 是 2022+ 选择器 */
.has-ruby   { line-height: 2; }   /* 兜底：老 KF8 / 老 KOReader 用类名 */
```

> `:has()` 是 CSS Selectors 4，2022 后的 WebKit / Chromium 才支持。Apple Books 17+、Thorium 3.x、Calibre 7.x 没问题；老固件 Kindle KF8、KOReader 2023- 不识别——所以同时给 `.has-ruby` 类名做兜底，HTML 里给含 ruby 的段落手动加 `class="has-ruby"`。

### 2.3 阅读器兼容

| 阅读器 | `<ruby>` 渲染 | `ruby-position` | 竖排注音位置 |
|---|---|---|---|
| Apple Books | ✅ | ✅ | 自动右侧 |
| Thorium | ✅ | ✅ | 自动右侧 |
| Calibre | ✅ | ✅ | 自动右侧 |
| Kindle KFX | ✅ | ⚠️ 默认 over 即可 | 仅 KFX 竖排版 |
| KOReader | ✅ | ✅ | 自动右侧 |
| 老 KF8 | ⚠️ 部分固件不支持 | ❌ | — |

**rp 后备的实际价值**：除老 KF8 外，2025 主流阅读器全部支持 `<ruby>`，rp 主要为极少数纯文本导出工具与字母盲读器服务。**建议保留**，写起来不费事，向后兼容。

### 2.4 常见坑

| 现象 | 原因 |
|---|---|
| 注音字过大 / 撑高行距 | 没给 `rt { font-size: 0.5em; line-height: 1 }` |
| 注音中英文混杂样式不齐 | `rt` 单独设字体族；外文音用拉丁字体 |
| Kindle 上注音粘在一起 | 段落 `line-height` 至少 1.8–2 |
| 竖排时注音错位 | 仅 Apple Books / Thorium 完整支持竖排 + ruby；KFX 部分支持 |

---

## 三、对主手册的补丁建议（本主题相关）

| 主手册位置 | 改动 |
|---|---|
| §六 基础样式表 / 脚注 | 追加 §一.2 中的 `role` 兜底与 `sup a` 样式 |
| §七 常见组件 / 脚注 | 替换为 §一.1 的 HTML 模板（加 `role`、加 `<section epub:type="footnotes">`） |
| §七 常见组件 / 注音 | 替换为 §二.1 完整模板（带 `<rb>` `<rp>`） |
| §九 避坑清单 / 文件与结构 | 补一行"全书脚注集中到一个文件 → 改用每章自带 `<section epub:type=footnotes>`" |
| §十一 验证测试清单 | 补"点击每条脚注，确认 Apple Books / Kindle Previewer 3 / Thorium 都弹出" |

---

## 四、阅读器兼容矩阵（节选）

| 特性 | Apple Books 4.x | Thorium 3.x | Calibre 7.x | Kindle KFX | KOReader 2024+ |
|---|---|---|---|---|---|
| `epub:type="noteref"` 弹注 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `role="doc-noteref"` 弹注 | ✅ | ✅ | ⚠️ | ⚠️ | ✅ |
| `<aside>` 跨文件弹注 | ❌ 退化跳转 | ✅ | ✅ | ✅ | ✅ |
| KF8 双向锚点弹注（老 Kindle） | N/A | N/A | N/A | ✅ | N/A |
| `<ruby>` + `<rb>` + `<rp>` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ruby-position` 控制 | ✅ | ✅ | ✅ | ⚠️ 默认 over | ✅ |
| 竖排 + ruby 自动右侧 | ✅ | ✅ | ⚠️ | ⚠️ | ⚠️ |

> ✅ 正常 ⚠️ 部分版本 / 部分场景 ❌ 不支持

---

**文档版本**：2026-05-15
**对应标准**：EPUB 3.3 + ARIA DPUB
**关联文档**：`EPUB3_制作完全参考手册.md`（主手册）、《EPub指南——从入门到放弃》（赤霓，2023-04-18）§10.6
