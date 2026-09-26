# book-starter

最小可成书骨架：标题页 + 一章正文 + nav + NCX，预装 literary-cn preset（自由模式，
body 不锁字体）。用途是「十分钟出一本结构合规的书」，进阶场景从
`templates/epub-style-demo/` 按页复制。

## 用法

从手册仓库根目录一条命令创建书级 Git 工作区：

```sh
sh templates/book-starter/new-book.sh work-epub/my-book
```

改书级工作区的 `03 制作工作区/epub/OEBPS/`。先更新 `package.opf` 的 `dc:title`、
`dc:creator`、`dc:identifier`（换一个新 UUID，并同步 `toc.ncx` 的 `dtb:uid`），再编辑
`Text/` 中的章节。新增章节时仍须同步 OPF manifest+spine、`nav.xhtml` 和 `toc.ncx`。
版本控制只提交解包源与制作说明；`.pipeline/` 和 `dist/` 已在书级 `.gitignore` 中。

构建并检查：

```sh
sh '03 制作工作区/epub/build.sh'
```

最新产物固定为 `03 制作工作区/dist/book.epub`。通过导航审计和全项 redline 后会覆盖旧产物；
中间 EPUB 与报告留在忽略的 `.pipeline/`，构建失败不会覆盖上一版通过检查的产物。

若从仓库源码运行新 capability，先构建当前 CLI 并传入路径：

```sh
go build -o /tmp/epub-handbook-cli ./cmd/epub
EPUB_BIN=/tmp/epub-handbook-cli sh '03 制作工作区/epub/build.sh'
```

## 字体

完整字体母版放在 `OEBPS/Fonts/`，正常登记到 OPF 和 CSS，并随书级 Git 保存。安装独立
provider 后，构建会从完整母版生成当前正文所需子集；后续新增生僻字时仍从完整源字体重新生成，
不会因上一次的子集而缺字：

```sh
uv tool install --editable tools-font/epub-font
```

无字体的书不需要安装 provider。若母版放在 EPUB 外部，在书根建立 `fonts.json`，其中
`target` 是 ZIP 内 manifest 字体路径、`master` 相对 `fonts.json`；配置格式见
[`epub-font` 文档](../../tools-font/epub-font/README.md)。字体来源与许可记入书级
`THIRD_PARTY.md`。完整书级维护与已有 EPUB 接入方式见
[`docs/pipeline/book-workspace.md`](../../docs/pipeline/book-workspace.md)。

## 换 preset

```sh
cp <仓库路径>/templates/style-presets/academic-cn/Styles/*.css OEBPS/Styles/
```

三个 preset 的文件名只有主题层不同（`literary.css` / `academic.css` /
`classical.css`），换完后把 `package.opf` 里 `css-theme` 那一行的 href 改成对应
文件名，并删除旧主题层文件。页面默认只 link `fonts.css` + `base.css`；需要弹注 /
文字效果 / 图文混排时按 `docs/final/SPEC-实现约束.md` §7 的分层约定补 link。

## 模式说明

默认自由模式（SPEC §8）：`body` 与普通正文 `p` 都不声明字体。整书锁定字体时，
取消 `fonts.css` 中直接 `body` 规则的注释；不必修改每页 XHTML，也不要给裸 `p`
重复指定字体。随后在
`package.opf` metadata 加 `<meta property="ibooks:specified-fonts">true</meta>`
并在 `<package>` 声明 ibooks prefix。meta 与字体锁定的配对关系暂无独立 lint 能力，
以 `epub run epub.package.nav.audit` 的 findings、demo 校验器（本仓模板）与人工
diff review 复核（见 `docs/pipeline/go-rewrite-handoff.md` 遗留项 4）。
