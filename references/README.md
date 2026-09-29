# Third-party references

This directory holds third-party material retained for inspection only. Before adding
anything, record its source, author, licence and retention reason in
[`../THIRD_PARTY.md`](../THIRD_PARTY.md); that file is the authoritative provenance
and licensing record.

Reference samples do not establish EPUB rules or reader-compatibility conclusions.
Those require a project demo artifact and the evidence recorded in
`docs/final/reader-matrix.yaml`.

## 保留理由

`references/epubs/EPub指南——从入门到放弃 20230418 (赤霓) (Z-Library).epub`
按所有者 2026-09-29 的 Q15 裁决继续保留在 git 中，作为唯一的真书回归样本。
当前没有权利人提供的许可或授权记录；保留决定不表示已取得再分发许可。

依赖该样本的测试：

- `internal/pipeline/chain_semantics_test.go`
- `internal/zipfs/passthrough_test.go`
- `internal/caps/structure_normalize/markup_rewrite_test.go`
- `internal/caps/metadata/realbook_test.go`
- `internal/caps/cover/realbook_test.go`
