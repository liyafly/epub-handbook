# 书级工作区

日常流程：改 `epub/` → `epub book preview` → `epub book build` → 阅读器检查。面向普通用户的版本见 [快速开始](../guide/quick-start.md)；本页是维护者参考。

## 命令

```sh
epub book new <dir> [--from-preset <name>] [--title <title>] [--author <author>] [--git]
epub book import <original.epub> <dir> [--git]
epub book preview [<dir>] [--variant <name>] [--open[=<app>]]
epub book build [<dir>] [--variant <name>]
epub book fonts [<dir>] [--update-lock]
epub book clean [<dir>]
epub book accept-dist [<dir>] [--variant <name>]
epub book recipes [<name> [--add <dir>]]
epub book reader-layer [<dir>]
epub book add-chapter [<dir>] --title <title> [--file <name>] [--after <chapter>]
```

纯 Go 实现，Windows、macOS、Linux 共用；省略 `<dir>` 表示当前目录。安全属性与旧 sh 构建器一致：冻结原件与 SHA、DRM 停止、导航审计、字体锁与子集、全项 redline、原子替换、`dist-sha256` 收据、`accept-dist` 逐成员比对、构建锁、失败现场保留。

## 两种布局

书目录含 `book.json` 为新布局，否则按旧布局处理。

```text
<book>/                  # 新布局
  book.json              # 书名、输出名、字体锁、original 的 SHA、readers、variants
  original.epub          # import 时冻结，只读
  epub/                  # 解包源，唯一编辑处
  out/book.epub          # 成品
  .epub/                 # 报告、收据、delivery.tsv（只留最近一次）、预览、失败现场 build.*；可清理
  制作说明.md            # new / import 生成的一屏提纲
  素材/                  # 手建，纳入书级 Git：脚本、源图/源 HTML、验收证据、历史.md
```

`.epub/` 只放工具生成、能重新生成的东西，随时可清；要长期保留的放 `素材/`（两种布局都适用；旧布局也可继续用 `02 校对材料/`）。`clean` 不碰 `素材/`。

```text
work-epub/               # 旧布局
  fonts/                 # 共享字体库：全量字体、README.md、字体清单.tsv
  <book>/                # 独立 Git
    01 源文件/           # 冻结输入
    02 校对材料/参考|正文校订|版式|历史/
    03 制作工作区/
      epub/              # 解包源；可保留原有子集字体
      fonts.json         # 本书字体与锁
      交付记录.tsv       # 成功构建自动追加
      dist/book.epub     # 忽略；既有正式文件名可保留
      .pipeline/         # 忽略；报告、收据、失败现场
    build.sh             # 转发到手册统一构建器
    制作说明.md          # 当前状态、目标阅读器、配方、待办与决策链接
```

旧布局书根 `build.sh` 里的 `EPUB_OUTPUT_NAME` / `EPUB_LEGACY_LAYOUT` 由 `epub book` 按行读取，不执行脚本。旧书无需迁移。

## 导入

`import` 冻结原件与 SHA，运行导航审计，解包，盘点内嵌字体；加 `--git` 时提交基线。加密、损坏或审计阻断时停止，冻结输入与报告保留供分析，不自动转目录，不解密。

非 OEBPS 布局（或 OEBPS/META-INF 之外有资源）自动原样保留：`book.json` 记 `legacyLayout`，构建时按原布局打包，不改名、不猜测；导入时打印一行提示。预设、配方 `--add` 和覆盖层都假定 `OEBPS/`，这类书需手工处理。旧参数 `--legacy-layout` 仍接受，已无作用。

完整字体按 SHA 去重进共享库，许可先标"待核"。工具判断可能为子集的字体留在源树，报告列出字形数和判断；这个判断不能证明字体一定是全量。

OEBPS 布局的书在解包后加一个空的 `reader.css`（放在第一份样式表的目录），OPF 的 manifest 末尾登记，每个 spine XHTML 在 `</head>` 前链接，排在所有样式之后。全部用字节区间插入，不重序列化；有页面无法安全编辑（没有闭合的 `<head>`、非 UTF-8）时只警告、一个文件都不改，修好后用 `epub book reader-layer` 补。书里已有 `reader.css` 时跳过。非 OEBPS 布局不加。之前导入的书也用 `reader-layer` 补，旧布局同样适用。

`new` 给每本书生成新的 `urn:uuid` 标识符（OPF 与 NCX 相同）和建书时刻的 `dcterms:modified`；之后构建不再改它。`--title`、`--author` 写进 OPF、NCX、书名页、版权页和 `book.json`。

`add-chapter` 加一章：新文件放在 spine 最后一个 XHTML 的目录，默认名 `<NN>-<slug>.xhtml`、排在末尾；样式表 link 照抄最后一章。同步 OPF manifest、spine、nav 的 toc 和 NCX（`playOrder` 重排；没有 NCX 就跳过）。`--after <文件>` 把目录项插在指向它的最后一条之后，作为同级；它在 nav 里没有条目就拒绝。只做字节区间插入；任何一处不能安全编辑就一个文件都不改。两种布局都可用，多卷工作区要进各卷目录执行。

导入后没有必要时，不做规范化、EPUB3 迁移或 CSS 清洗；确有问题才走 [cleanup-flow.md](cleanup-flow.md) 对应步骤。

## 构建

`build` 依次：复制源 → 按锁注入完整字体 → 打包 → 子集化与独立覆盖检查 → 导航审计 → 全项 redline → Kindle 检查 → 原子替换成品，登记收据和交付记录。Kindle 检查（`epub.kindle.compatibility.check`）对每个产物都跑，终端一行汇总，报告写 `kindle-check.json`、明细进 `report.html`；只有名为 `kindle` 的变体遇到 error 级 finding 才失败（这时所有产物都不写出），其他产物只提示。`readers` 仍只是备注。另有一步与上一版成品的 redline 比较，只供审阅，不阻断。

- 源树里有不在 SPEC-实现约束 §7 分层表（加 `reader.css`、`decoration.css`）里的样式文件时，打一行 `INFO style layers`，不阻断。导入书原有的样式表不必改名；这条是提醒新规则写进对应分层，不要另起文件。
- 终端每项检查一行：`PASS` / `FAIL` / `WARN` / `INFO`，必要时缩进附几条说明（失败列 error finding 和位置，不再打印整份 JSON）。字体缺字合成一行，只报缺字最多的字体，正文缺字与 CSS/配置/生成文本里保守加入的字分开计数。成品超过 50 MB 时提示 Send to Kindle 可能拒收。
- 每次构建写 `report.html`（新布局 `.epub/`，旧布局 `.pipeline/`）汇总各项；原始报告同目录。
- 成功只保留固定交付包。失败保留 `build.*` 现场与报告（含 `report.html`），不替换已有成品。
- `clean` 只清旧的 `build.*`，默认保留最近一次；遇到构建锁停止；不清收据和其他材料。
- 构建锁 `build.lock/` 里记着进程号和主机名。主人是本机已结束的进程（被强行结束、断电）时，下次 `build` / `clean` 自动清掉并打印一行；没有主人信息、别的主机或进程仍在运行的锁照旧阻断，确认没有构建在跑后手动删除。
- 现有成品的 SHA 必须与收据一致；缺收据或不一致都拒绝覆盖。
- 编辑过成品后：先保存副本，做全项 redline 与人工 diff，把修改并回源；再运行 `accept-dist`。它重新生成并检查候选，要求包内所有成员与当前成品一致（不只比正文红线），通过才登记收据。它不替代人工审核，也不允许跳过源同步。

## 预览

`preview` 复制源 → 套用变体覆盖层 → 注入库字体（不子集化）→ 打包到 `.epub/preview[-<name>].epub`（旧布局在 `.pipeline/`）。**不跑任何检查，不写收据与交付记录，不碰成品**，交付前必须 `build`。

`--open` 用系统默认程序打开（macOS `open`，Windows `cmd /c start`，Linux `xdg-open`）；`--open=<app>` 指定应用，macOS 上如 `Reeden`、`Books`、`Readest`、`Kindle Previewer 4`。打不开只警告，文件路径已打印。多册工作区逐册预览。

## 变体

变体 = 同一份源 + 一个覆盖层目录。阅读器版本和"锁定字体 / 正文自由"这类版本差异共用这套机制。

新布局写在 `book.json`：

```json
"variants": {
  "kindle": {"overlay": "variants/kindle", "output": "book-kindle.epub", "fonts": []}
}
```

- `output` 省略时为 `<通用输出名去掉 .epub>-<name>.epub`。
- `fonts` 省略则沿用通用字体配置；`[]` 表示本变体不嵌字体。
- `"readers": [...]` 仅作文档，不生成任何东西。
- 旧布局把同样的 `{"variants": {...}}` 写在制作区的 `variants.json`，覆盖层路径相对制作区；它不影响布局识别。

覆盖层镜像 EPUB 树（如 `OEBPS/Styles/reader.css`），**只允许替换或新增 `.css`、替换 OPF**；出现 XHTML、nav、NCX、图片、字体即在构建前报错。`new` 和 `import` 的书自带空的 `reader.css`，在 OPF 和每个 XHTML 里排在最后，覆盖层只需替换它，只写与通用版不同的规则；不要整份复制已有样式表。Kindle 上调不好的装饰页用配方 [page-image](../../templates/recipes/page-image/recipe.md)：通用版隐藏整图，覆盖层显示整图、隐藏文字版。不要写阅读器识别查询（SPEC §5.6.1）。

构建时先造通用版候选，再造各变体候选，各自走完整门禁（字体注入/子集/覆盖检查、导航审计、全项 redline）；每个变体另与通用版比对正文（`text` 红线）。**全部候选通过后才统一替换**：任一失败则 `out/` 一个文件都不动，错误信息列出失败的输出。每个输出有自己的收据（通用版 `dist-sha256`，变体 `dist-sha256.<name>`）和同样的"改过的成品不覆盖"保护；交付记录每个输出一行，变体行末多一列 `variant:<name>`。

`--variant <name>`（`build` / `accept-dist` / `preview`）只处理该变体，不重建也不触碰通用版成品与收据。`accept-dist` 不带 `--variant` 时逐个输出做成员比对，全部一致才登记收据。

## 字体

完整母版不重复提交到每本书。字体库取 `EPUB_FONT_LIBRARY`；未设置时从书配置（`book.json` 或 `fonts.json`）的位置向上找与祖先同级的 `fonts/`；`import` 同样先找已有的库，找不到才在书目录旁新建，所以书可以放在分类子目录（如 `work-epub/<分类>/<book>/`）。显式设置的目录不存在时失败，不悄悄回退。库内 `字体清单.tsv` 记录文件名、SHA、来源、许可与备注，书级 `THIRD_PARTY.md` 引用它；正文和图片仍在书级记录来源。库文件缺失或 SHA 不符即停，不擅自换字体版本。

provider 先查 PATH，再回落 `$HOME/.local/bin`（Windows 为 `%USERPROFILE%\.local\bin`，即 `uv tool install` 的默认位置）。`EPUB_LOCAL_BIN=<目录>` 指定位置，显式设空则只用调用者原始 PATH；`EPUB_FONT_BIN` 指定 provider 程序。

字体配置（新布局在 `book.json` 的 `usage`、`fonts`，旧布局在 `fonts.json`）：

```json
{
  "version": 1,
  "usage": "css",
  "fonts": [
    {"target": "OEBPS/Fonts/st.otf", "from": "body-master.otf", "sha256": "<完整小写SHA256>"},
    {"target": "OEBPS/Fonts/ht.ttf", "from": "heading-variable.ttf", "sha256": "<完整小写SHA256>",
     "variation": {"mode": "instance", "axes": {"wght": 700}}},
    {"target": "OEBPS/Fonts/retained-subset.ttf"}
  ]
}
```

- `target` 是 OPF/CSS 引用的包内路径；新书用角色别名，旧书保留原名。`from` 只能是库内文件名。没有 `from` 的项继续用源树字体，两种可以混用；原有子集不入库。
- `usage` 省略时按全书字符集；显式 `css` 才按保守的 CSS 角色范围。未知选择器会扩大字符集；活动内容、无法证明安全的样式或字符生成方式会拒绝 CSS 模式。
- 可变字体只接受静态实例化；MATH 字体按原字节保留；原有加密边界不变。
- 母版本来缺的字在子集这一步只给 warning（终端一行汇总，样本与定位在报告里）。这条警告按全书字符算，英文字体没有中文也会算进去。下一步 `fixed-role fonts` 列出真正用到却缺的字：指定了字体的文字（字体链第一段是嵌入字体）里，这条链的嵌入字体都没有的字，清单在 `.epub/font-roles.json`。它也只警告，因为回退字体可能显示正常，要在多个真实阅读器里查看，规则见 [SPEC §8](../final/SPEC-实现约束.md#8-字体链规则)。子集阶段：子集新增的缺字、空字形或 IVS/SVS 回归仍阻断。子集源缺少后来新增的字无法自动恢复，需找到授权的完整母版并更新配置。
- 更换母版版本后，审阅来源、许可与字形变化，再显式更新锁：`epub book fonts --update-lock`（旧布局也可用 `epub-font inject --config '03 制作工作区/fonts.json' --update-lock`）。

`epub book fonts` 把锁定的母版注入源树供编辑器预览；源树里对应的母版路径必须被 Git 忽略，混用时只忽略库字体路径，子集字体继续跟踪。不注入时编辑器显示回退字体。交付总是从库重新注入临时副本，不复用预览文件。

## 交付记录与制作说明

交付记录（新布局 `.epub/delivery.tsv` 只留最近一次；旧布局 `交付记录.tsv` 逐次追加）自动写入时间、书源 commit（有未提交修改带 `-dirty`）、手册 commit、成品 SHA 和字体锁。复现时取记录中的提交；字体库与手册版本一起备份。

`new` / `import` 生成 `制作说明.md` 骨架（已存在不覆盖），只放一屏：当前状态、成品与原件 SHA、目标阅读器、使用的配方、待办、已知差异和决策链接。逐轮修改经过写 `素材/历史.md`（旧布局可放 `02 校对材料/历史/`），不删除历史。

按 [AGENTS.md](../../AGENTS.md)「过程记录」，日常构建不另写记录。配方首测或写兼容结论时，保留完整 artifact/SHA、阅读器版本和逐场景截图，并回写 [reader matrix](../final/reader-matrix.yaml)；出问题时保留失败现场与报告，确认交付后可清理中间调参；正文校订走 SPEC §10.1.1，含文决策留书级材料，被 gate 引用的文件与路径映射不得提前删除。

## 配方

`epub book recipes` 列出 [templates/recipes](../../templates/recipes/README.md) 里的配方和降级路线；`epub book recipes <name>` 打印配方 CSS；`--add <dir>` 把它追加到该书 `OEBPS/Styles/design.css` 末尾，带 `/* recipe: <name> ... */` 标记，已存在则拒绝。只追加，不改已有规则；XHTML 里的类名仍要手工加（参考同目录 `sample.xhtml`）。

## 旧布局的 build.sh

旧书书根 `build.sh` 转发到 `${EPUB_HANDBOOK_ROOT:-$HOME/Developer/epub-handbook}/templates/book-starter/build.sh`，后者只再转发给 `epub book <命令> <书目录>`，构建逻辑只有 Go 一份：

```sh
sh build.sh            # 在书根；另有 fonts / clean / accept-dist / preview
```

`epub` 从 PATH 或 `$HOME/.local/bin` 找，`EPUB_BIN` 可指定。多册工作区由 `epub book` 逐册处理。配置优先制作区 `fonts.json`，再兼容书根配置；既有正式名用转发脚本的 `EPUB_OUTPUT_NAME` 保留，非 OEBPS 书可设 `EPUB_LEGACY_LAYOUT=1`。旧的 `new-book.sh`、`import-book.sh` 已删除，新书用 `epub book new|import`。

## Git 与第三方材料

书级 Git（旧布局必有，新布局用 `--git` 可选）保留源、配置、决策和交付记录；共享字体库与书一起备份，不建书级 Git。书级 Git 不重写历史，不作为主仓 submodule。

迁移前先核对并保存已有改动、冻结旧构建基线；字体和目录调整单独提交，布局合并另作提交；迁移后跑全项 redline，并逐项对比非字体成员。来源或许可待核的内容只留本地，不配置公开 remote，不发布。完整证据与真实书目信息留本地，回填主仓按 AGENTS 脱敏。
