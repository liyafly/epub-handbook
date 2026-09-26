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
# 1. 一条命令创建书级 Git 工作区和最小 EPUB 骨架
sh templates/book-starter/new-book.sh work-epub/my-book

# 2. 改书名/作者，写正文
#    - 编辑 work-epub/my-book/03 制作工作区/epub/OEBPS/package.opf
#    - 编辑 work-epub/my-book/03 制作工作区/epub/OEBPS/Text/01-chapter.xhtml

# 3. 构建 + 体检，成功后覆盖唯一的 dist/book.epub
sh 'work-epub/my-book/03 制作工作区/epub/build.sh'
```

解包源和完整字体母版由书级 Git 维护；临时 EPUB 放进 `.pipeline/`，失败时保留上一份通过检查的
产物。字体 provider 安装方法、已有 EPUB 接入和 Git 约定见[一书一 Git 工作区](../pipeline/book-workspace.md)。
体检通过后，拖进 Apple Books 或 Kindle Previewer 看效果，并把实测 reader 版本和产物 SHA 记入 `制作说明.md`。
详细步骤见[做一本书](做一本书.md)。

> 想从零一行行理解每个文件怎么来的，再看 [手写 XML 的原理路径](做一本书.md#附录想理解每个文件怎么来的)。

### C. 我有一本别人做的 EPUB，想修 / 清洗它

首次接入仍要冻结底本、识别风险并审查候选，但无需为每一步保存一份 EPUB。先做只读预览：

```sh
epub clean /path/to/别人的.epub --out /path/to/clean-preview --json
```

依据预览选实际需要的步骤，再单独审查变换计划；迁移、CSS 清理等只用一条明确步骤链：

```sh
epub clean /path/to/别人的.epub --out /path/to/clean-plan \
  --steps normalize,migrate,css --json
```

计划确认后，使用相同范围加 `--approve` 生成唯一最终候选。正文排版还需明确 preset 和范围；不要为了“走完整流程”重复迁移或运行无关能力。候选通过 redline 和人工 diff review 后，按[一书一 Git 工作区](../pipeline/book-workspace.md)接入解包源；后续普通修改只需编辑源文件并运行书内 `build.sh`。

临时计划和中间报告放在 `03 制作工作区/.pipeline/`，最终候选另行审核；不保留按步骤生成的 EPUB 堆。完整参数和红线见[清洗流水线](../pipeline/cleanup-flow.md)。

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
| **清洗后正文文字被改了** | 触发红线，这是事故 | 回滚并重跑 `epub redline --check all`（[FAQ](07-faq.md#ai-协作)） |

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
