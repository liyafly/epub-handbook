# 异形环绕独立样本

保留的合成场景包含四方向 L 形、左右动物轮廓和左右波形。正文连续流动，
由 `float`、显式 `shape-outside: polygon(...)` 与 `shape-margin` 控制环绕。
本模板用于后续配方验证，不改变主样式 demo。

```sh
sh templates/shape-wrap-demo/build.sh
```

只依赖 Python 3 标准库。脚本将 `src-v3/` 中已保存的 XHTML、CSS 和图片打包为
`dist/shape-wrap-scenes-v3.epub`；`mimetype` 位于首项且不压缩。
构建不生成图片、不修改源码、不覆盖历史观察记录。固定 ZIP 顺序与时间戳，
是否复现历史产物仍以本机重建后的 SHA-256 对比为准。

`evidence/` 保存迁入的原始观察、截图、验证日志、测试副本归档和原生成脚本
`build-scenes-original.py`；`cleanup-receipt.json` 保存历史清理收据。
这些材料原样保留在本地并被 Git 忽略。公开的合成场景截图及去除本地路径和字体偏好的观察索引在 [reader-evidence/](reader-evidence/README.md)，不包含书级素材。
旧记录中的实验路径是历史位置，现从本模板目录查找；历史测试副本在
`evidence/standard-v3-test-inputs.zip` 中。

历史观察记录了 Apple Books、Readest 的轮廓环绕和 Kindle Previewer 的矩形回退。
这些结论只属于记录中的产物、版本与配置，迁移和静态检查不构成新一轮阅读器验收。
独立样本没有主样式模板的完整资源，`epub.style.demo.maintain` 的相应失败须单列，
不得将其写成通过。配方规则与阅读器矩阵回写在后续阶段处理。
