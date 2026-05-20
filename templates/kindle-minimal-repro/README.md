# Kindle Minimal Repro

这个模板用于二分 Kindle Previewer 打不开或转换失败的问题。它只保留：

- OPF `cover-image` + `<meta name="cover">`
- EPUB 3 `nav.xhtml`
- NCX fallback + `spine toc="ncx"`
- PNG 封面
- 一个 SVG 正文图片
- 一个普通 CSS 文件
- 一个长串换行压力样本

构建：

```sh
sh templates/kindle-minimal-repro/build.sh
```

如果 Kindle Previewer 仍打不开，下一步优先移除 `Images/poster.svg` 和对应 `<img>` 复测；若可打开，再回到完整 demo 检查页面级 CSS 或资源组合。
