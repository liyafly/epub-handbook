# EPUB Style Demo

这是一个最小 EPUB 3 样式样本，用来快速检查项目推荐样式在不同阅读器上的显示。

## 生成

```sh
sh templates/epub-style-demo/build.sh
```

脚本每次成功后覆盖同一个文件：

```text
templates/epub-style-demo/dist/epub-style-demo.epub
```

构建前会在临时目录复制源树、统一文件时间戳并按固定路径顺序打包。同一源提交、同一平台且 `zip -v` 显示相同构建时会得到相同 SHA-256；跨平台或 zip 构建不同不承诺字节级一致。macOS 系统 zip 不会为非 ASCII 条目名设置 UTF-8 标志，EPUB 条目名请使用 ASCII。

## 样本页

1. `00-title.xhtml`：封面式标题页。
2. `01-body.xhtml`：中文正文、引用、着重、图片。
3. `02-ruby-note.xhtml`：Ruby 注音和标准弹出注释结构。
4. `03-vertical-alite.xhtml`：A-lite 整页海报样本（title + subtitle）。
5. `03c-poster-contain.xhtml`：单图卷封 contain + `<img>` fallback 对照样本。
6. `04-lists-tables-code.xhtml`：列表、表格、代码块和键盘文本。
7. `05-legacy-note-fallback.xhtml`：在标准弹注结构上叠加多看旧版 fallback。
8. `07-font-family-order.xhtml`：系统优先 / 书内优先 / 混合链的 font-family 顺序验证；整本 demo 由 `fonts.css` 的直接 `body` 规则演示正文锁定模式。
9. `08-long-mixed-flow.xhtml`：长段落、中英混排、大字号标题与右侧裁切压力测试。
10. `09-kindle-risk.xhtml`：Kindle Previewer 专项风险项。
11. `10-text-effects.xhtml`：着重号、波浪线、首字下沉与 Ruby。
12. `14-vertical-body.xhtml`：整页正文竖排。
13. `15-frontmatter.xhtml`：存量书逐行版权真文本、结构化 `dl` 增强、题献与题记。
14. `16-math.xhtml`：语义 MathML，以及 presentation table 保守编号、Grid 增强、方程组布局和保留 `<thead>`/`<tbody>`/`scope`/`rowspan` 的长公式数据表压力场景；局部相对字号仅为待复测候选。
15. `17-image-layout.xhtml`：figure 图文环绕与 Kindle 大字号回归测试，以及由 figure 控制的单图实例宽度、默认上下且宽屏 Flex 增强为非等宽并排的图片场景。
16. `18-english-fiction.xhtml`：英文小说正文、轻量首字、手写体下沉首字、居中插图与大字号回归测试。
17. `19-border-shadow-notes.xhtml`：边框、阴影、斜角感、SVG 花边实验、长条投影和不规则便签文本框。
18. `20-chapter-head-image.xhtml`：小型章节头图、满栏横幅头图、kicker、真实标题、窄屏保守宽度与单书 fallback。
19. `21-classical-modern.xhtml`：文白 / 原译对照条目结构、局部目录、默认上下、宽屏 38/58、48/48、58/38 preset 和大字号回归测试。
20. `22-chapter-title-bg.xhtml` / `23-chapter-title-inline.xhtml`：`文心` 式 `『忽然做了大人与古人了』` + `一` 章题；5.8em 靠右两列 table-cell 骨架、左上系列行、左下 25% 饰图，分别测试 body background 与 inline absolute，页面仅保留章题组件以观察是否分第二页。
21. `24-text-emphasis.xhtml`：横排着重号对照（标准 `under right`、Kindle/WebKit 单值 `under`、ruby 字面圆点与空 `rt` 生成圆点）及可换行短段。
22. `25-text-emphasis-vertical.xhtml`：body 真竖排着重号对照。
23. `26-prosody-fallback.xhtml`：横排 `△/▽` 语调标记对照；A 使用 `ruby.prosody` 的真实 `rt`，B 使用真实 HTML 双层 `inline-table/table-row/table-cell`，覆盖单字符、多字符连续、标点旁和行末换行附近，且 B 不依赖 generated content。
24. `28-chapter-opening-block.xhtml`：生产书反向提炼的横排块级独立章首页；章次与章名在同一 `h1` 中作两个 block span，标题组以 `25%` 上边距和 `5%` 右安全区靠右；背景色、左下饰图与 `5.5em auto` 尺寸只写在 `poster.css` 的 body modifier，XHTML 无 inline style、无真实饰图 `<img>`，专门观察 Kindle 顶部留白、Readest body 背景绘制与 Reeden 块级标题间距。
25. `29-poetry.xhtml`：自造短诗、多节、长行折行与长诗分页样例；仅对已存在的 `.poetry` / `.stanza` 结构生效。
26. `30-letter.xhtml`：自造书信，含称谓、长段正文、祝语、署名和日期；长信可自然跨页。
27. `31-dialogue.xhtml`：自造短对白与长回答；发言者由真实文本标识，不靠颜色。

退役对照页放在 `retired/`，不进入默认 OPF/nav/toc：`03b-poster-fullbleed.xhtml`、`06-multi-legacy-note-fallback.xhtml`、`11-chapter-opening.xhtml`、`12-literary-fiction.xhtml`、`13-duokan-rich-fallback.xhtml`。这些页面只保留历史对照价值；新增规则优先补活跃页或场景指南。

OPF 还声明 `Images/cover.png` 为 raster 封面图，用于覆盖 Kindle Previewer 的封面识别检查。

> 注：本 demo 的 `fonts.css` 直接给 `body` 绑定字体，`package.opf` 保留书级
> `<meta property="ibooks:specified-fonts">true</meta>`，整书按锁定模式处理。
> 真实书籍应全书统一自由或锁定模式，见 `docs/final/SPEC-实现约束.md` §8。

完整覆盖关系见 [`SCENE_MATRIX.md`](SCENE_MATRIX.md)。按页面角色查源码、定位 class、查看依赖和降级建议，可从 [可复用版式配方](GALLERY.md) 开始。

新增 fixture 不继承任何外部书籍或旧构建产物的 `pass`。构建后仍须在目标阅读器、目标字号和目标字体设置下实测，再回写阅读器矩阵。

### 阅读器测试副本的 identity

Apple Books、Readest 等会按 EPUB identity 复用书库条目。每个「候选 EPUB × 阅读器」组合都先复制为临时测试副本，再只在副本中修改 OPF `unique-identifier` 指向的 `dc:identifier`：用 RFC 4122 URL namespace 的 UUIDv5，对 `urn:sha256:<候选 SHA-256>#reader=<reader id>` 确定性生成一个 `urn:uuid:` 值。按 EPUB 要求重新打包（`mimetype` 必须位于首项且不压缩），记录测试副本 SHA-256；生产候选的 identity 不改。截图或日志应关联阅读器完整版本、profile 和测试副本 SHA。没有可复核截图或日志的结果记为 `warn`，不记 `pass`。

以 Readest 为例，`C` 指未改 identity 的候选 EPUB；每个阅读器使用稳定的 `READER_ID`（如 `readest`、`ibooks_macos`、`kindle_previewer`）：

```sh
C=/path/to/candidate.epub
C_SHA=$(shasum -a 256 "$C" | awk '{print $1}')
READER_ID=readest
READER_UUID=$(python3 -c 'import uuid,sys; print(uuid.uuid5(uuid.NAMESPACE_URL, f"urn:sha256:{sys.argv[1]}#reader={sys.argv[2]}"))' "$C_SHA" "$READER_ID")
epub run epub.metadata.edit --input "$C" --output "$C.readest.epub" --json "metadata_json={\"identifier\":\"urn:uuid:${READER_UUID}\"}"
```

该命令预期 exit 1：唯一的 `error` finding 是 `dc:identifier changed`，这是 metadata 红线对测试副本 identity 变化的提示；流水线仍会写出 `C.readest.epub`，供人工检查并记录其 SHA-256。不要把此预期失败写成通过，也不要覆盖原候选。

## 验证建议

- Apple Books：重点看嵌入样式、Ruby、弹注和 A-lite 分页。
- Kindle / KFX：重点看封面识别、弹注触发、竖排、表格和背景图。
- KOReader / Thorium / Calibre：重点看 CSS 兼容差异和窄屏重排。

这个模板不包含第三方字体和版权图片，便于直接纳入版本管理。当前章题/着重号/语调标记兼容构建已使用独立 EPUB identity，以避免阅读器命中旧 UUID 缓存；本次同步保留该 identity，仅加入脱敏的 scene 28 横排生产骨架，饰图只由共享 CSS 的 body background 绘制。
