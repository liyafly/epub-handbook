package merge

import (
	"fmt"
	"strings"
	"testing"
)

func TestRewriteRootedHrefWarnsAndStaysUnchanged(t *testing.T) {
	const document = "OEBPS/Text/chapter.xhtml"
	const href = "/Images/old.png"
	const input = `<a href="/Images/old.png">cover</a>`
	pathMap := map[string]string{"OEBPS/Images/old.png": "OEBPS/Images/new.png"}
	knownFiles := map[string]bool{"OEBPS/Images/old.png": true}
	var warnings []string
	warn := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }

	got, err := rewriteMarkupReferences(input, document, document, pathMap, knownFiles, warn)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Fatalf("rooted href rewrite = %q, want unchanged %q", got, input)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], href) {
		t.Fatalf("warnings = %q, want one warning naming the rooted href", warnings)
	}
}
