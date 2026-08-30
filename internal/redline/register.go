// register.go 是红线注册表（INV-7 白名单：注册表仅在 init() 期写入）。
// archguard TestRedlineClosure 以本文件里的 Register("name", ...) 字面量
// 调用为事实来源，与 contracts/capabilities/v1/*.json 的 redLines 对账。
package redline

import (
	"regexp"
	"sort"
)

// registry 是红线名 → 校验器 的注册表。
var registry = map[string]Validator{}

// Register 登记一条红线的校验器。仅供各校验器文件的 init() 调用。
func Register(name string, v Validator) {
	registry[name] = v
}

// All 返回全部已注册校验器，按名字排序。
func All() []Validator {
	names := Names()
	out := make([]Validator, 0, len(names))
	for _, n := range names {
		out = append(out, registry[n])
	}
	return out
}

// Names 返回全部已注册红线名，按字典序。
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ---- 共享不可变表（与注册表同属 init 期写入的包级状态，统一收在本文件）----

// CheckOrder 是六条红线的名字（legacy --check 参数的合法值全集）。
var CheckOrder = []string{CheckText, CheckMetadata, CheckSpine, CheckCover, CheckDRM, CheckAnchors}

// checkExecOrder 是 validate() 的问题行输出顺序（DRM 门禁另行前置）。
var checkExecOrder = []string{CheckText, CheckAnchors, CheckMetadata, CheckSpine, CheckCover}

// coreMetadataFields 是 metadata 红线覆盖的 DC 核心字段（CORE_METADATA）。
var coreMetadataFields = []string{"title", "creator", "identifier", "language"}

// blockTags 是文本红线认可的块级标签（BLOCK_TAGS）。
var blockTags = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true,
	"h6": true, "li": true, "td": true, "blockquote": true, "pre": true, "div": true,
}

// ignoredTextTags 是文本提取时整棵剔除的标签（IGNORED_TEXT_TAGS）。
var ignoredTextTags = map[string]bool{"rt": true, "rp": true, "script": true, "style": true}

// controlTextEpubTypes 是控制性锚点的 epub:type 分词（CONTROL_TEXT_EPUB_TYPES）。
var controlTextEpubTypes = map[string]bool{"noteref": true, "backlink": true}

// fontObfuscationAlgorithms 是允许的标准字体混淆算法（FONT_OBFUSCATION_ALGORITHMS）。
var fontObfuscationAlgorithms = map[string]bool{
	"http://www.idpf.org/2008/embedding": true,
	"http://ns.adobe.com/pdf/enc#RC":     true,
}

// 共享正则（init 期编译，此后不可变）。

// doctypeRe 对齐 sanitize_xml 的 `<!DOCTYPE[^>]*>`（re.I|re.S，全量删除）。
var doctypeRe = regexp.MustCompile(`(?is)<!DOCTYPE[^>]*>`)
