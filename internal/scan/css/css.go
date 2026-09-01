// Package css provides a lossless CSS token and source-span adapter.
//
// The parser deliberately does not render a stylesheet. Parse returns a
// projection of the original bytes: every token, rule, declaration, and URL
// reference points back into the input with a byte span. Callers that need to
// change a stylesheet must turn those spans into editset edits.
package css

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	parseinput "github.com/tdewolff/parse/v2"
	parsercss "github.com/tdewolff/parse/v2/css"
)

// Span is a half-open byte range [Start, End).
type Span struct {
	Start int
	End   int
}

// Len returns the number of bytes in the span.
func (s Span) Len() int { return s.End - s.Start }

// Valid reports whether the span is ordered and non-negative.
func (s Span) Valid() bool { return s.Start >= 0 && s.End >= s.Start }

// CommentSpan is a CSS comment, including its /* */ delimiters.
type CommentSpan struct {
	Span
	Text string
}

// TokenKind is the lossless scanner's small, semantic token vocabulary.
type TokenKind uint8

const (
	TokenUnknown TokenKind = iota
	TokenBOM
	TokenWhitespace
	TokenComment
	TokenString
	TokenURL
	TokenIdent
	TokenAtKeyword
	TokenCustomProperty
	TokenNumber
	TokenColon
	TokenSemicolon
	TokenComma
	TokenLeftBrace
	TokenRightBrace
	TokenLeftBracket
	TokenRightBracket
	TokenLeftParenthesis
	TokenRightParenthesis
	TokenDelim
)

// String returns a stable diagnostic name for a token kind.
func (k TokenKind) String() string {
	switch k {
	case TokenBOM:
		return "BOM"
	case TokenWhitespace:
		return "Whitespace"
	case TokenComment:
		return "Comment"
	case TokenString:
		return "String"
	case TokenURL:
		return "URL"
	case TokenIdent:
		return "Ident"
	case TokenAtKeyword:
		return "AtKeyword"
	case TokenCustomProperty:
		return "CustomProperty"
	case TokenNumber:
		return "Number"
	case TokenColon:
		return "Colon"
	case TokenSemicolon:
		return "Semicolon"
	case TokenComma:
		return "Comma"
	case TokenLeftBrace:
		return "LeftBrace"
	case TokenRightBrace:
		return "RightBrace"
	case TokenLeftBracket:
		return "LeftBracket"
	case TokenRightBracket:
		return "RightBracket"
	case TokenLeftParenthesis:
		return "LeftParenthesis"
	case TokenRightParenthesis:
		return "RightParenthesis"
	case TokenDelim:
		return "Delim"
	default:
		return "Unknown"
	}
}

// Token is a token and its exact source span. Data aliases the input slice.
type Token struct {
	Kind TokenKind
	Span Span
	Data []byte
}

// Declaration is a CSS declaration with absolute source spans.
type Declaration struct {
	Name          string
	Value         string
	Span          Span // segment, including leading/trailing source whitespace
	NameSpan      Span
	ValueSpan     Span
	SemicolonSpan Span
	HasSemicolon  bool
}

// Decl is retained as a concise compatibility name for Declaration.
type Decl = Declaration

// Rule is a qualified rule or an at-rule. Qualified rule selector/body spans
// are absolute source ranges. At-rules are included in the projection so a
// caller can make a conservative decision about unknown syntax; nested
// qualified rules are also returned as individual Rule values.
type Rule struct {
	Selector     string
	SelectorSpan Span
	Body         string
	BodySpan     Span
	Span         Span // complete rule, including braces when present

	AtRule       bool
	AtRuleName   string
	HasBlock     bool
	Nested       bool
	Declarations []Declaration
}

// ReferenceKind identifies a resource-bearing CSS construct.
type ReferenceKind string

const (
	ReferenceURL    ReferenceKind = "url"
	ReferenceImport ReferenceKind = "import"
)

// Reference is a URL or @import value. ValueSpan excludes quotes and outer
// url() syntax, but retains escapes exactly as written.
type Reference struct {
	Kind      ReferenceKind
	Span      Span
	ValueSpan Span
	Value     string
	Quote     byte
	DataURL   bool
}

// Stylesheet is the lossless source projection produced by Parse.
type Stylesheet struct {
	Tokens       []Token
	Rules        []Rule
	Declarations []Declaration
	References   []Reference
}

// ParseError indicates that a stylesheet cannot be safely used for a write.
// It may wrap a lexer/parser error or a source-span scanner error.
type ParseError struct {
	Offset int
	Err    error
}

func (e *ParseError) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("css parse error at byte %d: %v", e.Offset, e.Err)
	}
	return "css parse error: " + e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

var (
	// ErrInvalidUTF8 is returned instead of replacing bytes on a write path.
	ErrInvalidUTF8 = errors.New("css: invalid UTF-8")
	// ErrUnterminated is returned for unterminated strings, comments, URLs, or blocks.
	ErrUnterminated = errors.New("css: unterminated token or block")
)

// Parse validates data with the CSS Syntax Level 3 parser and then builds a
// byte-preserving projection. The third-party parser is never used for
// serialization. A recovery boundary converts an upstream parser panic on a
// malformed input into an ordinary error.
func Parse(data []byte) (sheet *Stylesheet, err error) {
	defer func() {
		if r := recover(); r != nil {
			sheet = nil
			err = &ParseError{Offset: -1, Err: fmt.Errorf("parser panic: %v", r)}
		}
	}()

	if !utf8.Valid(data) {
		return nil, &ParseError{Offset: firstInvalidUTF8(data), Err: ErrInvalidUTF8}
	}
	if err := validateWithParser(data); err != nil {
		return nil, err
	}

	s := sourceScanner{data: data}
	tokens, err := s.lex()
	if err != nil {
		return nil, err
	}
	rules, declarations, err := parseRuleList(data, 0, len(data), false)
	if err != nil {
		return nil, err
	}
	return &Stylesheet{
		Tokens:       tokens,
		Rules:        rules,
		Declarations: declarations,
		References:   references(data, tokens),
	}, nil
}

// ParseDeclarations parses a declaration list whose braces are not included.
// Spans are absolute within data. Unlike the stylesheet parser, this helper
// does not require a surrounding rule and is useful for a known declaration
// block span.
func ParseDeclarations(data []byte) ([]Declaration, error) {
	return parseDeclarations(data, 0, len(data))
}

func validateWithParser(data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{Offset: -1, Err: fmt.Errorf("parser panic: %v", r)}
		}
	}()

	// NewInputBytes appends a sentinel byte and may use spare capacity. Keep
	// that implementation detail away from the caller's raw input.
	input := parseinput.NewInputBytes(bytes.Clone(data))
	p := parsercss.NewParser(input, false)
	limit := len(data)*8 + 1024
	for n := 0; n < limit; n++ {
		grammar, _, _ := p.Next()
		if grammar != parsercss.ErrorGrammar {
			continue
		}
		if p.HasParseError() {
			return &ParseError{Offset: p.Offset(), Err: p.Err()}
		}
		if parseErr := p.Err(); parseErr != nil && !errors.Is(parseErr, io.EOF) {
			return &ParseError{Offset: p.Offset(), Err: parseErr}
		}
		return nil
	}
	return &ParseError{Offset: -1, Err: errors.New("parser exceeded token limit")}
}

func firstInvalidUTF8(data []byte) int {
	for i := 0; i < len(data); {
		_, size := utf8.DecodeRune(data[i:])
		if size == 1 && data[i] >= utf8.RuneSelf {
			return i
		}
		i += size
	}
	return -1
}

// Comments returns comment spans for a valid source string. It is retained
// for callers that only need a best-effort read-only query; malformed input
// returns the spans found before the error.
func Comments(text string) []CommentSpan {
	data := []byte(text)
	var out []CommentSpan
	for i := 0; i+1 < len(data); {
		if data[i] != '/' || data[i+1] != '*' {
			i++
			continue
		}
		start := i
		i += 2
		for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
			i++
		}
		if i+1 >= len(data) {
			break
		}
		i += 2
		out = append(out, CommentSpan{Span: Span{Start: start, End: i}, Text: text[start:i]})
	}
	return out
}

// StripComments removes comments from a read-only semantic projection. It is
// not used by any write path; comments in the source remain untouched.
func StripComments(text string) string {
	data := []byte(text)
	var out strings.Builder
	last := 0
	for _, c := range Comments(text) {
		out.Write(data[last:c.Start])
		last = c.End
	}
	out.Write(data[last:])
	return out.String()
}

// Rules parses text and returns qualified/at-rule spans. Parse failures are
// represented by an empty result for compatibility with the old read-only
// helper; callers that must distinguish malformed input should call Parse.
func Rules(text string) []Rule {
	sheet, err := Parse([]byte(text))
	if err != nil {
		return nil
	}
	return sheet.Rules
}

// Declarations parses a declaration body and returns spans relative to body.
// Callers requiring an error should use ParseDeclarations.
func Declarations(body string) []Decl {
	decls, err := ParseDeclarations([]byte(body))
	if err != nil {
		return nil
	}
	return decls
}

// FontFamilyDecls returns declarations named font-family, with a byte span
// covering only the value. Matching is token-aware and therefore ignores
// strings, comments, and semicolons in functions.
func FontFamilyDecls(body string) []FontFamilyDecl {
	decls, err := ParseDeclarations([]byte(body))
	if err != nil {
		return nil
	}
	data := []byte(body)
	var out []FontFamilyDecl
	for _, d := range decls {
		if !strings.EqualFold(strings.TrimSpace(d.Name), "font-family") {
			continue
		}
		out = append(out, FontFamilyDecl{
			WholeSpan: d.Span,
			PrefixEnd: d.ValueSpan.Start,
			ValueSpan: d.ValueSpan,
			Value:     string(data[d.ValueSpan.Start:d.ValueSpan.End]),
		})
	}
	return out
}

// FontFamilyDecl is a compatibility view of a Declaration.
type FontFamilyDecl struct {
	WholeSpan Span
	ValueSpan Span
	PrefixEnd int
	Value     string
}

type sourceScanner struct {
	data []byte
}

func (s sourceScanner) lex() ([]Token, error) {
	data := s.data
	var tokens []Token
	for i := 0; i < len(data); {
		start := i
		if i == 0 && len(data) >= 3 && bytes.Equal(data[:3], []byte{0xef, 0xbb, 0xbf}) {
			i += 3
			tokens = append(tokens, Token{Kind: TokenBOM, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if isCSSWhitespace(data[i]) {
			i++
			for i < len(data) && isCSSWhitespace(data[i]) {
				i++
			}
			tokens = append(tokens, Token{Kind: TokenWhitespace, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
			end, err := skipComment(data, i)
			if err != nil {
				return nil, parseErrAt(i, err)
			}
			i = end
			tokens = append(tokens, Token{Kind: TokenComment, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if data[i] == '\'' || data[i] == '"' {
			end, err := skipString(data, i)
			if err != nil {
				return nil, parseErrAt(i, err)
			}
			i = end
			tokens = append(tokens, Token{Kind: TokenString, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if data[i] == '@' {
			i++
			end := consumeName(data, i)
			if end == i {
				tokens = append(tokens, Token{Kind: TokenDelim, Span: Span{start, i}, Data: data[start:i]})
				continue
			}
			i = end
			tokens = append(tokens, Token{Kind: TokenAtKeyword, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if isNameStart(data, i) {
			end := consumeName(data, i)
			if end > i {
				if urlEnd, ok, err := consumeURL(data, start, end); ok {
					if err != nil {
						return nil, parseErrAt(start, err)
					}
					i = urlEnd
					tokens = append(tokens, Token{Kind: TokenURL, Span: Span{start, i}, Data: data[start:i]})
					continue
				}
			}
			i = end
			kind := TokenIdent
			if len(data[start:i]) >= 2 && data[start] == '-' && data[start+1] == '-' {
				kind = TokenCustomProperty
			}
			tokens = append(tokens, Token{Kind: kind, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if data[i] == '\\' {
			end := consumeName(data, i)
			if end == i {
				return nil, parseErrAt(i, errors.New("invalid escape"))
			}
			i = end
			tokens = append(tokens, Token{Kind: TokenIdent, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		if isNumberStart(data, i) {
			i = consumeNumber(data, i)
			tokens = append(tokens, Token{Kind: TokenNumber, Span: Span{start, i}, Data: data[start:i]})
			continue
		}
		kind := TokenDelim
		switch data[i] {
		case ':':
			kind = TokenColon
		case ';':
			kind = TokenSemicolon
		case ',':
			kind = TokenComma
		case '{':
			kind = TokenLeftBrace
		case '}':
			kind = TokenRightBrace
		case '[':
			kind = TokenLeftBracket
		case ']':
			kind = TokenRightBracket
		case '(':
			kind = TokenLeftParenthesis
		case ')':
			kind = TokenRightParenthesis
		}
		i++
		tokens = append(tokens, Token{Kind: kind, Span: Span{start, i}, Data: data[start:i]})
	}
	return tokens, nil
}

func parseRuleList(data []byte, start, end int, nested bool) ([]Rule, []Declaration, error) {
	var rules []Rule
	var allDecls []Declaration
	preludeStart := start
	parenDepth, bracketDepth := 0, 0
	for i := start; i < end; {
		if next, ok, err := skipNonStructural(data, i, end); ok {
			if err != nil {
				return nil, nil, parseErrAt(i, err)
			}
			i = next
			continue
		}
		switch data[i] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 {
				return nil, nil, parseErrAt(i, errors.New("unexpected ')'"))
			}
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth == 0 {
				return nil, nil, parseErrAt(i, errors.New("unexpected ']'"))
			}
			bracketDepth--
		case '{':
			if parenDepth != 0 || bracketDepth != 0 {
				i++
				continue
			}
			close, err := matchingBrace(data, i, end)
			if err != nil {
				return nil, nil, parseErrAt(i, err)
			}
			prelude := trimSpan(data, preludeStart, i)
			if !prelude.Valid() || prelude.Start == prelude.End || onlyIgnorable(data[prelude.Start:prelude.End]) {
				return nil, nil, parseErrAt(i, errors.New("empty rule prelude"))
			}
			if atName, ok := atRuleName(data, prelude.Start, prelude.End); ok {
				rule := Rule{
					AtRule:     true,
					AtRuleName: atName,
					HasBlock:   true,
					Span:       Span{prelude.Start, close + 1},
					BodySpan:   Span{i + 1, close},
					Body:       string(data[i+1 : close]),
				}
				if isDeclarationAtRule(atName) {
					decls, declErr := parseDeclarations(data, i+1, close)
					if declErr != nil {
						return nil, nil, declErr
					}
					rule.Declarations = decls
					allDecls = append(allDecls, decls...)
				} else if isNestedRuleAtRule(atName) {
					nestedRules, nestedDecls, nestedErr := parseRuleList(data, i+1, close, true)
					if nestedErr != nil {
						return nil, nil, nestedErr
					}
					rule.Nested = len(nestedRules) > 0
					rules = append(rules, rule)
					rules = append(rules, nestedRules...)
					allDecls = append(allDecls, nestedDecls...)
				} else {
					// Unknown at-rule blocks are intentionally opaque. Their
					// bytes are valid input but unsafe for shape factoring.
					rules = append(rules, rule)
				}
			} else {
				selector := trimSpan(data, prelude.Start, prelude.End)
				decls, declErr := parseDeclarations(data, i+1, close)
				if declErr != nil {
					return nil, nil, declErr
				}
				rules = append(rules, Rule{
					Selector:     string(data[selector.Start:selector.End]),
					SelectorSpan: selector,
					Body:         string(data[i+1 : close]),
					BodySpan:     Span{i + 1, close},
					Span:         Span{selector.Start, close + 1},
					Declarations: decls,
				})
				allDecls = append(allDecls, decls...)
			}
			i = close + 1
			preludeStart = i
			continue
		case ';':
			if parenDepth == 0 && bracketDepth == 0 {
				span := trimSpan(data, preludeStart, i)
				if span.Start < span.End {
					if atName, ok := atRuleName(data, span.Start, span.End); ok {
						rules = append(rules, Rule{AtRule: true, AtRuleName: atName, Span: Span{span.Start, i + 1}})
					} else if !nested {
						return nil, nil, parseErrAt(i, errors.New("top-level declaration"))
					}
				}
				preludeStart = i + 1
			}
		case '}':
			return nil, nil, parseErrAt(i, errors.New("unexpected '}'"))
		}
		i++
	}
	if parenDepth != 0 || bracketDepth != 0 {
		return nil, nil, parseErrAt(end, ErrUnterminated)
	}
	if span := trimSpan(data, preludeStart, end); span.Start < span.End && !onlyIgnorable(data[span.Start:span.End]) {
		if _, ok := atRuleName(data, span.Start, span.End); !ok {
			return nil, nil, parseErrAt(span.Start, errors.New("unterminated rule"))
		}
	}
	return rules, allDecls, nil
}

func parseDeclarations(data []byte, start, end int) ([]Declaration, error) {
	var out []Declaration
	segmentStart := start
	parenDepth, bracketDepth, braceDepth := 0, 0, 0
	for i := start; i < end; {
		if next, ok, err := skipNonStructural(data, i, end); ok {
			if err != nil {
				return nil, parseErrAt(i, err)
			}
			i = next
			continue
		}
		switch data[i] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 {
				return nil, parseErrAt(i, errors.New("unexpected ')' in declaration"))
			}
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth == 0 {
				return nil, parseErrAt(i, errors.New("unexpected ']' in declaration"))
			}
			bracketDepth--
		case '{':
			braceDepth++
		case '}':
			if braceDepth == 0 {
				return nil, parseErrAt(i, errors.New("unexpected '}' in declaration"))
			}
			braceDepth--
		case ';':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				decl, ok, err := declarationSegment(data, segmentStart, i, true)
				if err != nil {
					return nil, err
				}
				if ok {
					out = append(out, decl)
				}
				segmentStart = i + 1
			}
		}
		i++
	}
	if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
		return nil, parseErrAt(end, ErrUnterminated)
	}
	decl, ok, err := declarationSegment(data, segmentStart, end, false)
	if err != nil {
		return nil, err
	}
	if ok {
		out = append(out, decl)
	}
	return out, nil
}

func declarationSegment(data []byte, start, end int, hasSemicolon bool) (Declaration, bool, error) {
	trimmed := trimSpan(data, start, end)
	if trimmed.Start == trimmed.End || onlyIgnorable(data[trimmed.Start:trimmed.End]) {
		return Declaration{}, false, nil
	}
	colon := findTopLevelColon(data, trimmed.Start, trimmed.End)
	if colon < 0 {
		return Declaration{}, false, parseErrAt(trimmed.Start, errors.New("declaration missing ':'"))
	}
	nameSpan := trimSpan(data, trimmed.Start, colon)
	if nameSpan.Start == nameSpan.End {
		return Declaration{}, false, parseErrAt(colon, errors.New("empty declaration name"))
	}
	valueSpan := trimSpan(data, colon+1, trimmed.End)
	decl := Declaration{
		Name:      string(data[nameSpan.Start:nameSpan.End]),
		Value:     string(data[valueSpan.Start:valueSpan.End]),
		Span:      Span{start, end},
		NameSpan:  nameSpan,
		ValueSpan: valueSpan,
	}
	if hasSemicolon {
		decl.HasSemicolon = true
		decl.SemicolonSpan = Span{end, end + 1}
	}
	return decl, true, nil
}

func findTopLevelColon(data []byte, start, end int) int {
	parenDepth, bracketDepth, braceDepth := 0, 0, 0
	for i := start; i < end; {
		if next, ok, _ := skipNonStructural(data, i, end); ok {
			i = next
			continue
		}
		switch data[i] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case ':':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

func references(data []byte, tokens []Token) []Reference {
	var out []Reference
	for i, token := range tokens {
		if token.Kind != TokenURL {
			continue
		}
		valueSpan, quote, ok := urlValueSpan(data, token.Span)
		if !ok {
			continue
		}
		kind := ReferenceURL
		if previousImport(tokens, i) {
			kind = ReferenceImport
		}
		value := string(data[valueSpan.Start:valueSpan.End])
		out = append(out, Reference{Kind: kind, Span: token.Span, ValueSpan: valueSpan, Value: value, Quote: quote, DataURL: strings.HasPrefix(strings.ToLower(value), "data:")})
	}
	for i, token := range tokens {
		if token.Kind != TokenAtKeyword || !strings.EqualFold(string(token.Data), "@import") {
			continue
		}
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Kind == TokenWhitespace || tokens[j].Kind == TokenComment {
				continue
			}
			if tokens[j].Kind == TokenString {
				span, quote, ok := quotedValueSpan(data, tokens[j].Span)
				if ok {
					value := string(data[span.Start:span.End])
					out = append(out, Reference{Kind: ReferenceImport, Span: tokens[j].Span, ValueSpan: span, Value: value, Quote: quote, DataURL: strings.HasPrefix(strings.ToLower(value), "data:")})
				}
				break
			}
			break
		}
	}
	return out
}

func previousImport(tokens []Token, index int) bool {
	for i := index - 1; i >= 0; i-- {
		if tokens[i].Kind == TokenWhitespace || tokens[i].Kind == TokenComment {
			continue
		}
		if tokens[i].Kind == TokenSemicolon || tokens[i].Kind == TokenLeftBrace || tokens[i].Kind == TokenRightBrace {
			return false
		}
		return tokens[i].Kind == TokenAtKeyword && strings.EqualFold(string(tokens[i].Data), "@import")
	}
	return false
}

func urlValueSpan(data []byte, span Span) (Span, byte, bool) {
	start, end := span.Start, span.End
	if start < 0 || end > len(data) || end-start < 5 {
		return Span{}, 0, false
	}
	i := start
	for i < end && data[i] != '(' {
		i++
	}
	if i >= end {
		return Span{}, 0, false
	}
	i++
	for i < end && isCSSWhitespace(data[i]) {
		i++
	}
	if i < end && (data[i] == '\'' || data[i] == '"') {
		q := data[i]
		valueStart := i + 1
		valueEnd, err := skipString(data, i)
		if err != nil || valueEnd > end || valueEnd == 0 || data[valueEnd-1] != q {
			return Span{}, 0, false
		}
		return Span{valueStart, valueEnd - 1}, q, true
	}
	valueStart := i
	valueEnd := end - 1
	for valueEnd >= valueStart && isCSSWhitespace(data[valueEnd]) {
		valueEnd--
	}
	if valueEnd < valueStart || data[end-1] != ')' {
		return Span{}, 0, false
	}
	return Span{valueStart, valueEnd + 1}, 0, true
}

func quotedValueSpan(data []byte, span Span) (Span, byte, bool) {
	if span.End-span.Start < 2 {
		return Span{}, 0, false
	}
	return Span{span.Start + 1, span.End - 1}, data[span.Start], true
}

func atRuleName(data []byte, start, end int) (string, bool) {
	i := start
	for i < end {
		if isCSSWhitespace(data[i]) {
			i++
			continue
		}
		if i+1 < end && data[i] == '/' && data[i+1] == '*' {
			next, err := skipComment(data, i)
			if err != nil || next > end {
				return "", false
			}
			i = next
			continue
		}
		break
	}
	if i >= end || data[i] != '@' {
		return "", false
	}
	nameStart := i + 1
	nameEnd := consumeName(data, nameStart)
	if nameEnd == nameStart {
		return "", false
	}
	return strings.ToLower(string(data[nameStart:nameEnd])), true
}

func isDeclarationAtRule(name string) bool {
	switch strings.ToLower(name) {
	case "font-face", "page", "counter-style", "property", "viewport", "-ms-viewport", "color-profile", "font-palette-values", "position-try":
		return true
	default:
		return false
	}
}

func isNestedRuleAtRule(name string) bool {
	switch strings.ToLower(name) {
	case "media", "supports", "document", "layer", "keyframes", "-webkit-keyframes", "-moz-keyframes", "container", "scope", "starting-style":
		return true
	default:
		return false
	}
}

func matchingBrace(data []byte, open, end int) (int, error) {
	depth := 1
	for i := open + 1; i < end; {
		if next, ok, err := skipNonStructural(data, i, end); ok {
			if err != nil {
				return 0, err
			}
			i = next
			continue
		}
		switch data[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
		i++
	}
	return 0, ErrUnterminated
}

func skipNonStructural(data []byte, pos, end int) (next int, ok bool, err error) {
	if pos >= end {
		return pos, false, nil
	}
	if pos+1 < end && data[pos] == '/' && data[pos+1] == '*' {
		next, err := skipComment(data, pos)
		return next, true, err
	}
	if data[pos] == '\'' || data[pos] == '"' {
		next, err := skipString(data, pos)
		return next, true, err
	}
	if isNameStart(data, pos) {
		nameEnd := consumeName(data, pos)
		if next, isURL, err := consumeURL(data, pos, nameEnd); isURL {
			return next, true, err
		}
	}
	return pos, false, nil
}

func consumeURL(data []byte, start, nameEnd int) (int, bool, error) {
	if nameEnd-start != 3 || !strings.EqualFold(string(data[start:nameEnd]), "url") {
		return start, false, nil
	}
	i := nameEnd
	for i < len(data) && isCSSWhitespace(data[i]) {
		i++
	}
	if i >= len(data) || data[i] != '(' {
		return start, false, nil
	}
	i++
	for i < len(data) {
		if data[i] == '\'' || data[i] == '"' {
			next, err := skipString(data, i)
			if err != nil {
				return 0, true, err
			}
			i = next
			continue
		}
		if data[i] == '\\' {
			i = min(i+2, len(data))
			continue
		}
		if data[i] == ')' {
			return i + 1, true, nil
		}
		i++
	}
	return 0, true, ErrUnterminated
}

func skipComment(data []byte, start int) (int, error) {
	for i := start + 2; i+1 < len(data); i++ {
		if data[i] == '*' && data[i+1] == '/' {
			return i + 2, nil
		}
	}
	return len(data), ErrUnterminated
}

func skipString(data []byte, start int) (int, error) {
	q := data[start]
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case q:
			return i + 1, nil
		case '\\':
			if i+1 >= len(data) {
				return len(data), ErrUnterminated
			}
			if data[i+1] == '\r' && i+2 < len(data) && data[i+2] == '\n' {
				i += 2
			} else {
				i++
			}
		case '\n', '\r':
			return i, errors.New("unescaped newline in string")
		}
	}
	return len(data), ErrUnterminated
}

func trimSpan(data []byte, start, end int) Span {
	if start == 0 && end-start >= 3 && bytes.Equal(data[:3], []byte{0xef, 0xbb, 0xbf}) {
		start += 3
	}
	for start < end && isCSSWhitespace(data[start]) {
		start++
	}
	for end > start && isCSSWhitespace(data[end-1]) {
		end--
	}
	return Span{start, end}
}

func onlyIgnorable(data []byte) bool {
	for i := 0; i < len(data); {
		if isCSSWhitespace(data[i]) {
			i++
			continue
		}
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
			next, err := skipComment(data, i)
			if err != nil {
				return false
			}
			i = next
			continue
		}
		return false
	}
	return true
}

func consumeName(data []byte, start int) int {
	i := start
	for i < len(data) {
		if isNameByte(data[i]) {
			i++
			continue
		}
		if data[i] == '\\' {
			if i+1 >= len(data) {
				return i
			}
			i += 2
			continue
		}
		break
	}
	return i
}

func isNameStart(data []byte, i int) bool {
	if i >= len(data) {
		return false
	}
	b := data[i]
	return isNameByte(b) || b == '\\'
}

func isNameByte(b byte) bool {
	return b == '-' || b == '_' || b >= 0x80 || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func isNumberStart(data []byte, i int) bool {
	if i >= len(data) {
		return false
	}
	return bIsDigit(data[i]) || data[i] == '.' && i+1 < len(data) && bIsDigit(data[i+1])
}

func consumeNumber(data []byte, start int) int {
	i := start
	if i < len(data) && data[i] == '.' {
		i++
	}
	for i < len(data) && bIsDigit(data[i]) {
		i++
	}
	if i < len(data) && (data[i] == 'e' || data[i] == 'E') {
		j := i + 1
		if j < len(data) && (data[j] == '+' || data[j] == '-') {
			j++
		}
		begin := j
		for j < len(data) && bIsDigit(data[j]) {
			j++
		}
		if j > begin {
			i = j
		}
	}
	return i
}

func bIsDigit(b byte) bool { return b >= '0' && b <= '9' }

func isCSSWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func parseErrAt(offset int, err error) error {
	return &ParseError{Offset: offset, Err: err}
}
