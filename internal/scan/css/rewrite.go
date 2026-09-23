package css

import "github.com/liyafly/epub-handbook/internal/editset"

// RewriteCSS returns lossless edits for CSS resource references. It leaves
// parsing policy and URI resolution with the caller and never returns a whole
// rewritten document (INV-2).
func RewriteCSS(path string, source []byte, rewrite func(string) string) ([]editset.Edit, error) {
	return ReferenceEdits(path, source, rewrite)
}
