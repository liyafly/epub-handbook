# 书级 EPUB 工作区

日常在书根：改 `03 制作工作区/epub/` → `sh build.sh` → 阅读器检查 → 提交。根脚本依赖 EPUB Handbook 统一构建器，路径可用 `EPUB_HANDBOOK_ROOT` 指定；需可用的 `epub` CLI，含字体时另装 `epub-font` provider。

- `sh build.sh`：注入锁定字体、子集化、独立覆盖检查、导航审计与全项 redline，通过后更新 dist、收据与 `交付记录.tsv`。
- `sh build.sh fonts`：将库中完整字体注入源树供编辑器预览；库字体路径 Git 忽略，子集字体仍跟踪。
- `sh build.sh clean`：清旧失败构建，保留最近一次现场；不清收据或其他材料。
- `sh build.sh accept-dist`：先将编辑过的 dist 修改并回源并完成人工 diff；命令验证重建候选每个成员一致后才重新登记 SHA。

字体配置在 `03 制作工作区/fonts.json`。`target` 是包内路径，`from` 是共享字体库文件名，`sha256` 锁定完整版本；可变字体保留 `variation.mode=instance` 与 axes。库优先 `EPUB_FONT_LIBRARY`，默认向上找 `fonts/`。完整母版只在共享库保存一份；来源和许可见库内 `字体清单.tsv`。既有子集留源树，不入库；缺字 warning 不代表覆盖通过，子集引入的回归仍会阻断。更新锁须审阅后显式执行 `epub-font inject --config '03 制作工作区/fonts.json' --update-lock`。

`01 源文件/` 冻结原始输入；参考、正文校订、版式截图和历史记录分别放 `02 校对材料/` 相应子目录。书根制作说明只写当前状态、目标阅读器、配方、待办和决策链接。成功记录自动写入制作区 TSV，含书源/手册提交、SHA 和字体锁；`dist/`、`.pipeline/` 与预览母版不提交。

构建保留唯一固定交付件，失败现场可复核；dist 与收据不匹配时停止，不静默覆盖人工修改。可复现性取决于源、字体锁、手册/provider 及打包工具版本，静态成功不代表阅读器验收。已有 EPUB 导入只支持 OEBPS 布局；其他布局保留已验证的构建路径，不能为适配工具猜改目录。

详细约定见 [书级工作区](https://github.com/liyafly/epub-handbook/blob/main/docs/pipeline/book-workspace.md)、[实现约束](https://github.com/liyafly/epub-handbook/blob/main/docs/final/SPEC-%E5%AE%9E%E7%8E%B0%E7%BA%A6%E6%9D%9F.md) 和 [字体 provider](https://github.com/liyafly/epub-handbook/blob/main/tools-font/epub-font/README.md)。
