package navaudit

import (
	"context"
	"fmt"
	"strings"

	cssscan "github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// checkReaderFontScaling 报告会覆盖阅读器字号设置的根/正文容器字号声明。
//
// 多数自定义字号的阅读器把用户字号写到 html（或 body）上；书内样式表若在
// 同一元素上再声明 font-size / font，即使值是 100% 或 1em，也会在层叠中
// 盖掉阅读器设置，表现为“部分阅读器无法放大字体”。html / :root 已有书级
// 与探针实测（templates/font-scale-probe），按 error 阻断；同一探针中 body
// 上的 1em / 100% 未锁定字号，且尚无阅读器复现，按 info 记录。
// A-lite 海报页 body.fullpage 的固定基准是规范内的有意锁定，不报告。
// 单独的 `*` 选择器同样命中 html，按 html 处理。
//
// 其它元素上的绝对字号（px/pt/… 或 small 等绝对关键字）与视口字号（vw/vh）
// 不随根字号变化，阅读器放大时这些文字不动，按 warn 报告；A-lite 海报页
// (.fullpage) 与 0/1px 隐藏文字惯用法除外。
func (ins *inspector) checkReaderFontScaling(ctx context.Context, pkg *opf.Package) {
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
			if rule.AtRule {
				continue
			}
			decl, ok := fontSizeDeclaration(rule.Declarations)
			if !ok {
				continue
			}
			root, body := selectorTargetsReaderScale(rule.Selector)
			selector := collapseSpace(cssscan.StripComments(rule.Selector))
			detail := fmt.Sprintf("%s -> %s { %s: %s }", item.Href, selector, decl.Property(), collapseSpace(decl.PropertyValue()))
			switch {
			case root:
				ins.addFinding("error", "CSS font-size on html/:root overrides reader font scaling", detail, "css-root-font-size")
				ins.addSkill("epub-cleanup", "error")
			case body:
				ins.addFinding("info", "CSS font-size on body is redundant; not observed to block reader font scaling", detail, "css-body-font-size")
			case isFixedFontSize(decl) && !strings.Contains(strings.ToLower(selector), ".fullpage"):
				ins.addFinding("warn", "CSS font-size uses an absolute or viewport unit and will not follow reader font scaling", detail, "css-fixed-font-size")
				ins.addSkill("epub-cleanup", "warn")
			}
		}
	}
}

func fontSizeDeclaration(decls []cssscan.Declaration) (cssscan.Declaration, bool) {
	for _, decl := range decls {
		switch decl.Property() {
		case "font-size", "font":
			return decl, true
		}
	}
	return cssscan.Declaration{}, false
}

// selectorTargetsReaderScale 判断选择器列表中是否有某个选择器的主体
// （最后一个复合选择器）是 html / :root 或 body。
func selectorTargetsReaderScale(selector string) (root, body bool) {
	for _, sel := range splitSelectorList(cssscan.StripComments(selector)) {
		subject := strings.ToLower(lastCompound(sel))
		if subject == "" {
			continue
		}
		name := leadingTypeName(subject)
		if universal := strings.TrimSpace(sel); universal == "*" || universal == "*|*" {
			root = true
		} else if name == "html" || containsTopLevel(subject, ":root") {
			root = true
		} else if name == "body" && !containsTopLevel(subject, ".fullpage") {
			// A-lite 海报页按 SPEC §2 在 body.fullpage 上固定内部基准，属于有意锁定。
			body = true
		}
	}
	return root, body
}

// splitSelectorList 按顶层逗号切分选择器列表（忽略括号与引号内的逗号）。
func splitSelectorList(s string) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case c == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// lastCompound 返回选择器中最后一个顶层组合符之后的复合选择器。
func lastCompound(sel string) string {
	sel = strings.TrimSpace(sel)
	depth := 0
	var quote byte
	cut := 0
	for i := 0; i < len(sel); i++ {
		c := sel[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case depth == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '>' || c == '+' || c == '~'):
			cut = i + 1
		}
	}
	return strings.TrimSpace(sel[cut:])
}

// leadingTypeName 返回复合选择器开头的类型选择器名（可带 ns| 前缀）。
func leadingTypeName(compound string) string {
	if i := strings.IndexByte(compound, '|'); i >= 0 && !strings.ContainsAny(compound[:i], ".#:[") {
		compound = compound[i+1:]
	}
	end := 0
	for end < len(compound) {
		c := compound[end]
		if c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			end++
			continue
		}
		break
	}
	return compound[:end]
}

// containsTopLevel 判断 needle 是否出现在括号外（排除 :not(:root) 等）。
func containsTopLevel(compound, needle string) bool {
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
				rest := compound[i+len(needle):]
				if rest == "" || !isIdentByte(rest[0]) {
					return true
				}
			}
		}
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// isFixedFontSize 判断字号值是否使用绝对单位、绝对关键字或视口单位。
// font 简写只看斜杠前的各个 token。0 与 1px（隐藏文字惯用法）不算。
func isFixedFontSize(decl cssscan.Declaration) bool {
	value := decl.PropertyValue()
	value = strings.TrimSpace(strings.TrimSuffix(value, "!important"))
	var tokens []string
	if decl.Property() == "font" {
		for _, tok := range strings.Fields(value) {
			if i := strings.IndexByte(tok, '/'); i >= 0 {
				if i > 0 {
					tokens = append(tokens, tok[:i])
				}
				break
			}
			tokens = append(tokens, tok)
		}
	} else {
		tokens = []string{value}
	}
	for _, tok := range tokens {
		if fixedFontSizeToken(strings.TrimSpace(tok)) {
			return true
		}
	}
	return false
}

func fixedFontSizeToken(tok string) bool {
	switch tok {
	case "xx-small", "x-small", "small", "medium", "large", "x-large", "xx-large", "xxx-large":
		return true
	}
	i := 0
	for i < len(tok) && (tok[i] == '.' || tok[i] == '+' || (tok[i] >= '0' && tok[i] <= '9')) {
		i++
	}
	if i == 0 || i == len(tok) {
		return false
	}
	number, unit := tok[:i], tok[i:]
	switch unit {
	case "px":
		return number != "0" && number != "1" && number != "1.0"
	case "pt", "pc", "cm", "mm", "in", "q",
		"vw", "vh", "vmin", "vmax", "svw", "svh", "lvw", "lvh", "dvw", "dvh", "vi", "vb":
		return strings.Trim(number, "0.") != ""
	}
	return false
}
