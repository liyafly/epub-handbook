# 字体工具

本目录是 EPUB Handbook 的字体相关独立工具。与 `scripts/`（Python）、`swift/`（Swift）按 capability 并存。

## font-preview.html

**单文件、离线、双击即用、零第三方依赖**的字体预览工具。

- 拖入 `.ttf` / `.otf` / `.woff` / `.ttc` / `.woff2` 字体文件
- 显示字体名与版本（手写 SFNT name 表解析器）
- 输入任意文字，在多个字体间实时对比渲染效果
- 字号可调（12–96px）
- 不上传、不联网、纯本地运行

用任何浏览器打开 `font-preview.html` 即可使用。
