package navaudit

import (
	"context"
	"fmt"
	"strings"

	cssscan "github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// checkPageColorLock 报告在页面级容器上锁定颜色或背景的声明。
//
// SPEC §7：html、普通 body（含带类名的 body）与 `*` 不设置页面级 color /
// background / background-color，以免覆盖阅读器的夜间与护眼主题。这是安全
// 底座的第三项；存量书中常见（如 body 上的固定正文色），影响是夜间模式
// 对比度，需人工判断，因此只报 warn、不改书。
//
// 值为 inherit / transparent / none / initial / unset / currentcolor 时不锁定
// 配色，不报告。A-lite 海报页的背景按 SPEC §2 放在 poster-* modifier 上
// （如 body.poster-bg），主体复合选择器含 `.poster-` 类时豁免；body.fullpage
// 本身不允许设置页面配色，不豁免。
func (ins *inspector) checkPageColorLock(ctx context.Context, pkg *opf.Package) {
	seen := make(map[string]bool)
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.MediaType != "text/css" || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		if !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		raw, err := ins.b.Current(item.ArchivePath)
		if err != nil {
			continue
		}
		sheet, err := cssscan.Parse(raw)
		if err != nil {
			// 解析失败已由 CSS reference 扫描或其它检查报告，这里不重复。
			continue
		}
		for _, rule := range sheet.Rules {
			if rule.AtRule || !selectorTargetsPageColor(rule.Selector) {
				continue
			}
			selector := collapseSpace(cssscan.StripComments(rule.Selector))
			for _, decl := range rule.Declarations {
				if !isPageColorProperty(decl.Property()) || !locksPageColor(decl.PropertyValue()) {
					continue
				}
				detail := fmt.Sprintf("%s -> %s { %s: %s }", item.Href, selector, decl.Property(), collapseSpace(decl.PropertyValue()))
				ins.addFinding("warn", "CSS page-level color/background locks the reader theme", detail, "css-page-color-lock")
				ins.addSkill("epub-cleanup", "warn")
			}
		}
	}
}

func isPageColorProperty(name string) bool {
	return name == "color" || name == "background" || name == "background-color"
}

// locksPageColor 判断声明值是否固定了颜色；继承、透明等中性值不算。
func locksPageColor(value string) bool {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
	switch strings.ToLower(value) {
	case "", "inherit", "transparent", "none", "initial", "unset", "currentcolor":
		return false
	}
	return true
}

// selectorTargetsPageColor 判断选择器列表中是否有某个选择器的主体
// （最后一个复合选择器）是 html / :root / `*` / body，且不带 poster-* 类。
func selectorTargetsPageColor(selector string) bool {
	for _, sel := range splitSelectorList(cssscan.StripComments(selector)) {
		subject := strings.ToLower(lastCompound(sel))
		if subject == "" {
			continue
		}
		name := leadingTypeName(subject)
		universal := strings.TrimSpace(sel) == "*" || strings.TrimSpace(sel) == "*|*"
		if !universal && name != "html" && name != "body" && !containsTopLevel(subject, ":root") {
			continue
		}
		if !hasTopLevelClassPrefix(subject, "poster-") {
			return true
		}
	}
	return false
}

// hasTopLevelClassPrefix 判断复合选择器括号外是否含以 prefix 开头的类。
func hasTopLevelClassPrefix(compound, prefix string) bool {
	needle := "." + prefix
	depth := 0
	for i := 0; i < len(compound); i++ {
		switch compound[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(compound[i:], needle) {
				return true
			}
		}
	}
	return false
}
