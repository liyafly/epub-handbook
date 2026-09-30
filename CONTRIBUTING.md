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
   # 需要 Go 1.27 或更新版本：https://go.dev/dl/
   go version
   ```

2. 建分支：

   ```sh
   git checkout -b feat/your-topic
   ```

3. 修改：遵守 [AGENTS.md](AGENTS.md) 的规范来源优先级和最小验证矩阵。架构相关改动
   先读 `docs/final/SPEC-go-architecture.md`；`internal/archguard/` 下的守卫测试禁止修改。

4. 跑校验：

   若本地 checkout 含有被忽略的 `work-epub/` 书稿，Go 的 `./...` 仍会发现其中的脚本目录。此时本地使用下面列出的根包、`cmd` 与 `internal` 包集合；在干净 checkout 和 CI 中继续运行完整 `./...` 检查。pre-commit hook 只提供快反馈。

   修改 `docs/final/EPUB 3 HTML CSS 属性速查表.md` 时，必须同步重新生成或手工同步同名 `.html` 派生文件，并核对主体内容一致。

   ```sh
   go build . ./cmd/... ./internal/...
   go test -count=1 . ./cmd/... ./internal/...
   go vet . ./cmd/... ./internal/...
   go test -race . ./cmd/... ./internal/...
   go test ./internal/archguard/ -v
   go test ./internal/docguard/
   bash templates/epub-style-demo/build.sh
   EPUB=templates/epub-style-demo/dist/epub-style-demo.epub
   go run ./cmd/epub run epub.style.demo.maintain --input "$EPUB" --json
   go run ./cmd/epub run epub.notes.popup.normalize --input "$EPUB" --json
   ```

5. commit：使用 [conventional commits](https://www.conventionalcommits.org/) 风格，如 `feat:` / `fix:` / `docs:` / `chore:`。

6. PR：说明动机、范围、是否影响 reader-matrix、是否需要新实测。

## 维护者发布步骤

1. 在发布提交中更新 CHANGELOG 版本标题和 `cmd/epub/version.go` 的 `<版本>-dev`，确认 release workflow 会从该标题抽取非空 `dist/RELEASE_NOTES.md`。
2. 将发布提交推到 `main`，等待 Build EPUB Demo、Architecture Guard、Font Provider 全部通过；需要时单独 dispatch Release CLI 并检查四个平台的外部 smoke。
3. 在已验证的发布提交上创建带注释的版本 tag，例如 `git tag -a v0.5.0 -m 'Release v0.5.0'`。发布前确认 CHANGELOG 的本版正文包含升级说明、已知限制和未实测的阅读器声明。
4. 只推送指定 tag，例如 `git push origin v0.5.0`；不要使用 `git push --tags` 或移动已发布 tag。发布失败时重跑失败的 workflow job，不要移动 tag。
5. 核对 GitHub Release 正文和四个平台附件、`SHA256SUMS`。发布完成后另开提交，把 CLI 版本升到下一个 `-dev`，并同步 README 与 Go rewrite handoff 中的当前发布版本。

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
  screenshot: <截图路径（如有）>
  log: <阅读器日志路径（如有）>
  conversion_log: <转换日志路径（如有）>
```

不允许在没有实测的情况下写 `pass`。`pass` 必须附上 `screenshot`、`log` 或 `conversion_log` 之一；没测过就写 `warn` + `pending-<reader>-version`。

## 提 issue 时

附上：

1. 你的环境（OS / Go 版本 / 阅读器与版本）。
2. 复现命令。
3. 完整错误输出。
4. 你期望的行为。

## 行为规范

技术讨论保持就事论事；不歧视；不发广告。

## 许可

提 PR 即视为同意你的贡献按本仓许可证（代码 MIT、文档参照 [THIRD_PARTY.md](THIRD_PARTY.md)）发布。
