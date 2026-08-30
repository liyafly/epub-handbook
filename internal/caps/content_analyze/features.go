// features.go 复刻 _features 与 _role：统计、级联规则与排版建议查询。
// 级联顺序（:353-411）即语义，禁止重排。
package contentanalyze

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/report"
)

// featuresLegacy 对齐 _features(text)。
//
// 已知近似（任务要求的注明差异）：
//   - Python char.isdigit() 的非 Nd 数字用 pyIsDigit 覆盖（上标/下标、
//     圆圈数字、杭州数码、〇、爱琴海数字）；未覆盖的极罕见区块可能有差异。
//   - unicodedata.category(c) 以 P 开头 ↔ unicode.IsPunct（类别 P），两者一致，
//     仅 Unicode 表版本可能有极小差异。
//   - latin 判定 isascii() and isalpha() 与 Go 完全一致（A-Za-z）。
func featuresLegacy(text string) legacyFeatures {
	var cjk, latin, digits, punctuation, quotes, visible int
	for _, r := range text {
		cp := uint32(r)
		if (cp >= 0x3400 && cp <= 0x9FFF) || (cp >= 0x20000 && cp <= 0x3134F) {
			cjk++
		} else if r < 0x80 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			latin++
		} else if pyIsDigit(r) {
			digits++
		}
		if unicode.IsPunct(r) {
			punctuation++
		}
		if strings.ContainsRune(quoteChars, r) {
			quotes++
		}
		if !isPySpace(r) {
			visible++
		}
	}
	return legacyFeatures{
		VisibleChars:     visible,
		CJKCount:         cjk,
		LatinCount:       latin,
		DigitCount:       digits,
		PunctuationCount: punctuation,
		QuoteCount:       quotes,
		LineCount:        maxInt(1, len(pySplitLines(text))),
		CJKRatio:         ratio(cjk, visible),
		LatinRatio:       ratio(latin, visible),
	}
}

// ratio 复刻 round(cjk / visible, 4) if visible else 0.0。
func ratio(n, visible int) report.PyFloat {
	if visible == 0 {
		return report.PyFloat(0)
	}
	return report.PyFloat(round4(float64(n) / float64(visible)))
}

// round4 用 FormatFloat('f',4) 的正确舍入（半数偶）复刻 Python round(x, 4)，
// 再 ParseFloat 回 float64，repr 由 PyFloat 输出最短形式。
func round4(v float64) float64 {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	out, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return v
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// roleResult 是 _role 的返回（角色, 候选, 置信度, 待复核, 证据）。
type roleResult struct {
	primary    string
	candidates []string
	confidence string
	review     bool
	evidence   []string
}

func explicitRole(primary, evidence string) roleResult {
	return roleResult{
		primary:    primary,
		candidates: []string{primary},
		confidence: "high",
		review:     false,
		evidence:   []string{evidence},
	}
}

func heuristicRole(primary string, candidates []string, confidence string, review bool, evidence string) roleResult {
	return roleResult{
		primary:    primary,
		candidates: candidates,
		confidence: confidence,
		review:     review,
		evidence:   []string{evidence},
	}
}

// hasClass 对齐 _has_class：pattern 是子串包含（不是全等）。
func hasClass(block textBlock, patterns ...string) bool {
	for _, value := range block.classes {
		lower := strings.ToLower(value)
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				return true
			}
		}
	}
	return false
}

func classSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[strings.ToLower(v)] = true
	}
	return out
}

// roleOf 对齐 _role 的级联规则（:353-411），顺序即语义。
func roleOf(block textBlock, f legacyFeatures) roleResult {
	tag := block.tag
	text := pyTrimSpace(block.text)
	tags := make(map[string]bool, len(block.ancestorTags))
	for _, t := range block.ancestorTags {
		tags[t] = true
	}
	types := classSet(block.epubTypes)

	// ---- 显式结构规则 ----
	switch {
	case hasClass(block, "subtitle", "sub-title"):
		return explicitRole("subtitle", "subtitle class")
	case tag == "h1" && (hasClass(block, "book-title", "main-title", "title-page") || types["titlepage"]):
		return explicitRole("title", "title-page heading structure")
	case tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" || tag == "h6":
		return explicitRole("heading", "heading element")
	case tag == "figcaption" || tag == "caption":
		return explicitRole("caption", tag+" element")
	case tag == "hr" || hasClass(block, "scene-break", "separator"):
		return explicitRole("scene-break", "scene-break structure")
	case tag == "code" || tag == "pre":
		return explicitRole("code", tag+" element")
	case tag == "li" || tag == "dt" || tag == "dd":
		return explicitRole("list", tag+" element")
	case types["footnote"] || types["endnote"] || types["rearnote"] || types["note"] ||
		hasClass(block, "footnote", "endnote", "duokan-note"):
		return explicitRole("note", "note semantic ancestor")
	case hasClass(block, "epigraph"):
		return explicitRole("epigraph", "epigraph class")
	case hasClass(block, "poem", "verse", "stanza", "poetry"):
		return explicitRole("verse", "verse/poem class")
	case hasClass(block, "letter", "correspondence") || types["letter"]:
		return explicitRole("letter", "letter semantic ancestor")
	case hasClass(block, "classical-text", "classical", "original-text"):
		return explicitRole("classical", "classical/original class")
	case hasClass(block, "modern-text", "translation", "translated-text"):
		return explicitRole("modern-translation", "modern/translation class")
	case tag == "blockquote" || tags["blockquote"] || hasClass(block, "quotation", "quote"):
		return explicitRole("quotation", "blockquote/quotation structure")
	case hasClass(block, "dialogue", "speech"):
		return explicitRole("dialogue", "dialogue class")
	}

	// ---- 启发式规则 ----
	if chapterRe.MatchString(text) {
		return heuristicRole("heading", []string{"heading", "body"}, "medium", true,
			"chapter-like opening without heading markup")
	}
	if text != "" {
		if first, _ := utf8.DecodeRuneInString(text); strings.ContainsRune(quoteChars, first) && f.QuoteCount >= 2 {
			return heuristicRole("dialogue", []string{"dialogue", "quotation", "body"}, "medium", true,
				"quoted paragraph content")
		}
	}
	if strings.HasPrefix(text, dashEmEm) || strings.HasPrefix(text, dashEm) {
		if f.VisibleChars <= visibleMax {
			return heuristicRole("dialogue", []string{"dialogue", "body"}, "medium", true,
				"dash-led paragraph resembles dialogue")
		}
	}
	if sceneRe.MatchString(text) {
		return heuristicRole("scene-break", []string{"scene-break"}, "medium", true,
			"separator-like punctuation-only paragraph")
	}
	lines := pySplitLines(text)
	if f.LineCount >= 2 && allShortLines(lines) {
		return heuristicRole("verse", []string{"verse", "body"}, "medium", true,
			"multiple short lines resemble verse")
	}
	cjk := f.CJKCount
	if cjk >= 2 && cjk <= 14 && !strings.ContainsAny(text, sentenceEnd) {
		return heuristicRole("unknown", []string{"unknown", "body", "verse", "subtitle"}, "low", true,
			"short CJK paragraph is structurally ambiguous")
	}
	if cjk >= 15 && float64(f.CJKRatio) >= 0.8 && f.PunctuationCount == 0 {
		return heuristicRole("unknown", []string{"unknown", "classical", "body", "verse"}, "low", true,
			"unpunctuated CJK prose may be classical text")
	}
	if f.VisibleChars > 0 {
		return heuristicRole("body", []string{"body"}, "medium", false,
			"ordinary paragraph-like content")
	}
	return heuristicRole("unknown", []string{"unknown"}, "low", true, "no visible text")
}

// allShortLines 对齐 all(len(line.strip()) <= 24 for line in text.splitlines() if line.strip())。
func allShortLines(lines []string) bool {
	for _, line := range lines {
		trimmed := pyTrimSpace(line)
		if trimmed == "" {
			continue
		}
		if utf8.RuneCountInString(trimmed) > verseLineMax {
			return false
		}
	}
	return true
}

// publicize 对齐 _public_block：文本块 → 报告块（含 SHA-256 与排版建议）。
func publicize(block textBlock, includeSnippets bool) legacyBlock {
	feats := featuresLegacy(block.text)
	role := roleOf(block, feats)
	sum := sha256.Sum256([]byte(block.text))
	lb := legacyBlock{
		Source:         block.source,
		Locator:        block.locator,
		Tag:            block.tag,
		Classes:        block.classes,
		Language:       block.language,
		PreviousTag:    block.previousTag,
		NextTag:        block.nextTag,
		TextSHA256:     hex.EncodeToString(sum[:]),
		Features:       feats,
		PrimaryRole:    role.primary,
		CandidateRoles: role.candidates,
		Confidence:     role.confidence,
		ReviewRequired: role.review,
		Evidence:       role.evidence,
		Typography:     typographyTable[role.primary],
	}
	if includeSnippets {
		lb.Snippet = runeCut(block.text, 160)
	}
	return lb
}

// pyIsDigit 复刻 Python str.isdigit()：Nd 类别之外，还包含
// Numeric_Type=Digit/Decimal 的字符（上标/下标数字、圆圈数字、
// 括号数字、杭州数码、爱琴海数字等；注意 〇(Nl) 的 isdigit 为 False）。
func pyIsDigit(r rune) bool {
	if unicode.IsDigit(r) {
		return true
	}
	// 规则：只认「个位数值」（Numeric_Type=Digit/Decimal，值 0-9）。
	// ⑩❿⒑ 等两位数值是 Numeric_Type=Numeric，Python isdigit 为 False。
	switch {
	case r == 0x00B2 || r == 0x00B3 || r == 0x00B9: // ² ³ ¹
		return true
	case r >= 0x2070 && r <= 0x2079: // ⁰-⁹（ⁱ 不是数字）
		return r != 0x2071
	case r >= 0x2080 && r <= 0x2089: // ₀-₉
		return true
	case r >= 0x2460 && r <= 0x2468: // ①-⑨（⑩起不算）
		return true
	case r >= 0x2488 && r <= 0x2490: // ⒈-⒐（⒑起不算）
		return true
	case r == 0x24EA: // ⓪
		return true
	case r >= 0x2776 && r <= 0x277E: // ❶-❾（❿不算）
		return true
	case r >= 0x3021 && r <= 0x3029: // 〡-〩
		return true
	case r >= 0x10107 && r <= 0x1010F: // 爱琴海个位
		return true
	}
	return false
}
