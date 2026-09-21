package cover

import (
	"bytes"
	"testing"
)

func TestTransformResourceCSSReferencesAreLossless(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	tests := []struct {
		name  string
		path  string
		input string
		want  string
	}{
		{
			name: "standalone stylesheet",
			path: "OEBPS/Styles/main.css",
			input: `@import "../Images/old.png"; p::after{content:"url(../Images/old.png)";}` +
				`/* url(../Images/old.png) */` +
				`[data-copy="url(../Images/old.png)"]{background:url(../Images/old.png)}` +
				`a{background:url(data:image/png;base64,AAAA)}` +
				`b{background:url(https://example.test/Images/old.png)}` +
				`@import "https://example.test/theme.css";`,
			want: `@import "../Images/new.png"; p::after{content:"url(../Images/old.png)";}` +
				`/* url(../Images/old.png) */` +
				`[data-copy="url(../Images/old.png)"]{background:url(../Images/new.png)}` +
				`a{background:url(data:image/png;base64,AAAA)}` +
				`b{background:url(https://example.test/Images/old.png)}` +
				`@import "https://example.test/theme.css";`,
		},
		{
			name: "style element",
			path: "OEBPS/Text/chapter.xhtml",
			input: `<html><head><style>@import "../Images/old.png";` +
				`p::after{content:"url(../Images/old.png)";}` +
				`/* url(../Images/old.png) */` +
				`[data-copy="url(../Images/old.png)"]{background:url(../Images/old.png)}` +
				`</style></head><body></body></html>`,
			want: `<html><head><style>@import "../Images/new.png";` +
				`p::after{content:"url(../Images/old.png)";}` +
				`/* url(../Images/old.png) */` +
				`[data-copy="url(../Images/old.png)"]{background:url(../Images/new.png)}` +
				`</style></head><body></body></html>`,
		},
		{
			name:  "style attribute",
			path:  "OEBPS/Text/chapter.xhtml",
			input: `<p style='content:"url(../Images/old.png)"; /* url(../Images/old.png) */ background:url(../Images/old.png)'></p>`,
			want:  `<p style='content:"url(../Images/old.png)"; /* url(../Images/old.png) */ background:url(../Images/new.png)'></p>`,
		},
		{
			name:  "unquoted url keeps its closing parenthesis",
			path:  "OEBPS/Styles/main.css",
			input: `a{background:url(../Images/old.png)}`,
			want:  `a{background:url(../Images/new.png)}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformResource([]byte(tt.input), tt.path, tt.path, pathMap, knownFiles, nil)
			if err != nil {
				t.Fatalf("transformResource: %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("transformResource bytes differ:\n got  = %q\n want = %q", got, tt.want)
			}
		})
	}
}

func TestTransformResourceRejectsUnsafeOrUnclosedCSSWithoutOutput(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	tests := []struct {
		name  string
		path  string
		input string
	}{
		{
			name:  "escaped local URL",
			path:  "OEBPS/Styles/main.css",
			input: `a{background:url(../Images/old\.png);color:red}`,
		},
		{
			name:  "unterminated stylesheet string",
			path:  "OEBPS/Styles/main.css",
			input: `a{background:url(../Images/old.png);content:"unterminated}`,
		},
		{
			name:  "unterminated style element CSS",
			path:  "OEBPS/Text/chapter.xhtml",
			input: `<html><head><style>a{background:url(../Images/old.png);content:"unterminated}</style></head></html>`,
		},
		{
			name:  "unterminated inline CSS comment",
			path:  "OEBPS/Text/chapter.xhtml",
			input: `<p style='background:url(../Images/old.png); /* unterminated'></p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformResource([]byte(tt.input), tt.path, tt.path, pathMap, knownFiles, nil)
			if err == nil {
				t.Fatalf("transformResource accepted unsafe CSS and returned %q", got)
			}
			if len(got) != 0 {
				t.Errorf("transformResource returned partial output on error: %q", got)
			}
		})
	}
}

func TestTransformResourceRejectsEntityEncodedCSSStrings(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	for _, input := range []string{
		`<p style="content:&quot;url(../Images/old.png)&quot;;background:url(../Images/old.png)"/>`,
		`<html><head><style>p{content:&quot;url(../Images/old.png)&quot;;}</style></head></html>`,
	} {
		data, err := transformResource([]byte(input), "OEBPS/Text/chapter.xhtml", "OEBPS/Text/chapter.xhtml", pathMap, knownFiles, nil)
		if err == nil || data != nil {
			t.Fatalf("encoded CSS must fail without partial rewrite: %q, %v", data, err)
		}
	}
}
