## 改了什么

<!-- 一两句话。 -->

## 架构自检

<!-- 见 docs/final/SPEC-go-architecture.md。勾不上的项请在下面说明。 -->

- [ ] 本次改动**没有**触碰 `internal/archguard/`
      <!-- 若触碰了：说明为什么守卫本身有误，而不是代码有误。 -->
- [ ] 没有往 markdown 里新增 `python3 scripts/...` 或 `scripts/*.sh` 引用（INV-10 棘轮；迁移已完成归零，须保持为零）
- [ ] 新增/修改的 capability 走了 SPEC §6.1 模板，只碰了模板列出的 5 处文件
- [ ] 新增 capability 的实现经过 parity gate（SPEC §5.2）或为全新能力并附测试
