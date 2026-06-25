# 阅读器矩阵

不同阅读器对 EPUB 3 和 CSS 的支持差异很大。本仓把兼容性实测写进 [reader-matrix.yaml](../final/reader-matrix.yaml)，不要只凭直觉改规则。

> matrix 现状以 `warn`（待复测 / provisional）为主，`pass` 较少。`warn` 不代表坏，而是「尚未在该 reader + 版本上确认」。把它当「已知风险地图」，不要当「全绿放行清单」。

## 常用基线

| 阅读器 | 用途 | 优点 | 风险 |
| --- | --- | --- | --- |
| Apple Books | macOS / iOS 基线 | CSS 支持完整，容易安装 | 缓存强，重新导入前要删旧书 |
| Kindle Previewer 3 | Kindle / KDP 发行 | 官方转换器，能看 KFX 风险 | CSS 子集更保守 |
| Thorium Reader | Readium 系桌面重排对照 | 桌面跨平台，适合看 EPUB2 / EPUB3 重排 | 与真实移动 reader 仍不同 |
| Readest | 新兴跨平台对照 | 中文 EPUB 体验友好 | 仍需版本记录 |
| KOReader | 电子墨水设备保守降级对照 | 支持 EPUB，允许覆盖字体与样式 | 自定义引擎与设备环境会放大 CSS 差异 |

Readium 是阅读系统工具链与生态，不是单一终端 App。Thorium 基于 Readium Desktop toolkit；“Thorium 实测通过”不能自动外推为所有 Readium 下游 App 都通过。结构、EPUB2 / EPUB3 和渐进兼容策略见 [08-epub2-epub3-compatibility.md](08-epub2-epub3-compatibility.md)。

## 我该测哪个阅读器？

### 场景 A：你只能选一个

选 Apple Books。它安装门槛低，CSS 支持完整；Apple Books 都打不开时，EPUB 自身大概率有问题。

### 场景 B：目标是 Kindle 商业发行

必测 Kindle Previewer 3。至少测三个 profile：默认电子书阅读器、Paperwhite、Scribe；至少测三档字号：1、4、7。

### 场景 C：目标是中文读者

加测多看阅读和 Readest。多看对中文排版细节更敏感；Readest 适合作为跨平台重排对照。

### 场景 D：你想做兼容性矩阵

按这个顺序测：

1. Apple Books
2. Kindle Previewer
3. Thorium Reader
4. Readest
5. KOReader
6. 多看

这个顺序也是本仓 `reader-matrix.yaml` 收录的优先级。

## 记录方式

每条实测必须写清楚：

- reader id 与版本；
- case id；
- artifact 路径；
- status：`pass | warn | fail | na`；
- 现象、处理动作、临时 workaround。

未测过不要写 `pass`，用 `warn` + `pending-<reader>-version`。
