# 贡献指南

## 你可以贡献什么

- 阅读器实测：把 reader / 字号 / profile 下的结果写进 `docs/final/reader-matrix.yaml`。
- fixture / 场景：在 `templates/epub-style-demo/` 添加新场景。
- bug 修复：让 scripts、fixture 或文档更稳。
- skill 改进：修订 `skills/*/SKILL.md`，保持 frontmatter 字段名不变。
- 文档补充：`docs/how-to/` 场景指南、`docs/learn/` 入门说明或
  `docs/pipeline/` 清洗流程。
- 第三方来源：只在有明确保留理由和许可记录时添加实体 EPUB，并同步更新
  `THIRD_PARTY.md`；普通参考材料放在 `references/`。

## 你不要贡献什么

- 受版权保护的 EPUB。
- 你不能合法分发的字体。
- 不带实测的 reader 兼容性主张。
- 改 `docs/final/` 但不补 fixture / reader-matrix 的规则。

## 流程

1. Fork + clone：

   ```sh
   git clone <your fork URL>
   cd epub-handbook
   # 没装 uv 时，macOS 可先运行：brew install uv
   uv sync
   ```

2. 建分支：

   ```sh
   git checkout -b feat/your-topic
   ```

3. 修改：遵守 [AGENTS.md](AGENTS.md) 的规范来源优先级和最小验证矩阵。

4. 跑校验：

   如果修改 `scripts/`，请先本地运行对应的 `scripts/test_*.py`；pre-commit hook 只提供快反馈，CI 会运行更完整的验证矩阵。

   修改 `docs/final/EPUB 3 HTML CSS 属性速查表.md` 时，必须同步重新生成或手工同步同名 `.html` 派生文件，并核对主体内容一致。

   ```sh
   bash templates/epub-style-demo/build.sh
   NEW=$(ls -t templates/epub-style-demo/dist/ | head -1)
   bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/"$NEW"
   bash scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/"$NEW"
   uv run python scripts/validate_ai_entrypoints.py
   uv run python scripts/validate_skills_basic.py
   ```

5. commit：使用 [conventional commits](https://www.conventionalcommits.org/) 风格，如 `feat:` / `fix:` / `docs:` / `chore:`。

6. PR：说明动机、范围、是否影响 reader-matrix、是否需要新实测。

## reader-matrix 回写规范

每条 expectation 必须包含：

```yaml
- reader: <reader_id>
  case: <case_id>
  status: pass | warn | fail | na
  reader_version: <真实版本号 or "pending-*">
  artifact: <对应的 dist epub 路径>
  issue: <一句话现象>
  action: <你做了什么>
  workaround: <临时回避方法（如有）>
```

不允许在没有实测的情况下写 `pass`。没测过就写 `warn` + `pending-<reader>-version`。

## 提 issue 时

附上：

1. 你的环境（OS / Python 版本 / browser）。
2. 复现命令。
3. 完整错误输出。
4. 你期望的行为。

## 行为规范

技术讨论保持就事论事；不歧视；不发广告。

## 许可

提 PR 即视为同意你的贡献按本仓许可证（代码 MIT、文档参照 [THIRD_PARTY.md](THIRD_PARTY.md)）发布。
