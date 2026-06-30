# Kindle 字体渲染深度参考

> 定位：解释 Kindle 上 `font-family` 回退为什么会偏离 CSS 规范、生僻字为何变方块，以及应对策略。本文是 SPEC §8 字体链规则的**底层机理补充**——规则见 SPEC，原理见本文。

## 1. 核心问题：Kindle 的字体回退不沿你的链走

### 规范行为 vs Kindle 实际行为

CSS `font-family` 是一个有序列表（字体链）：

```css
body { font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif; }
```

符合规范的引擎（WebKit/Blink）做**逐字形回退**：对**每一个字符**，从链首往后找第一个能提供该字形的字体。一个字由链首渲染，另一个字由链尾渲染——这是正常且预期的。

**Kindle 偏离了这个行为**：当链首字体缺某字形时，Kindle 不沿链继续往后试你的内嵌字体，而是直接跳到 Kindle 自带的系统字体。系统字体不含扩展 B+ 汉字或 PUA 自造字 → 方块（□）。

> 诚实标注：亚马逊未公开 KFX 渲染器内部实现。以上结论来自可观察行为 + 社区实测（越狱用户替换系统字体路径后生僻字即正常显示——反证 Kindle 在做回退，只是回退目标被钉死在系统字体上）。

### 各阅读器引擎差异

| 阅读器 | 引擎 | 逐字形回退 | 备注 |
| --- | --- | --- | --- |
| Apple Books | WebKit | ✅ 接近浏览器 | 受 `ibooks:specified-fonts` 影响 |
| Thorium / Readest | Chromium | ✅ 接近浏览器 | |
| KOReader | crengine | ✅ 引擎自有逻辑 | 与浏览器有出入 |
| Kindle | 亚马逊自有 | ❌ **不可靠** | 格式/代际差异大（见下） |
| 多看 | 私有 | ⚠️ 部分支持 | 旧版仅识别 `duokan-footnote` 等私有类 |

### Kindle 格式分层（行为随格式变化）

- **MOBI7（老）**：CSS 支持很弱，基本不支持嵌入字体。已基本退场。
- **KF8 / AZW3**：支持较完整 CSS 子集与嵌入字体；需手动选"出版方字体"。
- **KFX（Enhanced Typesetting）**：较新格式，排版与字体处理是另一套行为；Enhanced Typesetting 开/关影响嵌入字体支持。

此外，投递/转换链（Send to Kindle、KDP 上传转换）会**重写或丢弃部分 CSS**，使源 EPUB 的链与 Kindle 实际拿到的链不一致。

---

## 2. 三种应对策略

核心思路：**别让 Kindle 去做它做不好的「该往哪回退」，把这个选择动作消灭掉。**

| 策略 | 做法 | 抗 Kindle 坏回退 | 体积代价 | 适用场景 |
| --- | --- | --- | --- | --- |
| **A. 单一大字库放链首** | 嵌一个覆盖全书的大字库，放每条链第 1 位 | **最强**：链首啥都有 → 永不触发回退 | 高（需子集化） | 全书生僻字多、需设计统一 |
| **B. span 包裹 + 专用字体** | 生僻字用 `<span class="rare">` 包起来，CSS 显式指定那一款字体 | 强：被包住的字直接命中专用字体 | 低（只嵌小子集） | 生僻字「少但散」 |
| **C. 不嵌入，走系统字体** | 不嵌字体，靠阅读器系统字体 + generic 兜底 | 弱：完全依赖阅读器 | 无 | 全常用字、可接受阅读器差异 |

### 策略 A 详解（对应 SPEC §8 模式 C1-body）

把实际用到的字形合并进同一款字体，正文只写这一款：

```css
body.body-font-locked {
  font-family: "st-all", "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}
```

只有一款字、字字俱全，Kindle 无可回退，自然不掉字。需配合 `fontspec=forceAll` + OPF `ibooks:specified-fonts=true`。

### 策略 B 详解（对应 SPEC §8 模式 B，`.rare` 类）

```html
正文正文<span class="rare">𠀀</span>正文正文
```
```css
.rare { font-family: "tszt-rare", serif; }  /* 只写一款，别再列链 */
```

**关键**：span 的 `font-family` 只写一款字体。写成链就把回退又塞回去了。

> `@font-face` + `unicode-range` 按码位段分配字体能连 span 都省，但 **Kindle 对 `unicode-range` 支持不稳**——在 Kindle 上更信显式 span，真要用务必先真机验过。

### 策略选择决策树

```
生僻字多吗？
├── 多（几十个以上，遍布全书）→ 策略 A（单一大字库）
├── 少（零星几个）            → 策略 B（span 包裹）
└── 没有                      → 策略 C（默认，不嵌入）
```

---

## 3. 生僻字处理全流程

### 3.1 先确认：是「缺字形」还是「回退没生效」

同一个 EPUB 在 Apple Books / Thorium 正常、Kindle 上方块 → 是回退没生效（字形在链里但 Kindle 没退到）。所有阅读器都方块 → 是真缺字（链里没一个有这个字形）。

### 3.2 查码位 → 选字库

**九成情况是「你这几款字体没覆盖」，而非「全世界都没这个字」。** Unicode CJK 扩展区这些年持续扩充：

| 扩展区 | 码位段 | 收录年份 |
| --- | --- | --- |
| Ext B | `U+20000`–`U+2A6DF` | 2001 |
| Ext C | `U+2A700`–`U+2B73F` | 2009 |
| Ext D | `U+2B740`–`U+2B81F` | 2010 |
| Ext E | `U+2B820`–`U+2CEAF` | 2015 |
| Ext F | `U+2CEB0`–`U+2EBEF` | 2017 |
| Ext G | `U+30000`–`U+3134F` | 2017 |
| Ext H | `U+31350`–`U+323AF` | 2022 |
| Ext I | `U+2EBF0`–`U+2EE5F` | 2024 |

查码位工具：汉典(zdic.net)、Unihan、GlyphWiki、字统网。

### 3.3 推荐大字库（古籍生僻字覆盖）

| 字体 | 覆盖范围 | 授权 |
| --- | --- | --- |
| **思源宋体** Source Han Serif | 常用字 + Ext A、部分 Ext B | OFL |
| **花园明朝** HanaMin A+B | ~10 万字（URO+Ext.A–E），几乎收全 CJK | 开源（GlyphWiki） |
| **BabelStone Han** | 六万余汉字，含古文字、甲骨金文学术转写 | OFL |
| **Plangothic** | 扩展区 G/H/I 补充较全，黑体，大陆字形 | 开源（GitHub） |
| **全字库 CNS**（正宋体/正楷体） | CNS 11643 全平面，含大量异体字 | 开源、免费商用 |

> 即便这些大字库，对个别极冷僻字仍可能都没有字形——那才轮到真正造字。

### 3.4 造字（无码位时的最后手段）

1. **组字**：GlyphWiki / 字统网拼现成部件（古籍缺字多是部件组合），可直接导出矢量。
2. **描摹**：从古籍扫描用 potrace / Inkscape 描摹成矢量轮廓。
3. **从零画**：FontForge / Glyphs，最费工。

码位选择：有真码位用真码位（可复制可检索）；真无码用 PUA（`U+E000`–`U+F8FF`），但 PUA 字的复制/检索/查词典会失效。**务必留 PUA↔字映射表**，将来正式编码后可一键替换。

### 3.5 Kindle 兼容要点

- 内嵌字体用 TrueType（glyf）轮廓，CFF/PostScript 轮廓的 OTF 会触发 `W14029` 警告。
- KF8/AZW3 才支持自定义字体，确保不被降级成 MOBI7。
- 设备/预览器里必须手动选「出版方字体（Use Publisher Font）」。
- **Kindle Previewer 结果 ≠ 真机 ≠ 云端推送后**——最终一定在真机上确认。

---

## 4. 参考工具

- **查码位/字形**：汉典、Unihan、GlyphWiki、字统网
- **组字/造字**：GlyphWiki、FontForge
- **覆盖检测/合并/subset**：fontTools（Python 脚本化）
- **扫描描摹**：Inkscape + potrace

外部参考：
- Kindle 古籍生僻字与系统字体回退：`sspai.com/post/109625`
- Kindle 多字体嵌入与"出版方字体"：`bookfere.com/post/324.html`
- CSS `font-family` 逐字回退规范：MDN
- 中文开源字体集 OSFCC：`drxie.github.io/OSFCC/`
