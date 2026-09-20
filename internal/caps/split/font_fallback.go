package split

import (
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/css"
)

type resourceReference struct {
	uri               string
	localFontFallback bool
}

func collectCSSReferences(text string) ([]resourceReference, error) {
	// Keep strict URI/escape validation; the lossless parser is used only to
	// prove an optional font dependency, never to suppress a parse failure.
	uris, err := collectCSSURIsStrict(text)
	if err != nil {
		return nil, err
	}
	out := make([]resourceReference, len(uris))
	for i, uri := range uris {
		out[i].uri = uri
	}
	sheet, err := css.Parse([]byte(text))
	if err != nil {
		return out, nil
	} // inline declarations: no font-face proof
	optional := map[string]bool{}
	for _, ref := range sheet.References {
		fallback := false
		for _, rule := range sheet.Rules {
			if !rule.AtRule || !strings.EqualFold(rule.AtRuleName, "font-face") {
				continue
			}
			for _, decl := range rule.Declarations {
				if strings.EqualFold(decl.Name, "src") && decl.ValueSpan.Start <= ref.Span.Start && ref.Span.End <= decl.ValueSpan.End && hasLocalSource(sheet.Tokens, decl.ValueSpan) {
					fallback = true
				}
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
