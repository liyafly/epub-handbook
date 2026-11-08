package pipeline

// 书根制作说明骨架：new / import 时写一份只有一屏的提纲。逐轮修改经过放 素材/历史.md，
// 需要长期保留的脚本、源图与验收证据放 素材/（纳入书级 Git，`.epub/` 只放可重新生成的东西）。

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const productionNotesName = "制作说明.md"

func productionNotes(cfg bookConfig) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s 制作说明\n\n", cfg.Title)
	b.WriteString("只写一屏：当前状态和要点。逐轮修改经过写 `素材/历史.md`；脚本、源图和验收证据放 `素材/`。\n\n")
	b.WriteString("## 状态\n\n制作中。\n\n")
	fmt.Fprintf(&b, "## 成品\n\n- 通用版：`out/%s`（SHA 见 `.epub/delivery.tsv`）\n\n", cfg.Output)
	if cfg.Original != nil {
		fmt.Fprintf(&b, "## 原件\n\n- `original.epub`（导入自 `%s`），SHA-256 `%s`\n\n", cfg.Original.Name, cfg.Original.SHA256)
	}
	b.WriteString("## 目标阅读器\n\n\n## 配方与版本差异\n\n（用了哪些配方、有哪些变体，为什么）\n\n## 待办\n\n\n## 决策\n\n（只放结论和链接，过程写进 `素材/历史.md`）\n")
	return []byte(b.String())
}

// writeProductionNotes 在 dir 写制作说明骨架；已存在时不覆盖。
func writeProductionNotes(dir string, cfg bookConfig) error {
	target := filepath.Join(dir, productionNotesName)
	if isFile(target) {
		return nil
	}
	return zipfs.WriteFileAtomic(target, productionNotes(cfg), 0o644)
}
