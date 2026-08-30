# Go capability 实现约定（迁移期内部参考）

> 本文是给移植 agent 的 API 速查与硬规则清单。硬约束原文在
> `docs/final/SPEC-go-architecture.md`（第一档，冲突以它为准）。

## 硬规则（违者 archguard 变红）

1. **禁止修改 `internal/archguard/`**。守卫红了改你的代码，不是改守卫。
2. `internal/caps/<name>` 之间**禁止互相 import**；需要上游结果走契约
   `requires`，由 pipeline 经 `Upstream` 注入。
3. `internal/**` **禁止包级 `var`**，两个例外：
   - `Err` / `err` 前缀的 error 哨兵；
   - 文件名恰为 `register.go` 的注册表文件（仅 init 期写入）。
   正则表、查找表、常量表都要放进 `register.go`。
4. `internal/scan/**` 的导出**函数**不得返回 `[]byte`，不得有
   `Marshal/Serialize/Render/Encode/Format/ToXML/ToHTML/ToXHTML/ToCSS/Dump/Emit`
   前缀。只产出 `[]editset.Edit` 或区间信息。
5. `archive/zip` 只能出现在 `internal/zipfs`；`os/exec` 只能出现在
   `internal/extern`；`os.Create/WriteFile/OpenFile/Rename/MkdirAll...`
   只能出现在 `internal/zipfs` 与 `internal/extern`。
6. capability 实现签名**固定**为三段式（扫描 → 应用 → 报告）：

```go
package structurenormalize

// Run 执行本 capability。禁止修改 b 之外的任何状态。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
    edits, err := scanPhase(b, p)          // 1. 只读扫描
    if err != nil { return report.Result{}, err }
    if err := b.Apply(edits); err != nil { return report.Result{}, err } // 2. 唯一写点
    return report.Result{...}, nil         // 3. 报告（不落盘）
}
```

7. 每个 capability 都要落 `testdata/<name>/` 下的 golden（SPEC §6.1 第 5 项）
   与单测；能对 Python oracle 做 parity 的必须做。

## 基础层 API 速查

### internal/book

```go
b, err := book.Open(path)          // 打开 EPUB（自动剔除目录项与 .DS_Store）
defer b.Close()
b.OriginalNames() []string         // 输入容器序（不含 mimetype 之外的重排）
b.Names() []string                 // 输出投影 = 原序(去删除) + 新增
b.Has(name) bool                   // 输出投影中是否存在
b.IsModified(name) bool
b.Original(name) ([]byte, error)   // 输入字节（惰性 + 缓存；红线依赖它不变）
b.Current(name) ([]byte, error)    // 当前字节
b.Apply(edits []editset.Edit) error // 唯一写入口
b.WriteTo(path) error              // 唯一落盘点（pipeline 调；cap 不许调）
b.ModifiedNames() []string
```

`Apply` 语义：
- `editset.Edit{Path, Offset, Length, Replacement}`（Replacement != nil）
  = 区间替换；Length=0 为插入；`[]byte{}` 为清空区间。
- 指向**不存在** entry 且 Offset==0、Length==0 → 新建该 entry。
- `editset.Delete(path)`（Replacement == nil）→ 删除整个 entry。
- 同批编辑自动按 (Path, Offset) 排序；重叠区间报 `editset.ErrOverlap`。

### internal/editset

```go
editset.Edit{Path string; Offset, Length int64; Replacement []byte}
editset.Replace(path, offset, length, repl) / Insert(...) / Delete(path)
editset.Apply(name, content, edits) ([]byte, error)  // 单 entry 的字节拼接
editset.Validate(edits) error
```

### internal/scan/opf（只读 OPF / container / encryption）

```go
opf.FindOPFPath(container []byte) (string, error)   // rootfile 需属容器命名空间
pkg, err := opf.Parse(opfPath, data)                // *opf.Package
pkg.Manifest []opf.ManifestItem{ID,Href,MediaType,Properties,ArchivePath}
pkg.Spine []opf.SpineItem{IDRef,Linear,Properties}
pkg.Metadata map[string][]string  // dc:title/creator/identifier/language 全文
pkg.Metas []opf.MetaValue{Name,Property,Content,Refines,Text}
pkg.ItemByID / ItemByHref / NavItem / CoverItem / NCXItem / OPFDir
opf.ParseEncryption(data) ([]opf.EncryptionRecord, error)
opf.LocalName(tag) / opf.IsExternalURI(uri) / opf.ResolveHref(opfPath, href)
```

### internal/scan/xhtml（只读标签定位）

```go
tag, ok := xhtml.FindOpenTag(content, "html", from) // 大小写不敏感，词边界
tag.Span / tag.Attrs / tag.SelfClose
tag.Attr(name) (xhtml.Attr, bool)   // 先原文名后 local 名
attr.ValueSpan / attr.Quote / attr.Value
span, ok := xhtml.FindCloseTag(content, name, from)
tags := xhtml.TagsIn(content, name)
span, repl, ok := xhtml.AttrEdit(content, tag, attr, value) // 值替换或末尾追加
```

### internal/scan/css（只读规则定位）

```go
css.Comments(text) / css.StripComments(text)
rules := css.Rules(strippedText)     // 对齐 Python RULE_RE 语义（@media 内层被捕获）
rule.Selector / rule.SelectorSpan / rule.Body / rule.BodySpan / rule.Span
decls := css.Declarations(rule.Body) // 区间相对 body
css.FontFamilyDecls(body)            // 对齐 FONT_FAMILY_RE
```

### internal/redline（六条红线 + in-process 比对）

```go
findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
    []string{"text","metadata","spine","anchors","cover","drm"},
    redline.Options{PathMap: renames, AllowList: []string{"*/nav.xhtml"}})
redline.CompareFiles(beforePath, afterPath, "all", redline.Options{}) // legacy 两文件协议
redline.AddPathMapping(m, from, to)  // 链式改名展开
redline.LoadPathMap(jsonBytes)       // structure_tool 报告形状
```

由 pipeline 在 caps 跑完后统一执行；cap 自身一般不调 Check。

### internal/report

```go
report.Result{
    Capability: "epub.structure.normalize",
    Status:     report.StatusComplete | StatusFailed | StatusApprovalRequired,
    Facts:      map[string]any{...},     // 信封 facts（键会被 pipeline 加上 cap 前缀）
    Findings:   []report.Finding{{Level:"error|warn|info", ID, Title, Detail, Location}},
    Events:     []report.Event{{Step, Status, Message}},
    NextCommands: []string{"epub run ..."},
    Renames:    map[string]string,        // 改名记录 → pipeline 汇给红线
}
report.MarshalLegacy(v)  // Python json.dumps(ensure_ascii=False, indent=2) 形状
report.PyFloat(0.8)      // Python float repr（1.0 → "1.0"）
```

### internal/pipeline（注册）

caps 不自己注册；把以下信息报告给维护者，由 `internal/pipeline/register.go`
统一挂接（避免多 agent 冲突）：

```go
// 注册形态（由维护者写入 pipeline/register.go）：
register("epub.structure.normalize", func(ctx context.Context, b *book.Book, args Args, up Upstream) (report.Result, error) {
    return structurenormalize.Run(ctx, b, structurenormalize.Params{...args/up 翻译...})
})
```

## legacy-report 约定（P2 parity）

- 每个迁移的 capability 支持隐藏参数 `legacy_report=true`（经 Args），
  把 Python oracle 的**原始 JSON 形状**放进 `Result.Facts["legacyReport"]`。
  JSON 生成必须用 `report.MarshalLegacy`（键序 = Python dict 插入序 → 用
  结构体字段序复刻；浮点数用 `report.PyFloat`）。
- 时间戳/随机路径等不确定字段：保持 Python 语义（每次运行变化），
  parity 比对时忽略；不要为了比对而伪造固定值。
- 退出码语义必须与 Python 脚本一致（信封换了，0/非0 含义不变）。
- Parity 测试模式参照 `internal/redline/parity_test.go`：构建 fixture →
  同一输入跑 Python 脚本（exec）与 Go 实现 → 逐字节比对。
```
