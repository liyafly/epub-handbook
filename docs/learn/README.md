# 新手必读：3 分钟看懂，带着问题直接查

这一页写给第一次做电子书、还不懂术语的人。

**两种用法，选一种：**

- **只想弄懂 EPUB 是什么** → 读完 [§1](#1-一句话什么是-epub) 就够了。
- **手上已经有具体问题**（注释打不开、字体变方块、目录没了……）→ 直接跳到 [§3 带着问题直接查](#3-带着问题直接查)，不用从头读。

> 你不需要先背规范，也不需要先学 CSS。会在命令行里复制粘贴命令就行。遇到不懂的词，随时查 [术语表](glossary.md)。

---

## 1. 一句话：什么是 EPUB

**EPUB 就是一个“会自动换行的网站”，打包成了一个文件。** 里面是 HTML、CSS、图片和目录，压成一个 zip 包，后缀改成 `.epub`。

阅读器打开它时，会根据你的屏幕大小和字号设置，把文字**重新排版**。

这也是它和 PDF 最大的区别：

|  | PDF | EPUB |
| --- | --- | --- |
| 像什么 | 印好的纸，字号版式固定 | 会自动换行的网页 |
| 放大字号 | 整页放大，要左右拖 | 文字重新排，仍然一屏读完 |

因为是“重新排版”，同一本书在不同阅读器（Apple Books、Kindle、多看……）里可能长得不一样——**这就是本仓库要帮你解决的核心问题：做一本在常见阅读器里都不容易崩的书。**

---

## 2. 你想做什么？选一条路

### A. 我只想读懂 EPUB 是什么

本页读完就够了。想再深入结构原理，看 [§5](#5-想更深入再看什么进阶不是必读)。

### B. 我想做一本自己的书（最快路径）

**不要**从手写 XML 开始。直接复制现成骨架，三步出书：

```sh
# 1. 复制骨架（已经是一本结构合规的最小书）
cp -r templates/book-starter ~/my-book && cd ~/my-book

# 2. 改书名/作者，写正文
#    - 编辑 OEBPS/package.opf 里的 dc:title / dc:creator
#    - 编辑 OEBPS/Text/01-chapter.xhtml 写你的正文

# 3. 构建 + 体检
sh build.sh
python3 <仓库路径>/scripts/epub_lint.py dist/*.epub   # error 清零就算过关
```

体检通过后，拖进 Apple Books 或 Kindle Previewer 看效果。
详细步骤见 [做一本书](做一本书.md)。

> 想从零一行行理解每个文件怎么来的，再看 [手写 XML 的原理路径](做一本书.md#附录想理解每个文件怎么来的)。

### C. 我有一本别人做的 EPUB，想修 / 清洗它

一条命令跑清洗流水线，它会**保留原件**、检查风险、生成清洗后的版本：

```sh
python3 scripts/epub_cleanup_pipeline.py /path/to/别人的.epub \
  --work-dir 'work-epub/book-a/03 制作工作区/.pipeline'
```

它会在该书 `03 制作工作区/.pipeline/` 下留下改前备份、清洗结果和一份报告。一本书的完整目录见 [一书一 Git 工作区](../pipeline/book-workspace.md)。
完整流程和红线（哪些内容绝对不许改）见 [清洗流水线](../pipeline/cleanup-flow.md)。

---

## 3. 带着问题直接查

遇到下面这些常见症状，直接点对应链接，不用顺着读全套教程。

| 我遇到的问题 | 大概率原因 | 去哪修 |
| --- | --- | --- |
| **注释 / 弹注点不开** | 缺 `xmlns:epub` 声明，或注释目标和链接不在同一文件 | [常见问题 · 弹注](07-faq.md#epub2--epub3)、[多看弹注 fallback](../how-to/duokan-footnote-fallback-fix.md) |
| **字体变方块 / 生僻字缺字** | 嵌入字体没覆盖到这些字，Kindle 回退失败 | [Kindle 字体渲染深度参考](../how-to/kindle-font-rendering-deep-dive.md) |
| **字体没生效** | 字体文件、OPF 声明、CSS `@font-face` 三处没对齐 | [常见问题 · 阅读器](07-faq.md#阅读器) |
| **目录失效 / 打开没目录** | OPF 漏标 `properties="nav"`，或 NCX 的 `dtb:uid` 和书的 id 不一致 | [结构与兼容 · 导航双轨](进阶-结构与兼容.md#4-导航双轨navxhtml-与-tocncx-的分工) |
| **图片溢出屏幕右边** | 外层容器 padding 把 `width:100%` 的图撑出去了 | [章首图与图文混排](../how-to/chapter-head-image.md) |
| **字号调大后版面挤坏** | 排版用了固定 `px` / `vh`，没用可缩放的 `em` / `%` | [SPEC 实现约束](../final/SPEC-实现约束.md) |
| **Kindle Previewer 转换失败** | EPUB 本身结构有问题，或 Kindle 不支持某些写法 | [常见问题 · 阅读器](07-faq.md#阅读器)、[reader-matrix](../final/reader-matrix.yaml) |
| **Apple Books 改了不刷新** | Apple Books 会缓存 | 先在 Books 里删掉旧版本，再重新拖入（[FAQ](07-faq.md#阅读器)） |
| **清洗后正文文字被改了** | 触发红线，这是事故 | 回滚并重跑 `scripts/validate_text_invariance.py`（[FAQ](07-faq.md#ai-协作)） |

> 你的问题不在表里？先翻 [完整常见问题](07-faq.md)，再看 [reader-matrix.yaml](../final/reader-matrix.yaml) 是否已有记录。

---

## 4. 五个迟早会碰到的词（先记这几个就够）

真正需要的时候再记，不用背：

- **EPUB** = 一个装着 HTML / CSS / 图片的 zip 包。
- **OPF**（`package.opf`）= 这本书的“项目清单”，登记有哪些文件、书名作者是谁。
- **spine** = 阅读顺序（先读哪章后读哪章），写在 OPF 里。
- **nav.xhtml / NCX** = 目录。前者给新阅读器用，后者给旧阅读器和 Kindle 用，通常两个都留。
- **弹注** = 点一下脚注就地弹出小窗，不用跳走。

完整术语随时查 [术语表](glossary.md)。

---

## 5. 想更深入再看什么（进阶，不是必读）

以下是给“想彻底搞懂”或“要给团队定规范”的人准备的，**小白可以先跳过**：

- **EPUB 内部结构、版本差异与兼容** → [进阶：结构与兼容](进阶-结构与兼容.md)
- **对外硬规则**（违反等于事故）→ [SPEC 实现约束](../final/SPEC-实现约束.md)
- **用 AI 帮忙修书** → [AI skills 怎么用](04-skills.md)
- **完整清洗案例（改前 / 改后全过程）** → [清洗案例](05-case-study.md)
- **选择阅读器和测试范围** → [阅读器矩阵](03-readers.md)
- **测试自己的 EPUB** → [测试自己的 EPUB](06-test-your-own.md)

---

看不懂任何一步，就回到这一页重新选路。做书走 [B](#b-我想做一本自己的书最快路径)，修书走 [C](#c-我有一本别人做的-epub想修--清洗它)，查问题走 [§3](#3-带着问题直接查)。
