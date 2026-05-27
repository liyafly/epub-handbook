# 阅读器矩阵

不同阅读器对 EPUB 3 和 CSS 的支持差异很大。本仓把兼容性实测写进 [reader-matrix.yaml](../final/reader-matrix.yaml)，不要只凭直觉改规则。

## 四个常用基线

| 阅读器 | 用途 | 优点 | 风险 |
| --- | --- | --- | --- |
| Apple Books | macOS / iOS 基线 | CSS 支持完整，容易安装 | 缓存强，重新导入前要删旧书 |
| Kindle Previewer 3 | Kindle / KDP 发行 | 官方转换器，能看 KFX 风险 | CSS 子集更保守 |
| Thorium Reader | EPUB 3 规范对照 | 桌面跨平台，规范实现严 | 与真实移动 reader 仍不同 |
| Readest | 新兴跨平台对照 | 中文 EPUB 体验友好 | 仍需版本记录 |

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
5. 多看

这个顺序也是本仓 `reader-matrix.yaml` 收录的优先级。

## 记录方式

每条实测必须写清楚：

- reader id 与版本；
- case id；
- artifact 路径；
- status：`pass | warn | fail | na`；
- 现象、处理动作、临时 workaround。

未测过不要写 `pass`，用 `warn` + `pending-<reader>-version`。
