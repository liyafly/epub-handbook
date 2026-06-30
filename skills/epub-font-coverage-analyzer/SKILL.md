---
name: epub-font-coverage-analyzer
description: 用于 EPUB 出现生僻字、方块字、跨阅读器字形不一致、Kindle 字体回退失败，或需要验证嵌入字体、子集字体和候选大字库是否覆盖全书字符时。
---

# EPUB 字体覆盖分析

先区分“字体里没有字形”和“字形存在但阅读器没有走到回退字体”。本 skill 只读，不嵌入、不子集化、不包裹 span。

## 工作流

1. 先运行 EPUB preflight；发现真实加密资源时停止。
2. 运行公开入口：

   ```sh
   python3 scripts/epub_font_coverage_adapter.py book.epub --format json > work/font-coverage.json
   ```

3. 查看：
   - `summary.by_profile_risk`：不同 reader profile 的 `ok/risk/fail`。
   - `char_inventory`：问题字、出现位置、覆盖位置和原因。
   - `unresolved`：无法解析的 CSS；非零时结论不完整。
   - `chain_health`：字体链顺序、缺失资源和回退风险。
4. 按原因分派：
   - `true-missing`：链上没有字体覆盖，寻找大字库或造字。
   - `fallback-not-reached`：调整链首或改用 `.rare` 专用类，不先造字。
   - `subset-cut`：检查子集输入字符和 `fontspec`。
   - `only-non-embedded`：属于阅读器依赖，只能结合目标 reader 实测。
5. 字体链或 class 修改交给 `epub-typography-optimizer`；正文角色不确定时先用 `epub-content-analyzer`。

## 决策表

| 目标 | 建议 |
| --- | --- |
| 普通正文无缺字 | 保持自由模式，不为“锁字体”额外嵌入 |
| 少量生僻字 | `.rare` + 专用补字字体 |
| 全书必须统一特定字形且字体覆盖完整 | 评估 C1-body、体积和 reader matrix |
| Kindle 只在后备字体有字 | 不依赖逐字回退；调整链或拆专用类 |
| CSS unresolved | 先修解析边界或人工检查，不声称全覆盖 |

## 示例

若 Apple Books 正常而 Kindle 显示方块，报告显示问题字只在 `later-embedded` 命中，应判断为回退未到达；把同一字体再重复塞进链中不会解决问题。

## 禁止事项

- 不把系统字体名当作已证明的字形覆盖。
- 不把悲观 reader profile 当作真实设备实测。
- 不因覆盖报告改正文码位或替换字符。
- 不自动处理 DRM 或未知字体混淆。

## 验证

```sh
python3 scripts/test_epub_font_coverage_adapter.py
cd tools-font/coverage-detector && uv run pytest -q
```
