# 一书一 Git 工作区

日常只有改源 → `sh build.sh` → 阅读器检查。主仓忽略整个 `work-epub/`，书级 Git 保留源、配置、决策和交付记录；共享字体库与书一起备份，不建立书级 Git。

## 初始化与接入

```sh
sh templates/book-starter/new-book.sh work-epub/<book>
# 已有、可维护的 OEBPS EPUB
sh templates/book-starter/import-book.sh <原始.epub> work-epub/<book>
```

导入冻结原件与 SHA，运行导航审计，解包并盘点内嵌字体，提交基线。加密、损坏、审计阻断或非 OEBPS 布局停止，冻结输入与报告保留供分析，不自动转目录或解密。完整字体按 SHA 去重进入 `work-epub/fonts/`，许可先标“待核”；覆盖工具判断可能为子集的字体留在源树，报告列出字形数和判断，不能据此证明一定为全量。没有必要时不做规范化、EPUB3 迁移或 CSS 清洗；确有问题才走 [cleanup-flow.md](cleanup-flow.md) 对应步骤。

```text
work-epub/
  fonts/                         # 全量字体、README.md、字体清单.tsv
  <book>/                        # 独立 Git
    01 源文件/                   # 冻结输入
    02 校对材料/参考|正文校订|版式|历史/
    03 制作工作区/
      epub/                      # 解包源；可保留原有子集字体
      fonts.json                 # 本书字体与锁
      交付记录.tsv               # 成功构建自动追加
      dist/book.epub             # 忽略；已有正式文件名可保留
      .pipeline/                 # 忽略；报告、收据、失败现场
    build.sh                     # 转发到手册统一构建器
    制作说明.md                  # 当前状态、目标阅读器、配方、待办与决策链接
```

## 字体库与配置

完整母版不重复提交到每本书源树。字体库先取 `EPUB_FONT_LIBRARY`，未设置时从书配置位置向上寻找共享的 `fonts/`；显式设置的目录不存在时失败，不悄悄回退。库内 `字体清单.tsv` 记录文件名、SHA、来源、许可与备注，书级 `THIRD_PARTY.md` 引用清单。正文和图片仍在书级记录来源。库文件缺失或 SHA 不符即停，不擅自换字体版本。

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

`target` 是 OPF/CSS 引用的包内路径，新书使用角色别名，旧书保留原名。`from` 只能是库内文件名。没有 `from` 的项继续使用源树字体，两者可混用；原有子集不入库。`usage` 省略时按全书字符集，显式 `css` 才按保守 CSS 角色范围。未知选择器会扩大字符集；活动内容、无法证明安全的样式或字符生成方式会拒绝 CSS 模式。可变字体只接受静态实例化；MATH 字体按原字节保留；原有加密边界不变。

母版原本缺字会打印 warning，提供缺字与文件定位样本，不阻断；新增缺字、空字形或 IVS/SVS 的子集回归仍阻断。子集源缺少后来新增的字不能自动恢复，需找到授权完整母版并更新配置。更换母版版本后，审阅来源、许可与字形变化，再显式更新锁：

```sh
epub-font inject --config '03 制作工作区/fonts.json' --update-lock
```

`sh build.sh fonts` 将锁定母版注入源树供编辑器预览，源树内相应母版路径必须被 Git 忽略；混用时仅忽略库字体路径，保留子集字体跟踪。没有预览注入时，编辑器显示回退字体；交付始终从库重新注入临时副本，不复用预览文件。

## 日常修改和构建

```sh
sh build.sh
sh build.sh fonts
sh build.sh clean
sh build.sh accept-dist
```

书根脚本只转发到 `${EPUB_HANDBOOK_ROOT:-$HOME/Developer/epub-handbook}/templates/book-starter/build.sh`，将书目录作为首参数；多册可各自将包含 `epub/` 的制作子目录传入。构建器兼容旧式复制到 `03 制作工作区/epub/` 的脚本与源树完整字体，但维护逻辑只更新手册这一份。配置优先制作区 `fonts.json`，再兼容书根配置；既有正式名用转发脚本的 `EPUB_OUTPUT_NAME` 保留。已有非 OEBPS 书可在核对资源集合后显式设 `EPUB_LEGACY_LAYOUT=1`，原样打包原布局；导入命令仍拒绝自动接入非 OEBPS 布局，不做猜测改名。

构建复制源 → 按锁注入完整字体 → 打包 → 子集与独立覆盖检查 → 导航审计 → 全项 redline → 原子替换 dist 并登记收据和交付记录。成功只保留固定交付包；失败保留 `.pipeline/build.*` 与报告，不替换已有 dist。`clean` 只清旧 `build.*`，默认保留最近一次现场，且遇到构建锁停止，不清整个 `.pipeline/`。

现有 dist 的 SHA 必须匹配 `.pipeline/dist-sha256`，缺收据或不匹配时不覆盖。编辑过 dist 后，先保存副本，做全项 redline 与人工 diff，把修改并回源；`accept-dist` 会重新生成并检查候选，要求所有包内成员内容与当前 dist 一致（不只比较正文红线），然后才登记收据。它不替代人工审核，不允许跳过源同步。

`交付记录.tsv` 自动记录时间、书源 commit（有修改带 `-dirty`）、手册 commit、dist SHA 和字体锁版本。`制作说明.md` 保留一屏当前状态、目标阅读器、使用配方、待办、已知差异和决策链接；旧长记录、带日期交接放 `02 校对材料/历史/`，不删除历史。依赖库与手册版本一起备份，复现时取记录中的提交。

## 阅读器与交付证据

配方首测保留完整 artifact/SHA、版本、逐场景截图并回写 reader matrix；书级交付保留每个配方在每个主要阅读器的截图和构建报告；中间调参可在确认交付后清理。静态检查与真实阅读器分开记录，没有实测不能写 pass。正文校订仍走 SPEC §10.1.1，含文决策留书级校对材料，被 gate 引用的文件与路径映射不得提前删除。

## Git 与第三方材料

书级 Git 不重写历史，不将书添加为主仓 submodule。迁移前先核对并保存已有改动、冻结旧构建基线；字体和目录调整单独提交，布局合并另作提交；迁移后全项 redline 与非字体成员逐项对比。来源或许可待核的内容只保留本地，不配置公开 remote、不发布。完整证据与真实书目信息留本地，经验回填主仓须按 AGENTS 脱敏。
