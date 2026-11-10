# 固定版式 demo

整本固定版式（`rendition:layout=pre-paginated`）的最小样本，对应 [SPEC §5.13](../../docs/final/SPEC-实现约束.md#513-固定版式整本-pre-paginated)。五个页面文件，每页 768 × 1024 CSS 像素：

| 文件 | 场景 | spine 属性 |
| --- | --- | --- |
| `Text/cover.xhtml` | 封面：底图 + 真文字书名 | `rendition:page-spread-center` |
| `Text/p01-spread-left.xhtml`、`p02-spread-right.xhtml` | 跨页对开：一张图分成左右两半 | `page-spread-left` / `page-spread-right` |
| `Text/p03-overlay.xhtml` | 图上压字 | `page-spread-left` |
| `Text/p04-text.xhtml` | 纯文字页 | `page-spread-right` |

文字都是真文字，用 `position: absolute` 和 px 定位。图是 [make-art.py](make-art.py) 生成的纯色渐变 PNG，文字是自写的，没有第三方素材。不嵌入字体，英文用 `Georgia, serif`（SPEC §8 允许英文系统字体优先），所以各设备上的字形和行宽会略有差别。

## 打包

```sh
sh templates/fxl-demo/build.sh
```

输出 `templates/fxl-demo/dist/fxl-demo.epub`，同一份源码重复打包 SHA 相同。改图后先跑 `python3 templates/fxl-demo/make-art.py`。

## 验证

- `epub run epub.package.nav.audit --input templates/fxl-demo/dist/fxl-demo.epub --json`：没有 error。CI 另跑 EPUBCheck。
- 阅读器：Apple Books、Kindle、Reeden 都待测，记在 `reader-matrix.yaml` 的 `fxl-demo`。要看对开是否成对、封面是否单页居中、横竖屏切换、文字能否选中和搜索。

`epub book new --fixed` 暂不做，等这个 demo 在三个阅读器实测过再定。
