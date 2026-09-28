# 2026-09-27 样式预设阅读器证据

本记录只覆盖 T09 创建的三个**局部预设候选**，不代表默认 EPUB Style Demo 构建、其他版本、其他阅读器或生产书籍通过。

> **复核结论（2026-09-28）：** 旧 T09 候选文件目前无法定位，原 GUI 会话也没有保存可提交的截图或日志；Readest 的三个候选还与默认 demo 共用 identity。旧观察因此不再作为 `pass` 证据，reader matrix 中 9 条记录均已降为 `warn`。下方 GUI 表只保留历史观察，不代表当前可复现的 artifact 已由阅读器验收。

当前可重建候选来自源提交 `1133ba0253aaf892fc0b55d8f739a6f8c305f341`，先运行 `sh templates/epub-style-demo/build.sh`，再对指定页运行 `epub.typography.optimize`：

| 候选 | scope | 基础 demo SHA-256 | 候选 SHA-256 |
|---|---|---|---|
| `plain-cn` | `OEBPS/Text/01-body.xhtml` | `0b2c881251d51ff61edb00f5c17584c6cd8950125bf6bf7e8ab4bcf03b9825fc` | `3a553e501b37fe75d1c3d00ff6fdacfe6722871bf919058fa1c70d37c96068e5` |
| `poetry-cn` | `OEBPS/Text/29-poetry.xhtml` | 同上 | `e4623f453ce5e0b34360f54123462f0eb0933a8e824520cd0ccc128c2bd7d0aa` |
| `fiction-en` | `OEBPS/Text/18-english-fiction.xhtml` | 同上 | `c51b82efdb359ffb6963b6da5e5bf9029464216bb0681ff9b0f9c1ee92bf9387` |

这些 SHA 只标识可重建的生产候选；要做阅读器复测，必须按 [demo README 的 identity 规则](../../../templates/epub-style-demo/README.md#阅读器测试副本的-identity) 创建临时唯一 identity 副本，并把副本 SHA 与截图或日志绑定。当前生成过程的静态 redline 为 0 findings；基础 demo nav audit 为 0 errors / 1 warning。静态结果不构成阅读器证据。

## 候选和静态门禁

共同基线 `base.epub` SHA-256：`1376692a1e405014d38878bf9db600797efa8d0658c0ab23b0979ee87d0b2790`。

| 候选 | 应用范围 | SHA-256 |
|---|---|---|
| `plain-cn.epub` | `OEBPS/Text/01-body.xhtml` | `b4d409009b84551ef820e1d5870588f708e2ccacdd8bacd8c410fb8eae58b6dc` |
| `poetry-cn.epub` | `OEBPS/Text/29-poetry.xhtml` | `f3bec46ba41db73897fa1743e3601d26d9867916b2d63a55c57d376da18e7f32` |
| `fiction-en.epub` | `OEBPS/Text/18-english-fiction.xhtml` | `acf3b4ba66ff2fe941f9edc9ab8eee7369fbcd053d81af40a946b9c9e49b32ef` |

三个候选均完成 nav audit、`epub.style.demo.maintain`、`epub.notes.popup.normalize` 和 `epub redline --check all`。Maintain 报告 0 errors；popup violations 为 0；redline 无 finding。Nav audit 为 0 errors、1 warning：未改动的 `OEBPS/Text/07-font-family-order.xhtml` 有一个空段落。Maintain 提示 EPUBCheck 由 CI 执行，本机没有将它记为通过。

整理证据前，从当前工作树重建默认 demo 得到 SHA-256 `1376692a1e405014d38878bf9db600797efa8d0658c0ab23b0979ee87d0b2790`，与 T09 基线完全相同。当前构建的 maintain 为 0 errors、popup 为 0 violations、nav audit 为 0 errors / 1 warning，针对基线的全项 redline 通过。这验证本次只改了矩阵和文档；候选各自的 gate 结果仍以上述候选记录为准。

GitHub Actions 的 `Build EPUB Demo` 在基线代码提交 [`3a0ca1e`](https://github.com/liyafly/epub-handbook/commit/3a0ca1e22efe15a683a8f1132cd79385c6a44546) 上成功，构建并检查了同三种预设及对应页面范围的 CI 候选。该运行证明同代码与范围的 CI 生成候选通过 EPUBCheck；CI 文件与本地文件 SHA 未做同一性声明。后续 T09 文档提交的专属 CI 结果见任务执行记录。

## 历史 GUI 阅读器观察（当前不计为 pass）

每个 pass 都绑定上表中的完整 SHA。应用只记录实际打开、跳转和观察到的页面；没有把转换成功或静态检查当作视觉结果。会话中检查的 UI 截图未保存为可提交的 PNG，故本文件以阅读器观察记录为证据，不声称仓库内有截图附件。

| 候选 | 阅读器 / 版本 | 实际观察与设置 |
|---|---|---|
| `plain-cn.epub` | Apple Books macOS `9.0 (6655)` | 默认正文、段落和图片显示正常；默认与 +1 字号可读；检查深色外观和自定义宋体主题后无可见裁切。恢复 Light/Original 和默认字号。 |
| `plain-cn.epub` | Readest `0.12.10 (20260922.053953)` | 从精确本地路径导入成功并跳转普通中文正文；标题、段落、引用、图片显示正常。它与基础 demo 共用 EPUB identity，Readest 复用同一书库条目，因此该 pass 不验证同 UUID 导入隔离。 |
| `plain-cn.epub` | Kindle Previewer 4.0.1（当前安装包版本） | Kindle e-reader portrait、Song 字体；转换完成后跳转中文正文，字号 4 与 12 抽查无可见裁切，最后恢复字号 4。 |
| `poetry-cn.epub` | Apple Books macOS `9.0 (6655)` | 跳转诗歌页 68–69；多节间距和长行自然折行可见；+1 字号仍自然重排，无可见裁切；恢复默认字号与 Light/Original。 |
| `poetry-cn.epub` | Readest `0.12.10 (20260922.053953)` | 诗题、说明、多节和长行可见；字号从 16 增至 22 后由 1 页自然重排为 2 页，无可见裁切；恢复 16。冷退出再启动后再次打开并显示目标章节。候选与基础 demo 共用 identity，书库去重范围同上。 |
| `poetry-cn.epub` | Kindle Previewer 4.0.1（当前安装包版本） | Kindle e-reader portrait、Song 字体；默认字号 4 下多节分隔可见；字号 12 下长行自然折行并延续到后续诗节，无可见裁切；恢复 4。 |
| `fiction-en.epub` | Apple Books macOS `9.0 (6655)` | 章题、T 首字、手写体浮动 A、章首及行内插图、图注均可见；字号 +1 后正文自然重排，无可见碰撞或裁切；恢复默认字号与 Light/Original。 |
| `fiction-en.epub` | Readest `0.12.10 (20260922.053953)` | 本地导入后跳转英文小说正文；章首图、T 首字、手写体浮动 A、正文和行内图注可见。冷退出再启动后再次打开目标章节。本轮 Readest 未单独做大字号测试；候选与基础 demo 共用 identity，书库去重范围同上。 |
| `fiction-en.epub` | Kindle Previewer 4.0.1（当前安装包版本） | Kindle e-reader portrait、Song 字体；转换完成后跳转英文正文，T 首字与浮动手写体 A 可见且文字自然环绕；字号 12 抽查无可见裁切，恢复 4。 |

## 结论边界与待办

- 原记录声称九个 `reader × candidate` 组合完成 GUI 检查；因候选文件和截图/日志未保留，这些只是历史观察，不能作为当前 `pass`。
- Readest 中三个候选与基础 demo 共用 EPUB identity，重导入可能复用书库条目。已确认每次选取对应精确本地路径并出现导入成功/目标内容，但这组测试不证明同 UUID 缓存隔离。
- `fiction-en` 未在 Readest 单独做字号变化；Apple Books 与 Kindle Previewer 已覆盖大字号回归。未将其写成 Readest 字号通过。
- Thorium 未实测；默认 demo（不应用局部预设）的这三个场景也未由本记录验收；长诗持续分页、iOS Apple Books、其他 Kindle 设备 profile 和用户字体切换矩阵仍待后续任务。
- 本机未运行 EPUBCheck。对应 CI 工作流在精确基线代码提交上通过了同 preset/scope 的 CI 候选检查；最终文档提交的 CI 需另核验并登记。
