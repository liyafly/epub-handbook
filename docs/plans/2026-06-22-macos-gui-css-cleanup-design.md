# macOS GUI 原生 CSS Cleanup 写入设计

> 状态：用户已批准实施。范围仅为“执行 CSS Cleanup”；不提供任意 CSS 编辑器、Python bridge 或 skill/harness 调用。

## 目标

`HandbookMac` 在已有只读 inspection 的基础上，允许用户对已选择 EPUB 发起一次原生 CSS cleanup，并把结果写到用户通过 `NSSavePanel` 选择的全新 EPUB 文件。

## 交互与数据流

```text
NSOpenPanel 选择 EPUB
  → PackageInspector 只读检查
  → 启用“执行 CSS Cleanup…”
  → NSAlert 说明写入范围与红线
  → NSSavePanel 选择新输出路径
  → SwiftCLIService.normalizeCSS
       (before/staging workspace + native gates)
  → RunReport
  → GUI 显示成功输出路径或失败 gate / 错误
```

输入永远不覆盖。输出已存在时，native `Transaction` 拒绝 commit；GUI 显示失败原因，不做覆盖或删除。

## 调用边界

GUI 依赖 `EPUBCLI`，在进程内调用：

```swift
await SwiftCLIService.normalizeCSS(
    input: input,
    output: output,
    workspaceRoot: workspace
)
```

`EPUBCLI` 再调用 `EPUBStylesheets`、`EPUBValidation` 和 `EPUBRuntime` 的已有 native 实现。GUI 不执行 `epub-handbook-swift` executable，不调用 Python、`SKILL.md` 或 harness。

## 安全与 Sandbox

- 输入由 `NSOpenPanel` 授权，执行期间通过 security-scoped access 读取。
- 输出由 `NSSavePanel` 授权，entitlement 从 user-selected read-only 升级为 read-write；只写该用户明确选择的路径。
- workspace 放入 App Support 的 `CSSCleanup/<UUID>/`，保存 before、staging 与 transaction audit；不写入输入目录。
- 可写按钮只在 inspection 成功且保留选中输入 URL 后启用。

## 界面范围

保留单窗口 AppKit 表面。新增一个 disabled-by-default 的 `执行 CSS Cleanup…` 按钮，并将原本的 detail 文本框用于显示 native `RunReport` 的 gate、输出路径和错误消息。

不新增：CSS 文本编辑、selector 预览、手工 manifest 编辑、批量队列、popup apply 或任何未迁移 capability。

## 验证

1. `CSSCleanupRunPresentation` 单元测试覆盖成功、失败与建议输出名。
2. 既有 `SwiftCLIService.normalizeCSS` transaction 测试继续验证真实 artifact、redline 和 rollback。
3. `swift test`、Python parity、Tuist generation 和 `xcodebuild test` 必须通过。
4. GUI 工程只添加 `EPUBCLI` package dependency 和 Sandbox 的 user-selected read-write entitlement。
