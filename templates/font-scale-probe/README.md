# 字号缩放探针

最小合成样本，用来在真实阅读器里确认哪些书内 CSS 会让“调字号”失效。规则见
[SPEC §5.5.1](../../docs/final/SPEC-实现约束.md)。

```sh
sh templates/font-scale-probe/build.sh
```

产物：`templates/font-scale-probe/dist/font-scale-probe.epub`。源文件在 `src/`。每页共用
不含字号声明的 `base.css`，再各加一条待测声明：

| 页 | 附加声明 | 用途 |
|---|---|---|
| 01 | 无 | 对照；本页不放大则阅读器本身无法测，其余结论无效 |
| 02 | `body { font-size: 1em; }` | 判断阅读器是否把字号写在 body 上 |
| 03 | `body { font-size: 100%; }` | 与 02 交叉验证 |
| 04 | `html { font-size: 100%; }` | 已知问题写法（两本存量书实测） |
| 05 | `section { font-size: 1em; }` | 非根元素上的相对字号，预期安全 |
| 06 | `p { font-size: 16px; }` | 绝对单位，预期正文不放大 |

实测方法：默认字号下记下每页第一段首行字数，把阅读器字号放大两级后逐页比较。字数不变即
该页被锁定。每个阅读器记录名称、版本、设备、产物 SHA-256 与截图，再写入
[reader-matrix](../../docs/final/reader-matrix.yaml)。

## 实测记录

| 日期 | 阅读器 | 产物 SHA-256 | 结果 | 证据 |
|---|---|---|---|---|
| 2026-11-03 | Reeden 测试版（版本号未记录） | `ff289b9ec2e01b1ed213e0653b331171e1a2e355b947aad74a36ac44a31a6958` | 仅 04 不放大；01–03、05、06 均放大 | 用户口述，无截图 → reader-matrix `warn` |

解读：Reeden 是自绘（Flutter）引擎，不是 WebView。根元素字号声明会盖掉它的字号设置；`body`
上的相对字号不会；`px` 也被它一起放大（推测引擎自行缩放 px，未证实）。06 的结果不能外推到
WebView 类阅读器。

本样本故意包含违规写法，`epub.package.nav.audit` 会报 `css-root-font-size` error
、`css-fixed-font-size` warn 及 `css-body-font-size` info；这是预期结果，不用书级构建器打包。
未经阅读器实测，各页结论只是待验证假设。
