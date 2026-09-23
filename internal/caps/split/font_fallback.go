package split

import (
	"fmt"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/css"
)

type resourceReference struct {
	uri               string
	localFontFallback bool
}

func collectCSSReferences(text string) ([]resourceReference, error) {
	sheet, err := css.Parse([]byte(text))
	if err != nil {
		// Keep strict URI/escape validation even when the stylesheet parser
		// cannot prove font-face structure (for example, inline declarations).
		references, scanErr := css.ScanReferences([]byte(text))
		if scanErr != nil {
			return nil, fmt.Errorf("invalid CSS: %w", scanErr)
		}
		out := make([]resourceReference, 0, len(references))
		for _, ref := range references {
			if strings.ContainsRune(ref.Value, '\\') {
				return nil, fmt.Errorf("invalid CSS: escaped URL at byte %d", ref.ValueSpan.Start)
			}
			if ref.Value != "" {
				out = append(out, resourceReference{uri: ref.Value})
			}
		}
		return out, nil
	}

	// CSS references and tokens are already part of Parse's lossless projection.
	// Reuse them instead of lexing the same stylesheet again through ScanReferences.
	refs := sheet.References
	out := make([]resourceReference, 0, len(refs))
	for _, ref := range refs {
		if strings.ContainsRune(ref.Value, '\\') {
			return nil, fmt.Errorf("invalid CSS: escaped URL at byte %d", ref.ValueSpan.Start)
		}
		if ref.Value != "" {
			out = append(out, resourceReference{uri: ref.Value})
		}
	}

	type fontSource struct {
		span     css.Span
		hasLocal bool
	}
	var sources []fontSource
	for _, rule := range sheet.Rules {
		if !rule.AtRule || !strings.EqualFold(rule.AtRuleName, "font-face") {
			continue
		}
		for _, decl := range rule.Declarations {
			if strings.EqualFold(decl.Name, "src") {
				sources = append(sources, fontSource{span: decl.ValueSpan, hasLocal: hasLocalSource(sheet.Tokens, decl.ValueSpan)})
			}
		}
	}
	// Rule/declaration traversal is source ordered; retain an explicit sort for
	// the binary search invariant if the parser projection ever changes.
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].span.End != sources[j].span.End {
			return sources[i].span.End < sources[j].span.End
		}
		return sources[i].span.Start < sources[j].span.Start
	})
	optional := map[string]bool{}
	for _, ref := range refs {
		fallback := false
		idx := sort.Search(len(sources), func(i int) bool { return sources[i].span.End >= ref.Span.Start })
		for i := idx; i < len(sources) && sources[i].span.Start <= ref.Span.Start; i++ {
			if sources[i].span.Start <= ref.Span.Start && ref.Span.End <= sources[i].span.End && sources[i].hasLocal {
				fallback = true
			}
		}
		// A repeated URL used by a hard dependency must never inherit a font
		// exception from a different declaration in the same stylesheet.
		if old, seen := optional[ref.Value]; seen {
			fallback = old && fallback
		}
		optional[ref.Value] = fallback
	}
	for i := range out {
		out[i].localFontFallback = optional[out[i].uri]
	}
	return out, nil
}

// Prove a nonempty local(...) source at the top-level of src. Strings,
// comments, nested format(local(...)), and empty local() are not proof.
func hasLocalSource(tokens []css.Token, span css.Span) bool {
	var value []css.Token
	for _, token := range tokens {
		if token.Span.Start >= span.Start && token.Span.End <= span.End && token.Kind != css.TokenWhitespace && token.Kind != css.TokenComment {
			value = append(value, token)
		}
	}
	depth := 0
	for i, token := range value {
		if depth == 0 && (i == 0 || value[i-1].Kind == css.TokenComma) && token.Kind == css.TokenIdent && strings.EqualFold(string(token.Data), "local") && i+3 < len(value) && value[i+1].Kind == css.TokenLeftParenthesis && token.Span.End == value[i+1].Span.Start {
			j := i + 2
			valid, nonempty := true, false
			for ; j < len(value) && value[j].Kind != css.TokenRightParenthesis; j++ {
				part := value[j]
				if part.Kind != css.TokenIdent && part.Kind != css.TokenString {
					valid = false
				}
				if part.Kind == css.TokenString && (j != i+2 || j+1 >= len(value) || value[j+1].Kind != css.TokenRightParenthesis) {
					valid = false
				}
				if strings.TrimSpace(strings.Trim(string(part.Data), `"'`)) != "" {
					nonempty = true
				}
			}
			if valid && nonempty && j < len(value) && (j+1 == len(value) || value[j+1].Kind == css.TokenComma) {
				return true
			}
		}
		switch token.Kind {
		case css.TokenLeftParenthesis:
			depth++
		case css.TokenRightParenthesis:
			depth--
		}
	}
	return false
}
