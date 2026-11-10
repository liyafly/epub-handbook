package navaudit

// OPF 元数据的 refines、丛书挂接与无障碍词表检查。只读 OPF 的区间树，不改书。

import (
	"context"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// accessModeValues 是 schema.org accessMode 的标准词表。
func accessModeValues() map[string]bool {
	return setOf("auditory", "chartOnVisual", "chemOnVisual", "colorDependent", "diagramOnTactile",
		"mathOnVisual", "musicOnVisual", "tactile", "textOnVisual", "textual", "visual")
}

// accessibilityHazardValues 是 schema.org accessibilityHazard 的标准词表。
func accessibilityHazardValues() map[string]bool {
	return setOf("flashing", "noFlashingHazard", "unknownFlashingHazard",
		"motionSimulation", "noMotionSimulationHazard", "unknownMotionSimulationHazard",
		"sound", "noSoundHazard", "unknownSoundHazard", "unknown", "none")
}

func setOf(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

// checkMetadataRefines 检查 metadata 里的 meta：
//   - refines 指向不存在的 id → error；
//   - collection-type / group-position 没挂在 belongs-to-collection 上 → warn；
//   - schema:accessMode / schema:accessibilityHazard 的值不在标准词表 → warn。
func (ins *inspector) checkMetadataRefines(ctx context.Context) {
	raw, err := ins.b.Current(ins.opfPath)
	if err != nil {
		return
	}
	root, err := opf.ScanSpanTreeContext(ctx, raw)
	if err != nil {
		return
	}
	ids := map[string]*opf.SpanNode{}
	for _, n := range root.Walk() {
		if id, ok := n.AttrByLocal("", "id"); ok && id != "" {
			if _, seen := ids[id]; !seen {
				ids[id] = n
			}
		}
	}
	metadata := root.ChildByLocal(opf.OPFURI, "metadata")
	if metadata == nil {
		return
	}
	modes, hazards := accessModeValues(), accessibilityHazardValues()
	for _, n := range metadata.Walk() {
		if ctx.Err() != nil {
			return
		}
		if n.Name.Local != "meta" {
			continue
		}
		property, _ := n.AttrByLocal("", "property")
		refines, hasRefines := n.AttrByLocal("", "refines")
		target := (*opf.SpanNode)(nil)
		if hasRefines {
			id := strings.TrimPrefix(strings.TrimSpace(refines), "#")
			target = ids[id]
			if target == nil {
				ins.addFinding("error", "OPF meta refines a missing id: "+refines, ins.opfPath, "metadata-refines-missing")
				ins.addSkill("epub-package-ops", "error")
				continue
			}
		}
		value := strings.TrimSpace(n.IterText())
		switch property {
		case "collection-type", "group-position":
			targetProperty := ""
			if target != nil && target.Name.Local == "meta" {
				targetProperty, _ = target.AttrByLocal("", "property")
			}
			if targetProperty != "belongs-to-collection" {
				ins.addFinding("warn", property+" must refine a belongs-to-collection meta", ins.opfPath, "collection-meta-without-collection")
				ins.addSkill("epub-package-ops", "warn")
			}
		case "schema:accessMode":
			if !modes[value] {
				ins.addFinding("warn", "schema:accessMode value is not in the schema.org vocabulary: "+value, ins.opfPath, "access-mode-vocabulary")
				ins.addSkill("epub-package-ops", "warn")
			}
		case "schema:accessibilityHazard":
			if !hazards[value] {
				ins.addFinding("warn", "schema:accessibilityHazard value is not in the schema.org vocabulary: "+value, ins.opfPath, "accessibility-hazard-vocabulary")
				ins.addSkill("epub-package-ops", "warn")
			}
		}
	}
}
