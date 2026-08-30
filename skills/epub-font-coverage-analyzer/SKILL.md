---
name: epub-font-coverage-analyzer
description: 检查 EPUB 嵌入字体与字体链对全书字符的真实覆盖。用于出现生僻字、方块字、跨阅读器字形不一致、Kindle 字体回退失败，或验证嵌入字体、子集字体和候选大字库是否覆盖全书字符时；只读分析，不嵌入、不子集化。
---

# EPUB 字体覆盖分析

## 何时用

- 先区分「字体里没有字形」和「字形存在但阅读器没有走到回退字体」。本能力只读：不嵌入、不子集化、不包裹 span；字体链或 class 修改交给 `epub-typography-optimizer`，正文角色不确定时先用 `epub-content-analyzer`。
- 实际字形覆盖检测由外部字体 provider 完成，`epub` CLI 自动调用，无需手工准备环境。
- 解读报告的先决知识：
  - `summary.by_profile_risk` 是不同 reader profile 的 `ok/risk/fail` 计数；`kindle-pessimistic` 是悲观档案，不等于真实设备实测。
  - 结合报告中的 text run、CSS 继承、覆盖和最终字体链核对，不只看普通 `p`。
  - 当前分析器不会采集 `quotes`、`content` 等 CSS 生成字符；须从 CSS 另行枚举，加入子集工具的 `extraCodepoints`，并对写出字体的 `cmap` 人工复核。
- 禁止事项：
  - 不把系统字体名当作已证明的字形覆盖；不把悲观 reader profile 当作真实设备实测。
  - 不因覆盖报告改正文码位或替换字符；中文标点或数字样式异常时先核对源码码位，再核对命中字体角色的 `cmap`。`“”`（U+201C/U+201D）与 ASCII 引号、`〇`（U+3007）与 `○`（U+25CB）都不是可由子集化流程互换的字符。
  - 不自动处理 DRM 或未知字体混淆；preflight 发现真实加密资源时停止。

## 调什么

```sh
epub run epub.font.coverage.analyze --input <书> --json
```

可选 KEY=VALUE：`profile=ideal-browser|kindle-pessimistic`（缺省 `kindle-pessimistic`）。只读能力，不需要 `--output`。

需要旧报告形状明细（`char_inventory` 问题字与位置、`chain_health` 字体链健康、`unresolved` 逐条、`text_runs` 等）时加 `legacy_report=true`：

```sh
epub run epub.font.coverage.analyze --input <书> --json legacy_report=true
```

## 返回怎么读

- `status`：`complete | failed | approval-required`；`findings[].level`：`error | warn | info`；退出码：0 成功；1 失败或存在 error 级 finding；2 approval-required；3 用法错误。
- facts 键前缀 `epub.font.coverage.analyze.`：
  - `profile`：本次使用的检测档案。
  - `status`：`pass | warn | fail`（按 `by_profile_risk` 的 fail/risk 计数与 unresolved 数判定）。
  - `summary`：detector 汇总（`by_profile_risk`、`unresolved_runs` 等）。
- findings：
  - `error fontcoverage.fail`：该 profile 下存在 fail 级覆盖缺口。
  - `warn fontcoverage.risk`：存在 risk 级缺口或 unresolved CSS run。
  - `error fontcoverage.adapter`：外部检测器未产出合法报告（输入缺失、provider 不可用等），此时结论无效。
- `legacy_report=true` 时 `facts` 额外含 `legacyReport`（detector 原始 JSON，含 `char_inventory`、`unresolved`、`chain_health`、`summary` 等）。

## 依据返回怎么判断

- `status == pass` 且无 `fontcoverage.risk` → 当前 profile 覆盖成立；换 `profile=` 复跑目标阅读器档案再下结论。
- `unresolved` 非零 → 先修解析边界或人工检查对应 CSS，不声称全覆盖。
- 按 `char_inventory` 中每个问题字的原因分派（legacy 报告明细）：
  - `true-missing`：链上没有字体覆盖 → 寻找大字库或造字。
  - `fallback-not-reached`：字形存在但回退未到达 → 调整链首或改用 `.rare` 专用类，不先造字。示例：Apple Books 正常而 Kindle 显示方块，问题字只在 `later-embedded` 命中，即属此类；把同一字体重复塞进链中不解决问题。
  - `subset-cut`：子集漏字 → 检查子集输入字符和 `fontspec`。
  - `only-non-embedded`：阅读器依赖系统字体 → 只能结合目标 reader 实测。
- 决策表：
  - 普通正文无缺字 → 保持自由模式，不为「锁字体」额外嵌入。
  - 少量生僻字 → `.rare` + 专用补字字体。
  - 设计要求锁定正文，或全书必须统一特定字形，且正文角色覆盖完整 → 评估 C1-body、体积和 reader matrix。
  - Kindle 只在后备字体有字 → 不依赖逐字回退；调整链或拆专用类。
- 修改字体链 / class → `epub-typography-optimizer`；改后用 `epub redline --check all <before.epub> <after.epub>` 并复跑本能力复核覆盖。
- `error fontcoverage.adapter` → 检查输入 EPUB 与 provider 环境，修好后重跑；不要基于失败运行的报告下任何覆盖结论。
