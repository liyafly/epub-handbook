package css

import (
	"bytes"
	"errors"
	"testing"
)

func TestParseLosslessFixtures(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantRefs int
	}{
		{
			name: "strings and comments",
			data: []byte(`/* keep {; } */
h1 { content: "{};"; color: red; }`),
		},
		{
			name:     "data and escaped urls",
			data:     []byte(`.icon { background: url(data:image/svg+xml,<svg>{;}</svg>); src: url(foo\)bar.woff2); }`),
			wantRefs: 2,
		},
		{
			name: "custom property",
			data: []byte(`:root {
  --theme: "a;b{}";
  color: var(--theme);
}`),
		},
		{
			name: "nested at rules",
			data: []byte(`@media screen {
  @supports (display: grid) {
    .x, .y:is(.a,.b) { color: red; }
  }
}`),
		},
		{
			name:     "font face",
			data:     []byte(`@font-face { font-family: "Demo"; src: url("demo.woff2"); }`),
			wantRefs: 1,
		},
		{
			name:     "bom crlf",
			data:     []byte("\xef\xbb\xbf@import 'screen.css';\r\nbody {\r\n  color: black;\r\n}\r\n"),
			wantRefs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := bytes.Clone(tt.data)
			sheet, err := Parse(tt.data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !bytes.Equal(tt.data, original) {
				t.Fatal("Parse modified the source bytes")
			}
			if len(sheet.References) != tt.wantRefs {
				t.Fatalf("references=%d, want %d: %+v", len(sheet.References), tt.wantRefs, sheet.References)
			}
			assertSpans(t, tt.data, sheet)
		})
	}
}

func assertSpans(t *testing.T, data []byte, sheet *Stylesheet) {
	t.Helper()
	check := func(label string, span Span) {
		if !span.Valid() || span.End > len(data) {
			t.Fatalf("%s out of bounds: %+v for %d bytes", label, span, len(data))
		}
	}
	for i, token := range sheet.Tokens {
		check("token", token.Span)
		if !bytes.Equal(token.Data, data[token.Span.Start:token.Span.End]) {
			t.Fatalf("token %d data does not alias its source span", i)
		}
	}
	for _, rule := range sheet.Rules {
		check("rule", rule.Span)
		if rule.HasBlock {
			check("rule body", rule.BodySpan)
		}
		if !rule.AtRule {
			check("selector", rule.SelectorSpan)
		}
		for _, decl := range rule.Declarations {
			check("declaration", decl.Span)
			check("declaration name", decl.NameSpan)
			check("declaration value", decl.ValueSpan)
			if decl.HasSemicolon {
				check("semicolon", decl.SemicolonSpan)
			}
		}
	}
	for _, decl := range sheet.Declarations {
		check("declaration", decl.Span)
		check("declaration name", decl.NameSpan)
		check("declaration value", decl.ValueSpan)
	}
	for _, ref := range sheet.References {
		check("reference", ref.Span)
		check("reference value", ref.ValueSpan)
		if got := string(data[ref.ValueSpan.Start:ref.ValueSpan.End]); got != ref.Value {
			t.Fatalf("reference value=%q, source=%q", ref.Value, got)
		}
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	tests := [][]byte{
		[]byte{'a', '{', 'x', ':', 0xff, '}'},
		[]byte(`a { content: "unterminated }`),
		[]byte(`a { /* unterminated`),
		[]byte(`a { color: red`),
		[]byte(`a { background: url(foo`),
		[]byte("a { content: \"line\nbreak\"; }"),
	}
	for _, data := range tests {
		data := data
		t.Run(string(data), func(t *testing.T) {
			if sheet, err := Parse(data); err == nil || sheet != nil {
				t.Fatalf("Parse accepted malformed source: sheet=%+v err=%v", sheet, err)
			}
		})
	}
	var parseErr *ParseError
	if _, err := Parse([]byte{'a', '{', 0xff}); !errors.As(err, &parseErr) || !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("invalid UTF-8 error=%v, want ParseError wrapping ErrInvalidUTF8", err)
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`a{color:red}`),
		[]byte(`@media all{.x{content:"{};"}}`),
		[]byte{0xff, '{', '}'},
		[]byte(`a{background:url(data:text/css,a;b{})}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(data)
	})
}
