package css

import (
	"bytes"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestReferenceEditsOnlyChangesReferenceValues(t *testing.T) {
	const path = "styles/main.css"
	input := []byte(`[data-icon="url(attribute.svg)"]::before {
  content: "url(content.svg)"; /* url(comment.svg) */
  background:  url( image.png ) ; mask: url('mask.svg');
  src: url(keep.woff2);
}
@import  "theme.css" screen;
`)
	want := []byte(`[data-icon="url(attribute.svg)"]::before {
  content: "url(content.svg)"; /* url(comment.svg) */
  background:  url( assets/image.png ) ; mask: url('assets/mask.svg');
  src: url(keep.woff2);
}
@import  "assets/theme.css" screen;
`)

	edits, err := ReferenceEdits(path, input, func(value string) string {
		switch value {
		case "image.png", "mask.svg", "theme.css":
			return "assets/" + value
		default:
			return value
		}
	})
	if err != nil {
		t.Fatalf("ReferenceEdits: %v", err)
	}
	got, err := editset.Apply(path, input, edits)
	if err != nil {
		t.Fatalf("editset.Apply: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("result mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestReferenceEditsAcceptsInlineDeclarationList(t *testing.T) {
	const path = "inline"
	input := []byte("background: url(bg.png); src: url('font.woff2')")
	edits, err := ReferenceEdits(path, input, func(value string) string {
		return "new/" + value
	})
	if err != nil {
		t.Fatalf("ReferenceEdits: %v", err)
	}
	got, err := editset.Apply(path, input, edits)
	if err != nil {
		t.Fatalf("editset.Apply: %v", err)
	}
	want := []byte("background: url(new/bg.png); src: url('new/font.woff2')")
	if !bytes.Equal(got, want) {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

func TestReferenceEditsSkipsDataAndExternalURIs(t *testing.T) {
	const path = "styles/refs.css"
	input := []byte(`a { a: url(data:image/png;base64,AA==); b: url(https://example.test/a.png); c: url(/root/a.png); d: url(//cdn.example.test/a.png); e: url(local.png); }`)
	var called []string
	edits, err := ReferenceEdits(path, input, func(value string) string {
		called = append(called, value)
		return "new/" + value
	})
	if err != nil {
		t.Fatalf("ReferenceEdits: %v", err)
	}
	if len(called) != 1 || called[0] != "local.png" {
		t.Fatalf("rewrite called with %q, want only local.png", called)
	}
	got, err := editset.Apply(path, input, edits)
	if err != nil {
		t.Fatalf("editset.Apply: %v", err)
	}
	want := []byte(`a { a: url(data:image/png;base64,AA==); b: url(https://example.test/a.png); c: url(/root/a.png); d: url(//cdn.example.test/a.png); e: url(new/local.png); }`)
	if !bytes.Equal(got, want) {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

func TestReferenceEditsRejectsLocalBackslashWithoutEdits(t *testing.T) {
	const path = "styles/escaped.css"
	input := []byte(`a { background: url(icon\20.svg); }`)
	called := false
	edits, err := ReferenceEdits(path, input, func(value string) string {
		called = true
		return "new/" + value
	})
	if err == nil || !strings.Contains(err.Error(), "backslash escape") {
		t.Fatalf("error = %v, want explicit backslash escape error", err)
	}
	if edits != nil {
		t.Fatalf("edits = %#v, want nil on error", edits)
	}
	if called {
		t.Fatal("rewrite callback called for an uninterpreted CSS escape")
	}
}

func TestReferenceEditsScannerFailureReturnsNoPartialEdits(t *testing.T) {
	const path = "styles/malformed.css"
	input := []byte(`a { background: url(first.png); content: "unterminated }`)
	called := 0
	edits, err := ReferenceEdits(path, input, func(value string) string {
		called++
		return "new/" + value
	})
	if err == nil {
		t.Fatal("ReferenceEdits succeeded on malformed CSS")
	}
	if edits != nil {
		t.Fatalf("edits = %#v, want nil on scanner failure", edits)
	}
	if called != 0 {
		t.Fatalf("rewrite callback called %d times before scan failed", called)
	}
}

func TestReferenceEditsRejectsUnsafeRewritesWithoutPartialEdits(t *testing.T) {
	unsafeValues := []string{
		`new"path`, `new'path`, "new path", "new(path", "new)path",
		`new\path`, "new\npath", "new\x00path", string([]byte{'n', 'e', 'w', 0xff}),
	}
	for _, unsafe := range unsafeValues {
		t.Run(unsafe, func(t *testing.T) {
			const path = "styles/unsafe.css"
			input := []byte(`a { background: url(first.png); mask: url(second.svg); }`)
			calls := 0
			edits, err := ReferenceEdits(path, input, func(value string) string {
				calls++
				if value == "first.png" {
					return "safe/path.png"
				}
				return unsafe
			})
			if err == nil {
				t.Fatalf("ReferenceEdits accepted unsafe value %q", unsafe)
			}
			if edits != nil {
				t.Fatalf("edits = %#v, want nil on unsafe rewrite", edits)
			}
			if calls != 2 {
				t.Fatalf("rewrite called %d times, want 2", calls)
			}
		})
	}
}

func TestReferenceEditsRejectsNilRewriteCallback(t *testing.T) {
	edits, err := ReferenceEdits("styles/main.css", []byte("a{background:url(a.png)}"), nil)
	if err == nil {
		t.Fatal("ReferenceEdits succeeded with a nil callback")
	}
	if edits != nil {
		t.Fatalf("edits = %#v, want nil", edits)
	}
}
