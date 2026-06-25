# HandbookMac

> **⚠️ PARKED — 当前非焦点，不投入、不作依赖。** 执行逻辑以后向 `swift/` 收口。本目录保留为未来薄前端骨架，不删。不再为它新增功能；如有重逻辑滞留在 gui/，记 TODO 指向"迁往 swift/"，本次不执行迁移。

这是 AppKit-first 的 macOS 只读垂直切片：文件选择、security-scoped access 和 EPUB package inspection 都在此处完成；EPUB ZIP / XML 规则仍位于 `../swift` 的 Swift package。

```sh
mise install
cd gui
tuist generate
open EPUBHandbook.xcworkspace
```

生成的 workspace 和 project 不提交。首期只读；输入 EPUB 不会被 GUI 改写。后续所有 apply 流程必须先通过 `ExecutionPlan`、approval gate、红线校验和 transaction 输出目录。
