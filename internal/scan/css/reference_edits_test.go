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

func TestScanReferencesImageSetStrings(t *testing.T) {
	input := []byte(`a { background: image-set("a.png" 1x, /* candidate */ 'b.png' 2x); content: "image-set(\"ghost.png\")"; }`)
	references, err := ScanReferences(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 2 {
		t.Fatalf("references = %+v, want only the two image-set candidates", references)
	}
	wantValues := []string{"a.png", "b.png"}
	for i, ref := range references {
		if ref.Value != wantValues[i] || string(input[ref.ValueSpan.Start:ref.ValueSpan.End]) != wantValues[i] {
			t.Fatalf("reference %d = %+v, want %q with matching source span", i, ref, wantValues[i])
		}
	}
	edits, err := ReferenceEdits("styles/main.css", input, func(value string) string {
		return "images/" + value
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := editset.Apply("styles/main.css", input, edits)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`a { background: image-set("images/a.png" 1x, /* candidate */ 'images/b.png' 2x); content: "image-set(\"ghost.png\")"; }`)
	if !bytes.Equal(got, want) {
		t.Fatalf("image-set edits changed unrelated CSS\n got: %s\nwant: %s", got, want)
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
