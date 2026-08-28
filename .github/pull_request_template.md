## 改了什么

<!-- 一两句话。 -->

## 架构自检

<!-- 见 docs/final/SPEC-go-architecture.md。勾不上的项请在下面说明。 -->

- [ ] 本次改动**没有**触碰 `internal/archguard/`
      <!-- 若触碰了：说明为什么守卫本身有误，而不是代码有误。 -->
- [ ] 没有往 markdown 里新增 `python3 scripts/...` 或 `scripts/*.sh` 引用（INV-10 棘轮只减不增）
- [ ] 新增/修改的 capability 走了 SPEC §6.1 模板，只碰了模板列出的 5 处文件
- [ ] 若迁移了 capability：parity gate 三级已达标（SPEC §5.2），对应 Python 脚本才删

## 迁移进度

<!-- 若本 PR 推进了 Go 重写，填这两行；否则删掉本节。 -->

- 涉及 capability：
- 棘轮基线变化：`tools/parity/legacy-refs.txt` 由 ___ 条降至 ___ 条
