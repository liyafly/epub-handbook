# SPEC-Go 现代编程指南（Go 1.27）

> 本文件是 Go 实现的编程风格与 API 选择指南，不取代架构、安全、契约或用户授权。
> 当前仓库 `go.mod` 的 `go` 指令是 `1.27`。修改 Go 前仍须按本文件 §2 重新读取目标文件适用的规则。

## §0 优先级与适用边界

现代化的目的，是让新代码更清楚、更容易验证、更少分配；不是为了把所有旧写法批量替换一遍。发生冲突时按以下顺序裁决，序号越小优先级越高：

1. 用户对正文、元数据、文件结构和输出的明确授权，以及 DRM、路径和输入安全边界。
2. [`SPEC-go-architecture.md`](SPEC-go-architecture.md) 的依赖方向、十条不变式、单次落盘、`archguard` 和任务模板。尤其是字节透传、禁止整文档序列化、红线闭包和报告 schema。
3. EPUB 正文不变 gate、`text` / `metadata` / `spine` / `anchors` / `cover` / `drm` 红线，以及 lossless byte-range edit 约束。
4. `contracts/` 与 `contracts/schemas/` 的机器契约和已有 JSON/退出码行为。对外 wire 行为优先于内部类型的漂亮写法。
5. 本文件的现代 Go 规则和工程建议。

因此，某条现代规则若会改变 EPUB 字节、文本哈希、资源顺序、JSON 信封、字段存在性、错误/退出码或公开 API，必须暂停机械改写；记录理由，补 golden、schema、红线或兼容性测试后再决定。架构守卫失败时改实现，不改 `internal/archguard/`。

本指南覆盖 JetBrains 上游固定版本列出的全部规则。规则按 `since_version` 不早于目标模块版本才可使用；“强制”表示新写或正在编辑的代码匹配该模式时默认采用，“条件”表示必须先满足条目中的语义条件。既有代码不是无授权的批量重构目标；迁移还须通过本节列出的兼容性 gate。

## §1 来源、版本与计数核对

一手事实来源是 [JetBrains/go-modern-guidelines](https://github.com/JetBrains/go-modern-guidelines)，固定到 commit [`91a30b36f05bb6424bd77e9817811c0e9c003aa2`](https://github.com/JetBrains/go-modern-guidelines/tree/91a30b36f05bb6424bd77e9817811c0e9c003aa2)：

- 规则 ID、版本、分类、modernizer 标记和核心描述：`internal/guidelines/guidelines.json`。
- 分类目录和影响等级：`FEATURES.md`。
- agent 的版本检测、`list` / `explain` 调用顺序：`plugin/skills/use-modern-go/SKILL.md`。
- 上游许可证：[`LICENSE`](https://github.com/JetBrains/go-modern-guidelines/blob/91a30b36f05bb6424bd77e9817811c0e9c003aa2)，Apache License 2.0。

在该固定 commit 中，`guidelines.json` 和生成的 `FEATURES.md` 各有 **54** 个唯一 guideline ID。任务描述所称“55 个”与固定来源的可审计事实不一致；本文件完整列出这 54 个 ID，不凭空补造一个上游规则。若上游 commit 改变，必须重新计数、核对差异并更新本表；不能把未来或未在该 commit 中的规则混入当前指南。

## §2 修改 Go 前的版本门禁

每次写、改、修、重构 Go 文件，都执行以下流程：

1. 找到文件所属模块的 `go.mod`；若由 `go.work` 管理多个模块，确认实际模块和工作区约束。读取 `go` 指令，不以本机默认 toolchain 或模型记忆代替。当前仓库为 `go 1.27`。
2. 使用上游 skill 提供的 CLI，对将要编辑的文件运行完整列表：

   ```sh
   sh "<upstream-skill-wrapper>" list --file-path path/to/file.go
   ```

   `<upstream-skill-wrapper>` 是上游 `use-modern-go` skill 提供的 shell wrapper；不要把 wrapper 或其缓存复制进本仓库。已确认目标版本时也可运行 `list --go-version 1.27`。必须读完全部输出；禁止用 `head`、`tail`、`grep`、`sed` 或其它截断管道隐藏旧版本规则。
3. 只采用 `since_version <= go.mod` 目标版本的规则。目标版本较低时，不因本机能切换 toolchain 就把新 API 写进模块；必要时采用兼容写法或 build tag。
4. 对可能适用但因 wire 行为、API、架构、性能或编译条件而不采用的 ID，先用 `explain <id>` 读完整细节，再在 review 或变更说明中写明跳过理由和验证证据。不要无 ID 地调用 `explain`。
5. 修改后执行 §7 的格式、编译、测试、竞态和架构守卫检查；若规则变更影响 JSON、EPUB 或外部工具，再执行 §3–§6 的对应 gate。

版本只是“能否使用”的门槛，不是“必须重写”的理由。当前所有 54 条从版本角度可见，但每条仍受下表的语义条件和 §0 优先级约束。

## §3 上游规则总表（54 条）

表中 `M` 是强制，`C` 是条件适用；`modernizer` 仅表示上游是否有自动 modernize analyzer，不能替代人工判断。

### Types（4）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `generic_methods` | 1.27 | — | C | 当操作自然属于某个类型时，优先放到该类型的泛型方法命名空间；包级泛型函数仍用于不属于单一接收者的操作。公开方法集、接口实现和调用点会变化，见 §4.4。 |
| `promoted_field_literals` | 1.27 | ✓ | M | 具名结构体字面量可直接设置嵌入结构提升的字段；不要同时写提升字段和产生提升的嵌入字段，指针嵌入路径不适用。仅在目标 toolchain 确认支持时使用。 |
| `reflect_type_for` | 1.22 | ✓ | M | 需要泛型类型的 `reflect.Type` 时用 `reflect.TypeFor[T]()`，不再用空指针取 `.Elem()` 的技巧；不改变反射结果和 nil 语义。 |
| `any` | 1.18 | ✓ | M | 新代码或正在编辑的无约束值、类型约束使用 `any`，不再写 `interface{}`；它是别名，通常不改变 wire，但不要借机改字段、标签或 JSON 类型。 |

### JSON（2）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `json_v2` | 1.27 | — | C | 新 JSON 代码可评估 `encoding/json/v2`；现有 `encoding/json` 不得机械迁移。v2 的非法 UTF-8、重复对象名、nil slice/map 编码和类型错误行为更严格，任何采用都必须走 §4 的 wire 迁移 gate。 |
| `json_omitzero` | 1.24 | ✓ | C | Go 零值代表“字段不存在”时才在 JSON tag 使用 `omitzero`（如 `false`、`0`、零时间、零结构体）；要表达 JSON 的空字符串、空 slice/map 等空值语义，继续使用 `omitempty`。改 tag 就是 wire 变更，必须有 golden。 |

### Strings（7）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `strings_bytes_cut_last` | 1.27 | ✓ | M | 围绕最后一个分隔符拆分时用 `strings.CutLast` / `bytes.CutLast`，不要手写 `LastIndex`、边界检查和切片算术；保留原有“未找到”分支语义。 |
| `strings_split_seq` | 1.24 | ✓ | C | 只需逐项遍历 split/fields 结果时用 `strings.SplitSeq`、`FieldsSeq` 或 bytes 对应版本，避免先分配完整 slice；需要随机访问或持久化列表时才物化。 |
| `bytes_clone` | 1.20 | ✓ | C | 确实需要独立 byte slice 时用 `bytes.Clone`；注意它是浅字节复制并保留 nil 行为，不要用它掩盖不必要的复制。 |
| `strings_cut_prefix_suffix` | 1.20 | ✓ | M | 同时需要裁剪结果和是否匹配时用 `strings.CutPrefix` / `CutSuffix`，避免重复 `HasPrefix`/`HasSuffix` 检查；前缀/后缀本身是协议数据时保持原样。 |
| `bytes_cut` | 1.18 | ✓ | M | 按第一个分隔符拆 byte slice 用 `bytes.Cut`，保留 before/after/found；不把它用于需要修改原始字节的场景。 |
| `strings_clone` | 1.18 | — | C | 从大字符串中保留很小片段且不应让大 backing memory 存活时用 `strings.Clone`；不要为普通短期字符串盲目复制。 |
| `strings_cut` | 1.18 | ✓ | M | 按第一个分隔符拆字符串用 `strings.Cut`，不要手写 `Index` 加切片；检查 found 后再作协议决策。 |

### Collections（18）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `maps_keys_values_iter` | 1.23 | — | C | 不需要 slice 时直接 range `maps.Keys` / `maps.Values` iterator；map 顺序不稳定，生成 EPUB、报告或 golden 前必须先建立明确顺序。 |
| `slices_collect` | 1.23 | — | C | iterator 结果确实需要 slice 时用 `slices.Collect`；可以流式消费就不要物化。 |
| `slices_sorted` | 1.23 | — | C | 需要从 iterator 收集并排序时用 `slices.Sorted`；特别适合把 map key 转成 deterministic 输出，排序键和 tie-break 必须是契约的一部分。 |
| `min_max` | 1.21 | ✓ | M | 单纯选择有序值的较小/较大者用内置 `min` / `max`，不写比较分支；含错误、空值或副作用的逻辑不要强行压成表达式。 |
| `clear` | 1.21 | — | C | 清空 map 或把 slice 元素归零时用 `clear`；slice 的长度和容量保持不变，不能把它当作截断或释放 backing array。 |
| `slices_contains` | 1.21 | ✓ | M | 只判断可比较元素是否存在时用 `slices.Contains`；若匹配条件更复杂，使用 `slices.IndexFunc`。 |
| `slices_index` | 1.21 | — | C | 查找可比较元素下标用 `slices.Index`，未找到返回 `-1`；调用者若把 `-1` 作为错误必须显式处理。 |
| `slices_index_func` | 1.21 | — | C | 按字段、派生值或其它谓词查找用 `slices.IndexFunc`；谓词应保持无副作用、可测试。 |
| `slices_sort_func` | 1.21 | — | C | 对结构体或自定义排序用 `slices.SortFunc`；优先 `cmp.Compare`，补足相等项 tie-break 以保证输出确定，不用 `sort.Slice` 闭包重复索引。 |
| `slices_sort` | 1.21 | ✓ | M | 有序元素的普通排序用 `slices.Sort`；调用前确认排序确实允许原地改变 slice 和其观察者。 |
| `slices_max_min` | 1.21 | — | C | 非空有序 slice 求极值用 `slices.Max` / `slices.Min`；空 slice、用户输入或自定义比较必须保留显式错误/分支。 |
| `slices_reverse` | 1.21 | — | C | 原地反转用 `slices.Reverse`；先确认输入顺序不是需要保留的 EPUB/报告契约。 |
| `slices_compact` | 1.21 | — | C | 只需移除相邻重复值时用 `slices.Compact`；它不是全局去重，且会原地复用 slice。 |
| `slices_clip` | 1.21 | ✓ | C | 需要阻止未来 append 复用隐藏容量或避免保留过大 backing array 时用 `slices.Clip`；不以此替代生命周期设计。 |
| `slices_clone` | 1.21 | ✓ | M | 需要浅复制 slice 时用 `slices.Clone`，保留 nil slice 行为；元素指向的对象仍需另行决定是否深拷贝。 |
| `maps_clone` | 1.21 | — | M | 需要浅复制 map 时用 `maps.Clone`，保留 nil map 语义；值为指针或 slice 时不要误称深复制。 |
| `maps_copy` | 1.21 | ✓ | M | 把一个 map 的 entry 复制到另一个 map 用 `maps.Copy`，明确它会覆盖目标同键值；输出前仍不可依赖 map 迭代顺序。 |
| `maps_delete_func` | 1.21 | — | C | 按谓词删除 map entry 用 `maps.DeleteFunc`；谓词不得依赖不确定的遍历顺序来产生可观察结果。 |

### Context（4）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `testing_t_context` | 1.24 | ✓ | C | 测试中的辅助工作应随测试生命周期停止时用 `t.Context()`；生产代码仍由调用者传入 context，不能用测试 context 代替真实取消契约。 |
| `context_after_func` | 1.21 | — | C | 只为等待 `ctx.Done()` 做清理 goroutine 时用 `context.AfterFunc` 和 stop 函数；认真处理取消竞态和清理是否已开始。 |
| `context_timeout_deadline_cause` | 1.21 | — | C | 调用方需要区分超时、截止时间和其它取消原因时使用带 cause 的 timeout/deadline，并用 `context.Cause` 读取；仅需通用取消状态时不增加复杂度。 |
| `context_cancel_cause` | 1.20 | — | C | 取消需要携带具体错误时用 `context.WithCancelCause` / `context.Cause`；cause 是诊断/控制流信息，不能替代返回 error 或掩盖真实失败。 |

### Errors（3）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `errors_as_type` | 1.26 | ✓ | C | 要匹配具体错误类型时用 `errors.AsType[T](err)` 取得类型化值和布尔结果；迁移前确认目标 toolchain、调用者和错误包装行为。 |
| `errors_join` | 1.20 | — | C | 需要报告多个独立失败时用 `errors.Join`；它保留 `errors.Is` / `errors.As` 匹配，零个非 nil 错误时结果为 nil，顺序仍应在报告层确定。 |
| `errors_is` | 1.13 | — | M | 判断 sentinel 或包装链中的错误用 `errors.Is`，不要用 `==`；返回给 agent 的 finding 和退出码仍需保持契约语义。 |

### Fmt（1）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `fmt_appendf` | 1.19 | ✓ | C | 已在累积 byte slice 且不需要中间 string 时用 `fmt.Appendf`；格式化报告仍须由 `internal/report` 构造并通过 schema，不能用性能优化绕过 wire 检查。 |

### HTTP（1）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `http_servemux_patterns` | 1.22 | — | C | 新 HTTP handler 可用带方法和命名 wildcard 的 `ServeMux` pattern，并用 `r.PathValue` 读取参数；既有路由迁移会改变匹配/冲突优先级，必须做方法、路径、状态码和参数的 integration golden。 |

### Loops（2）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `range_over_int` | 1.22 | ✓ | M | 纯粹遍历 `0` 到 `n-1` 用 `for i := range n`；需要非零起点、自定义步长、可变上限或错误处理时保留普通 `for`。 |
| `loopvar_capture` | 1.22 | ✓ | M | 每次迭代已有独立 loop variable，闭包、goroutine、defer 或取地址前不要无意义地再复制；若要指向原 slice 元素，明确使用 `&slice[i]`。 |

### Sync（4）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `sync_waitgroup_go` | 1.25 | ✓ | M | goroutine 生命周期恰好由 `WaitGroup` 追踪时用 `wg.Go`，让 Add/Done 成对管理；函数的 panic、取消和错误汇总仍需显式设计。 |
| `sync_once_func` | 1.21 | — | C | 一次性初始化或幂等清理函数用 `sync.OnceFunc`；不要把它用于需要重试或携带每次调用参数的操作。 |
| `sync_once_value` | 1.21 | — | C | 安全地惰性计算并缓存一个值时用 `sync.OnceValue`；检查失败值、panic 和取消是否适合永久缓存。 |
| `atomic_types` | 1.19 | ✓ | M | 使用 `atomic.Bool`、`atomic.Int64`、`atomic.Pointer[T]` 等类型化原子值，避免无类型原子函数；原子值开始使用后不可复制，所有访问都保持原子。 |

### Testing（1）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `testing_b_loop` | 1.24 | ✓ | M | benchmark 主循环用 `b.Loop()`，让 testing 管理迭代；计时范围、分配统计和 fixture 构造仍须明确分离。 |

### Time（3）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `time_tick_gc` | 1.23 | — | C | 简单永久循环且不需要停止/重置时可用 `time.Tick`；需要 `Stop`、`Reset`、context 取消或有限生命周期时使用 `time.NewTicker` 并释放。 |
| `time_until` | 1.8 | — | M | 求截止时间剩余时长用 `time.Until(deadline)`；保留负值/过期语义，不能把它当作安全授权检查。 |
| `time_since` | 1.0 | — | M | 求已流逝时长用 `time.Since(start)`；测试和 deterministic 输出不要把当前时间直接写进 golden。 |

### URL（1）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `url_clone` | 1.27 | — | C | 复制 `url.URL` 或 `url.Values` 时用其 `Clone` 深复制表示，避免漏字段或共享可变 slice；若 URL 出现在报告/EPUB 引用中，仍要保持规范化和编码结果。 |

### Utilities（3）

| ID | since | modernizer | 适用 | 本项目综合规则 |
|---|---:|:---:|:---:|---|
| `stdlib_uuid` | 1.27 | — | C | 面向 Go 1.27 的新代码优先使用标准库 UUID 能力；已有第三方库只有在明确迁移、目标版本和行为验证后替换。EPUB `urn:uuid:`、JSON 文本/二进制表示和大小写均是潜在 wire 数据，见 §4.3。 |
| `new_expression` | 1.26 | ✓ | M | 仅为取得值的指针时用 `new(value)`，尤其是结构体字面量的指针字段；保留真正增加校验、命名或行为的 helper。 |
| `cmp_or` | 1.22 | — | C | 纯 fallback 链使用 `cmp.Or`；所有参数会先求值，所以有 I/O、昂贵计算、错误或副作用时保留显式惰性分支。 |

## §4 JSON、API 与 wire compatibility 边界

本项目的 JSON 不是内部日志，而是 agent、CLI、测试、契约和未来 GUI 之间的机器接口；EPUB 中的 OPF、XHTML、CSS 和 ZIP entry 字节同样是 wire 数据。下列边界独立于风格表，优先级高于 `json_v2`、`json_omitzero`、`stdlib_uuid`、`http_servemux_patterns` 和 `generic_methods` 的便利性。

### 4.1 `encoding/json` 与 `encoding/json/v2`

- 当前已有 `encoding/json` 的 producer/consumer（包括 `cmd/epub`、`internal/pipeline`、`internal/report`、契约读取和测试）保持不变，除非任务明确授权迁移。一个能编译的 import 替换仍可能改变 wire 行为。
- 新 JSON 代码在目标 toolchain 确实提供 v2 且契约允许时，才评估 `encoding/json/v2`。v2 会拒绝非法 UTF-8 和重复对象名，nil slice/map 默认编码为空数组/对象，并对无效 JSON 相关 Go 类型更严格；这些都可能让旧消费者、输入 fixture 或 golden 发生变化。
- 显式迁移必须先盘点 producer、consumer、schema、退出码和 fixture；先以 v1 兼容选项（上游建议的 `jsonv1.DefaultOptionsV1()`）保持旧 wire 形状，再逐项移除或覆盖选项。后写的选项优先，但每一项都要说明原因。
- 迁移 gate 至少包含：既有输出的逐字节 golden、旧输入的接受/拒绝矩阵、schema 校验、无效 UTF-8/重复 key/nil 容器样本、报告退出码和 CLI 调用方检查；必要时提供双读/双写过渡期。没有这些证据不得把 v1 全仓替换为 v2。
- `internal/report` 仍是对外 JSON 的唯一构造处；正式输出遵守 `contracts/schemas/v2/envelope.schema.json`，迁移期的 v1/legacy 形状按架构 SPEC 保留，不在 capability 内私自发明信封。

### 4.2 `json_omitzero` 与 tag 语义

`omitempty` 与 `omitzero` 不是同义词。`false`、`0`、零 struct、零 time 是否省略，会影响字段是否存在；空字符串、空 slice/map 是否省略，又是另一种 JSON 语义。新增或修改 tag 前先写出“零值、空值、缺失”的契约表，再更新 schema、序列化 golden 和反序列化兼容测试。禁止全仓正则替换 `omitempty`。

### 4.3 UUID

标准库 UUID 规则只针对新代码的依赖和实现选择，不授权更换既有 UUID 库、生成算法或格式。迁移时必须锁定：解析可接受的文本形式、生成版本、`urn:uuid:` 前缀、大小写/连字符、JSON 编码、OPF `dc:identifier` 和任何报告字段。用真实 OPF/XHTML fixture 与 JSON golden 验证；正文、元数据和 entry 字节差异仍受 redline gate 约束。

### 4.4 Generic methods

`generic_methods` 可能改变类型的方法集、方法表达式、接口满足关系、导出 API 和调用点。只有当操作确实属于 receiver 且目标 Go toolchain 编译确认时才采用；包级 helper 若可用于多个不相关类型则保留。迁移公共 API 时先加调用点编译测试和 API/golden 记录，不能因为“更现代”就改变 capability 契约或跨包依赖方向。

### 4.5 `net/http.ServeMux`

method-aware pattern、命名 wildcard 和 `PathValue` 会改变路由匹配、冲突优先级、方法限制、尾斜杠处理和参数提取。它只适用于新 handler 或获得明确迁移授权的 handler。迁移前后用表格覆盖 HTTP method × path × expected status × wildcard values，并测试未匹配路径和错误 envelope；不要把路由升级混入 EPUB capability 的无关重构。

### 4.6 JSON/EPUB golden 的分层

内部结构可采用现代集合/错误/字符串 API，只要不改变可观察结果；一旦结果进入下列边界，必须 golden：

- `contracts/schemas/v1` / `v2` 的字段、tag、缺失与 null/empty 差异；
- report 的 `schemaVersion`、`status`、`facts`、`findings`、`events`、`nextCommands` 与退出码；
- OPF metadata、spine、nav/NCX、anchors、cover、DRM 标记、XHTML/CSS byte ranges；
- UUID、URL、外部工具输出和 HTTP 路由结果。

map 和并发结果不得凭迭代时序进入输出。需要稳定列表时先明确排序键并使用 `slices.Sorted` 或等价的确定性排序；“JSON 恰好能解析”不能代替字节级或 schema 级兼容证据。

## §5 本项目 Go 工程规则

### 5.1 Context 必须贯穿

- 对外命令、pipeline、capability、扫描大文件和外部 provider 的可取消入口，统一把 `context.Context` 作为第一个参数；与架构模板一致使用 `Run(ctx context.Context, b *book.Book, p Params)`。
- 调用链把同一个 context 继续向下传递；不要在库代码中用 `context.Background()` 或 `TODO()` 覆盖调用者的取消、deadline 和 cause，也不要把 context 存进长期结构体。
- 在 stage 边界、ZIP entry 读取/扫描循环、网络或 provider 调用前后检查 `ctx.Err()`；取消应停止后续 stage，且不能留下半成品 EPUB 或误报成功报告。
- 测试中需要绑定测试生命周期时用 `t.Context()`；需要区分取消原因时再使用 `WithCancelCause` / `WithTimeoutCause`，不要把 context error 当作唯一业务错误。

### 5.2 外部进程边界

所有外部进程只能由 `internal/extern` 负责，capability 不得 import `os/exec`。每次调用：

- 使用 `exec.CommandContext(ctx, path, args...)`，参数逐项传入，不拼 shell 命令；尊重取消和 deadline。
- 对 stdout、stderr、输入文件和进程运行时间设上限，避免 `io.ReadAll` 把无界 provider 输出读入内存；工具缺失返回明确 `ErrToolMissing`，由调用方决定跳过或失败。
- 校验 executable 路径、工作目录和输入路径；不要把用户提供的 EPUB 路径拼进 shell 或日志中的可执行文本。
- provider 失败要带上工具名、stage 和可诊断摘要并用 `%w` 包装根因；不因第三方进程异常 panic，也不把未验证输出写入最终 EPUB。

### 5.3 Deterministic outputs

重复输入、相同参数和相同 provider 版本应产生可比较的报告和尽可能稳定的 EPUB：

- 不直接依赖 map、并发完成次序、文件系统遍历顺序、当前时间或随机 UUID 生成报告字段、manifest/spine 顺序或 golden 内容。
- 从 map 产生列表时显式排序；并发任务收集后按稳定 key 汇总。排序如果影响原输入的有意义顺序，先把“保留输入顺序”写成契约，而不是盲目排序。
- 时间、临时路径、随机值只在契约明确要求时产生，并在测试中注入 clock/ID source 或从比较中剥离；不得污染正文不变 gate。
- ZIP 透传、压缩方法、entry 顺序和元数据的变化必须符合 `SPEC-go-architecture.md` 的字节透传规则；“看起来一样”不等于允许改写。

### 5.4 Bounded I/O 与 lossless 编辑

- 用户提供的 ZIP entry、路径、XML/CSS 片段、provider stdout/stderr 都是不可信输入。对 entry 数量、单个 entry 大小、解压膨胀量、路径长度和累计内存设置边界，采用分块或带上限 reader；超限返回可判断错误。
- `internal/scan/*` 只扫描并产出 `[]editset.Edit{Offset, Length, Replacement}`，不构造整文档 DOM 往返，不导出 `Marshal` / `Serialize` / `Render` 等整文档输出。
- 未命中的 entry 使用 `zip.Writer.Copy`/raw 透传；不要为了套用现代 API 先解压、重序列化、重排或改变实体、命名空间、DOCTYPE、空白和编码。
- 解析失败、短读、越界 offset、编辑冲突和不可信路径都返回错误；先校验再创建 `Edit`，禁止靠 panic 表示用户输入无效。

### 5.5 事务写盘与单次输出

- 单输出 capability 的一次 pipeline 运行只写一次最终 EPUB；`split` 等契约明确声明的多输出 capability 必须先在内存中完成全部投影与逐产物验证，再把整组产物作为一个目录事务提交。中间 stage 只传递 `book.Book`、editset 和 report，不暴露半成品。
- 由 `internal/zipfs` 建立同目录临时输出，成功完成并校验后再原子 rename 到明确 output path；失败时关闭并清理临时文件，不能让调用者看到伪成功或半成品。临时文件不是可被下一个 stage 消费的中间 EPUB。
- 不在输入原件上写入，不把用户唯一底本当作输出；输出路径必须经过参数校验且不能无意覆盖输入。
- 只有架构许可的磁盘边界（`internal/zipfs`、`internal/extern`）写文件；report 和 capability 不自行 `os.Create`/`os.WriteFile`。
- 写盘前后记录输入/输出 SHA-256、entry 透传/编辑事实和错误状态；报告成功只在 rename 与必要 redline/schema 检查完成后返回。

### 5.6 Error wrapping、退出码与 panic 边界

- 每一层给错误加上下文（capability、stage、entry/path、操作），保留根因：`fmt.Errorf("read %s: %w", name, err)`。调用者用 `errors.Is` 判断 sentinel，用 `errors.AsType` 或 `errors.As` 判断类型。
- 多个相互独立的错误可用 `errors.Join`，但对外 findings 必须稳定排序、明确 level/id，并遵守统一 envelope。
- 用户输入、损坏 ZIP、非法 XML/CSS/JSON、缺文件、超限和 provider 输出都走返回 error；不要 `panic`。只对不可恢复的程序员不变量使用极窄的初始化断言，并且不得成为用户输入路径。
- 退出码保持架构契约：0 为无 error finding 的成功，1 为失败或 error finding，2 为 `approval-required`，3 为用法错误。现代 error API 不得静默改变这些含义。

### 5.7 测试、fuzz 与 benchmark

- 新行为以表驱动单测、最小 fixture 和 golden 覆盖；对 JSON/CLI 用精确字节和 schema 双重断言，对 EPUB 用文本块哈希、redline、entry CRC/压缩属性和必要的人工 diff 证据。
- 对 ZIP 路径穿越、entry 截断/膨胀、XHTML/CSS 边界、UTF-8、JSON 重复 key/nil 容器、offset edit 冲突和取消路径添加 fuzz；fuzz target 必须有时间/内存边界，不把任意输入写到仓库或覆盖原件。
- 性能敏感路径使用 benchmark；主循环用 `b.Loop()`，fixture 构造不计入被测区段，记录分配和大 EPUB 的 bounded I/O 行为。
- 并发代码覆盖取消、错误汇总、只运行一次和 race；输出测试连续运行两次并比较稳定字段，避免偶然通过。

## §6 常见裁决与禁止的机械重构

### 6.1 可以直接采用的情形

代码正在编辑、目标版本满足、只影响内部实现且语义不变时，通常直接采用 `any`、`range_over_int`、`strings.Cut`、`slices.Contains`、`maps.Clone`、类型化 atomics、`errors.Is`、`time.Since` 等清晰替代。仍须跑格式、测试和 archguard。

### 6.2 必须先做兼容性审阅的情形

下列操作不能以 modernize analyzer 或全局替换完成：

- `encoding/json` → `encoding/json/v2`；任何 `omitempty` → `omitzero`；
- 更换 UUID 实现或格式；
- 迁移 `ServeMux` pattern；
- 把包级泛型 helper 变成 generic method；
- 改变 `slices.Reverse`、`Sort`、`Compact`、map iterator 造成的顺序或原地突变；
- 把 `time.Tick`、`sync.Once`、错误聚合或 context cause 引入长生命周期/取消敏感路径；
- 为了“更现代”而重排 EPUB、重新压缩 ZIP、重写 JSON 字段或触碰 capability API。

遇到这些情况，先 `explain` 相关 ID，写迁移计划和 golden，再局部实施；若没有授权或证据，保持原实现并记录跳过理由。

## §7 验证门禁

按改动范围运行，所有 Go 改动至少包含：

```sh
gofmt -w <changed-go-files>
go build ./...
go test ./...
go vet ./...
go test -race ./...
go test ./internal/archguard/ -v
git diff --check
```

额外门禁：

- JSON、report、contract 或 API 改动：schema 校验、序列化/反序列化 golden、旧输入兼容矩阵；显式迁移 v2 前完成 §4.1 全部样本。
- `internal/scan`、EPUB capability 或 ZIP 改动：文本不变/授权正文分支、entry 透传 CRC/压缩属性、redline、输出 SHA 和必要的人工 diff review。
- 外部工具改动：工具缺失、超时、取消、非零退出、超大 stdout/stderr 和路径安全测试。
- benchmark/fuzz 改动：`b.Loop()` benchmark、bounded fuzz、重复输出稳定性和 race 证据。

`internal/archguard/` 是不可修改的自动化定义；守卫失败必须回到实现、契约或文档引用修复。不得加白名单、删断言、加 `t.Skip` 或修改守卫来迎合现代化写法。

## §8 Review 清单

提交 Go 改动前，审阅者应能回答：

1. 已读取正确模块的 `go.mod` 和完整 `list` 输出了吗？每个采用/跳过的相关 ID 是否有理由？
2. 改动是否遵守架构层级、`Run(ctx, ...)`、单次落盘、无整文档序列化和 `internal/archguard` 禁止修改？
3. 是否改变 JSON 字段存在性、UUID/URL 形式、HTTP 路由、错误/退出码、EPUB 字节或资源顺序？若是，golden、schema、redline 和授权在哪里？
4. 用户输入、取消、超限、短读、provider 缺失/失败和 panic 边界是否可判定？
5. 重复运行是否得到稳定输出；测试、fuzz、benchmark、vet、race、build 和 archguard 是否都有记录？

若以上问题不能由代码和测试回答，优先保留兼容实现，补证据后再现代化。
