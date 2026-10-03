# 书级 EPUB 工作区

本仓库维护一本书的解包源、制作决策和必要材料。日常只改
`03 制作工作区/epub/OEBPS/`；打包与检查由脚本完成，不需要手动解压、复制多个 EPUB
或整理时间戳产物。初始书稿包含标题页、第一章、导航、NCX 和基础样式。

此 starter 和 `build.sh` 只支持 `mimetype`、`META-INF/`、`OEBPS/` 布局；starter 初始结构把 OPF 放在 `OEBPS/package.opf`。它不是任意已有 EPUB 的通用打包器；接入不同目录布局的书籍前，请按 [已有 EPUB 接入说明](../../docs/pipeline/book-workspace.md#已有-epub-的一次性接入) 核对并保留其可用构建路径。

## 日常修改

1. 修改 `package.opf`、`nav.xhtml`、`toc.ncx`、`Text/` 或 `Styles/` 中需要的文件。换书名时同步修改 OPF 标题、作者、UUID，以及 NCX 的 `dtb:uid`。新增章节时同步 OPF manifest/spine、nav 和 NCX。
2. 在书根检查并提交源文件：

   ```sh
   git status --short
   git diff --check
   git add -A
   git commit -m 'content: update book source'
   ```

   `dist/` 和 `.pipeline/` 已忽略，不会把构建产物加入书级 Git。
3. 在书根运行唯一构建命令：

   ```sh
   sh '03 制作工作区/epub/build.sh'
   ```

   未安装时：`EPUB_BIN=/path/to/epub sh '03 制作工作区/epub/build.sh'`

4. 交付文件固定为 `03 制作工作区/dist/book.epub`。有字体时，provider 会用独立字符收集器检查子集没有引入缺字、空字形或 IVS/SVS 序列损失；随后运行导航审计和全项 redline。所有检查通过后才覆盖，失败时保留上一版。成功构建会在忽略目录 `.pipeline/dist-sha256` 保存交付件 SHA-256；以后若 dist 被编辑、替换，或收据丢失，构建会停止并保留文件。这样可避免编辑器保存过的 EPUB 被下一次构建静默覆盖。临时 EPUB 会在构建结束时清理；报告使用固定文件名，不累积。

如果需要在 Sigil 等编辑器中修复已交付 EPUB，先保留该文件副本，再用全项 redline 和人工 diff 检查变化，并将需要的 XHTML、CSS、OPF、导航或资源变更并回 `03 制作工作区/epub/`。区分正文、导航、元数据、资源和序列化变化；需要重建后持续保留的转换行为应固化到书级脚本，并在书级验证器中加入可重复的断言。提交并验证源文件后，才把当前已审阅 EPUB 的 SHA 写入 `.pipeline/dist-sha256` 作为接受基线；缺少或不匹配收据时不要直接重建。macOS 可用 `shasum -a 256 '03 制作工作区/dist/book.epub' | awk '{print $1}' > '03 制作工作区/.pipeline/dist-sha256'`，Linux 可用 `sha256sum` 替换 `shasum -a 256`。若 `.pipeline/` 被清理而 dist 仍在，先重新审查并对齐源，再恢复收据。完整处理流程和《圣经的故事》的 Sigil 回写实例见[书级工作区文档](../../docs/pipeline/book-workspace.md#日常修改和构建)。
5. 交付时在 `制作说明.md` 记录源提交、产物 SHA-256 和真实阅读器实测。没有在目标阅读器中打开验证时，状态保持“待验证”。

打包前会在临时副本中统一源文件时间戳，并按固定路径顺序归档。同一源提交、同一平台且 `zip -v` 显示相同构建时可复现相同 EPUB SHA-256；跨平台或 zip 构建不同不承诺字节级一致。macOS 系统 zip 不会为非 ASCII 条目名设置 UTF-8 标志，EPUB 条目名请使用 ASCII。

## 字体

获准使用的完整 `.ttf` / `.otf` 母版放在 `03 制作工作区/epub/OEBPS/Fonts/` 的 manifest 目标路径，登记到 OPF manifest 和 CSS，并与解包源一起提交。构建会从完整母版为当前正文生成子集；新增字后会重新计算，不会把唯一字体母版替换成子集。字体来源和许可记入 `THIRD_PARTY.md`。`build.sh` 会扫描 `META-INF/` 与 `OEBPS/` 中的已知字体扩展名；字体能力随后按 entry 魔数拒绝未登记或媒体类型错误的字体。无字体书里仅改名、且没有 `fonts.json` 的字体仍不能触发 provider。解包源树不是交付物，禁止直接打包它；交付 EPUB 必须经过书内 `build.sh` 或 `epub.font.subset` 的候选与检查流程。

书中没有字体时不需要字体 provider。含字体时，需要安装 `epub-font` provider 并让 `epub-font` 命令可在 PATH 中找到，且 `epub` CLI 必须提供 `epub.font.subset` 能力。可在手册仓库根目录安装 provider：

```sh
uv tool install --editable tools-font/epub-font
```

自动字体处理会完整保留带 OpenType `MATH` 表的数学字体，无需在 `fonts.json` 中声明保留。旧配置里的 `action: "preserve"` 暂时作为弃用 no-op 接受并提示；可变字体只接受 `variation.mode: "instance"`。加密/混淆或损坏的字体不会因保留设置而放行。

## 进阶排版

默认使用自由字体模式。整书锁定字体时，还要在 `fonts.css` 设置 `body` 字体，并在 OPF 配对 `ibooks:specified-fonts` 元数据与 `ibooks` prefix。手工切换 CSS 层不等同于完整套用一个 preset：预设有多个分层文件以及对应的 manifest/head 引用；对已有 EPUB，请用 `epub.typography.optimize` 生成候选并审核报告。

## 需要查细节时

- [书级工作区与已有 EPUB 接入](https://github.com/liyafly/epub-handbook/blob/main/docs/pipeline/book-workspace.md)
- [排版实现约束](https://github.com/liyafly/epub-handbook/blob/main/docs/final/SPEC-%E5%AE%9E%E7%8E%B0%E7%BA%A6%E6%9D%9F.md)
- [可复用主题样式](https://github.com/liyafly/epub-handbook/tree/main/templates/style-presets)
- [字体 provider 配置](https://github.com/liyafly/epub-handbook/blob/main/tools-font/epub-font/README.md)
- [可选排版样例](https://github.com/liyafly/epub-handbook/tree/main/templates/epub-style-demo)
