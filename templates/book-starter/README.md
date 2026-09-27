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

4. 交付文件固定为 `03 制作工作区/dist/book.epub`。导航审计和全项 redline 通过后才覆盖；失败时保留上一版。临时 EPUB 会在构建结束时清理；当前报告固定保存在忽略目录 `.pipeline/`，每次构建先清掉上次报告，不会累积。
5. 交付时在 `制作说明.md` 记录源提交、产物 SHA-256 和真实阅读器实测。没有在目标阅读器中打开验证时，状态保持“待验证”。

## 字体

获准使用的完整 `.ttf` / `.otf` 母版放在 `03 制作工作区/epub/OEBPS/Fonts/`，登记到 OPF manifest 和 CSS，并与解包源一起提交。构建会从完整母版为当前正文生成子集；新增字后会重新计算，不会把唯一字体母版替换成子集。字体来源和许可记入 `THIRD_PARTY.md`。

书中没有字体时不需要字体 provider。含字体时，需要安装 `epub-font` provider，且 `epub` CLI 必须提供 `epub.font.subset` 能力。可在手册仓库根目录安装 provider：

```sh
uv tool install --editable tools-font/epub-font
```

若母版放在 EPUB 外部，可在书根放 `fonts.json`，`master` 路径相对该配置文件。

自动字体处理会完整保留带 OpenType `MATH` 表的数学字体；若配置 `fonts.json`，对这类字体写 `action: "preserve"`，普通字体默认仍执行子集化。加密/混淆或损坏的字体不会因保留设置而放行。

## 进阶排版

默认使用自由字体模式。整书锁定字体时，还要在 `fonts.css` 设置 `body` 字体，并在 OPF 配对 `ibooks:specified-fonts` 元数据与 `ibooks` prefix。手工切换 CSS 层不等同于完整套用一个 preset：预设有多个分层文件以及对应的 manifest/head 引用；对已有 EPUB，请用 `epub.typography.optimize` 生成候选并审核报告。

## 需要查细节时

- [书级工作区与已有 EPUB 接入](https://github.com/liyafly/epub-handbook/blob/main/docs/pipeline/book-workspace.md)
- [排版实现约束](https://github.com/liyafly/epub-handbook/blob/main/docs/final/SPEC-%E5%AE%9E%E7%8E%B0%E7%BA%A6%E6%9D%9F.md)
- [可复用主题样式](https://github.com/liyafly/epub-handbook/tree/main/templates/style-presets)
- [字体 provider 配置](https://github.com/liyafly/epub-handbook/blob/main/tools-font/epub-font/README.md)
- [可选排版样例](https://github.com/liyafly/epub-handbook/tree/main/templates/epub-style-demo)
