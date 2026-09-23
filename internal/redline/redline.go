// Package redline 实现六条红线校验器（text / metadata / spine / anchors /
// cover / drm），是 contracts/capabilities/v1/*.json 里 redLines 声明的
// 唯一执行点（INV-5）。
//
// 两个入口：Check 供 caps / pipeline 在内存中比对，CompareFiles 供
// `epub redline` 的两文件比对。输出行与退出码是对外契约（pre-commit hook
// 与 CI gate 都依赖它），沿用历史 CLI 的措辞：
//   - 0 成功；1 存在问题；2 DRM 拒绝或输入错误。
package redline

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// CheckName 是一条红线的名字。
type CheckName = string

// 六条红线的名字。
const (
	CheckText     = "text"
	CheckMetadata = "metadata"
	CheckSpine    = "spine"
	CheckCover    = "cover"
	CheckDRM      = "drm"
	CheckAnchors  = "anchors"
)

// State 是参与比对的一侧只读视图。*zipfs.Archive 直接满足该接口；
// book 的原始态与当前态由 bookstate.go 的适配器提供。
type State interface {
	// Path 返回该侧的可读标识（用于错误消息）。
	Path() string
	Names() []string
	Read(name string) ([]byte, error)
}

// Options 是一次比对的可调参数。
type Options struct {
	// AllowList 是 fnmatch 风格的 XHTML 路径豁免（如 */nav.xhtml）。
	AllowList []string
	// PathMap 是 before→after 的 entry 改名映射（链式展开后的最终形态）。
	PathMap map[string]string
	// AllowFontObfuscation 允许且仅允许标准 EPUB 字体混淆。
	AllowFontObfuscation bool
	// Verbose 附加 verbose 行（以 "verbose: " 开头）。
	Verbose bool
}

// Finding 是一条问题报告。Message 与 Python 版输出逐字节一致。
type Finding struct {
	Check   string // 哪条红线
	Message string // 整行措辞（已含前缀）
	Verbose bool   // verbose 行（非问题）
}

// Validator 是一条红线的校验器。解析失败以 error 返回（legacy 语义退出码 2）。
type Validator interface {
	Check(before, after State, o Options) ([]Finding, error)
}

// MappedPath 复刻 mapped_path：有映射用映射，没有保持原名。
func MappedPath(m map[string]string, name string) string {
	if v, ok := m[name]; ok {
		return v
	}
	return name
}

// ComposePathMaps composes two rename stages: first runs, then second.
// Entries in each input map are simultaneous; only names carried forward from
// the first stage are followed through the second.
func ComposePathMaps(first, second map[string]string) map[string]string {
	out := make(map[string]string, len(first)+len(second))
	image := make(map[string]bool, len(first))
	for _, mid := range first {
		image[mid] = true
	}
	for from, mid := range first {
		if to, ok := second[mid]; ok {
			out[from] = to
		} else {
			out[from] = mid
		}
	}
	for from, to := range second {
		if _, exists := first[from]; exists || image[from] {
			continue
		}
		out[from] = to
	}
	return out
}

// StagePathMap parses one simultaneous rename stage. Null/absent mappings are
// an empty stage; conflicting sources or destination collisions are rejected.
func StagePathMap(list []any) (map[string]string, error) {
	out := make(map[string]string, len(list))
	targets := make(map[string]string, len(list))
	for _, raw := range list {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, inputErr("each mapping must contain string from/to paths")
		}
		from, okFrom := item["from"].(string)
		to, okTo := item["to"].(string)
		if !okFrom || !okTo {
			return nil, inputErr("each mapping must contain string from/to paths")
		}
		if prior, exists := out[from]; exists && prior != to {
			return nil, inputErr("mapping source %q has conflicting targets %q and %q", from, prior, to)
		}
		if prior, exists := targets[to]; exists && prior != from {
			return nil, inputErr("mapping sources %q and %q have the same target %q", prior, from, to)
		}
		out[from] = to
		targets[to] = from
	}
	return out, nil
}

// skipped 复刻 skipped()：任一 fnmatch 模式命中即豁免。
func skipped(path string, patterns []string) bool {
	for _, p := range patterns {
		if fnmatch(p, path) {
			return true
		}
	}
	return false
}

// fnmatch 是 Python fnmatch.fnmatch 的 POSIX（大小写敏感）近似：
// * 跨目录段，? 单字符，[...] 字符类。
func fnmatch(pattern, name string) bool {
	return matchFnmatch(pattern, name)
}

// sha256Hex 返回字节内容的 SHA-256 十六进制摘要。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sortedNames 返回排序后的名字副本。
func sortedNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

// pythonRepr 以 Python 的 list-repr 形状渲染字符串切片，
// 供 legacy 消息（如 spine 列表）逐字节复刻。
func pythonRepr(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pythonStrRepr(s))
	}
	b.WriteByte(']')
	return b.String()
}

// pythonStrRepr 渲染 Python 风格的单引号字符串字面量。
func pythonStrRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
