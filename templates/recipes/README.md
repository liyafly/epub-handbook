# 组合效果配方

先保证普通正文可读，再选效果。`epub book recipes` 列出配方，`epub book recipes <名称> --add <书目录>` 追加到书的 `design.css`。每个目录有 `recipe.md`、可复制的 `recipe.css` 和最小 `sample.xhtml`；复制前检查类名、图片比例、CSS 加载顺序与本书级联。starter 的 `base.css` 保留基础规则，选中的片段写进 `design.css`，同步制作说明的“使用的配方”。字体仍单独放 `fonts.css`，标准弹注放 `notes.css`。

| 配方 | 效果与降级 | 证据状态 | 证据入口 |
| --- | --- | --- | --- |
| [shape-wrap](shape-wrap/recipe.md) | polygon 环绕 → 矩形 → 窄屏块图 | Readest 桌面及 Apple Books 首个展开页通过；窄分页栏未触发护栏；Kindle 矩形回退 | [历史截图与 SHA](../shape-wrap-demo/reader-evidence/README.md)；[本次矩阵记录](../../docs/final/reader-matrix.yaml) |
| [drop-cap](drop-cap/recipe.md) | 完整单词的首字装饰 → 普通正文 | Readest 18px / Kindle 8 通过；Apple Books 空白现象待定位 | [demo 18](../epub-style-demo/OEBPS/Text/18-english-fiction.xhtml) |
| [text-emphasis](text-emphasis/recipe.md) | 原生着重号 → 下划线语义强调 | Readest 横排 18/26px 通过；Kindle 双重提示、Apple Books 灰色阅读区 warn；竖排待测 | [demo 24 / 25](../epub-style-demo/OEBPS/Text/24-text-emphasis.xhtml) |
| [theme-adaptive-art](theme-adaptive-art/recipe.md) | 资源内白底保守图；单色透明图可选夜间反相 | Apple Books 深色旧包失败，增加资源内白底后通过；Readest 反相未触发、Kindle 夜间未测，均 warn | [修复前后证据](reader-evidence/observations-20261030-backing.json) |
| [parallel-pair](parallel-pair/recipe.md) | 短组宽屏双栏 → 按源序上下 | Readest 26px 与 Kindle 4/8 通过；Apple Books 灰色阅读区 warn；其他配置待测 | [demo 21](../epub-style-demo/OEBPS/Text/21-classical-modern.xhtml) |
| [page-image](page-image/recipe.md) | Kindle 版装饰页换整页图 → 其他阅读器用文字版 | 合成样本 Kindle Previewer 转换通过，目测待测；一本书书级通过 | [阅读器矩阵 untested_cases](../../docs/final/reader-matrix.yaml) |
| [vertical-title-art](vertical-title-art/recipe.md) | 左侧竖排章题 + 右侧章首图 → 横排章题在图上方 | 仅 Chrome 预检，阅读器待测 | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [image-preview](image-preview/recipe.md) | 多看 / Reeden 点图预览带题注 → 普通整宽图 + 图下题注；可选 spine 整屏图页 | 仅 Chrome 预检，阅读器待测 | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [letter-paper](letter-paper/recipe.md) | 无边框纸面、顶格/空两格/右对齐、固定书信字体；纯 CSS 横线逐行跟随文字；Kindle 经 `amzn-kf8` 省略，多看变体省略 | 通用包与多看变体用户确认，未记录版本；规则见 SPEC §5.9.1 | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [tate-chu-yoko](tate-chu-yoko/recipe.md) | 竖排里两三位数字横排成一格 → 数字侧躺 | 阅读器待测；规则见 SPEC §5.12（推导，待验证） | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [opentype-features](opentype-features/recipe.md) | 嵌入字体的旧式数字、小型大写、连字 → 普通字形 | 阅读器待测；公共包不带字体，效果只能书级验证；规则见 SPEC §8（推导，待验证） | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [cjk-punctuation](cjk-punctuation/recipe.md) | 标点挤压、行尾悬挂、中西文间距 → 完全不变 | 阅读器待测 | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |
| [english-hyphenation](english-hyphenation/recipe.md) | 英文断词与孤行控制 → 不断词 | 阅读器待测 | [reader-matrix untested_cases](../../docs/final/reader-matrix.yaml) |

“有历史记录”不代表新片段、新书或新 SHA 通过。尚未在至少两本书完成验证的做法只作为候选，不晋升为通用兼容规则；书级证据保留在书级 Git，公共目录只用合成材料。新 CSS 均需普通/大字号、窄屏与昼夜主题验证；静态 gate 与真实阅读器结果分别记录。

当前片段在 [reader-matrix](../../docs/final/reader-matrix.yaml) 的 `recipe-*` case 登记；尚未实测的片段留在 `untested_cases`。表中的历史观察只属于原 case；本次 shape-wrap 通过也只适用于记录的 SHA 与当前 Readest 桌面设置。

首批五项不包办所有书型。普通居中图与图注由 starter 基础层承载；矩形浮动是 shape-wrap 的 L1，不能对带图注的 figure 直接套轮廓。封面、章首饰图、全页图、诗歌、小型大写和标准弹注分别参考 [主 demo 场景索引](../epub-style-demo/SCENE_MATRIX.md)、[gallery](../epub-style-demo/GALLERY.md) 与 [专项技能](../../skills/README.md)。A-lite、竖排和公式继续走原有独立场景，不混进普通图文配方。

```sh
python3 templates/recipes/build.py
```

仅依赖 Python 标准库，独立打包样本到本目录 `dist/`，并生成保留同一正文、图片与元数据、CSS 为空的 `*-baseline.epub` 供全项 redline。`inventory.json` 记录 SHA；带 `reader.css` 的配方另打 `-kindle.epub`，带 `reader-duokan.css` 的另打 `-duokan.epub`（通用包里 `reader.css` 为空）。每个产物都要跑 nav audit、全项 redline；主 demo 的 maintain 资源完整性规则不适用于这些最小包，缺项应单列，不能记通过。浏览器打开 `sample.xhtml` 只作预检。
