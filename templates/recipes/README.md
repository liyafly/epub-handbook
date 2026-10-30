# 组合效果配方

先保证普通正文可读，再选效果。每个目录有 `recipe.md`、可复制的 `recipe.css` 和最小 `sample.xhtml`；复制前检查类名、图片比例、CSS 加载顺序与本书级联。starter 的 `base.css` 保留基础规则，选中的片段写进 `design.css`，同步制作说明的“使用的配方”。字体仍单独放 `fonts.css`，标准弹注放 `notes.css`。

| 配方 | 效果与降级 | 证据状态 | 证据入口 |
| --- | --- | --- | --- |
| [shape-wrap](shape-wrap/recipe.md) | polygon 环绕 → 矩形 → 窄屏块图 | 历史 v3 三款桌面阅读器有记录；新增护栏待测 | [历史截图与 SHA](../shape-wrap-demo/reader-evidence/README.md) |
| [drop-cap](drop-cap/recipe.md) | 完整单词的首字装饰 → 普通正文 | 三款阅读器历史记录均为 warn；本片段待测 | [demo 18](../epub-style-demo/OEBPS/Text/18-english-fiction.xhtml) |
| [text-emphasis](text-emphasis/recipe.md) | 原生着重号 → 下划线语义强调 | 候选；横排、竖排均待补实测 | [demo 24 / 25](../epub-style-demo/OEBPS/Text/24-text-emphasis.xhtml) |
| [theme-adaptive-art](theme-adaptive-art/recipe.md) | 单色透明插图可选夜间反相 → 白底局部衬片 | 候选；早期书级方案验收失败，当前片段待复测 | [复测说明](theme-adaptive-art/recipe.md#阅读器结果) |
| [parallel-pair](parallel-pair/recipe.md) | 短组宽屏双栏 → 按源序上下 | 继承 SPEC 的保守路径；Kindle 大字号仍待复测 | [demo 21](../epub-style-demo/OEBPS/Text/21-classical-modern.xhtml) |

“有历史记录”不代表新片段、新书或新 SHA 通过。尚未在至少两本书完成验证的做法只作为候选，不晋升为通用兼容规则；书级证据保留在书级 Git，公共目录只用合成材料。新 CSS 均需普通/大字号、窄屏与昼夜主题验证；静态 gate 与真实阅读器结果分别记录。

五个当前片段在 [reader-matrix](../../docs/final/reader-matrix.yaml) 的 `recipe-*` case 与 `untested_cases` 登记；表中的历史观察只属于原 case。

首批五项不包办所有书型。普通居中图与图注由 starter 基础层承载；矩形浮动是 shape-wrap 的 L1，不能对带图注的 figure 直接套轮廓。封面、章首饰图、全页图、诗歌、小型大写和标准弹注分别参考 [主 demo 场景索引](../epub-style-demo/SCENE_MATRIX.md)、[gallery](../epub-style-demo/GALLERY.md) 与 [专项技能](../../skills/README.md)。A-lite、竖排和公式继续走原有独立场景，不混进普通图文配方。

```sh
python3 templates/recipes/build.py
```

仅依赖 Python 标准库，独立打包五个样本到本目录 `dist/`，并生成保留同一正文、图片与元数据、CSS 为空的 `*-baseline.epub` 供全项 redline。`inventory.json` 记录 SHA。每个产物都要跑 nav audit、全项 redline；主 demo 的 maintain 资源完整性规则不适用于这五个最小包，缺项应单列，不能记通过。浏览器打开 `sample.xhtml` 只作预检。
