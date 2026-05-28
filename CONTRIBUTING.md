# 贡献指南

## 你可以贡献什么

- 阅读器实测：把 reader / 字号 / profile 下的结果写进 `docs/final/reader-matrix.yaml`。
- fixture / 场景：在 `templates/epub-style-demo/` 添加新场景。
- bug 修复：让 scripts、fixture 或文档更稳。
- skill 改进：修订 `skills/*/SKILL.md`，保持 frontmatter 字段名不变。
- 文档补充：guides / experiments / 入门。
- 公版书清洗 demo：按 `samples/third-party/` 规范添加新样本。

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
   ```

2. 建分支：

   ```sh
   git checkout -b feat/your-topic
   ```

3. 修改：遵守 [CLAUDE.md](CLAUDE.md) 的「修改优先级」。

4. 跑校验：

   ```sh
   bash templates/epub-style-demo/build.sh
   NEW=$(ls -t templates/epub-style-demo/dist/ | head -1)
   bash scripts/validate-epub-style-demo.sh --epub templates/epub-style-demo/dist/"$NEW"
   bash scripts/validate-popup-notes.sh --epub templates/epub-style-demo/dist/"$NEW"
   python3 scripts/validate_skills_basic.py
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
