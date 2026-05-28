# EPUB Diff 工具

> 状态：v1；浏览器内静态 web app。
> 用途：对比两个 EPUB 文件，只做文件对比，不涉及阅读器效果验收。
> 入口：`tools/epub-diff/index.html`。

## 用途

输入两份 epub，人工 review 文件层差异。

五层组织：

- 结构：OPF / spine / nav / ncx。
- 文本：XHTML 段落级 hash。
- 样式：CSS selector + line diff。
- 资源：图片 / 字体 / 音频。
- 元数据：dc:* / meta。

## 安装（一次）

```sh
bash tools/epub-diff/scripts/fetch-vendor.sh
```

这会把 `@pierre/diffs` 和 `zip.js` 抓到 `tools/epub-diff/assets/vendor/`。

## 使用

1. 把 `tools/epub-diff/index.html` 用浏览器打开。
2. 在两个 file picker 各选一个 `.epub`。
3. 点 Compare。
4. 按五层逐层 review。

## Demo 文件

```sh
bash samples/demo-books/build.sh
```

然后选择：

- `samples/demo-books/dist/city-field-notes-before.epub`
- `samples/demo-books/dist/city-field-notes-after-clean.epub`

这对文件应显示样式、资源和结构变化，文本层保持一致。需要看文本差异时，选择 `redline-trap-before.epub` 和 `redline-trap-after-text-changed.epub`。

## 浏览器要求

- Chrome / Edge 100+
- Safari 16.4+
- Firefox 101+

## 离线说明

工具完全离线运行；epub 文件不离开你的设备。

## 如果遇到 `file://` 模块加载问题

```sh
cd tools/epub-diff/
python3 -m http.server 8000
```

浏览器打开 `http://localhost:8000/`。

## 不做什么

- 不渲染 epub（不是阅读器）。
- 不集成 Kindle Previewer / Apple Books。
- 不评估「这是不是合法的清洗」（这由 `scripts/validate_text_invariance.py` 完成）。
- 不向任何服务器发数据。

## 与清洗流程集成

见 [epub-cleanup-flow.md](epub-cleanup-flow.md)。自动红线 gate 先跑；diff 工具只负责人工可视化 review。

## 故障排查

| 现象 | 原因 / 解决 |
| --- | --- |
| 打开 index.html 一片空白 | `assets/vendor/` 是空的；先跑 `fetch-vendor.sh` |
| 点 Compare 后报错 `Cannot read EPUB` | 输入不是合法 zip / 缺 container / 有读取错误 |
| 大 epub 浏览器卡死 | 浏览器内存超限；记录为开放问题 |
| Safari 拖拽不工作 | 改用 Choose file 按钮 |
| CSS diff 看不到高亮 | vendor 未加载时会退回内置 diff renderer |
