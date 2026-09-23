---
name: epub-font-coverage-analyzer
description: 只读检查嵌入字体 cmap、字体链回退、子集漏字和生僻字风险。用于方块字或跨阅读器字形差异；依赖外部字体 provider，不嵌入或子集化字体。
---

# EPUB 字体覆盖分析

## 何时用

先区分“字体没有字形”与“有字形但回退未到达”。字体链修改交 typography skill，角色不明先 content analyzer。按 [SPEC §4、§8](../../docs/final/SPEC-实现约束.md) 判断覆盖边界；CLI 自动调用已配置 provider，但发行包不自带它，缺失时不能宣称检测完成。

## 调什么

```sh
epub run epub.font.coverage.analyze --input "book.epub" --json
```

只读，可选 `profile=ideal-browser|kindle-pessimistic`，默认后者；profile 是模型，不是实际设备测试。

## 返回怎么读

- 前缀 `epub.font.coverage.analyze.`：`profile`、`status`（pass/warn/fail，区别于信封 status）、`summary`。
- 同一前缀下的 `charInventory`、`chainHealth`、`unresolved`、`textRuns` 是明细（如 `facts["epub.font.coverage.analyze.charInventory"]`）；结合位置、CSS 继承和字体链查看。
- `fontcoverage.fail/risk` 是覆盖问题；`fontcoverage.adapter` 是检测器失败，此时无有效覆盖结论。provider 成功返回报告后才可能有 `detectorExitCode/Stderr`。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- true-missing → 查可用字库；fallback-not-reached → 改链或局部专用类，不先造字；subset-cut → 查子集字符清单；only-non-embedded → 依赖目标阅读器验证。
- unresolved 非零必须标记分析缺口；系统字体名不能证明 cmap 覆盖，悲观 profile 不能写成 Kindle fail/pass。
- 分析器不采集 CSS `quotes/content` 生成字符：单独枚举，子集写出后复查 cmap。不得把形似码位（如 〇/○）互换。
- 普通正文默认自由；少量补字用局部类，锁定正文须覆盖最终解析到该角色的全部文字/标点，不只扫 p。修改后重跑覆盖与全项红线。
