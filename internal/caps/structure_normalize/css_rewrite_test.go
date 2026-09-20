package structurenormalize

import (
	"strings"
	"testing"
)

func TestCSSRewriteIgnoresExamplesAndPreservesSourceBytes(t *testing.T) {
	warnings := []string{}
	rw := &refRewriter{pathMap: map[string]string{"old/a.png": "new/img/a.png", "old/theme.css": "new/base.css"}, files: map[string]bool{"old/a.png": true, "old/theme.css": true}, warnings: &warnings}
	text := "/* url(missing.ttf); @import 'ghost.css'; */\r\n@import 'theme.css';\r\np { content: \"url(missing.png)\"; background: url( a.png ); }"
	want := strings.ReplaceAll(strings.ReplaceAll(text, "'theme.css'", "'base.css'"), "url( a.png )", "url( img/a.png )")
	got := rewriteCSSReferences(text, "old/page.css", "new/page.css", rw)
	if got != want || len(warnings) != 0 || rw.err != nil {
		t.Fatalf("got=%q want=%q warnings=%v err=%v", got, want, warnings, rw.err)
	}
}

func TestCSSRewriteRefusesAmbiguousLocalEscapes(t *testing.T) {
	for _, text := range []string{`p{background:url(a\)b.png)}`, `p{background:url("unterminated)`} {
		warnings := []string{}
		rw := &refRewriter{warnings: &warnings}
		if got := rewriteCSSReferences(text, "old/a.css", "new/a.css", rw); rw.err == nil || got != text {
			t.Fatalf("uncertain rewrite accepted: %q err=%v", got, rw.err)
		}
	}
}
