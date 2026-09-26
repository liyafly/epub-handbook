# epub-font：EPUB 字体子集化与全量校验

独立 Python + fontTools provider，与 `coverage-detector/` 同级，**不打包进 EPUB Handbook Go 发行包**。书级构建可经正式的 `epub.font.subset` capability 调用它；也可以直接使用下面的 CLI。
它只做一件事：把 EPUB 里**已存在**的字体条目替换成按全书字符集裁切后的字体字节，并逐项核验。
OPF、CSS、XHTML 与其他 entry 原样复制（同顺序、同压缩方式），所以字体 alias、包内路径、CSS URL 与 OPF id 都不变
（`docs/final/字体别名命名规范.md` §4.7）。

## 安装

```sh
uv tool install --editable tools-font/epub-font
epub-font --help
```

在仓库内开发或运行离线测试：

```sh
cd tools-font/epub-font
uv sync
uv run pytest -q          # 离线测试，使用合成字体，不需要下载
```

## 用法

```sh
epub-font subset BOOK.epub --out NEW.epub [--config fonts.json]
epub-font check NEW.epub [--font OEBPS/Fonts/st-all.ttf ...] [--json REPORT.json]
```

- `subset` 总是写出 `NEW.font-report.json`；只有全部字体检查通过时才写 `NEW.epub`。两个输出都必须不存在，`NEW.epub` 必须与输入不同。
- Go capability 会在私有临时目录调用 provider；provider 报告跟随这次临时目录清理，结果通过 capability envelope 的 facts/findings 返回。
- 省略 `--config` 时自动处理 OPF manifest 中的全部静态字体；配置文件可指定母版、额外字符或可变字体模式。
- `check` 省略 `--font` 和 `--font-file` 时检查 EPUB manifest 中的全部字体；`--font-file` 用于校验包外字体。
- `subset` 退出码：`0` 全部检查通过并写出 EPUB；`1` 字体核验失败（报告已写，EPUB 不写）；`2` 输入/配置错误或不支持的字体（报告与 EPUB 均不写）。`check` 退出码：`0` 全覆盖；`1` 有缺字；`2` 输入错误。

## fonts.json

```json
{
  "version": 1,
  "fonts": [
    {"target": "OEBPS/Fonts/st-all.ttf", "master": "../masters/NotoSerifSC-VF.ttf",
     "variation": {"mode": "instance", "axes": {"wght": 400}}, "extraText": "〓"}
  ]
}
```

| 键 | 必填 | 含义 |
| --- | --- | --- |
| `target` | 是 | EPUB 内已存在、且在 OPF manifest 中的字体 ZIP 路径；扩展名决定输出格式：`.ttf`（需 TrueType 轮廓）、`.otf`（需 CFF/CFF2 轮廓）、`.woff`、`.woff2` |
| `master` | 否 | 包外母版路径（相对 fonts.json 所在目录）；省略时用 `target` 当前字节原地子集化。可为 `.ttf/.otf/.woff/.woff2`，不支持 `.ttc/.otc` |
| `variation.mode` | 可变字体必填；静态字体省略 | `keep`（保留全部变体）/ `instance`（钉住所有轴 → 静态字体，手册 §4.6 推荐）/ `limit`（收窄轴范围，仍是 VF） |
| `variation.axes` | 视 mode | `instance`：`{"wght": 600}`，未写的轴取默认值；`limit`：`{"wght": [400, 700]}` 或数字（钉住该轴） |
| `extraText` | 否 | 额外保留的字符（SPEC §4 第 5 条 `extraCodepoints`） |

## 收集哪些字符（全书范围）

manifest 中全部 XHTML / SVG / NCX（含 nav）的文本节点，`alt` / `title` / `aria-label`，全部 CSS 字符串字面量
（CSS 文件、`<style>`、`style=""`，覆盖 `content:` / `quotes:`；跳过注释与 `url("…")`），外加固定基线
（ASCII、常用 CJK 标点、着重号 ﹅﹆•◦●○◉◎▲△、列表符号、〇一二三…万）、大小写变体，
以及 CSS 出现 `full-width` 时的全角变体。`<script>` 内容不收集。

## 每个字体的检查（report 的 `checks`）

| 检查 | 通过条件 |
| --- | --- |
| `cmap-coverage` | 母版能覆盖的所需字符，输出全部覆盖 |
| `uvs-sequences` | 母版有、且文本出现了 base 与选择符的 IVS/SVS 序列，输出保留 |
| `vertical-alternates` | 母版对所需字符有 `vert`/`vrt2` 替换的，输出仍有 |
| `outlines` | 每个字形的前进宽度 ±1、外框 ±1% em、面积 ±10%；全体面积漂移 ≤ 0.5%（抓错轴位置） |
| `variation` | keep：轴与母版一致；instance：无 fvar/gvar/CFF2，`usWeightClass` = wght；limit：轴范围与配置一致 |
| `format` | 输出轮廓/封装与目标扩展名一致 |

母版里本来就没有的字符列在 `notInMaster`（警告，不算失败）：它们要靠字体链后续字体兜底，交给 `epub.font.coverage.analyze` 判断。

## 全量校验：`epub-font check`

独立于子集工具的"字符是否全量在字体里"检查（不 import `epubtext` / `fontops`；文本用 lxml 收集，字体直接读 cmap），
用来复核子集产物，也可以检查任何 EPUB 的嵌入字体或包外母版：

```sh
epub-font check BOOK.epub --font OEBPS/Fonts/st-all.ttf [--font ...]       # 指定书内字体
epub-font check BOOK.epub                                                  # 默认检查所有 manifest 字体
epub-font check BOOK.epub --font-file rare.ttf --chars-file rare.txt       # 包外字体 + 指定字符清单
... [--json report.json]
```

- 要求的字符：全书实际用字（XHTML/SVG/NCX 文本、`alt`/`title`/`aria-label`、CSS 字符串）+ CSS 关键字生成的字符
  （`text-emphasis` 着重号、`list-style-type` 的 `cjk-decimal` 等序号、`<q>` 的默认引号、`hyphens: auto` 的连字符、`text-transform` 变体）。
  **不跳过** ASCII 与 U+2000–U+2E7F 标点（“”‘’——…）——这是 coverage-detector 的字符清单刻意跳过、因而无法证明"全量"的部分。
- 不收：`<script>`、XML/CSS 注释、`url("…")`。`--chars-file` 模式只按文件里的字符检查。
- 判定：`missing`（无 cmap 或映射到 .notdef）、`noInk`（映射到没有轮廓的字形，空格与格式字符除外）、
  `missingSequences`（文本里出现的 IVS/SVS 序列不在 cmap 14）→ 任一非空即 exit 1；
  `optionalMissing`（ZWSP、ZWJ、软连字符等格式字符）只报告不判失败。可变字体按默认实例检查字形。
- 退出码：`0` 全量；`1` 有缺失；`2` 输入错误（字体不在 manifest、字体被混淆等）。

## 已知限制

- 可变字体输出（keep/limit）在阅读器中的支持未实测；手册 §4.6 推荐 `instance` 出静态字重。
- CFF2 母版 instance 时，fontTools 不重算 `VORG`，竖排原点保留默认实例值（会给 warning）；竖排书优先用 TrueType 母版，并在阅读器实测。
- CFF2 instance 的坐标舍入会有 ≤ 约 0.7% em 的点位漂移（阅读字号下不可见），`outlines` 检查已按此设定容差。
- 含 `MATH` 表的数学字体直接拒绝（手册 §4.6 第 4 步：保留完整数学字体）。
- `META-INF/encryption.xml` 列出的字体（混淆/加密）直接拒绝。
- 只替换已存在的字体条目；新增 `@font-face` / manifest item 属于 CSS/OPF 修改，不在本工具范围。
- 不做授权判断：OFL 字体若有 Reserved Font Name（如思源的 "Source"），子集属于修改版，发布前自行核对许可。
