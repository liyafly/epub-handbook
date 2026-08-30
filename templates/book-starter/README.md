# book-starter

最小可成书骨架：标题页 + 一章正文 + nav + NCX，预装 literary-cn preset（自由模式，
body 不锁字体）。用途是「十分钟出一本结构合规的书」，进阶场景从
`templates/epub-style-demo/` 按页复制。

## 用法

1. 复制本目录为你的书目录；改 `package.opf` 的 `dc:title` / `dc:creator` /
   `dc:identifier`（换一个新 UUID，并同步 `toc.ncx` 的 `dtb:uid`）。
2. 在 `Text/` 写章节，每加一页同步 `package.opf` manifest+spine、`nav.xhtml`、`toc.ncx`。
3. 构建并体检：

   ```sh
   sh build.sh
   go run ./cmd/epub run epub.package.nav.audit --input dist/<产物>.epub --json
   ```

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
